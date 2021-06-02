package thumbnail

import "github.com/pkg/errors"

// errors
var (
	ErrInvalidThumbnailLink = errors.New("invalid thumbnail link")
	ErrThumbnailCreating    = errors.New("can't create thumbnail")
)
