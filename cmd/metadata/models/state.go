package models

import "github.com/dipdup-net/go-lib/state"

// StateRepository -
type StateRepository interface {
	GetState(indexName string) (state.State, error)
	UpdateState(s state.State) error
}
