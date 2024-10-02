package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/keybraker/mediarizer-2/duplicate"

	"github.com/rwcarlsen/goexif/exif"
)

var featureCollection FeatureCollection

func creator(
	sourcePath string,
	fileQueue chan<- FileInfo,
	warnQueue chan<- string,
	errorQueue chan<- error,
	geoLocation bool,
	moveUnknown bool,
	fileTypesToInclude []string,
	organisePhotos bool,
	organiseVideos bool,
	duplicateStrategy string,
	fileHashMap *sync.Map,
	hashCache *sync.Map,
) {
	filePaths := make(chan string, 100)

	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU() / 2

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range filePaths {
				processFile(
					path,
					fileQueue,
					warnQueue,
					errorQueue,
					geoLocation,
					moveUnknown,
					fileTypesToInclude,
					organisePhotos,
					organiseVideos,
					duplicateStrategy,
					fileHashMap,
					hashCache,
				)
			}
		}()
	}

	go func() {
		err := filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				errorQueue <- err
				return nil
			}
			if d.IsDir() || !os.FileMode(d.Type()).IsRegular() {
				return nil
			}

			filePaths <- path
			return nil
		})
		if err != nil {
			errorQueue <- err
		}
		close(filePaths)
	}()

	wg.Wait()
	close(fileQueue)
}

func processFile(
	path string,
	fileQueue chan<- FileInfo,
	warnQueue chan<- string,
	errorQueue chan<- error,
	geoLocation bool,
	moveUnknown bool,
	fileTypesToInclude []string,
	organisePhotos bool,
	organiseVideos bool,
	duplicateStrategy string,
	fileHashMap *sync.Map,
	hashCache *sync.Map,
) {
	fileType := getFileType(path, fileTypesToInclude, organisePhotos, organiseVideos)

	if fileType == Unknown {
		if moveUnknown {
			fileQueue <- FileInfo{Path: path, FileType: Unknown}
		}
		return
	}

	if fileType == FileTypeExcluded {
		return
	}

	isDuplicate, err := duplicate.IsDuplicate(path, duplicateStrategy, fileHashMap, hashCache)
	if err != nil {
		errorQueue <- err
		return
	}

	if isDuplicate {
		switch duplicateStrategy {
		case "skip":
			fmt.Printf("Skipped duplicate file: %v\n", path)
			logMoveAction(path, "", true, duplicateStrategy)
			return
		case "delete":
			if err := os.Remove(path); err != nil {
				errorQueue <- fmt.Errorf("failed to delete duplicate file: %v", err)
			} else {
				logMoveAction(path, "", true, duplicateStrategy)
			}
			return
		}
	}

	if geoLocation {
		country, err := getCountry(path)
		if err != nil {
			errorQueue <- err
			return
		} else if country == "" {
			warnQueue <- fmt.Sprintf("no country found for file: %v", path)
		}

		fileQueue <- FileInfo{Path: path, FileType: fileType, isDuplicate: isDuplicate, Country: country}
	} else {
		createdDate, hasCreationDate, err := getCreatedTime(path)
		if err != nil {
			errorQueue <- err
			return
		}

		fileQueue <- FileInfo{
			Path:            path,
			FileType:        fileType,
			isDuplicate:     isDuplicate,
			Created:         createdDate,
			HasCreationDate: hasCreationDate,
		}
	}
}

func getFileType(path string, fileTypesToInclude []string, organisePhotos bool, organiseVideos bool) FileType {
	file, err := os.Open(path)
	if err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("failed to open file %v: %v", path, err))
		return FileTypeUnknown
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		logger(LoggerTypeWarning, fmt.Sprintf("failed to get file info: %v", err))
		return FileTypeUnknown
	}

	if fileInfo.IsDir() {
		return FileTypeFolder
	}

	fileType := FileTypeUnknown
	if fileTypesToInclude != nil {
		fileType = FileTypeExcluded
	}

	extension := filepath.Ext(path)

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
	lowerStr := strings.ToLower(str)
	for _, val := range arr {
		if strings.ToLower(val) == lowerStr {
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
		return "", nil // exif data does not have lat lon
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
