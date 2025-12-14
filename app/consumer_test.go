package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateUniquePathName_AppendsCounter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.jpg")
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	p2, err := generateUniquePathName(p)
	if err != nil {
		t.Fatalf("generateUniquePathName: %v", err)
	}
	if p2 == p {
		t.Fatalf("expected different path")
	}
	if filepath.Ext(p2) != ".jpg" {
		t.Fatalf("expected same extension")
	}
}
