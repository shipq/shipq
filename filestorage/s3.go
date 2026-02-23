// Package filestorage provides an S3-compatible storage client for file
// upload and download operations using presigned URLs.
package filestorage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config holds configuration for the S3 client.
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string // Custom endpoint for MinIO, GCS, R2, etc. Empty for AWS.
	AccessKey string
	SecretKey string
}

// CompletedPart describes a completed part of a multipart upload.
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// S3Client wraps the AWS S3 client with presigned URL generation.
type S3Client struct {
	client   *s3.Client
	presign  *s3.PresignClient
	bucket   string
	urlTTL   time.Duration
}

// NewS3Client creates a new S3 client from the given configuration.
func NewS3Client(cfg S3Config) (*S3Client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}

	opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	}

	client := s3.NewFromConfig(awsCfg, opts)
	presigner := s3.NewPresignClient(client)

	return &S3Client{
		client:  client,
		presign: presigner,
		bucket:  cfg.Bucket,
		urlTTL:  15 * time.Minute,
	}, nil
}

// GenerateUploadURL generates a presigned PUT URL for uploading a single file.
func (s *S3Client) GenerateUploadURL(key, contentType string, size int64) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	result, err := s.presign.PresignPutObject(context.Background(), input, func(opts *s3.PresignOptions) {
		opts.Expires = s.urlTTL
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign PUT: %w", err)
	}

	return result.URL, nil
}

// InitiateMultipartUpload starts a multipart upload and returns the upload ID.
func (s *S3Client) InitiateMultipartUpload(key, contentType string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}

	result, err := s.client.CreateMultipartUpload(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("failed to create multipart upload: %w", err)
	}

	return *result.UploadId, nil
}

// GeneratePartUploadURL generates a presigned URL for uploading a single part
// of a multipart upload.
func (s *S3Client) GeneratePartUploadURL(key, uploadID string, partNumber int) (string, error) {
	input := &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
	}

	result, err := s.presign.PresignUploadPart(context.Background(), input, func(opts *s3.PresignOptions) {
		opts.Expires = s.urlTTL
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign upload part: %w", err)
	}

	return result.URL, nil
}

// CompleteMultipartUpload finishes a multipart upload by assembling the parts.
func (s *S3Client) CompleteMultipartUpload(key, uploadID string, parts []CompletedPart) error {
	s3Parts := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		s3Parts[i] = types.CompletedPart{
			PartNumber: aws.Int32(int32(p.PartNumber)),
			ETag:       aws.String(p.ETag),
		}
	}

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: s3Parts,
		},
	}

	_, err := s.client.CompleteMultipartUpload(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

// AbortMultipartUpload cancels a multipart upload, freeing uploaded parts.
func (s *S3Client) AbortMultipartUpload(key, uploadID string) error {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}

	_, err := s.client.AbortMultipartUpload(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	return nil
}

// GenerateDownloadURL generates a presigned GET URL for downloading a file.
// The Content-Disposition header is set to trigger a browser download with
// the original filename.
func (s *S3Client) GenerateDownloadURL(key, filename string) (string, error) {
	disposition := fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename))

	input := &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(disposition),
	}

	result, err := s.presign.PresignGetObject(context.Background(), input, func(opts *s3.PresignOptions) {
		opts.Expires = s.urlTTL
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign GET: %w", err)
	}

	return result.URL, nil
}

// DeleteObject deletes an object from S3.
func (s *S3Client) DeleteObject(key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}
