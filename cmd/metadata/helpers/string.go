package helpers

import (
	"bytes"
	"encoding/hex"
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
	return strings.HasPrefix(val, `"7b`) && strings.HasSuffix(val, `7d"`)
}

// Decode
func Decode(data []byte) ([]byte, error) {
	return hex.DecodeString(Trim(string(data)))
}

var escape = regexp.MustCompile("(\\\\u((d8|D8)[0-9a-fA-F]{2}|0000))")

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
