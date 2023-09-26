package main

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func consumer(destinationPath string, fileInfoQueue <-chan FileInfo, geoLocation bool, format string, verbose bool, totalFiles int, duplicateStrategy string, done chan<- struct{}) {
	processedImages := list.New()
	processedFiles := 0

	for fileInfo := range fileInfoQueue {
		var destPath string

		if geoLocation {
			switch fileInfo.FileType {
			case FileTypeImage:
				destPath = fmt.Sprintf("%s/%s/images/%s", destinationPath, fileInfo.Country, filepath.Base(fileInfo.Path))
			case FileTypeVideo:
				destPath = fmt.Sprintf("%s/%s/videos/%s", destinationPath, fileInfo.Country, filepath.Base(fileInfo.Path))
			case FileTypeUnknown:
				destPath = fmt.Sprintf("%s/unknown/%s", destinationPath, filepath.Base(fileInfo.Path))
			}
		} else {
			monthFolderName := monthFolder(fileInfo.Created.Month(), format)

			switch fileInfo.FileType {
			case FileTypeImage:
				destPath = fmt.Sprintf("%s/%04d/%s/images/%s", destinationPath, fileInfo.Created.Year(), monthFolderName, filepath.Base(fileInfo.Path))
			case FileTypeVideo:
				destPath = fmt.Sprintf("%s/%04d/%s/videos/%s", destinationPath, fileInfo.Created.Year(), monthFolderName, filepath.Base(fileInfo.Path))
			case FileTypeUnknown:
				destPath = fmt.Sprintf("%s/unknown/%s", destinationPath, filepath.Base(fileInfo.Path))
			}
		}

		if err := moveFile(fileInfo.Path, destPath, verbose, processedImages, processedFiles, totalFiles, duplicateStrategy); err != nil {
			fmt.Printf("failed to move %s to %s: %v\n", fileInfo.Path, destPath, err)
		}

		processedFiles++
	}

	done <- struct{}{}
}

func monthFolder(month time.Month, format string) string {
	switch format {
	case "word":
		return month.String()
	case "number":
		return fmt.Sprintf("%02d", month)
	case "combined":
		return fmt.Sprintf("%02d_%s", month, month.String())
	default:
		return month.String()
	}
}

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

func moveFile(sourcePath, destPath string, verbose bool, processedImages *list.List, processedFiles int, totalFiles int, duplicateStrategy string) error {
	err := createDestinationDirectory(filepath.Dir(destPath))
	if err != nil {
		return err
	}

	sourceHash, err := calculateFileHash(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to calculate source file hash: %v", err)
	}

	destFiles, err := os.ReadDir(filepath.Dir(destPath))
	if err != nil {
		return fmt.Errorf("failed to read destination directory: %v", err)
	}

	duplicateFileName := findDuplicateFile(sourceHash, destFiles, filepath.Dir(destPath))
	if duplicateFileName != "" {
		switch duplicateStrategy {
		case "move":
			destPath, err = handleDuplicates(destPath, duplicateFileName)
			if err != nil {
				return err
			}
		case "skip":
		case "delete":
			err := os.Remove(sourcePath)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid duplicateStrategy flag value")
		}
	} else {
		_, err := os.Stat(destPath)
		if !os.IsNotExist(err) {
			destPath, err = generateUniqueName(destPath)
			if err != nil {
				return err
			}
		}
	}

	if verbose {
		colorCode := "\033[32m"
		actionName := "Moved (original)"

		fileName := filepath.Base(sourcePath)

		if duplicateFileName != "" {
			switch duplicateStrategy {
			case "move":
				colorCode = "\033[33m"
				actionName = "Moved (duplicate)"
			case "skip":
				colorCode = "\033[34m"
				actionName = "Skipped (duplicate)"
			case "delete":
				colorCode = "\033[31m"
				actionName = "Deleted (duplicate)"
				fmt.Printf("\033[1m%s%s\033[0m %s\n", colorCode, actionName, fileName)
				return nil
			default:
				colorCode = "\033[35m"
				actionName = "Unknown Operation"
			}
		}

		const maxPathLength = 90
		var source, dest string

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		fileSizeMB := float64(fileInfo.Size()) / 1024.0 / 1024.0
		fileSizeStr := fmt.Sprintf("%.2fMb", fileSizeMB)

		sourceDir := filepath.Dir(sourcePath)
		if len(sourceDir) > maxPathLength {
			source = "..." + sourceDir[len(sourceDir)-maxPathLength:]
		} else {
			source = sourceDir
		}

		destDir := filepath.Dir(destPath)
		if len(destDir) > maxPathLength {
			dest = "..." + destDir[len(destDir)-maxPathLength:]
		} else {
			dest = destDir
		}

		percentage := math.Min(100, float64(processedFiles+1)/float64(totalFiles)*100)
		if percentage < 10.00 {
			fmt.Printf("\033[1m[0%.2f%% | %s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n", percentage, fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, dest)
		} else {
			fmt.Printf("\033[1m[%.2f%% | %s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n", percentage, fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, dest)
		}
	}

	err = renameFile(sourcePath, destPath)
	if err != nil {
		return err
	}

	return nil
}

func createDestinationDirectory(destDir string) error {
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destDir, err)
	}
	return nil
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

func renameFile(sourcePath, destPath string) error {
	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", sourcePath, destPath, err)
	}

	return nil
}

func generateUniqueName(destPath string) (string, error) {
	ext := filepath.Ext(destPath)
	nameWithoutExt := destPath[:len(destPath)-len(ext)]
	counter := 1
	newPath := destPath

	for {
		_, err := os.Stat(newPath)
		if os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", fmt.Errorf("failed to check destination file %s: %v", newPath, err)
		}

		newPath = fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		counter++
	}

	return newPath, nil
}
