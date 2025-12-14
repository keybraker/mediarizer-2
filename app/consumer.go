package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func consumer(
	destinationPath string,
	fileQueue <-chan FileInfo,
	errorQueue chan<- error,
	geoLocation bool,
	format string,
	verbose bool,
	processedFiles *int64,
	done chan<- struct{},
	hashCache *sync.Map) {

	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU() / 2
	if numWorkers < 1 {
		numWorkers = 1
	}

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
					hashCache,
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
	hashCache *sync.Map,
) {
	var generatedPath string
	var err error

	generatedPath, err = getDestinationPath(destinationPath, fileInfo, geoLocation, format)
	if err != nil {
		errorQueue <- err
		return
	}

	// Check if file already exists and add numeric suffix if needed
	_, err = os.Stat(generatedPath)
	if !os.IsNotExist(err) {
		generatedPath, err = generateUniquePathName(generatedPath)
		if err != nil {
			errorQueue <- err
			return
		}
	}

	err = moveFile(
		fileInfo.Path,
		generatedPath,
		verbose,
	)
	if err != nil {
		errorQueue <- fmt.Errorf("failed to move %s to %s: %v", fileInfo.Path, generatedPath, err)
	} else {
		// Update hash cache with new path
		if val, ok := hashCache.Load(fileInfo.Path); ok {
			hashCache.Store(generatedPath, val)
			hashCache.Delete(fileInfo.Path)
		} else {
			// If not in cache, try to load it (might have been added by creator or just missed)
			// But we don't want to calculate hash here if not needed.
			// Just try to get it from source path if it was there.
			// If it wasn't in cache, we leave it. HashImagesInPath will handle it later.
		}
	}
}

func moveFile(sourcePath, destinationPath string, verbose bool) error {
	destPath := filepath.Dir(destinationPath)
	if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destPath, err)
	}

	if verbose {
		moveActionLog, err := logMoveAction(sourcePath, destPath)
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
