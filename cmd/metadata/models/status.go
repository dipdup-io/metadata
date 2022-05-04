package models

// Status - metadata status
type Status int8

const (
	StatusNew Status = iota + 1
	StatusFailed
	StatusApplied
)

// String -
func (s Status) String() string {
	switch s {
	case StatusApplied:
		return "applied"
	case StatusFailed:
		return "failed"
	case StatusNew:
		return "new"
	default:
		return "unknown"
	}
}
