package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

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

// getFileHash retrieves or calculates the hash of the file at filePath.
func getFileHash(filePath string, hashCache *sync.Map) ([]byte, error) {
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

// createDuplicateFolder creates a folder for storing duplicates of the file.
func createDuplicateFolder(destinationPath, duplicateFileName string) (string, error) {
	ext := filepath.Ext(duplicateFileName)
	nameWithoutExt := strings.TrimSuffix(duplicateFileName, ext)
	underscoreExt := strings.ReplaceAll(ext, ".", "_")
	duplicatesFolder := filepath.Join(filepath.Dir(destinationPath), fmt.Sprintf("%s%s", nameWithoutExt, underscoreExt))

	err := os.MkdirAll(duplicatesFolder, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create duplicates folder: %v", err)
	}

	return duplicatesFolder, nil
}

// isDuplicate checks if the file is a duplicate and handles it based on the strategy.
func isDuplicate(path string, duplicateStrategy string, fileHashMap map[string]bool, hashCache *sync.Map) (bool, error) {
	fileHash, err := getFileHash(path, hashCache)
	if err != nil {
		return false, err
	}

	hashStr := fmt.Sprintf("%x", fileHash)

	if _, exists := fileHashMap[hashStr]; exists {
		return true, nil
	}

	fileHashMap[hashStr] = true
	return false, nil
}
