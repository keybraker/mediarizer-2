package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func consumer(destinationPath string, fileInfoQueue <-chan FileInfo, geoLocation bool, format string, done chan<- struct{}) {
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

func moveFile(sourcePath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", filepath.Dir(destPath), err)
	}

	sourceHash, err := calculateFileHash(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to calculate source file hash: %v", err)
	}

	destFiles, err := os.ReadDir(filepath.Dir(destPath))
	if err != nil {
		return fmt.Errorf("failed to read destination directory: %v", err)
	}

	duplicateFileName := ""
	for _, destFile := range destFiles {
		destFilePath := filepath.Join(filepath.Dir(destPath), destFile.Name())
		destHash, err := calculateFileHash(destFilePath)
		if err != nil {
			return fmt.Errorf("failed to calculate destination file hash: %v", err)
		}

		if bytes.Equal(sourceHash, destHash) {
			duplicateFileName = destFile.Name()
			break
		}
	}

	if duplicateFileName != "" {
		ext := filepath.Ext(duplicateFileName)
		nameWithoutExt := duplicateFileName[:len(duplicateFileName)-len(ext)]
		underscoreExt := strings.ReplaceAll(ext, ".", "_")
		duplicatesFolder := filepath.Join(filepath.Dir(destPath), fmt.Sprintf("%s%s_duplicates", nameWithoutExt, underscoreExt))
		if _, err := os.Stat(duplicatesFolder); os.IsNotExist(err) {
			if err := os.MkdirAll(duplicatesFolder, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create duplicates folder %s: %v", duplicatesFolder, err)
			}
		}
		destPath = filepath.Join(duplicatesFolder, filepath.Base(sourcePath))
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", sourcePath, destPath, err)
	}

	return nil
}
