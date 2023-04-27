package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func consumer(destinationPath string, queue <-chan FileInfo, geoLocation bool, format string, done chan<- struct{}) {
	for fileInfo := range queue {
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
		if err := moveFile(fileInfo.Path, destPath); err != nil {
			fmt.Printf("failed to move %s to %s: %v\n", fileInfo.Path, destPath, err)
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

func moveFile(sourcePath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", filepath.Dir(destPath), err)
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", sourcePath, destPath, err)
	}

	return nil
}
