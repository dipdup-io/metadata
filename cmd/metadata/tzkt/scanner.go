package tzkt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/go-lib/tzkt/events"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	pageSize = 1000
)

// Scanner -
type Scanner struct {
	api       *api.API
	client    *events.TzKT
	lastID    uint64
	level     uint64
	msg       Message
	contracts []string

	diffs    chan Message
	blocks   chan uint64
	wg       sync.WaitGroup
	initOnce sync.Once
}

// New -
func New(baseURL string, contracts ...string) *Scanner {
	return &Scanner{
		client:    events.NewTzKT(fmt.Sprintf("%s/%s", strings.TrimSuffix(baseURL, "/"), "v1/events")),
		api:       api.New(baseURL),
		msg:       newMessage(),
		contracts: contracts,
		diffs:     make(chan Message, 1024),
		blocks:    make(chan uint64, 10),
	}
}

// Start -
func (scanner *Scanner) Start(ctx context.Context, level uint64) {
	scanner.initOnce.Do(func() {
		scanner.wg.Add(1)
		go scanner.synchronization(ctx, level)
	})

}

func (scanner *Scanner) start(ctx context.Context) {
	if err := scanner.client.Connect(); err != nil {
		log.Error(err)
		return
	}

	if err := scanner.subscribe(); err != nil {
		log.Error(err)
		return
	}

	scanner.listen(ctx)
}

func (scanner *Scanner) synchronization(ctx context.Context, level uint64) {
	defer scanner.wg.Done()

	head, err := scanner.api.GetHead(ctx)
	if err != nil {
		log.Error(err)
		return
	}
	log.Infof("Current TzKT head is %d. Indexer state is %d.", head.Level, level)

	scanner.level = level

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if head.Level <= scanner.level {
				scanner.start(ctx)
				return
			}

			if err := scanner.sync(ctx, head.Level); err != nil {
				log.Error(err)
				return
			}

			head, err = scanner.api.GetHead(ctx)
			if err != nil {
				log.Error(err)
				return
			}
		}
	}
}

// Close -
func (scanner *Scanner) Close() error {
	scanner.wg.Wait()

	if scanner.client.IsConnected() {
		if err := scanner.client.Close(); err != nil {
			return err
		}
	}

	close(scanner.diffs)
	close(scanner.blocks)
	return nil
}

// BigMaps -
func (scanner *Scanner) BigMaps() <-chan Message {
	return scanner.diffs
}

// Blocks -
func (scanner *Scanner) Blocks() <-chan uint64 {
	return scanner.blocks
}

func (scanner *Scanner) subscribe() error {
	if err := scanner.client.SubscribeToBlocks(); err != nil {
		return err
	}

	if len(scanner.contracts) == 0 {
		if err := scanner.client.SubscribeToBigMaps(nil, "", "", events.BigMapTagMetadata, events.BigMapTagTokenMetadata); err != nil {
			return err
		}
	} else {
		for i := range scanner.contracts {
			if err := scanner.client.SubscribeToBigMaps(nil, scanner.contracts[i], "", events.BigMapTagMetadata, events.BigMapTagTokenMetadata); err != nil {
				return err
			}
		}
	}

	return nil
}

func (scanner *Scanner) listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-scanner.client.Listen():
			switch msg.Channel {
			case events.ChannelBlocks:
				if err := scanner.handleBlocks(msg); err != nil {
					log.Error(err)
				}
			case events.ChannelBigMap:
				if err := scanner.handleBigMaps(msg); err != nil {
					log.Error(err)
				}
			default:
				log.Errorf("Unknown channel %s", msg.Channel)
			}
		}
	}
}

func (scanner *Scanner) sync(ctx context.Context, headLevel uint64) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if headLevel <= scanner.level {
				if scanner.level < scanner.msg.Level {
					scanner.level = scanner.msg.Level
					scanner.diffs <- scanner.msg.copy()
					scanner.msg.clear()
				}
				return nil
			}

			updates, err := scanner.getSyncUpdates(ctx, headLevel)
			if err != nil {
				return err
			}

			if len(updates) > 0 {
				scanner.processSyncUpdates(updates)
			} else {
				scanner.level = headLevel
			}
		}
	}
}

func (scanner *Scanner) getSyncUpdates(ctx context.Context, headLevel uint64) ([]api.BigMapUpdate, error) {
	filters := map[string]string{
		"tags.any":  "token_metadata,metadata",
		"action.in": "add_key,update_key",
		"limit":     fmt.Sprintf("%d", pageSize),
		"level.le":  fmt.Sprintf("%d", headLevel),
		"sort.asc":  "id",
	}

	if scanner.lastID == 0 {
		filters["level.gt"] = fmt.Sprintf("%d", scanner.level)
	} else {
		filters["offset.cr"] = fmt.Sprintf("%d", scanner.lastID)
	}

	if len(scanner.contracts) > 0 {
		filters["contract.in"] = strings.Join(scanner.contracts, ",")
	}

	return scanner.api.GetBigmapUpdates(ctx, filters)
}

func (scanner *Scanner) processSyncUpdates(updates []api.BigMapUpdate) {
	for i := range updates {
		if scanner.msg.Level != 0 && scanner.msg.Level != updates[i].Level {
			scanner.level = scanner.msg.Level
			scanner.diffs <- scanner.msg.copy()
			scanner.msg.clear()
		}

		scanner.msg.Body = append(scanner.msg.Body, updates[i])
		scanner.msg.Level = updates[i].Level
		scanner.lastID = updates[i].ID
	}
}

func (scanner *Scanner) handleBlocks(msg events.Message) error {
	switch msg.Type {
	case events.MessageTypeData:
		body, ok := msg.Body.([]interface{})
		if !ok {
			return errors.Errorf("Invalid body type: %T", msg.Body)
		}
		if len(body) == 0 {
			return errors.Errorf("Empty body: %v", body)
		}
		m, ok := body[0].(map[string]interface{})
		if !ok {
			return errors.Errorf("Invalid message type: %T", body[0])
		}
		value, ok := m["level"]
		if !ok {
			return errors.Errorf("Unknown block level: %v", m)
		}
		level, ok := value.(float64)
		if !ok {
			return errors.Errorf("Invalid level type: %T", value)
		}

		scanner.blocks <- uint64(level)
	case events.MessageTypeReorg, events.MessageTypeSubscribed, events.MessageTypeState:
	}
	return nil
}

func (scanner *Scanner) handleBigMaps(msg events.Message) error {
	switch msg.Type {
	case events.MessageTypeData:
		b, err := json.Marshal(msg.Body)
		if err != nil {
			return err
		}
		var diffs []api.BigMapUpdate
		if err := json.Unmarshal(b, &diffs); err != nil {
			return err
		}
		scanner.diffs <- Message{
			Type:  msg.Type,
			Body:  diffs,
			Level: msg.State,
		}
	case events.MessageTypeReorg, events.MessageTypeSubscribed, events.MessageTypeState:
	}
	return nil
}
