package start

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

const (
	minioEndpoint  = "http://localhost:9000"
	minioAccessKey = "minioadmin"
	minioSecretKey = "minioadmin"
	minioRegion    = "us-east-1"
)

// StartMinio implements "shipq start minio".
// It starts a foreground MinIO server, waits for readiness, creates the
// configured bucket, applies CORS, then blocks until the process exits or a
// signal is received.
func StartMinio() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	// Check minio binary is on $PATH.
	if _, err := exec.LookPath("minio"); err != nil {
		cli.Fatal("minio not found on $PATH -- add it to your shell.nix")
	}

	dataDir := filepath.Join(roots.ShipqRoot, ".shipq", "data")
	minioDataDir := filepath.Join(dataDir, ".minio-data")

	if err := os.MkdirAll(minioDataDir, 0755); err != nil {
		cli.FatalErr("failed to create MinIO data directory", err)
	}

	// Read bucket name from shipq.ini if available.
	bucket := "shipq-dev"
	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	if ini, err := inifile.ParseFile(shipqIniPath); err == nil {
		if b := ini.Get("files", "s3_bucket"); b != "" {
			bucket = b
		}
	}

	cli.Info("Starting MinIO server...")
	cli.Infof("Data directory: %s", minioDataDir)
	cli.Infof("S3 endpoint:   %s", minioEndpoint)
	cli.Infof("Console:       http://localhost:9001")
	cli.Infof("Credentials:   %s / %s", minioAccessKey, minioSecretKey)
	cli.Infof("Bucket:        %s", bucket)
	cli.Info("")

	corsOrigins := "http://localhost:5173,http://localhost:4321"

	minioCmd := exec.Command("minio", "server", minioDataDir,
		"--address", ":9000",
		"--console-address", ":9001",
	)
	minioCmd.Env = append(os.Environ(),
		"MINIO_ROOT_USER="+minioAccessKey,
		"MINIO_ROOT_PASSWORD="+minioSecretKey,
		"MINIO_API_CORS_ALLOW_ORIGIN="+corsOrigins,
	)
	minioCmd.Stdout = os.Stdout
	minioCmd.Stderr = os.Stderr

	if err := minioCmd.Start(); err != nil {
		cli.FatalErr("failed to start MinIO", err)
	}

	// In a goroutine, wait for readiness then create bucket and configure CORS.
	go func() {
		if err := waitForMinio(minioEndpoint, 30); err != nil {
			cli.Warnf("MinIO readiness check failed: %v", err)
			return
		}
		cli.Success("MinIO is ready")

		if err := ensureMinioBucket(minioEndpoint, minioAccessKey, minioSecretKey, bucket, minioRegion); err != nil {
			cli.Warnf("Failed to create bucket %q: %v", bucket, err)
		} else {
			cli.Successf("Bucket %q ready", bucket)
		}

		cli.Successf("CORS configured via MINIO_API_CORS_ALLOW_ORIGIN=%s", corsOrigins)
	}()

	// Forward SIGINT/SIGTERM to MinIO.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		cli.Infof("Received %s, shutting down MinIO...", sig)
		if minioCmd.Process != nil {
			_ = minioCmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	if err := minioCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					cli.Info("MinIO stopped")
					return
				}
			}
		}
		cli.FatalErr("MinIO exited with error", err)
	}
}

// ── MinIO-specific helpers ────────────────────────────────────────────────────

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

// newMinioS3Client creates an S3 client pointed at the local MinIO instance.
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

// ensureMinioBucket creates the bucket if it does not already exist.
func ensureMinioBucket(endpoint, accessKey, secretKey, bucket, region string) error {
	client := newMinioS3Client(endpoint, accessKey, secretKey, region)
	ctx := context.Background()

	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		// Bucket already exists.
		return nil
	}

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %q: %w", bucket, err)
	}

	return nil
}
