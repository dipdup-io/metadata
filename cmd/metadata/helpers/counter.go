package helpers

import (
	"sync"
)

// Counter -
type Counter struct {
	value int64
	mux   sync.Mutex
}

// NewCounter -
func NewCounter(start int64) *Counter {
	return &Counter{value: start}
}

// Increment -
func (c *Counter) Increment() int64 {
	defer c.mux.Unlock()

	var val int64
	c.mux.Lock()
	{
		c.value += 1
		val = c.value
	}
	return val
}

// Set -
func (c *Counter) Set(val int64) {
	defer c.mux.Unlock()

	c.mux.Lock()
	{
		c.value = val
	}
}
