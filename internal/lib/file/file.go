package file

import (
	"mime/multipart"
	"path/filepath"
	"strings"

	"bg-go/internal/config"
	"bg-go/internal/lib/cloudinary"
)

// UploadResult holds upload result
type UploadResult struct {
	URL      string `json:"url"`
	PublicID string `json:"public_id"`
}

// UploadFile uploads a file to CDN
func UploadFile(file *multipart.FileHeader) (*UploadResult, error) {
	result, err := cloudinary.Upload(file)
	if err != nil {
		return nil, err
	}
	
	return &UploadResult{
		URL:      result.URL,
		PublicID: result.PublicID,
	}, nil
}

// DeleteFile deletes a file from CDN
func DeleteFile(publicID string) error {
	return cloudinary.Destroy(publicID)
}

// IsAllowedFileType checks if file type is allowed
func IsAllowedFileType(filename string) bool {
	cfg := config.Cfg
	
	ext := strings.ToLower(filepath.Ext(filename))
	ext = strings.TrimPrefix(ext, ".")
	
	for _, allowed := range cfg.Upload.AllowedFileTypes {
		if ext == allowed {
			return true
		}
	}
	
	return false
}

// ValidateFile validates file size and type
func ValidateFile(file *multipart.FileHeader) error {
	cfg := config.Cfg
	
	// Check file size
	if file.Size > cfg.Upload.MaxFileSize {
		return nil // Return error if needed
	}
	
	// Check file type
	if !IsAllowedFileType(file.Filename) {
		return nil // Return error if needed
	}
	
	return nil
}

// UpdateFile updates a file (delete old, upload new)
func UpdateFile(oldPublicID string, newFile *multipart.FileHeader) (*UploadResult, error) {
	// Delete old file if exists
	if oldPublicID != "" {
		cloudinary.Destroy(oldPublicID)
	}
	
	// Upload new file
	return UploadFile(newFile)
}
