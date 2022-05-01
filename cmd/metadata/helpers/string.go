package helpers

import (
	"bytes"
	"encoding/hex"
	"strings"
	"unicode"
)

// Trim -
func Trim(val string) string {
	return strings.TrimSuffix(strings.TrimPrefix(val, `"`), `"`)
}

// IsJSON -
func IsJSON(val string) bool {
	return strings.HasPrefix(val, `"7b`) && strings.HasSuffix(val, `7d"`)
}

// Decode
func Decode(data []byte) ([]byte, error) {
	return hex.DecodeString(Trim(string(data)))
}

// Escape -
func Escape(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	data = bytes.ReplaceAll(data, []byte(`\u0000`), nil)

	return bytes.Map(func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, data)
}
