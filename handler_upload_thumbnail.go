package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)
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
	imgFile, imgHeaders, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting file", err)
		return
	}
	imgType := imgHeaders.Header.Get("Content-Type")
	if imgType == "" {
		respondWithError(w, http.StatusInternalServerError, "Error reading file type", err)
		return
	}

	imgBS, err := io.ReadAll(imgFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading file", err)
		return
	}

	// videoThumbnails[videoID] = thumbnail{
	// 	data:      imgBS,
	// 	mediaType: imgType,
	// }
	newThumbnailDataURL := fmt.Sprintf("data:%v;base64,%v", imgType, base64.StdEncoding.EncodeToString(imgBS))
	dbVideo.ThumbnailURL = &newThumbnailDataURL

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating DB", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
