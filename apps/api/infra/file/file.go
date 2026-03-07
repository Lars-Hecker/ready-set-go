package file

import (
	"context"
	"time"
)

// UploadRequest contains metadata for requesting an upload URL.
type UploadRequest struct {
	Filename    string
	ContentType string
	Size        int64
}

// UploadResponse contains the presigned URL and the final object key.
type UploadResponse struct {
	URL string
	Key string
}

// DownloadResponse contains the presigned URL for downloading.
type DownloadResponse struct {
	URL string
}

// Service defines the interface for file storage operations.
// Implementations should use presigned URLs to allow direct client uploads/downloads.
type Service interface {
	// GenerateUploadURL creates a presigned URL for uploading a file.
	// The client should PUT the file directly to this URL.
	GenerateUploadURL(ctx context.Context, req UploadRequest) (*UploadResponse, error)

	// GenerateDownloadURL creates a presigned URL for downloading a file.
	GenerateDownloadURL(ctx context.Context, key string) (*DownloadResponse, error)

	// Delete removes a file from storage.
	Delete(ctx context.Context, key string) error

	// URLLifetime returns the configured lifetime for presigned URLs.
	URLLifetime() time.Duration
}