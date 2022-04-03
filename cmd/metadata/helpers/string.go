package helpers

import (
	"encoding/hex"
	"fmt"
	"regexp"
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

var regEscapedString = regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

// Escape -
func Escape(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	response := make([]byte, 0)
	for {
		loc := regEscapedString.FindIndex(data)
		if loc == nil {
			for i := 0; i < len(data); i++ {
				if data[i] <= 2 {
					response = append(response, []byte(fmt.Sprintf("0x%02x", data[i]))...)
				} else {
					response = append(response, data[i])
				}
			}
			break
		}
		for i := 0; i < loc[0]; i++ {
			if data[i] <= 2 {
				response = append(response, []byte(fmt.Sprintf("0x%02x", data[i]))...)
			} else {
				response = append(response, data[i])
			}
		}
		response = append(response, '0', 'x')
		response = append(response, data[loc[0]+2:loc[1]]...)
		data = data[loc[1]:]
	}

	return response
}
