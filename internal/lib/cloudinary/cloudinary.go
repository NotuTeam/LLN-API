package cloudinary

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"bg-go/internal/config"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// CDNClient holds Cloudinary client
type CDNClient struct {
	client *cloudinary.Cloudinary
	folder string
}

// UploadResult holds upload result
type UploadResult struct {
	URL      string `json:"url"`
	PublicID  string `json:"public_id"`
	ResourceType string `json:"resource_type"`
}

// CDN instance
var CDN *CDNClient

// Init initializes Cloudinary client
func Init() error {
	cfg := config.Cfg
	
	client, err := cloudinary.NewFromParams(
		cfg.CDN.CloudName,
		cfg.CDN.APIKey,
		cfg.CDN.APISecret,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Cloudinary: %v", err)
	}
	
	CDN = &CDNClient{
		client: client,
		folder: cfg.CDN.Folder,
	}
	
	return nil
}

// Upload uploads a file to Cloudinary
func Upload(file *multipart.FileHeader) (*UploadResult, error) {
	if CDN == nil {
		if err := Init(); err != nil {
			return nil, err
		}
	}
	
	// Open the file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()
	
	// Read file content
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	
	// Determine resource type
	resourceType := "auto"
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == ".pdf" || ext == ".doc" || ext == ".docx" {
		resourceType = "raw"
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	result, err := CDN.client.Upload.Upload(ctx, bytes.NewReader(fileBytes), uploader.UploadParams{
		Folder:       CDN.folder,
		ResourceType: resourceType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload: %v", err)
	}
	
	return &UploadResult{
		URL:          result.SecureURL,
		PublicID:     result.PublicID,
		ResourceType: result.ResourceType,
	}, nil
}

// UploadBytes uploads file bytes directly
func UploadBytes(fileBytes []byte, filename string) (*UploadResult, error) {
	if CDN == nil {
		if err := Init(); err != nil {
			return nil, err
		}
	}
	
	resourceType := "auto"
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".pdf" || ext == ".doc" || ext == ".docx" {
		resourceType = "raw"
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	result, err := CDN.client.Upload.Upload(ctx, bytes.NewReader(fileBytes), uploader.UploadParams{
		Folder:       CDN.folder,
		ResourceType: resourceType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload: %v", err)
	}
	
	return &UploadResult{
		URL:          result.SecureURL,
		PublicID:     result.PublicID,
		ResourceType: result.ResourceType,
	}, nil
}

// Destroy deletes a file from Cloudinary
func Destroy(publicID string) error {
	if publicID == "" {
		return nil
	}
	
	if CDN == nil {
		if err := Init(); err != nil {
			return err
		}
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	_, err := CDN.client.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: publicID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete: %v", err)
	}
	
	return nil
}

// DeleteImage deletes an image (alias for Destroy)
func DeleteImage(publicID string) error {
	return Destroy(publicID)
}

// DeleteMultiple deletes multiple files
func DeleteMultiple(publicIDs []string) error {
	for _, publicID := range publicIDs {
		if err := Destroy(publicID); err != nil {
			return err
		}
	}
	return nil
}
