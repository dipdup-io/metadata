package service

import (
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/prometheus"
)

// ServiceOption -
type ServiceOption[T models.Model] func(*Service[T])

// WithMaxRetryCount -
func WithMaxRetryCount[T models.Model](count int) ServiceOption[T] {
	return func(cs *Service[T]) {
		if count > 0 {
			cs.maxRetryCount = count
		}
	}
}

// WithWorkersCount -
func WithWorkersCount[T models.Model](count int) ServiceOption[T] {
	return func(cs *Service[T]) {
		if count > 0 {
			cs.workersCount = count
		}
	}
}

// WithPrometheus -
func WithPrometheus[T models.Model](prom *prometheus.Prometheus, gaugeType string) ServiceOption[T] {
	return func(cs *Service[T]) {
		cs.prom = prom
		cs.gaugeType = gaugeType
	}
}

// WithIPFSCache -
func WithIPFSCache[T models.Model](ipfsRepo *models.IPFS) ServiceOption[T] {
	return func(cs *Service[T]) {
		cs.ipfsRepo = ipfsRepo
	}
}

// WithDelay -
func WithDelay[T models.Model](delay int) ServiceOption[T] {
	return func(cs *Service[T]) {
		cs.delay = delay
	}
}
