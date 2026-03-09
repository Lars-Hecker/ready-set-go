package file

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"baseapp/infra/config"
)

type S3Service struct {
	client      *s3.Client
	presigner   *s3.PresignClient
	bucket      string
	urlLifetime time.Duration
}

func NewS3Service(ctx context.Context, cfg config.S3Config) (*S3Service, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg)
	return &S3Service{
		client:      client,
		presigner:   s3.NewPresignClient(client),
		bucket:      cfg.Bucket,
		urlLifetime: cfg.URLLifetime,
	}, nil
}

func (s *S3Service) GenerateUploadURL(ctx context.Context, req UploadRequest) (*UploadResponse, error) {
	key := fmt.Sprintf("uploads/%s/%s", uuid.New().String(), req.Filename)
	presigned, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(req.ContentType),
		ContentLength: aws.Int64(req.Size),
	}, s3.WithPresignExpires(s.urlLifetime))
	if err != nil {
		return nil, fmt.Errorf("presign put: %w", err)
	}
	return &UploadResponse{URL: presigned.URL, Key: key}, nil
}

func (s *S3Service) GenerateDownloadURL(ctx context.Context, key string) (*DownloadResponse, error) {
	presigned, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(s.urlLifetime))
	if err != nil {
		return nil, fmt.Errorf("presign get: %w", err)
	}
	return &DownloadResponse{URL: presigned.URL}, nil
}

func (s *S3Service) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Service) ObjectExists(ctx context.Context, key string) bool {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

func (s *S3Service) URLLifetime() time.Duration {
	return s.urlLifetime
}
