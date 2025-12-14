package duplicate

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/keybraker/mediarizer-2/hash"
)

func TestIsDuplicate_SecondOccurrenceIsDuplicate(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jpg")
	if err := os.WriteFile(p, []byte("same"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fileHashMap := &sync.Map{}
	hashCache := &sync.Map{}

	dup, err := IsDuplicate(p, "move", fileHashMap, hashCache)
	if err != nil {
		t.Fatalf("IsDuplicate #1: %v", err)
	}
	if dup {
		t.Fatalf("first occurrence should not be duplicate")
	}

	dup, err = IsDuplicate(p, "move", fileHashMap, hashCache)
	if err != nil {
		t.Fatalf("IsDuplicate #2: %v", err)
	}
	if !dup {
		t.Fatalf("second occurrence should be duplicate")
	}

	// Ensure hash was cached.
	if _, ok := hashCache.Load(p); !ok {
		t.Fatalf("expected hash cache entry")
	}

	// Sanity: cache entry type.
	if v, ok := hashCache.Load(p); ok {
		if _, ok := v.(hash.CachedFile); !ok {
			t.Fatalf("expected CachedFile type")
		}
	}
}

func TestCreateDuplicateFolder_CreatesFolder(t *testing.T) {
	dir := t.TempDir()
	fakeDest := filepath.Join(dir, "a.jpg")

	dupFolder, err := CreateDuplicateFolder(fakeDest, "DUPLICATE")
	if err != nil {
		t.Fatalf("CreateDuplicateFolder: %v", err)
	}

	if filepath.Base(dupFolder) != "DUPLICATE" {
		t.Fatalf("expected DUPLICATE folder name, got %s", filepath.Base(dupFolder))
	}
	if st, err := os.Stat(dupFolder); err != nil || !st.IsDir() {
		t.Fatalf("expected folder to exist")
	}
}
