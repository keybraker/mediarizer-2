package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// Define flags
	inputPath := flag.String("input", "", "path to file or directory")
	outputPath := flag.String("output", "", "path to output directory")
	fileType := flag.String("type", "", "organizes only given file type/s")
	organizePhotos := flag.Bool("photo", false, "organizes only photos")
	organizeVideos := flag.Bool("video", false, "organizes only videos")
	moveUnknown := flag.Bool("unknown", false, "move photos that have no metadata to undetermined folder")
	showHelp := flag.Bool("help", false, "displays a usage guide of Mediarizer")
	showVersion := flag.Bool("version", false, "displays current version")

	// Parse flags
	flag.Parse()

	if *showHelp {
		displayHelp()
		os.Exit(0)
	}
	if *showVersion {
		fmt.Println("Mediarizer 2 version 1.0.0")
		os.Exit(0)
	}

	// Check mandatory flags
	if *inputPath == "" || *outputPath == "" {
		log.Fatal("error: input and output paths are mandatory")
	}

	// Validate flags
	if (*organizePhotos || *organizeVideos) && *fileType != "" {
		log.Fatal("error: cannot use both -photo/-video and -type flags")
	}

	// Process input and output paths
	sourcePath := filepath.Clean(*inputPath)
	destinationPath := filepath.Clean(*outputPath)

	// create unknown directory in destination path
	if err := os.MkdirAll(filepath.Join(destinationPath, "unknown"), 0755); err != nil {
		log.Fatalf("error creating unknown directory: %v", err)
	}

	queue := make(chan FileInfo)
	done := make(chan struct{})

	go creator(sourcePath, queue, *moveUnknown)
	go consumer(destinationPath, queue, done)

	<-done
}

func displayHelp() {
	fmt.Println("Usage: mediarizer [flags]")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}
