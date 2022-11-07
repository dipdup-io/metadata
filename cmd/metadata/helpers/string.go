package helpers

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

// Trim -
func Trim(val string) string {
	return strings.TrimSuffix(strings.TrimPrefix(val, `"`), `"`)
}

// IsJSON -
func IsJSON(val string) bool {
	if len(val) < 2 {
		return false
	}
	data, err := hex.DecodeString(val[1 : len(val)-1])
	if err != nil {
		return false
	}
	return json.Valid(data)
}

// Decode
func Decode(data []byte) ([]byte, error) {
	return hex.DecodeString(Trim(string(data)))
}

var escape = regexp.MustCompile(`(\\u((d8|D8)[0-9a-fA-F]{2}|0000))`)

// Escape -
func Escape(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	data = escape.ReplaceAll(data, []byte("\\$1"))
	return bytes.Map(func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, data)
}
