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
	showHelp := flag.Bool("help", false, "Display usage guide")
	showVersion := flag.Bool("version", false, "Display version information")
	inputPath := flag.String("input", "", "Path to source file or directory")
	outputPath := flag.String("output", "", "Path to destination directory")
	moveUnknown := flag.Bool("unknown", true, "Move files with no metadata to undetermined folder")
	geoLocation := flag.Bool("location", false, "Organize files based on their geo location")
	fileTypesString := flag.String("types", "", "Comma separated file extensions to organize (.jpg, .png, .gif, .mp4, .avi, .mov, .mkv)")
	organisePhotos := flag.Bool("photo", true, "Organise only photos")
	organiseVideos := flag.Bool("video", true, "Organise only videos")
	format := flag.String("format", "word", "Naming format for month folders (word, number, combined)")
	verbose := flag.Bool("verbose", false, "Display progress information in console")

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

	fileInfoQueue := make(chan FileInfo)
	done := make(chan struct{})

	go creator(sourcePath, fileInfoQueue, *geoLocation, *moveUnknown, fileTypes, *organisePhotos, *organiseVideos)
	go consumer(destinationPath, fileInfoQueue, *geoLocation, *format, *verbose, done)

	<-done
}

func displayHelp() {
	fmt.Println("Usage: mediarizer [flags]")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}
