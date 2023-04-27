package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

var featureCollection FeatureCollection

func creator(
	sourcePath string,
	queue chan<- FileInfo,
	geoLocation bool,
	moveUnknown bool,
	fileTypesToInclude []string,
	organisePhotos bool,
	organiseVideos bool,
) {
	filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if !d.Type().IsRegular() {
			return nil // skip directories and other non-regular files
		}

		fileType := getFileType(path, fileTypesToInclude, organisePhotos, organiseVideos)

		if fileType == Unknown {
			if moveUnknown {
				queue <- FileInfo{Path: path, FileType: Unknown}
			}
			return nil
		}

		if geoLocation {
			country, err := getCountry(path)
			if err != nil {
				fmt.Println(err)
				return nil
			}

			if fileType != FileTypeExcluded {
				queue <- FileInfo{Path: path, FileType: fileType, Country: country}
			}
		} else {
			createdDate, hasCreationDate, err := getCreatedTime(path)
			if err != nil {
				fmt.Println(err)
				return nil
			}

			if fileType != FileTypeExcluded {
				queue <- FileInfo{Path: path, FileType: fileType, Created: createdDate, HasCreationDate: hasCreationDate}
			}
		}

		return nil
	})
	close(queue)
}

func getFileType(path string, fileTypesToInclude []string, organisePhotos bool, organiseVideos bool) FileType {
	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
		return FileTypeUnknown
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Println(err)
		return FileTypeUnknown
	}

	if fileInfo.IsDir() {
		return FileTypeFolder
	}

	fileType := FileTypeUnknown
	if fileTypesToInclude != nil {
		fileType = FileTypeExcluded
	}

	extension := strings.ToLower(filepath.Ext(path))

	if fileTypesToInclude != nil && !isStringInArray(extension, fileTypesToInclude) {
		fileType = FileTypeExcluded
	} else if organisePhotos && isPhoto(extension) {
		fileType = FileTypeImage
	} else if organiseVideos && isVideo(extension) {
		fileType = FileTypeVideo
	} else if fileTypesToInclude == nil && (organisePhotos || organiseVideos) {
		fileType = FileTypeUnknown
	}

	return fileType
}

func isStringInArray(str string, arr []string) bool {
	for _, val := range arr {
		if val == str {
			return true
		}
	}
	return false
}

func getExifData(path string) (exif.Exif, error) {
	file, err := os.Open(path)
	if err != nil {
		return exif.Exif{}, fmt.Errorf("failed to open file %v: %v", path, err)
	}
	defer file.Close()

	exifData, err := exif.Decode(file)
	if err != nil {
		return exif.Exif{}, fmt.Errorf("failed to decode file %v: %v", path, err)
	}

	return *exifData, nil
}

func getCreatedTime(path string) (time.Time, bool, error) {
	exifData, err := getExifData(path)
	if err == nil {
		dateTime, err := exifData.DateTime()
		if err == nil {
			return dateTime, true, nil
		}
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("failed to get file info: %e", err)
	}

	return fileInfo.ModTime(), false, nil
}

func getCountry(path string) (string, error) {
	exifData, err := getExifData(path)
	if err != nil {
		return "", err
	}

	lat, lon, err := exifData.LatLong()
	if err != nil {
		return "", fmt.Errorf("exif data does not have lat lon")
	}

	for _, feature := range featureCollection.Features {
		if feature.Geometry != nil && feature.Geometry.Type == "Polygon" {
			coords := feature.Geometry.Coordinates[0]
			if pointInPolygon(lon, lat, coords) {
				return feature.Properties["name"].(string), nil
			}
		}
	}

	return "", fmt.Errorf("no matching country found for coordinates")
}

func pointInPolygon(x, y float64, polyCoords [][]float64) bool {
	inside := false
	for i := 0; i < len(polyCoords); i++ {
		j := len(polyCoords) - 1
		if (polyCoords[i][1] > y) != (polyCoords[j][1] > y) &&
			(x < (polyCoords[j][0]-polyCoords[i][0])*(y-polyCoords[i][1])/(polyCoords[j][1]-polyCoords[i][1])+polyCoords[i][0]) {
			inside = !inside
		}
	}
	return inside
}
