package tezos

import "strings"

// prefixes
const (
	PrefixTezosStorage = "tezos-storage:"
)

// Is -
func Is(link string) bool {
	return strings.HasPrefix(link, PrefixTezosStorage)
}
