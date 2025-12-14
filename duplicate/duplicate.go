package duplicate

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/keybraker/mediarizer-2/hash"
)

// DuplicateChecker handles duplicate detection logic
type DuplicateChecker struct {
	destIndex *sync.Map // Map[hashString]filePath
	hashCache *sync.Map
	destPath  string
	skipIndex bool
}

// NewDuplicateChecker creates a new duplicate checker
func NewDuplicateChecker(destPath string, hashCache *sync.Map, skipIndex bool) *DuplicateChecker {
	return &DuplicateChecker{
		destIndex: &sync.Map{},
		hashCache: hashCache,
		destPath:  destPath,
		skipIndex: skipIndex,
	}
}

// Initialize builds the initial index of the destination folder
func (dc *DuplicateChecker) Initialize(progress *int64) error {
	if dc.skipIndex {
		return nil
	}

	fmt.Println("Building destination index...")
	index, err := hash.BuildDestinationHashIndex(dc.destPath, dc.hashCache, progress)
	if err != nil {
		return fmt.Errorf("failed to build destination index: %v", err)
	}
	dc.destIndex = index
	return nil
}

// CheckAndTrack checks if a file is a duplicate and tracks it if not
// Returns (isDuplicate, originalPath, error)
func (dc *DuplicateChecker) CheckAndTrack(sourcePath string) (bool, string, error) {
	// Calculate hash of source file
	hashStr, err := hash.GetFileHashString(sourcePath, dc.hashCache)
	if err != nil {
		return false, "", err
	}

	// Check if hash exists in index
	if existingPath, found := dc.destIndex.Load(hashStr); found {
		// Verify the file actually exists at that path (handle stale cache)
		if _, err := os.Stat(existingPath.(string)); err == nil {
			return true, existingPath.(string), nil
		}
		// If file doesn't exist, remove from index and continue
		dc.destIndex.Delete(hashStr)
	}

	// If we're skipping index, we might want to do a lazy check here
	// For now, we just assume it's unique if we skipped indexing
	// But we should still track it for the current session

	return false, "", nil
}

// TrackNewFile adds a newly moved file to the index
func (dc *DuplicateChecker) TrackNewFile(filePath string) error {
	hashStr, err := hash.GetFileHashString(filePath, dc.hashCache)
	if err != nil {
		return err
	}
	dc.destIndex.Store(hashStr, filePath)
	return nil
}

// MoveDuplicate moves a duplicate file to the duplicates folder
func MoveDuplicate(sourcePath, originalPath, duplicatesDir string) error {
	fileName := filepath.Base(sourcePath)
	destPath := filepath.Join(duplicatesDir, fileName)

	// Ensure unique filename in duplicates folder
	ext := filepath.Ext(fileName)
	name := fileName[:len(fileName)-len(ext)]
	counter := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		destPath = filepath.Join(duplicatesDir, fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create duplicates directory: %v", err)
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move duplicate file: %v", err)
	}

	return nil
}
