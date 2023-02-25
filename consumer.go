package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func consumer(destinationPath string, queue <-chan FileInfo, geoLocation bool, done chan<- struct{}) {
	for fileInfo := range queue {
		var destPath string

		if geoLocation {
			// Determine the destination path based on the file type and created time.
			switch fileInfo.FileType {
			case FileTypeImage:
				destPath = fmt.Sprintf("%s/%s/images/%s", destinationPath, fileInfo.Country, filepath.Base(fileInfo.Path))
			case FileTypeVideo:
				destPath = fmt.Sprintf("%s/%s/videos/%s", destinationPath, fileInfo.Country, filepath.Base(fileInfo.Path))
			case FileTypeUnknown:
				destPath = fmt.Sprintf("%s/unknown/%s", destinationPath, filepath.Base(fileInfo.Path))
			}
		} else {
			// Determine the destination path based on the file type and created time.
			switch fileInfo.FileType {
			case FileTypeImage:
				destPath = fmt.Sprintf("%s/%04d/%s/images/%s", destinationPath, fileInfo.Created.Year(), fileInfo.Created.Month().String(), filepath.Base(fileInfo.Path))
			case FileTypeVideo:
				destPath = fmt.Sprintf("%s/%04d/%s/videos/%s", destinationPath, fileInfo.Created.Year(), fileInfo.Created.Month().String(), filepath.Base(fileInfo.Path))
			case FileTypeUnknown:
				destPath = fmt.Sprintf("%s/unknown/%s", destinationPath, filepath.Base(fileInfo.Path))
			}
		}
		// Move the file to the destination path.
		if err := moveFile(fileInfo.Path, destPath); err != nil {
			fmt.Printf("failed to move %s to %s: %v\n", fileInfo.Path, destPath, err)
		}
	}

	done <- struct{}{}
}

func moveFile(sourcePath, destPath string) error {
	// Create the destination file's directory if it doesn't exist.
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", filepath.Dir(destPath), err)
	}

	// Rename the source file to the destination file.
	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", sourcePath, destPath, err)
	}

	return nil
}
