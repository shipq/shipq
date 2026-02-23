package db

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// waitForMinio polls the MinIO health endpoint until it returns HTTP 200
// or the timeout (in seconds) is reached.
func waitForMinio(endpoint string, timeoutSeconds int) error {
	healthURL := endpoint + "/minio/health/live"
	client := &http.Client{Timeout: 2 * time.Second}

	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	backoff := 250 * time.Millisecond

	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(backoff)
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}

	return fmt.Errorf("MinIO not ready at %s after %d seconds", endpoint, timeoutSeconds)
}

// newMinioS3Client creates an S3 client pointed at the local MinIO endpoint.
func newMinioS3Client(endpoint, accessKey, secretKey, region string) *s3.Client {
	cfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
}

// ensureMinioBucket creates the bucket if it doesn't already exist.
func ensureMinioBucket(endpoint, accessKey, secretKey, bucket, region string) error {
	client := newMinioS3Client(endpoint, accessKey, secretKey, region)
	ctx := context.Background()

	// Check if the bucket already exists
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		// Bucket already exists
		return nil
	}

	// Create the bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %q: %w", bucket, err)
	}

	return nil
}

// configureMinioCORS applies a CORS configuration to the bucket that allows
// browser-based uploads from common frontend dev servers.
func configureMinioCORS(endpoint, accessKey, secretKey, bucket string) error {
	client := newMinioS3Client(endpoint, accessKey, secretKey, "us-east-1")
	ctx := context.Background()

	_, err := client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{
				{
					AllowedOrigins: []string{
						"http://localhost:5173", // Vite
						"http://localhost:4321", // Astro
					},
					AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "HEAD"},
					AllowedHeaders: []string{"*"},
					ExposeHeaders:  []string{"ETag"}, // Required for multipart upload completion
					MaxAgeSeconds:  aws.Int32(3600),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set CORS on bucket %q: %w", bucket, err)
	}

	return nil
}
