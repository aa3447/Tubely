package main

import (
	"fmt"
	"io"
	"net/http"
	"encoding/base64"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 20 // 10 MB
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

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse form", err)
		return
	}

	imageFile, imageHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail from form", err)
		return
	}
	contentType := imageHeader.Header.Get("Content-Type")
	defer imageFile.Close()

	imageData, err := io.ReadAll(imageFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read image file", err)
		return
	}

	databaseVideo , err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	if databaseVideo == (database.Video{}) {
		respondWithError(w, http.StatusNotFound, "Video not found", nil)
		return
	}
	if databaseVideo.UserID != userID {
		respondWithError(w, http.StatusForbidden, "You don't own this video", nil)
		return
	}

	encodedImageString := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, encodedImageString)

	databaseVideo.ThumbnailURL = &dataURL

	err = cfg.db.UpdateVideo(databaseVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video with thumbnail URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, databaseVideo)
}
