package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keybraker/mediarizer-2/duplicate"
)

func consumer(
	destinationPath string,
	fileQueue <-chan FileInfo,
	errorQueue chan<- error,
	geoLocation bool,
	format string,
	verbose bool,
	totalFiles int,
	duplicateStrategy string,
	processedFiles *int64,
	done chan<- struct{}) {

	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU() / 2

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fileInfo := range fileQueue {
				processFileInfo(
					fileInfo,
					destinationPath,
					errorQueue,
					geoLocation,
					format,
					verbose,
					totalFiles,
					duplicateStrategy,
				)

				atomic.AddInt64(processedFiles, 1)
			}
		}()
	}

	wg.Wait()
	done <- struct{}{}
}

func processFileInfo(
	fileInfo FileInfo,
	destinationPath string,
	errorQueue chan<- error,
	geoLocation bool,
	format string,
	verbose bool,
	totalFiles int,
	duplicateStrategy string,
) {
	var generatedPath string
	var err error

	generatedPath, err = getDestinationPath(destinationPath, fileInfo, geoLocation, format)
	if err != nil {
		errorQueue <- err
		return
	}

	if fileInfo.isDuplicate {
		generatedPath, err = duplicate.CreateDuplicateFolder(generatedPath, "DUPLICATE")
		if err != nil {
			errorQueue <- err
			return
		}
		generatedPath = filepath.Join(generatedPath, filepath.Base(fileInfo.Path))
	} else {
		_, err = os.Stat(generatedPath)
		if !os.IsNotExist(err) {
			generatedPath, err = generateUniquePathName(generatedPath)
			if err != nil {
				errorQueue <- err
				return
			}
		}
	}

	err = moveFile(
		fileInfo.Path,
		generatedPath,
		verbose,
		/*processedImages,*/
		0,
		totalFiles,
		fileInfo.isDuplicate,
		duplicateStrategy,
	)
	if err != nil {
		errorQueue <- fmt.Errorf("failed to move %s to %s: %v", fileInfo.Path, generatedPath, err)
	}
}

func moveFile(sourcePath, destinationPath string, verbose bool /*processedImages *list.List,*/, processedFiles int, totalFiles int, isDuplicate bool, duplicateStrategy string) error {
	destPath := filepath.Dir(destinationPath)
	if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destPath, err)
	}

	if verbose {
		moveActionLog, err := logMoveAction(sourcePath, destPath, isDuplicate, duplicateStrategy, processedFiles, totalFiles)
		if err != nil {
			return err
		}

		logger(LoggerTypeVerbose, moveActionLog)
	}

	_, err := os.Stat(destinationPath)
	if !os.IsNotExist(err) {
		destinationPath, err = generateUniquePathName(destinationPath)
		if err != nil {
			return err
		}
	}

	err = renameFile(sourcePath, destinationPath)
	if err != nil {
		return err
	}

	return nil
}

func getMonthFormatted(month time.Month, format string) string {
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

func getDestinationPath(destinationPath string, fileInfo FileInfo, geoLocation bool, format string) (string, error) {
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
		monthFolderName := getMonthFormatted(fileInfo.Created.Month(), format)

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

func renameFile(sourcePath, destinationPath string) error {
	if err := os.Rename(sourcePath, destinationPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", sourcePath, destinationPath, err)
	}

	return nil
}

func generateUniquePathName(destinationPath string) (string, error) {
	ext := filepath.Ext(destinationPath)
	nameWithoutExtension := destinationPath[:len(destinationPath)-len(ext)]

	newPath := destinationPath
	counter := 1
	for {
		_, err := os.Stat(newPath)
		if os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", fmt.Errorf("failed to check destination file %s: %v", newPath, err)
		}

		newPath = fmt.Sprintf("%s_%d%s", nameWithoutExtension, counter, ext)
		counter++
	}

	return newPath, nil
}
