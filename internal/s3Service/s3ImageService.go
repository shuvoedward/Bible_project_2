package s3Service

import (
	"bytes"
	"context"
	"fmt"

	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/google/uuid"
)

// S3ImageService handles S3 operations for images
type S3ImageService struct {
	client     *s3.Client // AWS S3 client for operations
	uploader   *manager.Uploader
	bucketName string
	region     string
}

// NewS3ImageService creates a new S3ImageService
// awsConfig: The AWS SDK configuration (from LoadAWSConfig)
// bucketName: Which S3 bucket to store images in
// region: AWS region (stored for presigned URL generation)
func NewS3ImageService(awsConfig aws.Config, bucketName, region string) *S3ImageService {
	client := s3.NewFromConfig(awsConfig)

	// Create uploader (handles multipart uploads automatically for large files)
	uploader := manager.NewUploader(client)

	return &S3ImageService{
		client:     client,
		uploader:   uploader,
		bucketName: bucketName,
		region:     region,
	}
}

// UploadImage uploads an image to S3
func (s *S3ImageService) UploadImage(
	ctx context.Context,
	imageData []byte,
	fileName string,
	contentType string,
	userID int64,
) (string, error) {
	// 1. Generate unique s3 key (path in bucket)
	// Format:  users/123/notes/20251026-abc123-def456-ghi789.jpg)
	timeStamp := time.Now().Format("20060102")
	uniqueID := uuid.New().String()

	s3Key := fmt.Sprintf("users/%d/notes/%s-%s%s", userID, timeStamp, uniqueID, fileName)

	// 2. Upload to s3
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(imageData),
		ContentType: aws.String(contentType),
	})
	// fileHeader.Header.Get("Content-Type")

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return s3Key, nil
}

// DeleteImage deletes an image from S3
func (s *S3ImageService) DeleteImage(ctx context.Context, s3Key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(s3Key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GeneratePresignedURL generates a presigned URL for accessing an image
// Presigned URLs allow temporary public access to private S3 objects
func (s *S3ImageService) GeneratePresignedURL(
	ctx context.Context,
	s3Key string,
	expiration time.Duration,
) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	// Generate presigned GET request
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName), // ← Uses stored bucketName
		Key:    aws.String(s3Key),
	}, s3.WithPresignExpires(expiration))

	if err != nil {
		return "", err
	}

	return request.URL, nil

}

// func getFileExtension(fileName string) string {
// 	for i := len(fileName) - 1; i >= 0; i-- {
// 		if fileName[i] == '.' {
// 			return fileName[i:]
// 		}
// 	}
// 	return ""
// }
