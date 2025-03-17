package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/mmap"
)

type FileMeta struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type CachedFile struct {
	FileMeta
	Hash []byte `json:"hash"`
}

type hashCacheFile struct {
	Files map[string]serializedCachedFile `json:"files"`
}

type serializedCachedFile struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Hash    string    `json:"hash"` // Hex string representation
}

const DefaultCacheFilePath = "hash_cache.json"

type readerAtWrapper struct {
	readerAt io.ReaderAt
	offset   int64
	size     int64
}

func (r *readerAtWrapper) Read(p []byte) (n int, err error) {
	if r.offset >= r.size {
		return 0, io.EOF
	}
	n, err = r.readerAt.ReadAt(p, r.offset)
	r.offset += int64(n)
	return n, err
}

// isImageFile checks if the file is an image based on its extension.
func isImageFile(filePath string) bool {
	lowerFilePath := strings.ToLower(filePath)
	return strings.HasSuffix(lowerFilePath, ".jpg") || strings.HasSuffix(lowerFilePath, ".jpeg") ||
		strings.HasSuffix(lowerFilePath, ".png") || strings.HasSuffix(lowerFilePath, ".gif") ||
		strings.HasSuffix(lowerFilePath, ".bmp") || strings.HasSuffix(lowerFilePath, ".tiff")
}

// calculateFileHash calculates the SHA-256 hash of the file at the given filePath.
func calculateFileHash(filePath string) ([]byte, error) {
	readerAt, err := mmap.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to memory-map file %s: %v", filePath, err)
	}
	defer readerAt.Close()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %v", filePath, err)
	}
	fileSize := fileInfo.Size()

	reader := &readerAtWrapper{
		readerAt: readerAt,
		offset:   0,
		size:     fileSize,
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return nil, fmt.Errorf("failed to calculate hash for file %s: %v", filePath, err)
	}

	return hash.Sum(nil), nil
}

// GetFileHash retrieves or calculates the hash of the file at filePath.
func GetFileHash(filePath string, hashCache *sync.Map) ([]byte, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	meta := FileMeta{Size: info.Size(), ModTime: info.ModTime()}

	if cached, found := hashCache.Load(filePath); found {
		cachedFile := cached.(CachedFile)
		if cachedFile.Size == meta.Size && cachedFile.ModTime.Equal(meta.ModTime) {
			return cachedFile.Hash, nil
		}
	}

	hashValue, err := calculateFileHash(filePath)
	if err != nil {
		return nil, err
	}

	cachedFile := CachedFile{
		FileMeta: meta,
		Hash:     hashValue,
	}
	hashCache.Store(filePath, cachedFile)

	return hashValue, nil
}

// LoadHashCache loads the hash cache from the specified JSON file.
func LoadHashCache(cachePath string) (*sync.Map, error) {
	hashCache := &sync.Map{}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return hashCache, nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return hashCache, fmt.Errorf("failed to read hash cache file: %v", err)
	}

	var cacheFile hashCacheFile
	err = json.Unmarshal(data, &cacheFile)
	if err != nil {
		return hashCache, fmt.Errorf("failed to unmarshal hash cache: %v", err)
	}

	for filePath, serialized := range cacheFile.Files {
		hashBytes, err := hex.DecodeString(serialized.Hash)
		if err != nil {
			continue // Skip invalid entries
		}

		cachedFile := CachedFile{
			FileMeta: FileMeta{
				Size:    serialized.Size,
				ModTime: serialized.ModTime,
			},
			Hash: hashBytes,
		}

		hashCache.Store(filePath, cachedFile)
	}

	return hashCache, nil
}

// SaveHashCache saves the hash cache to a JSON file.
func SaveHashCache(hashCache *sync.Map, cachePath string) error {
	cacheFile := hashCacheFile{
		Files: make(map[string]serializedCachedFile),
	}

	hashCache.Range(func(key, value interface{}) bool {
		filePath, ok := key.(string)
		if !ok {
			return true
		}

		cachedFile, ok := value.(CachedFile)
		if !ok {
			return true
		}

		cacheFile.Files[filePath] = serializedCachedFile{
			Size:    cachedFile.Size,
			ModTime: cachedFile.ModTime,
			Hash:    hex.EncodeToString(cachedFile.Hash),
		}
		return true
	})

	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hash cache: %v", err)
	}

	err = os.WriteFile(cachePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write hash cache to file: %v", err)
	}

	return nil
}

// InitHashCache initializes the hash cache, loading from file if it exists.
func InitHashCache(cachePath string) (*sync.Map, error) {
	if cachePath == "" {
		cachePath = DefaultCacheFilePath
	}

	return LoadHashCache(cachePath)
}

// HashImagesInPath hashes all images in the given path and updates the fileHashMap.
func HashImagesInPath(path string, hashCache *sync.Map, hashedFiles *int64) (*sync.Map, error) {
	fileHashMap := &sync.Map{}
	fileChan := make(chan string)
	errChan := make(chan error)
	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU() * 4

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				if isImageFile(filePath) {
					hashValue, err := GetFileHash(filePath, hashCache)
					if err != nil {
						errChan <- fmt.Errorf("failed to get file hash for %s: %v", filePath, err)
						return
					}

					hashStr := hex.EncodeToString(hashValue)
					fileHashMap.Store(hashStr, true)

					atomic.AddInt64(hashedFiles, 1)
				}
			}
		}()
	}

	go func() {
		defer close(fileChan)
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				errChan <- fmt.Errorf("failed to walk path %s: %v", filePath, err)
				return err
			}

			if !info.IsDir() {
				fileChan <- filePath
			}

			return nil
		})

		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	if err := SaveHashCache(hashCache, DefaultCacheFilePath); err != nil {
		fmt.Printf("Warning: Failed to save hash cache: %v\n", err)
	}

	return fileHashMap, nil
}
