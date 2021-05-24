package helpers

import (
	"bytes"
	"encoding/hex"
	"strings"
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
	return bytes.ReplaceAll(data, []byte{0x5c, 0x75, 0x30, 0x30, 0x30, 0x30}, []byte("0x00")) // \u0000 => 0x00
}
