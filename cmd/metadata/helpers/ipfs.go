package helpers

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
)

const (
	prefixIpfs = "ipfs://"
)

// IPFSHash - separate IPFS hash from link
func IPFSHash(link string) (string, error) {
	hash := strings.TrimPrefix(link, prefixIpfs)
	if _, err := cid.Decode(hash); err != nil {
		return "", err
	}
	return hash, nil
}

// IPFSLink - get gateway link
func IPFSLink(gateway, hash string) string {
	return fmt.Sprintf("%s/ipfs/%s", gateway, hash)
}

var ipfsURL = regexp.MustCompile(`ipfs:\/\/(?P<hash>Qm[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]{44})`)

// FindAllIPFSLinks -
func FindAllIPFSLinks(data []byte) []string {
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
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled
}
