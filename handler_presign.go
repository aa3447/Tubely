package main

import (
	"time"
	"context"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error){
	objectParams := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	
	presignedClient := s3.NewPresignClient(s3Client)
	presignedObject, err := presignedClient.PresignGetObject(context.Background(), objectParams, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	
	return presignedObject.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil{
		return video, nil
	}

	bucket, key, found := strings.Cut(*video.VideoURL, ",")
	if !found {
		return database.Video{}, nil
	}

	signedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 10*time.Minute)
	if err != nil {
		return database.Video{}, err
	}
	
	video.VideoURL = &signedURL
	return video, nil
}