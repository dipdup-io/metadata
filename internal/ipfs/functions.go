package ipfs

import (
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
)

// Hash - separate IPFS hash from link
func Hash(link string) (string, error) {
	hash := FindAllLinks([]byte(link))
	if len(hash) != 1 {
		return "", errors.Errorf("invalid IPFS link: %s", link)
	}
	_, err := cid.Decode(hash[0])
	return hash[0], err
}

// Link - get gateway link
func Link(gateway, hash string) string {
	return fmt.Sprintf("%s/ipfs/%s", gateway, hash)
}

// Path - get path without protocol
func Path(link string) string {
	return strings.TrimPrefix(link, "ipfs://")
}

var ipfsURL = regexp.MustCompile(`ipfs:\/\/(?P<hash>(baf[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]{56})|Qm[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]{44})`)

// FindAllLinks -
func FindAllLinks(data []byte) []string {
	matches := ipfsURL.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return nil
	}

	res := make([]string, 0)
	for i := range matches {
		if len(matches[i]) != 2 {
			continue
		}
		res = append(res, string(matches[i][1]))
	}
	return res
}

// ShuffleGateways - shuffle gateways for different request order for different files
func ShuffleGateways(gateways []string) []string {
	if len(gateways) < 2 {
		return gateways
	}

	shuffled := make([]string, len(gateways))
	copy(shuffled, gateways)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled
}

// Is -
func Is(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	_, err = cid.Decode(u.Host)
	return err == nil
}
