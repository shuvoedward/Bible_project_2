package s3_service

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	Client     *s3.Client
	BucketName string
	Region     string
}

func NewS3Config(ctx context.Context) (*S3Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-2"))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)

	return &S3Config{
		Client:     client,
		BucketName: "bible-notes-app-images",
		Region:     "us-east-1",
	}, nil
}
