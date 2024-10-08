package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	LoggerTypeInfo    = "info"
	LoggerTypeVerbose = "verbose"
	LoggerTypeWarning = "warning"
	LoggerTypeError   = "error"
	LoggerTypeFatal   = "fatal"
)

func startLoggerHandlers(wg *sync.WaitGroup, infoQueue, warnQueue chan string, errorQueue chan error) {
	wg.Add(3)

	go func() {
		defer wg.Done()
		infoHandler(infoQueue)
	}()

	go func() {
		defer wg.Done()
		warnHandler(warnQueue)
	}()

	go func() {
		defer wg.Done()
		errorHandler(errorQueue)
	}()
}

func errorHandler(errorQueue chan error) {
	for err := range errorQueue {
		logger(LoggerTypeError, fmt.Sprintf("%v\n", err))
	}
}

func warnHandler(warnQueue chan string) {
	for warning := range warnQueue {
		logger(LoggerTypeWarning, fmt.Sprintf("%v\n", warning))
	}
}

func infoHandler(infoQueue chan string) {
	for message := range infoQueue {
		logger(LoggerTypeInfo, fmt.Sprintf("%v\n", message))
	}
}

func logMoveAction(sourcePath, destinationDirectory string, isDuplicate bool, duplicateStrategy string) (string, error) {
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

	log := fmt.Sprintf(
		"\033[1m[%s] %s%s\033[0m %s\n └─ from %s%s\033[0m\n └─── to %s%s\033[0m\n",
		fileSizeStr, colorCode, actionName, fileName, colorCode, source, colorCode, destination,
	)
	return log, nil
}

func logger(loggerType string, message string) {
	switch loggerType {
	case LoggerTypeInfo:
		InfoLogger.Println(message)
	case LoggerTypeVerbose:
		if *verbose {
			VerboseLogger.Println(message)
		}
	case LoggerTypeWarning:
		WarningLogger.Println(message)
	case LoggerTypeError:
		ErrorLogger.Println(message)
	case LoggerTypeFatal:
		ErrorLogger.Fatal(message)
	default:
		ErrorLogger.Println("Unknown logger type:", loggerType)
	}
}
