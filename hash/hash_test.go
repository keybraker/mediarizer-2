package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGetFileHash_CachesAndInvalidates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.jpg")

	if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cache := &sync.Map{}

	h1, err := GetFileHash(p, cache)
	if err != nil {
		t.Fatalf("GetFileHash #1: %v", err)
	}
	h2, err := GetFileHash(p, cache)
	if err != nil {
		t.Fatalf("GetFileHash #2: %v", err)
	}
	if hex.EncodeToString(h1) != hex.EncodeToString(h2) {
		t.Fatalf("expected cached hash to match")
	}

	if err := os.WriteFile(p, []byte("hello2"), 0644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	// Ensure modtime differs on coarse filesystems.
	now := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(p, now, now)

	h3, err := GetFileHash(p, cache)
	if err != nil {
		t.Fatalf("GetFileHash #3: %v", err)
	}
	if hex.EncodeToString(h1) == hex.EncodeToString(h3) {
		t.Fatalf("expected hash to change after modification")
	}
}

func TestSaveLoadHashCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")

	filePath := filepath.Join(dir, "b.jpg")
	if err := os.WriteFile(filePath, []byte("abc"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	h := sha256.Sum256([]byte("abc"))

	cache := &sync.Map{}
	cache.Store(filePath, CachedFile{FileMeta: FileMeta{Size: info.Size(), ModTime: info.ModTime()}, Hash: h[:]})

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	cache.Store("dir:"+dir, DirectoryHash{LastScanned: time.Now(), ModTime: dirInfo.ModTime(), FileCount: 1, Files: []string{filepath.Base(filePath)}})

	if err := SaveHashCache(cache, cachePath); err != nil {
		t.Fatalf("SaveHashCache: %v", err)
	}
	if _, err := os.Stat(cachePath + ".tmp"); err == nil {
		t.Fatalf("expected temp file to be removed")
	}

	loaded, err := LoadHashCache(cachePath)
	if err != nil {
		t.Fatalf("LoadHashCache: %v", err)
	}

	v, ok := loaded.Load(filePath)
	if !ok {
		t.Fatalf("expected file entry")
	}
	cf := v.(CachedFile)
	if cf.Size != info.Size() {
		t.Fatalf("size mismatch")
	}
	if !cf.ModTime.Equal(info.ModTime()) {
		t.Fatalf("modtime mismatch")
	}
	if hex.EncodeToString(cf.Hash) != hex.EncodeToString(h[:]) {
		t.Fatalf("hash mismatch")
	}

	v2, ok := loaded.Load("dir:" + dir)
	if !ok {
		t.Fatalf("expected dir entry")
	}
	dh := v2.(DirectoryHash)
	if dh.FileCount != 1 || len(dh.Files) != 1 {
		t.Fatalf("expected directory metadata")
	}
}

func TestHashImagesInPath_UsesDirectoryCacheFastPath(t *testing.T) {
	// HashImagesInPath writes DefaultCacheFilePath in the current working directory.
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	_ = os.Chdir(t.TempDir())

	root := t.TempDir()
	p := filepath.Join(root, "c.jpg")
	if err := os.WriteFile(p, []byte("payload"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cache := &sync.Map{}
	hashBytes, err := GetFileHash(p, cache)
	if err != nil {
		t.Fatalf("GetFileHash: %v", err)
	}

	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	cache.Store("dir:"+root, DirectoryHash{LastScanned: time.Now(), ModTime: rootInfo.ModTime(), FileCount: 1, Files: []string{filepath.Base(p)}})

	var hashed int64
	m, err := HashImagesInPath(root, cache, &hashed)
	if err != nil {
		t.Fatalf("HashImagesInPath: %v", err)
	}
	if hashed != 1 {
		t.Fatalf("expected hashedFiles=1, got %d", hashed)
	}

	exists := false
	hashStr := hex.EncodeToString(hashBytes)
	m.Range(func(k, v any) bool {
		if ks, ok := k.(string); ok && ks == hashStr {
			exists = true
			return false
		}
		return true
	})
	if !exists {
		t.Fatalf("expected returned map to contain hash")
	}
}
