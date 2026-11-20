package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var ErrR2NotConfigured = errors.New("S3/R2 object storage is not configured")

type ObjectStorage interface {
	GenerateUploadURL(ctx context.Context, objectKey, contentType string) (uploadURL, publicFileURL string, err error)
}

type R2Storage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	publicURL     string
	configured    bool
}

func NewR2Storage(endpoint, region, accessKey, secretKey, bucket, publicURL string) (*R2Storage, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	region = strings.TrimSpace(region)
	if region == "" {
		region = "auto"
	}

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return &R2Storage{
			bucketName: bucket,
			publicURL:  strings.TrimRight(publicURL, "/"),
			configured: false,
		}, nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config for R2: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &R2Storage{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucketName:    bucket,
		publicURL:     strings.TrimRight(publicURL, "/"),
		configured:    true,
	}, nil
}

// GenerateUploadURL tạo Presigned PUT URL (hết hạn 5 phút) cho Admin CMS upload trực tiếp
func (r *R2Storage) GenerateUploadURL(ctx context.Context, objectKey, contentType string) (uploadURL, publicFileURL string, err error) {
	if !r.configured || r.presignClient == nil {
		return "", "", ErrR2NotConfigured
	}

	cleanKey := strings.TrimLeft(objectKey, "/")
	req, err := r.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(cleanKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return "", "", fmt.Errorf("presign put object: %w", err)
	}

	return req.URL, fmt.Sprintf("%s/%s", r.publicURL, cleanKey), nil
}
