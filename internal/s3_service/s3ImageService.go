package s3_service

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"

	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/google/uuid"
)

type S3ImageService struct {
	S3Config *S3Config
	Uploader *manager.Uploader
}

func NewS3ImageService(s3Config *S3Config) *S3ImageService {
	uploader := manager.NewUploader(s3Config.Client)

	return &S3ImageService{
		S3Config: s3Config,
		Uploader: uploader,
	}
}

func (s *S3ImageService) UploadImage(ctx context.Context, compressedImage []byte, fileHeader *multipart.FileHeader, userID int64) (string, error) {
	// 1. Generate unique s3 key (file path: users/123/notes/20251026-abc123-def456-ghi789.jpg)
	timeStamp := time.Now().Format("20060102")
	uniqueID := uuid.New().String()
	extension := getFileExtension(fileHeader.Filename)

	s3Key := fmt.Sprintf("users/%d/notes/%s-%s%s", userID, timeStamp, uniqueID, extension)

	reader := bytes.NewReader(compressedImage)

	// 2. Upload to s3
	_, err := s.Uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.S3Config.BucketName),
		Key:         aws.String(s3Key),
		Body:        reader,
		ContentType: aws.String(fileHeader.Header.Get("Content-Type")),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return s3Key, nil
}

func (s *S3ImageService) DeleteImage(ctx context.Context, s3Key string) error {
	_, err := s.S3Config.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.S3Config.BucketName),
		Key:    aws.String(s3Key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

func (s *S3ImageService) GeneratePresignedURL(ctx context.Context, s3Key string, expiration time.Duration) (string, error) {
	presignedClient := s3.NewPresignClient(s.S3Config.Client)

	presignedReq, err := presignedClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.S3Config.BucketName),
		Key:    aws.String(s3Key),
	}, func(po *s3.PresignOptions) {
		po.Expires = expiration
	})

	if err != nil {
		return "", err
	}

	return presignedReq.URL, nil

}

func getFileExtension(fileName string) string {
	for i := len(fileName) - 1; i >= 0; i-- {
		if fileName[i] == '.' {
			return fileName[i:]
		}
	}
	return ""
}
