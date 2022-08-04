package models

// ModelRepository -
type ModelRepository[T Model] interface {
	Get(network string, status Status, limit, offset, retryCount, delay int) ([]T, error)
	Update(metadata []T) error
	Save(metadata []T) error
	LastUpdateID() (int64, error)
	CountByStatus(network string, status Status) (int, error)
}

// Model -
type Model interface {
	GetStatus() Status
	GetRetryCount() int8
	GetID() uint64
	GetLink() string
	TableName() string
	IncrementRetryCount()
	SetMetadata(data JSONB)
	SetStatus(status Status)
}
