package hash

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// isImageFile checks if the file is an image based on its extension.
func isImageFile(filePath string) bool {
	lowerFilePath := strings.ToLower(filePath)
	return strings.HasSuffix(lowerFilePath, ".jpg") || strings.HasSuffix(lowerFilePath, ".jpeg") ||
		strings.HasSuffix(lowerFilePath, ".png") || strings.HasSuffix(lowerFilePath, ".gif") ||
		strings.HasSuffix(lowerFilePath, ".bmp") || strings.HasSuffix(lowerFilePath, ".tiff")
}

// calculateFileHash calculates the SHA-256 hash of the file at the given filePath.
func calculateFileHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file at %s: %v", filePath, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to calculate hash for file: %v", err)
	}

	return hash.Sum(nil), nil
}

// GetFileHash retrieves or calculates the hash of the file at filePath.
func GetFileHash(filePath string, hashCache *sync.Map) ([]byte, error) {
	if hash, found := hashCache.Load(filePath); found {
		return hash.([]byte), nil
	}

	calculatedHash, err := calculateFileHash(filePath)
	if err != nil {
		return nil, err
	}

	hashCache.Store(filePath, calculatedHash)
	return calculatedHash, nil
}

// hashImagesInPath hashes all images in the given path and updates the fileHashMap.
func HashImagesInPath(path string, fileHashMap map[string]bool, hashCache *sync.Map) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if isImageFile(filePath) {
			hash, err := GetFileHash(filePath, hashCache)
			if err != nil {
				return fmt.Errorf("failed to get file hash for %s: %v", filePath, err)
			}

			hashStr := fmt.Sprintf("%x", hash)
			fileHashMap[hashStr] = true
		}

		return nil
	})
}
