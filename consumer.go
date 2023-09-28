package main

import (
	"container/list"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const BlockSize = 4096 // 4KB

func generateDestinationPath(
	destinationPath string,
	fileInfo FileInfo,
	geoLocation bool,
	format string,
) (string, error) {
	if geoLocation {
		switch fileInfo.FileType {
		case FileTypeImage:
			return fmt.Sprintf("%s/%s/images/%s", destinationPath, fileInfo.Country, filepath.Base(fileInfo.Path)), nil
		case FileTypeVideo:
			return fmt.Sprintf("%s/%s/videos/%s", destinationPath, fileInfo.Country, filepath.Base(fileInfo.Path)), nil
		case FileTypeUnknown:
			return fmt.Sprintf("%s/unknown/%s", destinationPath, filepath.Base(fileInfo.Path)), nil
		}
	} else {
		monthFolderName := monthFolder(fileInfo.Created.Month(), format)

		switch fileInfo.FileType {
		case FileTypeImage:
			return fmt.Sprintf("%s/%04d/%s/images/%s", destinationPath, fileInfo.Created.Year(), monthFolderName, filepath.Base(fileInfo.Path)), nil
		case FileTypeVideo:
			return fmt.Sprintf("%s/%04d/%s/videos/%s", destinationPath, fileInfo.Created.Year(), monthFolderName, filepath.Base(fileInfo.Path)), nil
		case FileTypeUnknown:
			return fmt.Sprintf("%s/unknown/%s", destinationPath, filepath.Base(fileInfo.Path)), nil
		}
	}

	return "", fmt.Errorf("failed to generate destination path for %s", fileInfo.Path)
}

func consumer(
	destinationPath string,
	fileInfoQueue <-chan FileInfo,
	geoLocation bool,
	format string,
	verbose bool,
	totalFiles int,
	duplicateStrategy string,
	errorQueue chan<- error,
	done chan<- struct{},
) {
	var hashCache sync.Map

	processedImages := list.New()
	processedFiles := 0

	for fileInfo := range fileInfoQueue {
		go func(fileInfo FileInfo) {
			destPath, err := generateDestinationPath(destinationPath, fileInfo, geoLocation, format)
			if err != nil {
				errorQueue <- err
			}
			destDir := filepath.Dir(destPath)

			destFiles, err := os.ReadDir(destDir)
			if err != nil {
				errorQueue <- fmt.Errorf("failed to read destination directory: %v", err)
			}

			duplicateFileName, err := findDuplicateFile(fileInfo.Path, destFiles, destDir, &hashCache)
			if err != nil {
				errorQueue <- err
			}

			if duplicateFileName != "" {
				switch duplicateStrategy {
				case "move":
					destPath, err = handleDuplicates(destPath, duplicateFileName)
					if err != nil {
						errorQueue <- err
					}
				case "skip":
					return
				case "delete":
					err := os.Remove(fileInfo.Path)
					if err != nil {
						errorQueue <- err
					}
					return
				default:
					panic("invalid duplicateStrategy flag value")
				}
			} else {
				_, err := os.Stat(destPath)
				if !os.IsNotExist(err) {
					destPath, err = generateUniqueName(destPath)
					if err != nil {
						errorQueue <- err
					}
				}
			}

			if err := moveFile(fileInfo.Path, destPath, verbose, processedImages, processedFiles, totalFiles, duplicateStrategy); err != nil {
				errorQueue <- fmt.Errorf("failed to move %s to %s: %v", fileInfo.Path, destPath, err)
			}
		}(fileInfo)

		processedFiles++
		if processedFiles%10 == 0 { // Every 10 files, sleep to let I/O catch up
			time.Sleep(100 * time.Millisecond)
		}
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

func logAction(sourcePath, destDir string, isDuplicate bool, duplicateStrategy string, processedFiles int, totalFiles int) {
	colorCode := "\033[32m"
	actionName := "Moved (original)"

	fileName := filepath.Base(sourcePath)

	if isDuplicate {
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
			log.Printf("\033[1m%s%s\033[0m %s\n", colorCode, actionName, fileName)
			return
		default:
			colorCode = "\033[35m"
			actionName = "Unknown Operation"
		}
	}

	const maxPathLength = 90
	var source, dest string

	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		return
	}

	fileSizeMB := float64(fileInfo.Size()) / 1024.0 / 1024.0
	fileSizeStr := fmt.Sprintf("%.2fMb", fileSizeMB)

	sourceDir := filepath.Dir(sourcePath)
	if len(sourceDir) > maxPathLength {
		source = "..." + sourceDir[len(sourceDir)-maxPathLength:]
	} else {
		source = sourceDir
	}

	if len(destDir) > maxPathLength {
		dest = "..." + destDir[len(destDir)-maxPathLength:]
	} else {
		dest = destDir
	}

	percentage := math.Min(100, float64(processedFiles+1)/float64(totalFiles)*100)
	if percentage < 10.00 {
		log.Printf("\033[1m[0%.2f%% | %s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n", percentage, fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, dest)
	} else {
		log.Printf("\033[1m[%.2f%% | %s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n", percentage, fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, dest)
	}
}

func moveFile(
	sourcePath,
	destPath string,
	verbose bool,
	processedImages *list.List,
	processedFiles int,
	totalFiles int,
	duplicateStrategy string,
) error {
	err := createDestinationDirectory(filepath.Dir(destPath))
	if err != nil {
		return err
	}

	if verbose {
		logAction(sourcePath, filepath.Dir(destPath), false, duplicateStrategy, processedFiles, totalFiles)
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
