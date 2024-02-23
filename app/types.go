package main

import "time"

type FileType int

const (
	Unknown FileType = iota
	Image
	Video
)

type FileInfo struct {
	Path            string
	FileType        FileType
	Created         time.Time
	Country         string
	HasCreationDate bool
	isDuplicate     bool
}

const (
	FileTypeUnknown FileType = iota
	FileTypeFolder
	FileTypeImage
	FileTypeVideo
	FileTypeExcluded
)

type PhotoType int

const (
	JPG PhotoType = iota
	JPEG
	PNG
	GIF
)

type VideoType int

const (
	MP4 VideoType = iota
	AVI
	MOV
	MKV
)

type FeatureCollection struct {
	Type     string     `json:"type"`
	Features []*Feature `json:"features"`
}

type Feature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   *Geometry              `json:"geometry"`
}

type Geometry struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}
