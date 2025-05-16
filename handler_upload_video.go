package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxSize = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxSize))

	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid videoID", err)
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to extract Bearer Token", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Failed to validate JWT", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve video metadata", err)
		return
	} else if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Must be Video owner", err)
		return
	}

	videoFile, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to parse form", err)
		return
	}
	defer videoFile.Close()

	mimeType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing mime type", err)
		return
	} else if mimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Incorrect file type", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to write video to file", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update file pointer", err)
		return
	}

	prefix, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to calculate aspect ratio", err)
		return
	}

	randStr := make([]byte, 32)
	_, err = rand.Read(randStr)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating file name", err)
		return
	}
	videoObjectKey := prefix + "/" + base64.RawURLEncoding.EncodeToString(randStr) + ".mp4"

	log.Printf("Uploading %s to S3\n", videoObjectKey)
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoObjectKey,
		Body:        tempFile,
		ContentType: &mimeType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload to S3", err)
		return
	}
	log.Println("Upload complete")

	s3VideoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, videoObjectKey)
	dbVideo.VideoURL = &s3VideoURL
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video in database", err)
		return
	}
}

func getVideoAspectRatio(filePath string) (string, error) {
	ffprobeCmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	cmdOutput := bytes.NewBuffer([]byte{})
	ffprobeCmd.Stdout = cmdOutput
	err := ffprobeCmd.Run()
	if err != nil {
		log.Println("Error when running ffprobe", err)
		return "", err
	}

	jd := json.NewDecoder(cmdOutput)
	var ffprobeResult struct {
		Streams []Ratio `json:"streams"`
	}
	err = jd.Decode(&ffprobeResult)
	if err != nil {
		log.Println("Failed to decode json from ffprobe output")
	}

	aspectRatio := Ratio{
		Width:  ffprobeResult.Streams[0].Width,
		Height: ffprobeResult.Streams[0].Height,
	}
	aspectRatio.Reduce()

	log.Printf("Video aspect ratio: %v", aspectRatio)
	log.Printf("Video orientation:  %s", aspectRatio.Orientation())
	return aspectRatio.Orientation(), nil
}
