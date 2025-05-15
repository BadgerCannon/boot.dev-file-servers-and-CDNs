package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	log.Println("uploading thumbnail for video", videoID, "by user", userID)
	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error looking up video", err)
		return
	}

	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "A video can only be modified by the owner", err)
		return
	}

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	imgFormFile, imgHeaders, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting file", err)
		return
	}
	imgType := imgHeaders.Header.Get("Content-Type")
	if imgType == "" {
		respondWithError(w, http.StatusInternalServerError, "Error reading file type", err)
		return
	} else if !slices.Contains([]string{"image/jpeg", "image/png"}, imgType) {
		respondWithError(w, http.StatusBadRequest, "Invalid File Type", err)
		return
	}

	randStr := make([]byte, 32)
	_, err = rand.Read(randStr)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating file name", err)
		return
	}
	imgFileName := fmt.Sprintf("%s.%s", base64.RawURLEncoding.EncodeToString(randStr), strings.TrimPrefix(imgType, "image/"))
	imgFile, err := os.Create(filepath.Join(cfg.assetsRoot, imgFileName))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}
	_, err = io.Copy(imgFile, imgFormFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving file", err)
		return
	}

	newThumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, imgFileName)
	dbVideo.ThumbnailURL = &newThumbnailURL

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating DB", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
