package utils

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bubblewhite-backend/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// allowedImageTypes maps sniffed MIME type -> file extension to write.
// Sniffing the actual bytes (not trusting the filename/extension the client
// sent) is what stops someone uploading a disguised executable as "photo.jpg".
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// UploadedFile is what callers get back after a successful upload.
type UploadedFile struct {
	URL      string `json:"url"`
	Key      string `json:"key"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// UploadImage validates the file is really an image (by content, not just
// its claimed extension) and uploads it to the R2 bucket under the given
// folder, e.g. UploadImage(fh, "products").
func UploadImage(fh *multipart.FileHeader, folder string) (*UploadedFile, error) {
	cfg := config.Get()

	file, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open upload: %w", err)
	}
	defer file.Close()

	// Sniff the real content type from the first 512 bytes.
	head := make([]byte, 512)
	n, _ := file.Read(head)
	contentType := http.DetectContentType(head[:n])

	ext, ok := allowedImageTypes[contentType]
	if !ok {
		return nil, fmt.Errorf("unsupported file type: %s (only JPEG, PNG, WEBP, GIF images are allowed)", contentType)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to rewind upload: %w", err)
	}

	safeFolder := strings.Trim(folder, "/")
	key := fmt.Sprintf("%s/%d-%s%s", safeFolder, time.Now().UnixNano(), randomSuffix(), ext)

	if config.R2Client == nil {
		return nil, fmt.Errorf("R2 client not configured")
	}

	_, err = config.R2Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.R2Bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to R2: %w", err)
	}

	publicBase := strings.TrimRight(cfg.R2PublicURL, "/")

	return &UploadedFile{
		URL:      publicBase + "/" + key,
		Key:      key,
		MimeType: contentType,
		Size:     fh.Size,
	}, nil
}

// DeleteImage removes an object from R2 by its key (as returned in UploadedFile.Key).
func DeleteImage(key string) error {
	cfg := config.Get()
	if config.R2Client == nil {
		return fmt.Errorf("R2 client not configured")
	}
	_, err := config.R2Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.R2Bucket),
		Key:    aws.String(key),
	})
	return err
}

func randomSuffix() string {
	return NewID("")[:8]
}

// SafeFileName strips path separators from a client-supplied filename —
// defense in depth even though we never actually use the client's filename
// for the storage key above.
func SafeFileName(name string) string {
	return filepath.Base(name)
}
