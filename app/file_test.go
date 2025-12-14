package main

import "testing"

func TestPhotoAndVideoDetection(t *testing.T) {
	if !isPhoto(".JPG") {
		t.Fatalf("expected .JPG to be photo")
	}
	if !isPhoto(".jpeg") {
		t.Fatalf("expected .jpeg to be photo")
	}
	if isPhoto(".txt") {
		t.Fatalf("expected .txt to not be photo")
	}

	if !isVideo(".mp4") {
		t.Fatalf("expected .mp4 to be video")
	}
	if isVideo(".jpg") {
		t.Fatalf("expected .jpg to not be video")
	}
}
