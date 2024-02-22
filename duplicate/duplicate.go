package duplicate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/keybraker/mediarizer-2/hash"
)

// createDuplicateFolder creates a folder for storing duplicates of the file.
func CreateDuplicateFolder(destinationPath, duplicateFileName string) (string, error) {
	ext := filepath.Ext(duplicateFileName)
	nameWithoutExt := strings.TrimSuffix(duplicateFileName, ext)
	underscoreExt := strings.ReplaceAll(ext, ".", "_")
	duplicatesFolder := filepath.Join(filepath.Dir(destinationPath), fmt.Sprintf("%s%s", nameWithoutExt, underscoreExt))

	err := os.MkdirAll(duplicatesFolder, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create duplicates folder: %v", err)
	}

	return duplicatesFolder, nil
}

// isDuplicate checks if the file is a duplicate and handles it based on the strategy.
func IsDuplicate(path string, duplicateStrategy string, fileHashMap map[string]bool, hashCache *sync.Map) (bool, error) {
	fileHash, err := hash.GetFileHash(path, hashCache)
	if err != nil {
		return false, err
	}

	hashStr := fmt.Sprintf("%x", fileHash)

	if _, exists := fileHashMap[hashStr]; exists {
		return true, nil
	}

	fileHashMap[hashStr] = true
	return false, nil
}
