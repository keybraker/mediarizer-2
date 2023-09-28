package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func findDuplicates(directoryPath string, hashCache *sync.Map) {
	// Key: file hash, Value: slice of file paths
	fileHashMap := make(map[string][]string)

	err := filepath.Walk(directoryPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileHash, err := getFileHash(path, hashCache)
			if err != nil {
				return err
			}

			hashStr := fmt.Sprintf("%x", fileHash)

			fileHashMap[hashStr] = append(fileHashMap[hashStr], path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("An error occurred: %v\n", err)
		return
	}

	// Identify duplicates
	for hash, files := range fileHashMap {
		if len(files) > 1 {
			fmt.Printf("Duplicate files found for hash %s: %v\n", hash, files)
			// Further actions like deletion can be implemented here
		}
	}
}

// func calculateFileHash(filePath string) (uint32, error) {
// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer file.Close()

// 	hasher := fnv.New32a()

// 	// Read first N bytes
// 	firstBlock := make([]byte, BlockSize)
// 	_, err = file.Read(firstBlock)
// 	if err != nil {
// 		return 0, err
// 	}
// 	hasher.Write(firstBlock)

// 	// Move to the last N bytes
// 	_, err = file.Seek(-BlockSize, os.SEEK_END)
// 	if err != nil {
// 		return 0, err
// 	}

// 	// Read last N bytes
// 	lastBlock := make([]byte, BlockSize)
// 	_, err = file.Read(lastBlock)
// 	if err != nil {
// 		return 0, err
// 	}
// 	hasher.Write(lastBlock)

// 	return hasher.Sum32(), nil
// }

func calculateFileHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func getFileHash(filePath string, hashCache *sync.Map) ([]byte, error) {
	if hash, found := hashCache.Load(filePath); found {
		return hash.([]byte), nil
	}

	calculatedHash, err := calculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate source file hash: %v", err)
	}
	hashCache.Store(filePath, calculatedHash)

	return calculatedHash, nil
}

func findDuplicateFile(sourcePath string, destFiles []fs.DirEntry, destDir string, hashCache *sync.Map) (string, error) {
	sourceHash, err := getFileHash(sourcePath, hashCache)
	if err != nil {
		return "", err
	}

	for _, destFile := range destFiles {
		destFilePath := filepath.Join(destDir, destFile.Name())

		destHash, err := getFileHash(destFilePath, hashCache)
		if err != nil {
			return "", err
		}

		if bytes.Equal(sourceHash, destHash) {
			return destFile.Name(), nil
		}
	}

	return "", nil
}

func handleDuplicates(destPath, duplicateFileName string) (string, error) {
	ext := filepath.Ext(duplicateFileName)
	nameWithoutExt := duplicateFileName[:len(duplicateFileName)-len(ext)]
	underscoreExt := strings.ReplaceAll(ext, ".", "_")
	duplicatesFolder := filepath.Join(filepath.Dir(destPath), fmt.Sprintf("%s%s_duplicates", nameWithoutExt, underscoreExt))

	err := createDestinationDirectory(duplicatesFolder)
	if err != nil {
		return "", err
	}

	return filepath.Join(duplicatesFolder, filepath.Base(destPath)), nil
}
