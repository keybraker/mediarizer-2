package main

import (
	"fmt"
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

// hashImagesInPath hashes all images in the given path and updates the fileHashMap.
func hashImagesInPath(path string, fileHashMap map[string]bool, hashCache *sync.Map) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if isImageFile(filePath) {
			hash, err := getFileHash(filePath, hashCache)
			if err != nil {
				return fmt.Errorf("failed to get file hash for %s: %v", filePath, err)
			}

			hashStr := fmt.Sprintf("%x", hash)
			fileHashMap[hashStr] = true
		}

		return nil
	})
}
