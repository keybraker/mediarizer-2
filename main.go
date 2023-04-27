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
	showHelp := flag.Bool("help", false, "displays a usage guide of Mediarizer")
	showVersion := flag.Bool("version", false, "displays current version")
	inputPath := flag.String("input", "", "path to file or directory")
	outputPath := flag.String("output", "", "path to output directory")
	moveUnknown := flag.Bool("unknown", true, "move media that have no metadata to undetermined folder")
	geoLocation := flag.Bool("location", false, "move media according to geo location instead of date")
	fileTypesString := flag.String("types", "", "organises only given file type/s (.jpg, .png, .gif,.mp4, .avi, .mov, .mkv)")
	organisePhotos := flag.Bool("photo", true, "organises only photos")
	organiseVideos := flag.Bool("video", true, "organises only videos")
	format := flag.String("format", "word", "specifies the naming format for month folders (word, number, combined)")

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

	queue := make(chan FileInfo)
	done := make(chan struct{})

	go creator(sourcePath, queue, *geoLocation, *moveUnknown, fileTypes, *organisePhotos, *organiseVideos)
	go consumer(destinationPath, queue, *geoLocation, *format, done)

	<-done
}

func displayHelp() {
	fmt.Println("Usage: mediarizer [flags]")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}
