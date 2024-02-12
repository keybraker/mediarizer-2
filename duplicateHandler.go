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

func processDuplicates(
	directoryPath string,
	duplicateStrategy string,
	verbose bool,
	fileHashMap map[string][]string,
	errorQueue chan<- error,
) {
	hashCache := &sync.Map{}
	totalFiles := 0

	InfoLogger.Println("Duplicate handling started")

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

	InfoLogger.Println("Duplicates located")

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
				destinationPath, err := handleDuplicates(filePath, "duplicates")
				if err != nil {
					errorQueue <- err
				}

				err = moveFile(filePath, destinationPath, verbose, nil, processedFiles, totalFiles, duplicateStrategy)
				if err != nil {
					errorQueue <- err
				} else {
					logMoveAction(filePath, destinationPath, true, duplicateStrategy, processedFiles, totalFiles)
				}
			case "delete":
				err := os.Remove(filePath)
				if err != nil {
					errorQueue <- err
				} else {
					logMoveAction(filePath, "", true, duplicateStrategy, processedFiles, totalFiles)
				}
			default:
				panic("invalid duplicateStrategy flag value")
			}

			processedFiles++
		}
	}

	InfoLogger.Println("Duplicates handling finished")
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

func handleDuplicates(destinationPath, duplicateFileName string) (string, error) {
	ext := filepath.Ext(duplicateFileName)
	nameWithoutExt := duplicateFileName[:len(duplicateFileName)-len(ext)]
	underscoreExt := strings.ReplaceAll(ext, ".", "_")
	duplicatesFolder := filepath.Join(filepath.Dir(destinationPath), fmt.Sprintf("%s%s", nameWithoutExt, underscoreExt))

	err := createDestinationDirectory(duplicatesFolder)
	if err != nil {
		return "", err
	}

	return filepath.Join(duplicatesFolder, filepath.Base(destinationPath)), nil
}

func findDuplicateFile(sourceHash []byte, destFiles []fs.DirEntry, destDir string) string {
	for _, destFile := range destFiles {
		destFilePath := filepath.Join(destDir, destFile.Name())
		destHash, err := calculateFileHash(destFilePath)
		if err != nil {
			return ""
		}

		if bytes.Equal(sourceHash, destHash) {
			return destFile.Name()
		}
	}

	return ""
}
