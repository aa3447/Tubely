package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"mime"
	"net/http"
	"path/filepath"

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

	imageMultiFile, imageHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail from form", err)
		return
	}
	
	contentType, _, err := mime.ParseMediaType(imageHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if !strings.HasSuffix(contentType, "jpeg") && !strings.HasSuffix(contentType, "png") {
		respondWithError(w, http.StatusBadRequest, "File is not an supported image", nil)
		return
	}
	
	extension := strings.Split(contentType, "/")[1]
	defer imageMultiFile.Close()

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


	fileName := fmt.Sprintf("%s.%s", videoID.String(), extension)
	imageFilePath := filepath.Join(cfg.assetsRoot, fileName)

	file, err := os.Create(imageFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create image file", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, imageMultiFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save image file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoID.String(), extension)
	databaseVideo.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(databaseVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video with thumbnail URL", err)
		return
	}

	respondWithJSON(w, http.StatusOK, databaseVideo)
}
