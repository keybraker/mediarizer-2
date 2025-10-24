package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keybraker/mediarizer-2/duplicate"
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

// ExecutionStep represents a step in the execution process with timing information
type ExecutionStep struct {
	Name     string
	Duration time.Duration
	Order    int
}

func main() {
	l0 := "   __  ___       ___          _                ___ "
	l1 := "  /  |/  /__ ___/ (_)__ _____(_)__ ___ ____   |_  |"
	l2 := " / /|_/ / -_) _  / / _ `/ __/ /_ // -_) __/  / __/ "
	l3 := "/_/  /_/\\__/\\_,_/_/\\_,_/_/ /_//__/\\__/_/    /____/ (v1.0.2)"
	fmt.Println("\n" + l0 + "\n" + l1 + "\n" + l2 + "\n" + l3 + "\n\n\t\t\t\tby Keybraker\n")

	startTotal := time.Now()
	executionSteps := []ExecutionStep{}

	flag.Parse()
	fileTypes := flagProcessor()

	sourcePath, destinationPath := validatePaths(*inputPath, *outputPath)

	fileQueue := make(chan FileInfo, 100)
	infoQueue := make(chan string, 50)
	warnQueue := make(chan string, 10)
	errorQueue := make(chan error, 50)

	var wg sync.WaitGroup

	startLoggerHandlers(&wg, infoQueue, warnQueue, errorQueue)

	stepStart := time.Now()
	logger(LoggerTypeInfo, "Counting files in path.")
	totalFilesToMove := countFiles(sourcePath, fileTypes, *organisePhotos, *organiseVideos)
	stepDuration := time.Since(stepStart)
	executionSteps = append(executionSteps, ExecutionStep{Name: "Count source files", Duration: stepDuration, Order: 1})

	if totalFilesToMove == 0 {
		logger(LoggerTypeInfo, "No files in path, exiting.")
		return
	} else {
		logger(LoggerTypeInfo, fmt.Sprintf("%d files to be processed.", totalFilesToMove))
	}

	stepStart = time.Now()
	hashCache, err := hash.InitHashCache("")
	if err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("Failed to load hash cache: %v. Using empty cache.", err))
		hashCache = &sync.Map{}
	} else {
		logger(LoggerTypeInfo, "Hash cache loaded successfully.")
	}
	stepDuration = time.Since(stepStart)
	executionSteps = append(executionSteps, ExecutionStep{Name: "Load hash cache", Duration: stepDuration, Order: 2})

	stepStart = time.Now()
	var processedFiles int64

	stopSpinner := make(chan bool)
	go spinner(stopSpinner, "Processing:", &processedFiles, totalFilesToMove)

	done := make(chan struct{})

	fileHashMap := &sync.Map{}

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
		&processedFiles,
		done,
	)

	<-done
	stopSpinner <- true
	stepDuration = time.Since(stepStart)
	executionSteps = append(executionSteps, ExecutionStep{Name: "Process and move files", Duration: stepDuration, Order: 4})

	stepStart = time.Now()
	logger(LoggerTypeInfo, "Organizing duplicates in destination path.")

	var processedDuplicateFiles int64
	stopDuplicateSpinner := make(chan bool)
	spinnerDone := make(chan bool)
	go func() {
		spinner(stopDuplicateSpinner, "Organizing:", &processedDuplicateFiles, 0)
		spinnerDone <- true
	}()

	err = organizeDuplicatesInDestination(destinationPath, fileTypes, *organisePhotos, *organiseVideos, *duplicateStrategy, hashCache, &processedDuplicateFiles)

	stopDuplicateSpinner <- true
	<-spinnerDone // Wait for spinner to finish clearing
	if err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("Failed to organize duplicates: %v", err))
	} else {
		logger(LoggerTypeInfo, "Duplicates organized successfully.")
	}
	stepDuration = time.Since(stepStart)
	executionSteps = append(executionSteps, ExecutionStep{Name: "Organize duplicates", Duration: stepDuration, Order: 5})

	stepStart = time.Now()
	if err := hash.SaveHashCache(hashCache, hash.DefaultCacheFilePath); err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("Failed to save hash cache: %v", err))
	} else {
		logger(LoggerTypeInfo, "Hash cache saved successfully.")
	}
	stepDuration = time.Since(stepStart)
	executionSteps = append(executionSteps, ExecutionStep{Name: "Save hash cache", Duration: stepDuration, Order: 6})

	totalElapsed := time.Since(startTotal)
	displayExecutionSummary(totalElapsed, executionSteps, totalFilesToMove)
}

// organizeDuplicatesInDestination scans the destination directory for duplicate files
// and organizes them into DUPLICATE folders according to the duplicateStrategy
func organizeDuplicatesInDestination(destinationPath string, fileTypes []string, organisePhotos bool, organiseVideos bool, duplicateStrategy string, hashCache *sync.Map, processedDuplicateFiles *int64) error {
	fileHashMap := &sync.Map{}
	var hashedFiles int64

	var err error
	fileHashMap, err = hash.HashImagesInPath(destinationPath, hashCache, &hashedFiles)
	if err != nil {
		return fmt.Errorf("failed to hash files in destination: %v", err)
	}

	// Scan for duplicates (spinner shows progress, so skip logging here)
	err = filepath.Walk(destinationPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		if !((organisePhotos && isPhoto(ext)) || (organiseVideos && isVideo(ext))) {
			return nil
		}

		if len(fileTypes) > 0 && !arrayContains(fileTypes, ext) {
			return nil
		}

		// Skip files already in DUPLICATE folders
		if strings.Contains(path, "DUPLICATE") {
			return nil
		}

		atomic.AddInt64(processedDuplicateFiles, 1)

		isDuplicate, err := duplicate.IsDuplicate(path, duplicateStrategy, fileHashMap, hashCache)
		if err != nil {
			return err
		}

		if isDuplicate {
			switch duplicateStrategy {
			case "skip":
				logger(LoggerTypeVerbose, fmt.Sprintf("Skipped duplicate: %s", filepath.Base(path)))
				return nil
			case "delete":
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("failed to delete duplicate file %s: %v", path, err)
				}
				logger(LoggerTypeVerbose, fmt.Sprintf("Deleted duplicate: %s", filepath.Base(path)))
				return nil
			case "move":
				dir := filepath.Dir(path)
				fileName := filepath.Base(path)
				duplicateFolderPath, err := duplicate.CreateDuplicateFolder(filepath.Join(dir, fileName), "DUPLICATE")
				if err != nil {
					return err
				}

				newPath := filepath.Join(duplicateFolderPath, fileName)

				_, err = os.Stat(newPath)
				if !os.IsNotExist(err) {
					newPath, err = generateUniquePathName(newPath)
					if err != nil {
						return err
					}
				}

				if err := os.Rename(path, newPath); err != nil {
					return fmt.Errorf("failed to move duplicate file %s to %s: %v", path, newPath, err)
				}

				logger(LoggerTypeVerbose, fmt.Sprintf("Moved duplicate: %s -> %s", fileName, newPath))
			}
		}

		return nil
	})

	return err
}

func formatElapsedTime(elapsed time.Duration) string {
	seconds := int(elapsed.Seconds())
	minutes := seconds / 60
	seconds = seconds % 60

	if minutes > 0 {
		if minutes == 1 {
			return fmt.Sprintf("%d min and %d secs", minutes, seconds)
		}
		return fmt.Sprintf("%d mins and %d secs", minutes, seconds)
	}

	return fmt.Sprintf("%.2f secs", elapsed.Seconds())
}

func displayExecutionSummary(totalElapsed time.Duration, steps []ExecutionStep, filesProcessed int) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("EXECUTION SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total files processed: %d\n", filesProcessed)
	fmt.Printf("Total execution time: %s\n", formatElapsedTime(totalElapsed))
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-40s | %20s | %14s\n", "Step", "Duration", "Percentage")
	fmt.Println(strings.Repeat("-", 80))

	for _, step := range steps {
		percentage := (float64(step.Duration.Milliseconds()) / float64(totalElapsed.Milliseconds())) * 100
		durationStr := formatElapsedTime(step.Duration)
		fmt.Printf("%-40s | %20s | %13.2f%%\n", step.Name, durationStr, percentage)
	}

	fmt.Println(strings.Repeat("=", 80))
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
			var output string
			if totalFiles > 0 {
				percentage := float64(processed) / float64(totalFiles) * 100
				output = fmt.Sprintf("\r%c | %s: %d/%d (%.2f%%)", spinChars[i], verb, processed, totalFiles, percentage)
			} else {
				output = fmt.Sprintf("\r%c | %s: %d files", spinChars[i], verb, processed)
			}
			fmt.Print(output)
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
