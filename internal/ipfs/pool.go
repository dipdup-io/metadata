package ipfs

import (
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// Pool -
type Pool struct {
	gateways []string
	limit    int64
	client   *http.Client
	limiter  *rate.Limiter
}

// NewPool -
func NewPool(gateways []string, limit int64) (*Pool, error) {
	if len(gateways) == 0 {
		return nil, ErrEmptyIPFSGatewayList
	}

	for i := range gateways {
		if _, err := url.Parse(gateways[i]); err != nil {
			return nil, errors.Wrap(ErrInvalidURI, gateways[i])
		}
	}
	return &Pool{
		gateways: gateways,
		limit:    limit,
		client: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		limiter: rate.NewLimiter(rate.Limit(10), 1),
	}, nil
}

// Get - returns result if one of node returns it
func (pool *Pool) Get(ctx context.Context, link string) (Data, error) {
	for _, node := range ShuffleGateways(pool.gateways) {
		if data, err := pool.request(ctx, link, node); err == nil {
			return Data{
				Raw:  data,
				Node: node,
			}, err
		}
	}
	return Data{}, ErrNoIPFSResponse
}

// GetFromRandomGateway - returns result if random node returns it
func (pool *Pool) GetFromRandomGateway(ctx context.Context, link string) (Data, error) {
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(pool.gateways))
	start := time.Now()
	data, err := pool.request(ctx, link, pool.gateways[index])
	if err != nil {
		return Data{
			Node: pool.gateways[index],
		}, err
	}
	return Data{
		Raw:          data,
		Node:         pool.gateways[index],
		ResponseTime: time.Since(start).Milliseconds(),
	}, nil
}

// GetFromNode - returns result if `node` returns it
func (pool *Pool) GetFromNode(ctx context.Context, link, node string) (Data, error) {
	data, err := pool.request(ctx, link, node)
	if err != nil {
		return Data{}, err
	}
	return Data{
		Raw:  data,
		Node: node,
	}, nil
}

func (pool *Pool) request(ctx context.Context, link, node string) ([]byte, error) {
	if err := pool.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	path := Path(link)
	gatewayURL := Link(node, path)

	if _, err := url.ParseRequestURI(gatewayURL); err != nil {
		return nil, errors.Wrap(ErrInvalidURI, gatewayURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pool.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(ErrHTTPRequest, err.Error())
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return ioutil.ReadAll(io.LimitReader(resp.Body, pool.limit))
	default:
		return nil, errors.Errorf("invalid status: %s", resp.Status)
	}
}
