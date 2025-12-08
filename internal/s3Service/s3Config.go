package s3Service

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds S3 configuration values
// These come from flags/environment variables in main.go
type S3Config struct {
	Region          string
	BucketName      string
	AccessKeyID     string
	SecretAccessKey string
	Client          *s3.Client
}

// LoadAWSConfig creates an aws.Config from S3Config
// aws.Config is the AWS SDK's configuration object
// It contains region, credintials, retry logic, etc.
func LoadAWSConfig(ctx context.Context, cfg S3Config) (aws.Config, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	// Option 1: IAM role / instance profile (For EC2)
	// This automatically loads credentials from EC2 instance metadata
	if cfg.AccessKeyID == "" && cfg.SecretAccessKey == "" {
		awsConfig, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region))
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
		}
		return awsConfig, nil
	}

	// Option 2: Explicit credentials (for local development or non-EC2)
	awsConfig, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"", // Session token (empty for IAM user credentials)
		)),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config with credentials: %w", err)
	}

	return awsConfig, nil
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
