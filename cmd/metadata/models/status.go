package models

// Status - metadata status
type Status int

const (
	StatusNew Status = iota + 1
	StatusFailed
	StatusApplied
)
