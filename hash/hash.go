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

type DirectoryHash struct {
	LastScanned time.Time `json:"last_scanned"`
	ModTime     time.Time `json:"mod_time"`
	FileCount   int       `json:"file_count"`
	Files       []string  `json:"files"`
}

type hashCacheFile struct {
	Files       map[string]serializedCachedFile `json:"files"`
	Directories map[string]DirectoryHash        `json:"directories"`
}

type serializedCachedFile struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Hash    string    `json:"hash"`
}

const DefaultCacheFilePath = "hash_cache.json"

var (
	supportedImageExts = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".tiff": true, ".dng": true, ".nef": true,
	}
	supportedVideoExts = map[string]bool{
		".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
	}
	skippableDirs = map[string]bool{
		"videos": true, "unknown": true, ".git": true,
		".cache": true, "node_modules": true, "duplicate": true,
	}
)

// IsSupportedMediaFile checks if the file is a supported media type
func IsSupportedMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return supportedImageExts[ext] || supportedVideoExts[ext]
}

// IsImageFile checks if the file is an image
func IsImageFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return supportedImageExts[ext]
}

// IsVideoFile checks if the file is a video
func IsVideoFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return supportedVideoExts[ext]
}

// IsSkippableDirectory checks if directory should be skipped
func IsSkippableDirectory(dirName string) bool {
	return skippableDirs[strings.ToLower(dirName)]
}

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

// calculateFileHash calculates the SHA-256 hash of the file
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

	reader := &readerAtWrapper{
		readerAt: readerAt,
		offset:   0,
		size:     fileInfo.Size(),
	}

	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return nil, fmt.Errorf("failed to calculate hash for file %s: %v", filePath, err)
	}

	return h.Sum(nil), nil
}

// GetFileHash retrieves or calculates the hash of the file
func GetFileHash(filePath string, hashCache *sync.Map) ([]byte, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	meta := FileMeta{Size: info.Size(), ModTime: info.ModTime()}

	if cached, found := hashCache.Load(filePath); found {
		if cachedFile, ok := cached.(CachedFile); ok {
			if cachedFile.Size == meta.Size && cachedFile.ModTime.Equal(meta.ModTime) {
				return cachedFile.Hash, nil
			}
		}
	}

	hashValue, err := calculateFileHash(filePath)
	if err != nil {
		return nil, err
	}

	hashCache.Store(filePath, CachedFile{FileMeta: meta, Hash: hashValue})
	return hashValue, nil
}

// GetFileHashString returns the hex-encoded hash string
func GetFileHashString(filePath string, hashCache *sync.Map) (string, error) {
	h, err := GetFileHash(filePath, hashCache)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h), nil
}

// InitHashCache initializes the hash cache from file
func InitHashCache(cachePath string) (*sync.Map, error) {
	if cachePath == "" {
		cachePath = DefaultCacheFilePath
	}
	return LoadHashCache(cachePath)
}

// LoadHashCache loads the hash cache from JSON file
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
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		return hashCache, fmt.Errorf("failed to unmarshal hash cache: %v", err)
	}

	for filePath, serialized := range cacheFile.Files {
		hashBytes, err := hex.DecodeString(serialized.Hash)
		if err != nil {
			continue
		}
		hashCache.Store(filePath, CachedFile{
			FileMeta: FileMeta{Size: serialized.Size, ModTime: serialized.ModTime},
			Hash:     hashBytes,
		})
	}

	for dirPath, dirHash := range cacheFile.Directories {
		hashCache.Store("dir:"+dirPath, dirHash)
	}

	return hashCache, nil
}

// SaveHashCache saves the hash cache to JSON file
func SaveHashCache(hashCache *sync.Map, cachePath string) error {
	cacheFile := hashCacheFile{
		Files:       make(map[string]serializedCachedFile),
		Directories: make(map[string]DirectoryHash),
	}

	hashCache.Range(func(key, value interface{}) bool {
		strKey, ok := key.(string)
		if !ok {
			return true
		}

		if strings.HasPrefix(strKey, "dir:") {
			if dirHash, ok := value.(DirectoryHash); ok {
				cacheFile.Directories[strings.TrimPrefix(strKey, "dir:")] = dirHash
			}
			return true
		}

		if cachedFile, ok := value.(CachedFile); ok {
			cacheFile.Files[strKey] = serializedCachedFile{
				Size:    cachedFile.Size,
				ModTime: cachedFile.ModTime,
				Hash:    hex.EncodeToString(cachedFile.Hash),
			}
		}
		return true
	})

	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hash cache: %v", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write hash cache temp file: %v", err)
	}

	_ = os.Remove(cachePath)
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return fmt.Errorf("failed to replace hash cache file: %v", err)
	}

	return nil
}

// HashFilesInPath hashes files incrementally using workers
func HashFilesInPath(path string, hashCache *sync.Map, hashedFiles *int64, fileFilter func(string) bool) (*sync.Map, error) {
	fileHashMap := &sync.Map{}
	fileChan := make(chan string, 500)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU() * 2
	if numWorkers < 4 {
		numWorkers = 4
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				if fileFilter != nil && !fileFilter(filePath) {
					continue
				}
				if !IsSupportedMediaFile(filePath) {
					continue
				}

				hashStr, err := GetFileHashString(filePath, hashCache)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("failed to hash %s: %v", filePath, err):
					default:
					}
					return
				}

				fileHashMap.Store(hashStr, filePath)
				atomic.AddInt64(hashedFiles, 1)
			}
		}()
	}

	// Walk directory
	go func() {
		defer close(fileChan)
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if IsSkippableDirectory(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type().IsRegular() {
				fileChan <- p
			}
			return nil
		})
	}()

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	return fileHashMap, nil
}

// BuildDestinationHashIndex builds a hash->path index for destination files
// This is optimized to use cache aggressively and skip already-indexed directories
func BuildDestinationHashIndex(destPath string, hashCache *sync.Map, progress *int64) (*sync.Map, error) {
	return HashFilesInPath(destPath, hashCache, progress, nil)
}
