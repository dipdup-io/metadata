package service

import "time"

// ServiceOption -
type ServiceOption func(*Service)

// WithTickerDuration -
func WithTickerDuration(d time.Duration) ServiceOption {
	return func(s *Service) {
		s.tickerDuration = d
	}
}

// WithName -
func WithName(name string) ServiceOption {
	return func(s *Service) {
		s.name = name
	}
}
