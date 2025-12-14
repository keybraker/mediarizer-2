package duplicate

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDuplicateChecker_CheckAndTrack(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a file in destination
	destFile := filepath.Join(destDir, "existing.jpg")
	if err := os.WriteFile(destFile, []byte("content"), 0644); err != nil {
		t.Fatalf("write dest file: %v", err)
	}

	// Create a source file with same content
	srcFile := filepath.Join(dir, "source.jpg")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatalf("write src file: %v", err)
	}

	// Create a source file with different content
	uniqueFile := filepath.Join(dir, "unique.jpg")
	if err := os.WriteFile(uniqueFile, []byte("unique"), 0644); err != nil {
		t.Fatalf("write unique file: %v", err)
	}

	hashCache := &sync.Map{}
	dc := NewDuplicateChecker(destDir, hashCache, false)

	var progress int64
	if err := dc.Initialize(&progress); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Check duplicate
	isDup, origPath, err := dc.CheckAndTrack(srcFile)
	if err != nil {
		t.Fatalf("CheckAndTrack duplicate: %v", err)
	}
	if !isDup {
		t.Fatalf("expected duplicate to be detected")
	}
	if origPath != destFile {
		t.Fatalf("expected original path %s, got %s", destFile, origPath)
	}

	// Check unique
	isDup, _, err = dc.CheckAndTrack(uniqueFile)
	if err != nil {
		t.Fatalf("CheckAndTrack unique: %v", err)
	}
	if isDup {
		t.Fatalf("expected unique file not to be duplicate")
	}

	// Track the unique file (simulating move)
	// We need to move it first because TrackNewFile expects file to exist at path?
	// Actually TrackNewFile just hashes the file at path and adds to index.
	// In real usage, we move then track.
	if err := dc.TrackNewFile(uniqueFile); err != nil {
		t.Fatalf("TrackNewFile: %v", err)
	}

	// Check unique again (should now be duplicate)
	isDup, _, err = dc.CheckAndTrack(uniqueFile)
	if err != nil {
		t.Fatalf("CheckAndTrack unique 2nd time: %v", err)
	}
	if !isDup {
		t.Fatalf("expected tracked file to be detected as duplicate")
	}
}

func TestMoveDuplicate(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "dup.jpg")
	if err := os.WriteFile(srcFile, []byte("dup"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	duplicatesDir := filepath.Join(dir, "duplicates")
	// duplicatesDir doesn't exist yet

	err := MoveDuplicate(srcFile, "original/path/ignored", duplicatesDir)
	if err != nil {
		t.Fatalf("MoveDuplicate: %v", err)
	}

	destFile := filepath.Join(duplicatesDir, "dup.jpg")
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Fatalf("expected file to be moved to %s", destFile)
	}

	// Test collision handling
	srcFile2 := filepath.Join(dir, "dup.jpg") // Recreate source
	if err := os.WriteFile(srcFile2, []byte("dup2"), 0644); err != nil {
		t.Fatalf("write src2: %v", err)
	}

	err = MoveDuplicate(srcFile2, "original/path/ignored", duplicatesDir)
	if err != nil {
		t.Fatalf("MoveDuplicate 2: %v", err)
	}

	destFile2 := filepath.Join(duplicatesDir, "dup_1.jpg")
	if _, err := os.Stat(destFile2); os.IsNotExist(err) {
		t.Fatalf("expected file to be renamed to %s", destFile2)
	}
}
