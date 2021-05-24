package storage

import "io"

// Storage -
type Storage interface {
	Upload(body io.Reader, filename string) error
	Download(filename string) (io.Reader, error)
}
