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

func moveFile(sourcePath, destinationPath string, verbose bool, processedImages *list.List, processedFiles int, totalFiles int, duplicateStrategy string) error {
	err := createDestinationDirectory(filepath.Dir(destinationPath))
	if err != nil {
		return err
	}

	sourceHash, err := calculateFileHash(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to calculate source file hash: %v", err)
	}

	destinationFiles, err := os.ReadDir(filepath.Dir(destinationPath))
	if err != nil {
		return fmt.Errorf("failed to read destination directory: %v", err)
	}

	if verbose {
		moveActionLog, err := logMoveAction(sourcePath, filepath.Dir(destinationPath), false, duplicateStrategy, processedFiles, totalFiles)
		if err != nil {
			return err
		}

		VerboseLogger.Println(moveActionLog)
	}

	duplicateFileName := findDuplicateFile(sourceHash, destinationFiles, filepath.Dir(destinationPath))
	if duplicateFileName != "" {
		switch duplicateStrategy {
		case "move":
			destinationPath, err = handleDuplicates(destinationPath, duplicateFileName)
			if err != nil {
				return err
			}
		case "delete":
			err := os.Remove(sourcePath)
			if err != nil {
				return err
			}
		case "skip":
		default:
			return fmt.Errorf("invalid duplicateStrategy flag value")
		}
	} else {
		_, err := os.Stat(destinationPath)
		if !os.IsNotExist(err) {
			destinationPath, err = generateUniqueName(destinationPath)
			if err != nil {
				return err
			}
		}
	}

	err = renameFile(sourcePath, destinationPath)
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

func renameFile(sourcePath, destinationPath string) error {
	if err := os.Rename(sourcePath, destinationPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", sourcePath, destinationPath, err)
	}

	return nil
}

func generateUniqueName(destinationPath string) (string, error) {
	ext := filepath.Ext(destinationPath)
	nameWithoutExt := destinationPath[:len(destinationPath)-len(ext)]
	counter := 1
	newPath := destinationPath

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
