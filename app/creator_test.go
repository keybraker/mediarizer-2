package main

import "testing"

func TestGetFileType_ExtensionOnly(t *testing.T) {
	if got := getFileType("C:/tmp/a.JPG", nil, true, true); got != FileTypeImage {
		t.Fatalf("expected image, got %v", got)
	}
	if got := getFileType("C:/tmp/a.MP4", nil, true, true); got != FileTypeVideo {
		t.Fatalf("expected video, got %v", got)
	}

	// When types list is provided and does not include extension, excluded.
	if got := getFileType("C:/tmp/a.jpg", []string{".mp4"}, true, true); got != FileTypeExcluded {
		t.Fatalf("expected excluded, got %v", got)
	}

	// Unknown extension becomes Unknown when no explicit types filter.
	if got := getFileType("C:/tmp/a.bin", nil, true, true); got != FileTypeUnknown {
		t.Fatalf("expected unknown, got %v", got)
	}
}
