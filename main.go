package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	inputPath := flag.String("input", "", "Path to source file or directory")
	outputPath := flag.String("output", "", "Path to destination directory")
	duplicateStrategy := flag.String("duplicate", "move", "Duplication handling, default \"move\" (move, skip, delete)")

	moveUnknown := flag.Bool("unknown", true, "Move files with no metadata to undetermined folder")
	geoLocation := flag.Bool("location", false, "Organize files based on their geo location")
	fileTypesString := flag.String("types", "", "Comma separated file extensions to organize (.jpg, .png, .gif, .mp4, .avi, .mov, .mkv)")

	organisePhotos := flag.Bool("photo", true, "Organise only photos")
	organiseVideos := flag.Bool("video", true, "Organise only videos")

	format := flag.String("format", "word", "Naming format for month folders, default \"word\" (word, number, combined)")

	showHelp := flag.Bool("help", false, "Display usage guide")
	verbose := flag.Bool("verbose", true, "Display progress information in console")
	showVersion := flag.Bool("version", false, "Display version information")

	flag.Parse()

	if *showHelp {
		displayHelp()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Println("Mediarizer 2 version 1.0.0")
		os.Exit(0)
	}

	if *inputPath == "" || *outputPath == "" {
		log.Fatal("error: input and output paths are mandatory")
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
			log.Fatal("error: one or more file types supplied are invalid")
		}
	}

	if *geoLocation {
		loadFeatureCollection()
	}

	sourcePath := filepath.Clean(*inputPath)
	destinationPath := filepath.Clean(*outputPath)

	sourceDrive := filepath.VolumeName(sourcePath)
	destDrive := filepath.VolumeName(destinationPath)

	if sourceDrive != "" && destDrive != "" && sourceDrive != destDrive {
		log.Fatal("error: input and output paths must be on the same disk drive")
	}

	totalFiles := 0
	if *verbose {
		totalFiles = countFiles(sourcePath, fileTypes, *organisePhotos, *organiseVideos)
	}

	fileInfoQueue := make(chan FileInfo)
	done := make(chan struct{})

	go creator(sourcePath, fileInfoQueue, *geoLocation, *moveUnknown, fileTypes, *organisePhotos, *organiseVideos)
	go consumer(destinationPath, fileInfoQueue, *geoLocation, *format, *verbose, totalFiles, *duplicateStrategy, done)

	<-done
}

func displayHelp() {
	fmt.Println("Usage: mediarizer [flags]")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

func countFiles(rootPath string, fileTypes []string, organisePhotos bool, organiseVideos bool) int {
	count := 0
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))

			if organisePhotos && isPhoto(ext) && (len(fileTypes) == 0 || contains(fileTypes, ext)) {
				count++
			} else if organiseVideos && isVideo(ext) && (len(fileTypes) == 0 || contains(fileTypes, ext)) {
				count++
			}
		}

		return nil
	})

	return count
}
