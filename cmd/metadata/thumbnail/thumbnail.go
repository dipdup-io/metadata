package thumbnail

import (
	_ "image/gif"
	_ "image/jpeg"
)

// Metadata -
type Metadata struct {
	Formats      []Format `json:"formats,omitempty"`
	ThumbnailURI string   `json:"thumbnailUri,omitempty"`
}

// Format -
type Format struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
}

// Mime types
const (
	MimeTypePNG  = "image/png"
	MimeTypeJPEG = "image/jpeg"
	MimeTypeGIF  = "image/gif"
)

const (
	defaultMaxFileSize   = 50
	defaultThumbnailSize = 100
)

var validMimes = map[string]struct{}{
	MimeTypePNG:  {},
	MimeTypeJPEG: {},
	MimeTypeGIF:  {},
}
