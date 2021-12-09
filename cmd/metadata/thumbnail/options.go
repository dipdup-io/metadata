package thumbnail

import "github.com/dipdup-net/go-lib/prometheus"

// ThumbnailOption -
type ThumbnailOption func(*Service)

// WithPrometheus -
func WithPrometheus(prom *prometheus.Service) ThumbnailOption {
	return func(m *Service) {
		m.prom = prom
	}
}

// WithWorkers -
func WithWorkers(workersCount int) ThumbnailOption {
	return func(m *Service) {
		if workersCount < 1 {
			workersCount = 1
		}
		m.workers = make(chan struct{}, workersCount)
	}
}

// WithFileSizeLimit -
func WithFileSizeLimit(maxFileSize int64) ThumbnailOption {
	return func(m *Service) {
		if maxFileSize < 1 {
			maxFileSize = defaultMaxFileSize
		}
		m.maxFileSizeMB = maxFileSize
	}
}

// WithSize -
func WithSize(thumbnailSize int) ThumbnailOption {
	return func(m *Service) {
		if thumbnailSize < 1 {
			thumbnailSize = defaultThumbnailSize
		}
		m.size = thumbnailSize
	}
}
