package service

import (
	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
)

// ServiceOption -
type ServiceOption[T models.Constraint] func(*Service[T])

// WithMaxRetryCount -
func WithMaxRetryCount[T models.Constraint](count int) ServiceOption[T] {
	return func(cs *Service[T]) {
		if count > 0 {
			cs.maxRetryCount = count
		}
	}
}

// WithWorkersCount -
func WithWorkersCount[T models.Constraint](count int) ServiceOption[T] {
	return func(cs *Service[T]) {
		if count > 0 {
			cs.workersCount = count
		}
	}
}

// WithPrometheus -
func WithPrometheus[T models.Constraint](prom *prometheus.Service, gaugeType string) ServiceOption[T] {
	return func(cs *Service[T]) {
		cs.prom = prom
		cs.gaugeType = gaugeType
	}
}

// WithIPFSCache -
func WithIPFSCache[T models.Constraint](ipfsRepo *models.IPFS) ServiceOption[T] {
	return func(cs *Service[T]) {
		cs.ipfsRepo = ipfsRepo
	}
}

// WithDelay -
func WithDelay[T models.Constraint](delay int) ServiceOption[T] {
	return func(cs *Service[T]) {
		cs.delay = delay
	}
}
