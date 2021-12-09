package models

import (
	"database/sql/driver"
	stdJSON "encoding/json"
	"errors"
)

// JSONB -
type JSONB stdJSON.RawMessage

// Value -
func (j JSONB) Value() (driver.Value, error) {
	if j.IsNull() {
		return nil, nil
	}
	return string(j), nil
}

// Scan -
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	s, ok := value.([]byte)
	if !ok {
		return errors.New("scan source was not []byte")
	}
	*j = append((*j)[0:0], s...)

	return nil
}

// IsNull -
func (j JSONB) IsNull() bool {
	return len(j) == 0 || string(j) == "null"
}
