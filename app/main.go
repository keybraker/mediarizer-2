package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keybraker/mediarizer-2/hash"
)

var (
	inputPath         *string
	outputPath        *string
	duplicateStrategy *string
	moveUnknown       *bool
	geoLocation       *bool
	fileTypesString   *string
	organisePhotos    *bool
	organiseVideos    *bool
	format            *string
	showHelp          *bool
	verbose           *bool
	showVersion       *bool

	InfoLogger    *log.Logger
	VerboseLogger *log.Logger
	WarningLogger *log.Logger
	ErrorLogger   *log.Logger
)

func main() {
	l0 := "   __  ___       ___          _                ___ "
	l1 := "  /  |/  /__ ___/ (_)__ _____(_)__ ___ ____   |_  |"
	l2 := " / /|_/ / -_) _  / / _ `/ __/ /_ // -_) __/  / __/ "
	l3 := "/_/  /_/\\__/\\_,_/_/\\_,_/_/ /_//__/\\__/_/    /____/ (v1.0.2)"
	fmt.Println("\n" + l0 + "\n" + l1 + "\n" + l2 + "\n" + l3 + "\n\n\t\t\t\tby Keybraker\n")

	start := time.Now()

	flag.Parse()
	fileTypes := flagProcessor()

	sourcePath, destinationPath := validatePaths(*inputPath, *outputPath)

	fileQueue := make(chan FileInfo, 100)
	infoQueue := make(chan string, 50)
	warnQueue := make(chan string, 10)
	errorQueue := make(chan error, 50)

	var wg sync.WaitGroup

	startLoggerHandlers(&wg, infoQueue, warnQueue, errorQueue)

	logger(LoggerTypeInfo, "Counting files in path.")
	totalFilesToMove := countFiles(sourcePath, fileTypes, *organisePhotos, *organiseVideos)

	if totalFilesToMove == 0 {
		logger(LoggerTypeInfo, "No files in path, exiting.")
		return
	} else {
		logger(LoggerTypeInfo, fmt.Sprintf("%d files to be processed.", totalFilesToMove))
	}

	hashCache, err := hash.InitHashCache("")
	if err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("Failed to load hash cache: %v. Using empty cache.", err))
		hashCache = &sync.Map{}
	} else {
		logger(LoggerTypeInfo, "Hash cache loaded successfully.")
	}

	logger(LoggerTypeInfo, "Creating file hash-map on the destination path.")
	totalFilesInDestination := countFiles(destinationPath, fileTypes, *organisePhotos, *organiseVideos)

	var hashedFiles int64
	stopHashSpinner := make(chan bool)
	go spinner(stopHashSpinner, "Hashing:", &hashedFiles, totalFilesInDestination)

	fileHashMap, err := hash.HashImagesInPath(destinationPath, hashCache, &hashedFiles)
	if err != nil {
		stopHashSpinner <- true
		logger(LoggerTypeInfo, "Failed to create file hash map.")
		logger(LoggerTypeFatal, err.Error())
	}

	stopHashSpinner <- true
	elapsed := time.Since(start)
	logger(LoggerTypeInfo, fmt.Sprintf("File hash-map created in %.2f seconds.", elapsed.Seconds()))

	var processedFiles int64

	stopSpinner := make(chan bool)
	go spinner(stopSpinner, "Processing:", &processedFiles, totalFilesToMove)

	done := make(chan struct{})

	go creator(
		sourcePath,
		fileQueue,
		warnQueue,
		errorQueue,
		*geoLocation,
		*moveUnknown,
		fileTypes,
		*organisePhotos,
		*organiseVideos,
		*duplicateStrategy,
		fileHashMap,
		hashCache,
	)

	go consumer(
		destinationPath,
		fileQueue,
		errorQueue,
		*geoLocation,
		*format,
		*verbose,
		*duplicateStrategy,
		&processedFiles,
		done,
	)

	<-done
	stopSpinner <- true

	// Save the hash cache to disk before exiting
	if err := hash.SaveHashCache(hashCache, hash.DefaultCacheFilePath); err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("Failed to save hash cache: %v", err))
	} else {
		logger(LoggerTypeInfo, "Hash cache saved successfully.")
	}

	elapsed = time.Since(start)
	elapsedString := formatElapsedTime(elapsed)

	logger(LoggerTypeInfo, strconv.Itoa(totalFilesToMove)+" files processed.")
	logger(LoggerTypeInfo, fmt.Sprintf("Processing completed in %s.", elapsedString))
}

func formatElapsedTime(elapsed time.Duration) string {
	seconds := int(elapsed.Seconds())
	minutes := seconds / 60
	seconds = seconds % 60

	if minutes > 0 {
		if minutes == 1 {
			return fmt.Sprintf("%d minute and %d seconds", minutes, seconds)
		}
		return fmt.Sprintf("%d minutes and %d seconds", minutes, seconds)
	}

	return fmt.Sprintf("%.2f seconds", elapsed.Seconds())
}

func spinner(stopSpinner chan bool, verb string, processedFiles *int64, totalFiles int) {
	spinChars := `-\|/`
	i := 0
	for {
		select {
		case <-stopSpinner:
			fmt.Printf("\r%s\r", strings.Repeat(" ", 80))
			return
		default:
			processed := atomic.LoadInt64(processedFiles)
			percentage := float64(processed) / float64(totalFiles) * 100
			fmt.Printf("\r%c | %s: %d/%d (%.2f%%)", spinChars[i], verb, processed, totalFiles, percentage)
			i = (i + 1) % len(spinChars)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func displayHelp() {
	flag.PrintDefaults()
}

func arrayContains(stringArray []string, stringCandidate string) bool {
	for _, string := range stringArray {
		if string == stringCandidate {
			return true
		}
	}

	return false
}

func countFiles(rootPath string, fileTypes []string, organisePhotos bool, organiseVideos bool) int {
	count := 0

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))

			if (organisePhotos && isPhoto(ext) || organiseVideos && isVideo(ext)) &&
				(len(fileTypes) == 0 || arrayContains(fileTypes, ext)) {
				count++
			}
		}

		return nil
	})

	return count
}

func init() {
	inputPath = flag.String("input", "", "Path to source file or directory")
	outputPath = flag.String("output", "", "Path to destination directory")
	duplicateStrategy = flag.String("duplicate", "move", "Duplication handling, default \"move\" (move, skip, delete)")
	moveUnknown = flag.Bool("unknown", true, "Move files with no metadata to undetermined folder")
	geoLocation = flag.Bool("location", false, "Organize files based on their geo location")
	fileTypesString = flag.String("types", "", "Comma separated file extensions to organize (.jpg, .png, .gif, .mp4, .avi, .mov, .mkv)")
	organisePhotos = flag.Bool("photo", true, "Organise only photos")
	organiseVideos = flag.Bool("video", true, "Organise only videos")
	format = flag.String("format", "word", "Naming format for month folders, default \"word\" (word, number, combined)")
	showHelp = flag.Bool("help", false, "Display usage guide")
	verbose = flag.Bool("verbose", false, "Display progress information in console")
	showVersion = flag.Bool("version", false, "Display version information")

	InfoLogger = log.New(os.Stdout, "\033[1m\033[34minfo\033[0m:\t", log.Lmsgprefix)
	VerboseLogger = log.New(os.Stdout, "\033[1m\033[36mverbose\033[0m:\t", log.Ldate|log.Ltime)
	WarningLogger = log.New(os.Stdout, "\033[1m\033[33mwarn\033[0m:\t", log.Ldate|log.Ltime)
	ErrorLogger = log.New(os.Stdout, "\033[1m\033[31merror\033[0m:\t", log.Ldate|log.Ltime)
}

func flagProcessor() []string {
	if *showHelp {
		displayHelp()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Println("v1.0.2")
		os.Exit(0)
	}

	if *inputPath == "" || *outputPath == "" {
		logger(LoggerTypeFatal, "input and output paths are mandatory")
	}

	var fileTypes []string
	if *fileTypesString != "" {
		isValidType := false
		fileTypes = strings.Split(*fileTypesString, ",")

		for i := range fileTypes {
			if isPhoto(strings.ToLower(fileTypes[i])) {
				isValidType = true
				break
			} else if isVideo(strings.ToLower(fileTypes[i])) {
				isValidType = true
				break
			}
		}

		if !isValidType {
			logger(LoggerTypeFatal, "one or more file types supplied are invalid")
		}
	}

	if *geoLocation {
		loadFeatureCollection()
	}

	return fileTypes
}

func directoryExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path %s does not exist", path)
	}
	return nil
}

func validatePaths(inputPath, outputPath string) (string, string) {
	sourcePath := filepath.Clean(inputPath)
	destinationPath := filepath.Clean(outputPath)

	if sourcePath == "" || destinationPath == "" {
		logger(LoggerTypeFatal, "input and output paths must be supplied")
	}

	sourceDrive := filepath.VolumeName(sourcePath)
	destinationDrive := filepath.VolumeName(destinationPath)

	if sourceDrive == "" || destinationDrive == "" {
		logger(LoggerTypeFatal, "input and output paths must be on drives")
	} else if sourceDrive != destinationDrive {
		logger(LoggerTypeFatal, fmt.Sprintf("input and output paths must be on the same drive: source drive (%s), destination drive (%s)", sourceDrive, destinationDrive))
	} else if err := directoryExists(sourcePath); err != nil {
		logger(LoggerTypeFatal, err.Error())
	} else if err := directoryExists(destinationPath); err != nil {
		logger(LoggerTypeFatal, err.Error())
	}

	return sourcePath, destinationPath
}
