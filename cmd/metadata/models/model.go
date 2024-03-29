package models

import "time"

// ModelRepository -
type ModelRepository[T Model] interface {
	Get(network string, status Status, limit, offset, retryCount, delay int) ([]T, error)
	Update(metadata []T) error
	Save(metadata []T) error
	LastUpdateID() (int64, error)
	CountByStatus(network string, status Status) (int, error)
	Retry(network string, retryCount int, window time.Duration) error
}

// Model -
type Model interface {
	GetStatus() Status
	GetID() uint64
	TableName() string
}
