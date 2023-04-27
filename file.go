package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

func isPhoto(fileExt string) bool {
	fileExToLower := strings.ToLower(fileExt)
	if getPhotoType(fileExToLower) == -1 {
		return false
	} else {
		return true
	}
}

func isVideo(fileExt string) bool {
	fileExToLower := strings.ToLower(fileExt)
	if getVideoType(fileExToLower) == -1 {
		return false
	} else {
		return true
	}
}

func getPhotoType(fileExt string) PhotoType {
	switch fileExt {
	case ".jpg", ".jpeg":
		return JPEG
	case ".png":
		return PNG
	case ".gif":
		return GIF
	default:
		return -1
	}
}

func getVideoType(fileExt string) VideoType {
	switch fileExt {
	case ".mp4":
		return MP4
	case ".avi":
		return AVI
	case ".mov":
		return MOV
	case ".mkv":
		return MKV
	default:
		return -1
	}
}

func loadFeatureCollection() (FeatureCollection, error) {
	file, err := os.Open("countries.json")
	if err != nil {
		return FeatureCollection{}, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return FeatureCollection{}, err
	}

	var featureCollection FeatureCollection
	err = json.Unmarshal(bytes, &featureCollection)
	if err != nil {
		return FeatureCollection{}, err
	}

	return featureCollection, nil
}
