package models

// Status - metadata status
type Status int

const (
	StatusNew Status = iota + 1
	StatusFailed
	StatusApplied
)

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
