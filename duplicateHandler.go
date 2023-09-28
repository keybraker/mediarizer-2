package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func processDuplicates(directoryPath string, duplicateStrategy string, verbose bool, errorQueue chan<- error) {
	hashCache := &sync.Map{}
	fileHashMap := make(map[string][]string)
	totalFiles := 0

	log.Println("Duplicate handling started")

	err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errorQueue <- err
			return nil
		}

		if !info.IsDir() {
			fileHash, err := getFileHash(path, hashCache)
			if err != nil {
				errorQueue <- err
				return nil
			}

			hashStr := fmt.Sprintf("%x", fileHash)

			fileHashMap[hashStr] = append(fileHashMap[hashStr], path)
			totalFiles++
		}

		return nil
	})
	if err != nil {
		errorQueue <- err
		return
	}

	log.Println("Duplicates located")

	processedFiles := 0
	for _, files := range fileHashMap {
		if len(files) <= 1 {
			continue
		}

		for i, filePath := range files {
			if i == 0 {
				continue // Skip the first file
			}

			switch duplicateStrategy {
			case "move":
				destPath, err := handleDuplicates(filePath, "duplicates")
				if err != nil {
					errorQueue <- err
				}

				err = moveFile(filePath, destPath, verbose, nil, processedFiles, totalFiles, duplicateStrategy)
				if err != nil {
					errorQueue <- err
				} else {
					logAction(filePath, destPath, true, duplicateStrategy, processedFiles, totalFiles)
				}
			case "delete":
				err := os.Remove(filePath)
				if err != nil {
					errorQueue <- err
				} else {
					logAction(filePath, "", true, duplicateStrategy, processedFiles, totalFiles)
				}
			default:
				panic("invalid duplicateStrategy flag value")
			}

			processedFiles++
		}
	}

	log.Println("Duplicates handling finished")
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

func handleDuplicates(destPath, duplicateFileName string) (string, error) {
	ext := filepath.Ext(duplicateFileName)
	nameWithoutExt := duplicateFileName[:len(duplicateFileName)-len(ext)]
	underscoreExt := strings.ReplaceAll(ext, ".", "_")
	duplicatesFolder := filepath.Join(filepath.Dir(destPath), fmt.Sprintf("%s%s", nameWithoutExt, underscoreExt))

	err := createDestinationDirectory(duplicatesFolder)
	if err != nil {
		return "", err
	}

	return filepath.Join(duplicatesFolder, filepath.Base(destPath)), nil
}
