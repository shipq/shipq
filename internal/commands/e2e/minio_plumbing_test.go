//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/shipq/shipq/filestorage"
)

// ---------------------------------------------------------------------------
// Constants -- must match minio-setup.sh / shipq start minio defaults
// ---------------------------------------------------------------------------

const (
	testMinioEndpoint  = "http://localhost:9000"
	testMinioAccessKey = "minioadmin"
	testMinioSecretKey = "minioadmin"
	testMinioBucket    = "shipq-dev"
	testMinioRegion    = "us-east-1"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pingMinio checks if MinIO is reachable via its health endpoint.
func pingMinio(endpoint string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint + "/minio/health/live")
	if err != nil {
		return fmt.Errorf("cannot reach MinIO at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MinIO health check returned status %d", resp.StatusCode)
	}
	return nil
}

// skipIfMinioDown skips the test if MinIO is not reachable.
func skipIfMinioDown(t *testing.T) {
	t.Helper()
	if err := pingMinio(testMinioEndpoint); err != nil {
		t.Skipf("MinIO not available, skipping: %v", err)
	}
}

// rawS3Client creates a raw AWS S3 client pointed at local MinIO.
func rawS3Client() *s3.Client {
	cfg := aws.Config{
		Region:      testMinioRegion,
		Credentials: credentials.NewStaticCredentialsProvider(testMinioAccessKey, testMinioSecretKey, ""),
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(testMinioEndpoint)
		o.UsePathStyle = true
	})
}

// minioS3Client creates a filestorage.S3Client pointed at local MinIO.
func minioS3Client(t *testing.T) *filestorage.S3Client {
	t.Helper()
	client, err := filestorage.NewS3Client(filestorage.S3Config{
		Bucket:    testMinioBucket,
		Region:    testMinioRegion,
		Endpoint:  testMinioEndpoint,
		AccessKey: testMinioAccessKey,
		SecretKey: testMinioSecretKey,
	})
	if err != nil {
		t.Fatalf("failed to create S3 client: %v", err)
	}
	return client
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPlumbing_Minio_BucketExists(t *testing.T) {
	skipIfMinioDown(t)

	client := rawS3Client()
	_, err := client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(testMinioBucket),
	})
	if err != nil {
		t.Fatalf("bucket %q does not exist: %v", testMinioBucket, err)
	}
}

func TestPlumbing_Minio_CORSConfigured(t *testing.T) {
	skipIfMinioDown(t)

	// MinIO does not implement the S3 GetBucketCors API — CORS is configured
	// at the server level via environment variables or `mc admin config`.
	// We verify CORS by sending an HTTP OPTIONS preflight request and
	// checking the response headers.

	httpClient := &http.Client{Timeout: 5 * time.Second}

	for _, origin := range []string{"http://localhost:5173", "http://localhost:4321"} {
		t.Run(origin, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodOptions, testMinioEndpoint+"/"+testMinioBucket, nil)
			if err != nil {
				t.Fatalf("failed to create OPTIONS request: %v", err)
			}
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", "PUT")

			resp, err := httpClient.Do(req)
			if err != nil {
				t.Fatalf("OPTIONS request failed: %v", err)
			}
			defer resp.Body.Close()

			acao := resp.Header.Get("Access-Control-Allow-Origin")
			if acao != origin && acao != "*" {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q (or *)", acao, origin)
			}

			acam := resp.Header.Get("Access-Control-Allow-Methods")
			if acam == "" {
				t.Error("Access-Control-Allow-Methods header is empty")
			} else {
				hasPUT := false
				for _, m := range splitCSV(acam) {
					if m == "PUT" {
						hasPUT = true
						break
					}
				}
				if !hasPUT {
					t.Errorf("PUT not in Access-Control-Allow-Methods: %q", acam)
				}
			}
		})
	}
}

// splitCSV splits a comma-separated header value into trimmed tokens.
func splitCSV(s string) []string {
	var out []string
	for _, part := range bytes.Split([]byte(s), []byte(",")) {
		trimmed := bytes.TrimSpace(part)
		if len(trimmed) > 0 {
			out = append(out, string(trimmed))
		}
	}
	return out
}

func TestPlumbing_Minio_PresignedUploadDownload(t *testing.T) {
	skipIfMinioDown(t)

	s3c := minioS3Client(t)
	key := fmt.Sprintf("test/presigned-%d.txt", time.Now().UnixNano())
	payload := []byte("hello from presigned upload test")

	// 1. Generate presigned PUT URL
	uploadURL, err := s3c.GenerateUploadURL(key, "text/plain", int64(len(payload)))
	if err != nil {
		t.Fatalf("GenerateUploadURL failed: %v", err)
	}

	// 2. Upload via presigned URL
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.ContentLength = int64(len(payload))

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("presigned PUT failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned PUT returned status %d", resp.StatusCode)
	}

	// 3. Generate presigned GET URL
	downloadURL, err := s3c.GenerateDownloadURL(key, "presigned-test.txt")
	if err != nil {
		t.Fatalf("GenerateDownloadURL failed: %v", err)
	}

	// 4. Download and verify
	getResp, err := httpClient.Get(downloadURL)
	if err != nil {
		t.Fatalf("presigned GET failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("presigned GET returned status %d", getResp.StatusCode)
	}

	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !bytes.Equal(body, payload) {
		t.Errorf("downloaded content mismatch: got %q, want %q", body, payload)
	}

	// 5. Clean up
	if err := s3c.DeleteObject(key); err != nil {
		t.Errorf("cleanup: failed to delete object: %v", err)
	}
}

func TestPlumbing_Minio_MultipartUpload(t *testing.T) {
	skipIfMinioDown(t)

	s3c := minioS3Client(t)
	key := fmt.Sprintf("test/multipart-%d.bin", time.Now().UnixNano())

	// MinIO minimum part size is 5 MiB (except last part). Use 5 MiB parts.
	partSize := 5 * 1024 * 1024
	part1Data := bytes.Repeat([]byte("A"), partSize)
	part2Data := bytes.Repeat([]byte("B"), partSize)

	// 1. Initiate multipart upload
	uploadID, err := s3c.InitiateMultipartUpload(key, "application/octet-stream")
	if err != nil {
		t.Fatalf("InitiateMultipartUpload failed: %v", err)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	var parts []filestorage.CompletedPart

	// 2. Upload part 1
	partURL1, err := s3c.GeneratePartUploadURL(key, uploadID, 1)
	if err != nil {
		t.Fatalf("GeneratePartUploadURL(1) failed: %v", err)
	}
	req1, err := http.NewRequest(http.MethodPut, partURL1, bytes.NewReader(part1Data))
	if err != nil {
		t.Fatalf("failed to create PUT request for part 1: %v", err)
	}
	req1.ContentLength = int64(len(part1Data))
	resp1, err := httpClient.Do(req1)
	if err != nil {
		t.Fatalf("part 1 upload failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("part 1 upload returned status %d", resp1.StatusCode)
	}
	etag1 := resp1.Header.Get("ETag")
	parts = append(parts, filestorage.CompletedPart{PartNumber: 1, ETag: etag1})

	// 3. Upload part 2
	partURL2, err := s3c.GeneratePartUploadURL(key, uploadID, 2)
	if err != nil {
		t.Fatalf("GeneratePartUploadURL(2) failed: %v", err)
	}
	req2, err := http.NewRequest(http.MethodPut, partURL2, bytes.NewReader(part2Data))
	if err != nil {
		t.Fatalf("failed to create PUT request for part 2: %v", err)
	}
	req2.ContentLength = int64(len(part2Data))
	resp2, err := httpClient.Do(req2)
	if err != nil {
		t.Fatalf("part 2 upload failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("part 2 upload returned status %d", resp2.StatusCode)
	}
	etag2 := resp2.Header.Get("ETag")
	parts = append(parts, filestorage.CompletedPart{PartNumber: 2, ETag: etag2})

	// 4. Complete multipart upload
	if err := s3c.CompleteMultipartUpload(key, uploadID, parts); err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// 5. Download and verify concatenated content
	downloadURL, err := s3c.GenerateDownloadURL(key, "multipart-test.bin")
	if err != nil {
		t.Fatalf("GenerateDownloadURL failed: %v", err)
	}

	getResp, err := httpClient.Get(downloadURL)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("download returned status %d", getResp.StatusCode)
	}

	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expectedLen := len(part1Data) + len(part2Data)
	if len(body) != expectedLen {
		t.Fatalf("downloaded content length mismatch: got %d, want %d", len(body), expectedLen)
	}

	// Verify first part is all 'A' and second part is all 'B'
	if !bytes.Equal(body[:partSize], part1Data) {
		t.Error("first part content mismatch")
	}
	if !bytes.Equal(body[partSize:], part2Data) {
		t.Error("second part content mismatch")
	}

	// 6. Clean up
	if err := s3c.DeleteObject(key); err != nil {
		t.Errorf("cleanup: failed to delete object: %v", err)
	}
}
