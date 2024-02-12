package main

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func consumer(destinationPath string, fileQueue <-chan FileInfo, errorQueue chan<- error, logQueue <-chan string, geoLocation bool, format string, verbose bool, totalFiles int, duplicateStrategy string, done chan<- struct{}) {
	processedImages := list.New()
	processedFiles := 0

	for fileInfo := range fileQueue {
		go func(fileInfo FileInfo) { // go
			generatedPath, err := generateDestinationPath(destinationPath, fileInfo, geoLocation, format)
			if err != nil {
				errorQueue <- err
			}

			_, err = os.Stat(generatedPath)
			if !os.IsNotExist(err) {
				generatedPath, err = generateUniqueName(generatedPath)
				if err != nil {
					errorQueue <- err
				}
			}

			err = moveFile(fileInfo.Path, generatedPath, verbose, processedImages, processedFiles, totalFiles, duplicateStrategy)
			if err != nil {
				errorQueue <- fmt.Errorf("failed to move %s to %s: %v", fileInfo.Path, generatedPath, err)
			}
		}(fileInfo)

		processedFiles++
		// if processedFiles%10 == 0 { // Every 10 files, sleep to let I/O catch up
		// 	time.Sleep(100 * time.Millisecond)
		// }
	}

	done <- struct{}{}
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

	if verbose {
		moveActionLog, err := logMoveAction(sourcePath, filepath.Dir(destPath), false, duplicateStrategy, processedFiles, totalFiles)
		if err != nil {
			return err
		}

		VerboseLogger.Println(moveActionLog)
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

	err = renameFile(sourcePath, destPath)
	if err != nil {
		return err
	}

	return nil
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

func generateDestinationPath(destinationPath string, fileInfo FileInfo, geoLocation bool, format string) (string, error) {
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
