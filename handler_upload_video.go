package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"encoding/base64"
	"crypto/rand"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30 // 1GB

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

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
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
		respondWithError(w, http.StatusUnauthorized, "You don't own this video", nil)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse form", err)
		return
	}

	videoMultiFile, videoHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video from form", err)
		return
	}
	defer videoMultiFile.Close()

	contentType, _, err := mime.ParseMediaType(videoHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if !strings.HasSuffix(contentType, "mp4"){
		respondWithError(w, http.StatusBadRequest, "File is not a supported video", nil)
		return
	}

	tempVideo, err := os.CreateTemp("", "tubely-temp-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}
	
	
	_, err = io.Copy(tempVideo, videoMultiFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write to temp file", err)
		return
	}


	aspectRatio , err := cfg.getVideoAspectRatio(tempVideo.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
		return
	}

	_, err = tempVideo.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't seek to beginning of temp file", err)
		return
	}

	
	processedFilePath, err := cfg.processVideoForFastStart(tempVideo.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process video for fast start", err)
		return
	}
	os.Remove(tempVideo.Name())
	tempVideo.Close()
	
	processedVideo, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open processed video file", err)
		return
	}
	defer os.Remove(processedFilePath)
	defer processedVideo.Close()

	key := make([]byte, 32)
	rand.Read(key)
	mediaNameEncoded := make([]byte, base64.RawURLEncoding.EncodedLen(len(key)))
 	base64.RawURLEncoding.Encode(mediaNameEncoded, key)
	videoKey := aspectRatio + "/" + string(mediaNameEncoded) + ".mp4"
	
	params := &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoKey,
		Body:        processedVideo,
		ContentType: &contentType,
	}
	
	_, err = cfg.s3Client.PutObject(r.Context(), params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video to S3", err)
		return
	}

	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, videoKey)
	databaseVideo.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(databaseVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video in database", err)
		return
	}
	
	databaseSignedVideo, err := cfg.dbVideoToSignedVideo(databaseVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate signed URL", err)
		return
	}
	respondWithJSON(w, http.StatusOK, databaseSignedVideo)
}
