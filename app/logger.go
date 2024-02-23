package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
)

func logMoveAction(sourcePath, destinationDirectory string, isDuplicate bool, duplicateStrategy string, processedFiles int, totalFiles int) (string, error) {
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
			return fmt.Sprintf("\033[1m%s%s\033[0m %s\n", colorCode, actionName, fileName), nil
		case "delete":
			colorCode = "\033[31m"
			actionName = "Deleted (duplicate)"
			return fmt.Sprintf("\033[1m%s%s\033[0m %s\n", colorCode, actionName, fileName), nil
		default:
			colorCode = "\033[35m"
			actionName = "Unknown Operation"
		}
	}

	const maxPathLength = 90
	var source, destination string

	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}

	fileSizeMB := float64(fileInfo.Size()) / 1024.0 / 1024.0
	fileSizeStr := fmt.Sprintf("%.2fMb", fileSizeMB)

	sourceDir := filepath.Dir(sourcePath)
	if len(sourceDir) > maxPathLength {
		source = "..." + sourceDir[len(sourceDir)-maxPathLength:]
	} else {
		source = sourceDir
	}

	if len(destinationDirectory) > maxPathLength {
		destination = "..." + destinationDirectory[len(destinationDirectory)-maxPathLength:]
	} else {
		destination = destinationDirectory
	}

	percentage := math.Min(100, float64(processedFiles+1)/float64(totalFiles)*100)

	if percentage < 10.00 {
		log := fmt.Sprintf("\033[1m[0%.2f%% | %s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n", percentage, fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, destination)
		return log, nil
	} else {
		log := fmt.Sprintf("\033[1m[%.2f%% | %s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n", percentage, fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, destination)
		return log, nil
	}
}
