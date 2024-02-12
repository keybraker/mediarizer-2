package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	verbose = flag.Bool("verbose", true, "Display progress information in console")
	showVersion = flag.Bool("version", false, "Display version information")

	InfoLogger = log.New(os.Stdout, "", log.Lmsgprefix)
	VerboseLogger = log.New(os.Stdout, "VERBOSE: ", log.Ldate|log.Ltime)
	WarningLogger = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime)
	ErrorLogger = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime)
}

func flagProcessor() []string {
	if *showHelp {
		displayHelp()
		os.Exit(0)
	}

	if *showVersion {
		InfoLogger.Println("v1.0.0")
		os.Exit(0)
	}

	if *inputPath == "" || *outputPath == "" {
		ErrorLogger.Fatal("input and output paths are mandatory")
	}

	var fileTypes []string
	if *fileTypesString != "" {
		isValidType := false
		fileTypes = strings.Split(*fileTypesString, ",")

		for i := range fileTypes {
			if isPhoto(strings.ToLower(fileTypes[i])) {
				isValidType = true
				break
			}
			if isVideo(strings.ToLower(fileTypes[i])) {
				isValidType = true
				break
			}
		}

		if !isValidType {
			ErrorLogger.Fatal("one or more file types supplied are invalid")
		}
	}

	if *geoLocation {
		loadFeatureCollection()
	}

	return fileTypes
}

func main() {
	flag.Parse()
	fileTypes := flagProcessor()

	sourcePath := filepath.Clean(*inputPath)
	destinationPath := filepath.Clean(*outputPath)

	sourceDrive := filepath.VolumeName(sourcePath)
	destinationDrive := filepath.VolumeName(destinationPath)

	if sourceDrive != "" && destinationDrive != "" && sourceDrive != destinationDrive {
		ErrorLogger.Fatal("input and output paths must be on the same disk drive")
	}

	totalFiles := 0
	if *verbose {
		totalFiles = countFiles(sourcePath, fileTypes, *organisePhotos, *organiseVideos)
	}

	// fileHashMap := make(map[string][]string)

	fileQueue := make(chan FileInfo, 100)
	logQueue := make(chan string, 100)
	defer close(logQueue)
	errorQueue := make(chan error, 100)
	defer close(errorQueue)

	done := make(chan struct{})

	go errorHandler(errorQueue)

	go creator(sourcePath, fileQueue, errorQueue, *geoLocation, *moveUnknown, fileTypes, *organisePhotos, *organiseVideos)
	go consumer(destinationPath, fileQueue, errorQueue, logQueue, *geoLocation, *format, *verbose, totalFiles, *duplicateStrategy, done)

	<-done

	InfoLogger.Println("Processed " + strconv.Itoa(totalFiles) + " files")

	// if *duplicateStrategy != "skip" {
	// 	processDuplicates(destinationPath, *duplicateStrategy, *verbose, fileHashMap, errorQueue)
	// }
}

func errorHandler(errorQueue chan error) {
	for err := range errorQueue {
		ErrorLogger.Printf("%v\n", err)
	}
}

func displayHelp() {
	InfoLogger.Println("Mediarizer 2 Flags:")
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

			if organisePhotos && isPhoto(ext) && (len(fileTypes) == 0 || arrayContains(fileTypes, ext)) {
				count++
			} else if organiseVideos && isVideo(ext) && (len(fileTypes) == 0 || arrayContains(fileTypes, ext)) {
				count++
			}
		}

		return nil
	})

	return count
}
