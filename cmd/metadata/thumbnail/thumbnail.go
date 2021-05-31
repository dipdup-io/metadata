package thumbnail

import (
	_ "image/gif"
	_ "image/jpeg"

	"github.com/pkg/errors"
)

var errThumbnailCreating = errors.New("Can't create thumbnail")

// Metadata -
type Metadata struct {
	Formats []Format `json:"formats,omitempty"`
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
	maxFileSize   = 52428800 // 50 MB
	thumbnailSize = 100
)

var validMimes = map[string]struct{}{
	MimeTypePNG:  {},
	MimeTypeJPEG: {},
	MimeTypeGIF:  {},
}
