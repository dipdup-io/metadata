package models

import (
	"bytes"
	"context"
	stdJSON "encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/database"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/go-pg/pg/v10"
	"github.com/pkg/errors"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Elastic -
type Elastic struct {
	*elasticsearch.Client
}

// NewElastic -
func NewElastic(cfg config.Database) (*Elastic, error) {
	retryBackoff := backoff.NewExponentialBackOff()
	elasticConfig := elasticsearch.Config{
		Addresses:     []string{cfg.Path},
		RetryOnStatus: []int{502, 503, 504, 429},
		RetryBackoff: func(i int) time.Duration {
			if i == 1 {
				retryBackoff.Reset()
			}
			return retryBackoff.NextBackOff()
		},
		MaxRetries: 5,
	}
	es, err := elasticsearch.NewClient(elasticConfig)
	if err != nil {
		return nil, err
	}

	entity := &Elastic{es}

	response, err := entity.Ping()
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := entity.createIndices(); err != nil {
		return nil, err
	}

	return entity, nil
}

type elasticResponse struct {
	Hits struct {
		Hits []struct {
			ID     string             `json:"_id"`
			Source stdJSON.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type bulkResponse struct {
	Took   int64              `json:"took"`
	Errors bool               `json:"errors"`
	Items  stdJSON.RawMessage `json:"items,omitempty"`
}

func (e *Elastic) search(query string, opts ...func(*esapi.SearchRequest)) (*elasticResponse, error) {
	body := strings.NewReader(query)
	opts = append(opts, e.Search.WithBody(body))
	response, err := e.Search(opts...)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.IsError() {
		return nil, errors.New(response.String())
	}

	var hits elasticResponse
	err = json.NewDecoder(response.Body).Decode(&hits)
	return &hits, err
}

func (e *Elastic) bulk(buf *bytes.Buffer) error {
	req := esapi.BulkRequest{
		Body:    bytes.NewReader(buf.Bytes()),
		Refresh: "true",
	}

	res, err := req.Do(context.Background(), e)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response bulkResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return err
	}
	if response.Errors {
		return errors.Errorf("Bulk error: %s", string(response.Items))
	}
	return nil
}

// GetContractMetadata -
// TODO: filter network
func (e *Elastic) GetContractMetadata(network string, status Status, limit, offset, retryCount int) ([]ContractMetadata, error) {
	hits, err := e.search(
		fmt.Sprintf(`{"query":{"term":{"status": %d}}}`, status),
		e.Search.WithIndex(ContractMetadata{}.TableName()),
		e.Search.WithSort("retry_count:asc"),
		e.Search.WithSize(limit),
		e.Search.WithFrom(offset),
	)
	if err != nil {
		return nil, err
	}

	contracts := make([]ContractMetadata, len(hits.Hits.Hits))
	for i, hit := range hits.Hits.Hits {
		if err := json.Unmarshal(hit.Source, &contracts[i]); err != nil {
			return nil, err
		}
		id, err := strconv.ParseUint(hit.ID, 10, 64)
		if err != nil {
			return nil, err
		}
		contracts[i].ID = id
	}
	return contracts, nil
}

// UpdateContractMetadata -
func (e *Elastic) UpdateContractMetadata(ctx context.Context, metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	bulk := bytes.NewBuffer([]byte{})
	for i := range metadata {
		metadata[i].UpdatedAt = metadata[i].CreatedAt
		meta := fmt.Sprintf(`{"update":{"_id":"%d","_index":"%s"}}`, time.Now().UnixNano(), metadata[i].TableName())
		if _, err := bulk.WriteString(meta); err != nil {
			return err
		}

		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		data, err := json.Marshal(metadata[i])
		if err != nil {
			return err
		}
		if err := stdJSON.Compact(bulk, data); err != nil {
			return err
		}
		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		if (i%1000 == 0 && i > 0) || i == len(metadata)-1 {
			if err := e.bulk(bulk); err != nil {
				return err
			}
			bulk.Reset()
		}
	}
	return nil
}

// SaveContractMetadata -
func (e *Elastic) SaveContractMetadata(ctx context.Context, metadata []*ContractMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	bulk := bytes.NewBuffer([]byte{})
	for i := range metadata {
		metadata[i].CreatedAt = time.Now().Unix()
		metadata[i].UpdatedAt = metadata[i].CreatedAt
		meta := fmt.Sprintf(`{"index":{"_id":"%d","_index":"%s"}}`, time.Now().UnixNano(), metadata[i].TableName())
		if _, err := bulk.WriteString(meta); err != nil {
			return err
		}

		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		data, err := json.Marshal(metadata[i])
		if err != nil {
			return err
		}
		if err := stdJSON.Compact(bulk, data); err != nil {
			return err
		}
		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		if (i%1000 == 0 && i > 0) || i == len(metadata)-1 {
			if err := e.bulk(bulk); err != nil {
				return err
			}
			bulk.Reset()
		}
	}
	return nil
}

// LastContractUpdateID -
func (e *Elastic) LastContractUpdateID() (value int64, err error) {
	// TODO: realize LastContractUpdateID
	return
}

// CountContractsByStatus -
func (e *Elastic) CountContractsByStatus(network string, status Status) (int, error) {
	// TODO: realize CountContractsByStatus
	return 0, nil
}

// GetContractMetadata -
// TODO: filter network
func (e *Elastic) GetTokenMetadata(network string, status Status, limit, offset, retryCount int) ([]TokenMetadata, error) {
	hits, err := e.search(
		fmt.Sprintf(`{"query":{"term":{"status": %d}}}`, status),
		e.Search.WithIndex(TokenMetadata{}.TableName()),
		e.Search.WithSort("retry_count:asc"),
		e.Search.WithSize(limit),
		e.Search.WithFrom(offset),
	)
	if err != nil {
		return nil, err
	}

	tokens := make([]TokenMetadata, len(hits.Hits.Hits))
	for i, hit := range hits.Hits.Hits {
		if err := json.Unmarshal(hit.Source, &tokens[i]); err != nil {
			return nil, err
		}
		id, err := strconv.ParseUint(hit.ID, 10, 64)
		if err != nil {
			return nil, err
		}
		tokens[i].ID = id
	}
	return tokens, nil
}

// UpdateTokenMetadata -
func (e *Elastic) UpdateTokenMetadata(ctx context.Context, metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	bulk := bytes.NewBuffer([]byte{})
	for i := range metadata {
		metadata[i].UpdatedAt = metadata[i].CreatedAt
		meta := fmt.Sprintf(`{"update":{"_id":"%d","_index":"%s"}}`, time.Now().UnixNano(), metadata[i].TableName())
		if _, err := bulk.WriteString(meta); err != nil {
			return err
		}

		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		data, err := json.Marshal(metadata[i])
		if err != nil {
			return err
		}
		if err := stdJSON.Compact(bulk, data); err != nil {
			return err
		}
		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		if (i%1000 == 0 && i > 0) || i == len(metadata)-1 {
			if err := e.bulk(bulk); err != nil {
				return err
			}
			bulk.Reset()
		}
	}
	return nil
}

// SaveContractMetadata -
func (e *Elastic) SaveTokenMetadata(ctx context.Context, metadata []*TokenMetadata) error {
	if len(metadata) == 0 {
		return nil
	}
	bulk := bytes.NewBuffer([]byte{})
	for i := range metadata {
		metadata[i].CreatedAt = time.Now().Unix()
		metadata[i].UpdatedAt = metadata[i].CreatedAt
		meta := fmt.Sprintf(`{"index":{"_id":"%d","_index":"%s"}}`, time.Now().UnixNano(), metadata[i].TableName())
		if _, err := bulk.WriteString(meta); err != nil {
			return err
		}

		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		data, err := json.Marshal(metadata[i])
		if err != nil {
			return err
		}
		if err := stdJSON.Compact(bulk, data); err != nil {
			return err
		}
		if err := bulk.WriteByte('\n'); err != nil {
			return err
		}

		if (i%1000 == 0 && i > 0) || i == len(metadata)-1 {
			if err := e.bulk(bulk); err != nil {
				return err
			}
			bulk.Reset()
		}
	}
	return nil
}

// LastTokenUpdateID -
func (e *Elastic) LastTokenUpdateID() (value int64, err error) {
	// TODO: realize LastTokenUpdateID
	return
}

// CountTokensByStatus -
func (e *Elastic) CountTokensByStatus(network string, status Status) (int, error) {
	// TODO: realize CountTokensByStatus
	return 0, nil
}

// SetImageProcessed -
func (e *Elastic) SetImageProcessed(token TokenMetadata) error {
	token.UpdatedAt = time.Now().Unix()
	response, err := e.Update(
		token.TableName(),
		fmt.Sprintf("%d", token.ID),
		strings.NewReader(`{"doc":{"image_processed":true}}`),
		e.Update.WithRefresh("true"),
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.IsError() {
		return errors.New(response.String())
	}
	return nil
}

// GetUnprocessedImage -
func (e *Elastic) GetUnprocessedImage(from uint64, limit int) ([]TokenMetadata, error) {
	var b strings.Builder
	b.WriteString(`{"query":{"bool":{"filter":[{"term":{"status": 3}},{"term":{"image_processed":false}}`)
	if from > 0 {
		b.WriteString(fmt.Sprintf(`,{"range":{"id":{"gte":%d}}}`, from))
	}
	b.WriteString(`]}}}`)
	hits, err := e.search(
		b.String(),
		e.Search.WithIndex(TokenMetadata{}.TableName()),
		e.Search.WithSort("retry_count:asc"),
		e.Search.WithSize(limit),
	)
	if err != nil {
		return nil, err
	}

	tokens := make([]TokenMetadata, len(hits.Hits.Hits))
	for i, hit := range hits.Hits.Hits {
		if err := json.Unmarshal(hit.Source, &tokens[i]); err != nil {
			return nil, err
		}
		id, err := strconv.ParseUint(hit.ID, 10, 64)
		if err != nil {
			return nil, err
		}
		tokens[i].ID = id
	}
	return tokens, nil
}

// CurrentContext -
func (e *Elastic) CurrentContext() ([]ContextItem, error) {
	hits, err := e.search(
		`{"query":{"match_all":{}}}`,
		e.Search.WithIndex(ContextItem{}.TableName()),
		e.Search.WithSize(10000),
	)
	if err != nil {
		return nil, err
	}
	updates := make([]ContextItem, len(hits.Hits.Hits))
	for i, hit := range hits.Hits.Hits {
		if err := json.Unmarshal(hit.Source, &updates[i]); err != nil {
			return nil, err
		}
		updates[i].ID, err = strconv.ParseUint(hit.ID, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	return updates, nil
}

// DumpContext -
func (e *Elastic) DumpContext(action Action, item ContextItem) error {
	switch action {
	case ActionCreate, ActionUpdate:
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		resp, err := e.Create(
			item.TableName(),
			fmt.Sprintf("%d", time.Now().UnixNano()),
			bytes.NewReader(data),
			e.Create.WithRefresh("true"),
		)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.IsError() {
			return errors.New(resp.String())
		}
	case ActionDelete:
		resp, err := e.Delete(
			item.TableName(),
			fmt.Sprintf("%d", time.Now().UnixNano()),
			e.Delete.WithRefresh("true"),
		)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.IsError() {
			return errors.New(resp.String())
		}
	}
	return nil
}

// State -
func (e *Elastic) State(indexName string) (s database.State, err error) {
	hits, err := e.search(
		fmt.Sprintf(`{"query":{"term":{"index_name":"%s"}}}`, indexName),
		e.Search.WithIndex(s.TableName()),
		e.Search.WithSize(1),
	)
	if err != nil {
		return
	}

	if len(hits.Hits.Hits) != 1 {
		return s, errors.Wrapf(pg.ErrNoRows, "%s %s", indexName, s.TableName())
	}
	err = json.Unmarshal(hits.Hits.Hits[0].Source, &s)
	return
}

// UpdateState -
func (e *Elastic) UpdateState(s database.State) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(`{"doc_as_upsert":true,"doc":`)
	b.Write(data)
	b.WriteString(`}`)

	resp, err := e.Update(
		s.TableName(),
		s.IndexName,
		strings.NewReader(b.String()),
		e.Update.WithRefresh("true"),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return errors.New(resp.String())
	}
	return nil
}

// CreateState -
func (e *Elastic) CreateState(s database.State) error {
	// TODO: implement
	return nil
}

// DeleteState -
func (e *Elastic) DeleteState(s database.State) error {
	// TODO: implement
	return nil
}

// Close -
func (e *Elastic) Close() error {
	return nil
}

func (e *Elastic) createIndices() error {
	if err := e.createIndex(database.State{}.TableName()); err != nil {
		return err
	}
	if err := e.createIndex(ContractMetadata{}.TableName()); err != nil {
		return err
	}
	if err := e.createIndex(TokenMetadata{}.TableName()); err != nil {
		return err
	}
	if err := e.createIndex(ContextItem{}.TableName()); err != nil {
		return err
	}
	return nil
}

func (e *Elastic) createIndex(name string) error {
	req := esapi.IndicesExistsRequest{
		Index: []string{name},
	}
	resp, err := req.Do(context.Background(), e)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	opts := []func(*esapi.IndicesCreateRequest){}
	mapping, err := os.Open(fmt.Sprintf("./mappings/%s.json", name))
	if err == nil {
		data, err := ioutil.ReadAll(mapping)
		if err != nil {
			return err
		}
		if err := mapping.Close(); err != nil {
			return err
		}
		opts = append(opts, e.Indices.Create.WithBody(bytes.NewReader(data)))
	}

	response, err := e.Indices.Create(name, opts...)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.IsError() {
		return errors.New(response.String())
	}
	return nil
}

// IPFSLink -
func (e *Elastic) IPFSLink(id int64) (link IPFSLink, err error) {
	// TODO: implement
	return
}

// IPFSLinks -
func (e *Elastic) IPFSLinks(limit, offset int) (links []IPFSLink, err error) {
	// TODO: implement
	return
}

// SaveIPFSLink -
func (e *Elastic) SaveIPFSLink(link IPFSLink) error {
	// TODO: implement
	return nil
}

// UpdateIPFSLink -
func (e *Elastic) UpdateIPFSLink(link IPFSLink) error {
	// TODO: implement
	return nil
}

// IPFSLinkByURL -
func (e *Elastic) IPFSLinkByURL(url string) (link IPFSLink, err error) {
	// TODO: implement
	return
}

// CreateIndices -
func (e *Elastic) CreateIndices() error {
	// TODO: implement
	return nil
}

// Exec -
func (e *Elastic) Exec(sql string) error {
	// TODO: implement
	return nil
}
