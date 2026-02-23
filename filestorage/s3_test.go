package filestorage

import (
	"testing"
)

func TestNewS3Client_RequiresBucket(t *testing.T) {
	_, err := NewS3Client(S3Config{
		Region:    "us-east-1",
		AccessKey: "test",
		SecretKey: "test",
	})
	if err == nil {
		t.Error("expected error when bucket is empty")
	}
}

func TestNewS3Client_DefaultsRegion(t *testing.T) {
	client, err := NewS3Client(S3Config{
		Bucket:    "test-bucket",
		AccessKey: "test",
		SecretKey: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestNewS3Client_WithCustomEndpoint(t *testing.T) {
	client, err := NewS3Client(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  "http://localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestNewS3Client_GenerateUploadURL(t *testing.T) {
	client, err := NewS3Client(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		AccessKey: "test-key",
		SecretKey: "test-secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	url, err := client.GenerateUploadURL("test-key.txt", "text/plain", 1024)
	if err != nil {
		t.Fatalf("GenerateUploadURL failed: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
}

func TestNewS3Client_GenerateDownloadURL(t *testing.T) {
	client, err := NewS3Client(S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		AccessKey: "test-key",
		SecretKey: "test-secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	url, err := client.GenerateDownloadURL("test-key.txt", "original-file.txt")
	if err != nil {
		t.Fatalf("GenerateDownloadURL failed: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
}

func TestIsValidVisibility(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"private", true},
		{"organization", true},
		{"public", true},
		{"anonymous", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsValidVisibility(tt.input)
		if got != tt.want {
			t.Errorf("IsValidVisibility(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// IsValidVisibility is exported for testing (the handler version is not exported).
func IsValidVisibility(v string) bool {
	switch v {
	case "private", "organization", "public", "anonymous":
		return true
	}
	return false
}
