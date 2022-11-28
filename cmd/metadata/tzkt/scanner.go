package tzkt

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/go-lib/tzkt/api"
	"github.com/dipdup-net/go-lib/tzkt/data"
	"github.com/dipdup-net/go-lib/tzkt/events"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
	blocks   chan data.Block
	wg       sync.WaitGroup
	initOnce sync.Once
}

// New -
func New(cfg config.DataSource, contracts ...string) (*Scanner, error) {
	baseURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}
	eventsURL := baseURL.JoinPath("v1/events")

	return &Scanner{
		client:    events.NewTzKT(eventsURL.String()),
		api:       api.New(baseURL.String()),
		msg:       newMessage(),
		contracts: contracts,
		diffs:     make(chan Message, 1024),
		blocks:    make(chan data.Block, 10),
	}, nil
}

// Start -
func (scanner *Scanner) Start(ctx context.Context, startLevel, endLevel uint64) {
	if endLevel > 0 && startLevel > 0 && startLevel > endLevel {
		return
	}

	scanner.initOnce.Do(func() {
		scanner.wg.Add(1)
		go scanner.synchronization(ctx, startLevel, endLevel)
	})

}

func (scanner *Scanner) start(ctx context.Context) {
	if err := scanner.client.Connect(ctx); err != nil {
		log.Err(err).Msg("")
		return
	}

	if err := scanner.subscribe(); err != nil {
		log.Err(err).Msg("")
		return
	}

	scanner.wg.Add(1)
	go scanner.listen(ctx)
}

func (scanner *Scanner) synchronization(ctx context.Context, startLevel, endLevel uint64) {
	defer scanner.wg.Done()

	head, err := scanner.api.GetHead(ctx)
	if err != nil {
		log.Err(err).Msg("")
		return
	}
	log.Info().Msgf("Current TzKT head is %d. Indexer state is %d.", head.Level, startLevel)

	scanner.level = startLevel

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if endLevel > 0 && scanner.level > endLevel {
				log.Warn().Msgf("synchronization was stopped due to last_level in config is equal to current level")
				return
			}
			if head.Level <= scanner.level {
				scanner.start(ctx)
				return
			}

			if err := scanner.sync(ctx, head.Level); err != nil {
				log.Err(err).Msg("")
				return
			}

			head, err = scanner.api.GetHead(ctx)
			if err != nil {
				log.Err(err).Msg("")
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
func (scanner *Scanner) Blocks() <-chan data.Block {
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
	defer scanner.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-scanner.client.Listen():
			switch msg.Type {
			case events.MessageTypeState:
				if scanner.level < msg.State {
					if err := scanner.client.Close(); err != nil {
						log.Err(err).Msg("scanner.client.Close")
					}
					scanner.synchronization(ctx, scanner.level, 0)
					return
				}

			case events.MessageTypeData:
				switch msg.Channel {
				case events.ChannelBlocks:
					if err := scanner.handleBlocks(msg); err != nil {
						log.Err(err).Msg("")
					}
				case events.ChannelBigMap:
					if err := scanner.handleBigMaps(msg); err != nil {
						log.Err(err).Msg("")
					}
				default:
					log.Error().Msgf("Unknown channel %s", msg.Channel)
				}
			case events.MessageTypeReorg, events.MessageTypeSubscribed:
			}
		}
	}
}

func (scanner *Scanner) sync(ctx context.Context, headLevel uint64) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if headLevel <= scanner.level {
				if scanner.msg.Level > 0 {
					scanner.level = scanner.msg.Level
					scanner.diffs <- scanner.msg.copy()
					scanner.msg.clear()
				}
				return nil
			}

			updates, err := scanner.getSyncUpdates(ctx, headLevel)
			if err != nil {
				log.Err(err).Msg("getSyncUpdates")
				time.Sleep(time.Second)
				continue
			}

			if len(updates) > 0 {
				scanner.processSyncUpdates(ctx, updates)
			} else {
				scanner.level = headLevel
			}
		}
	}
}

func (scanner *Scanner) getSyncUpdates(ctx context.Context, headLevel uint64) ([]data.BigMapUpdate, error) {
	filters := map[string]string{
		"path.as":   "*metadata",
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

func (scanner *Scanner) processSyncUpdates(ctx context.Context, updates []data.BigMapUpdate) {
	for i := range updates {
		select {
		case <-ctx.Done():
			return
		default:
			if scanner.msg.Level != 0 && scanner.msg.Level != updates[i].Level {
				scanner.level = scanner.msg.Level
				scanner.diffs <- scanner.msg.copy()
				scanner.blocks <- data.Block{
					Level:     scanner.msg.Level,
					Timestamp: updates[i].Timestamp.UTC(),
				}
				scanner.msg.clear()
			}

			scanner.msg.Body = append(scanner.msg.Body, updates[i])
			scanner.msg.Level = updates[i].Level
			scanner.lastID = updates[i].ID
		}
	}
}

func (scanner *Scanner) handleBlocks(msg events.Message) error {
	body, ok := msg.Body.([]data.Block)
	if !ok {
		return errors.Errorf("Invalid body type: %T", msg.Body)
	}
	if len(body) == 0 {
		return errors.Errorf("Empty body: %v", body)
	}

	scanner.blocks <- body[0]
	return nil
}

func (scanner *Scanner) handleBigMaps(msg events.Message) error {
	body, ok := msg.Body.([]data.BigMapUpdate)
	if !ok {
		return errors.Errorf("Invalid body type: %T", msg.Body)
	}
	if len(body) == 0 {
		return nil
	}

	diffs := make([]data.BigMapUpdate, len(body))
	for i := range body {
		diffs[i] = data.BigMapUpdate{
			ID:        body[i].ID,
			Level:     body[i].Level,
			Timestamp: body[i].Timestamp,
			Bigmap:    body[i].Bigmap,
			Contract:  body[i].Contract,
			Path:      body[i].Path,
			Action:    body[i].Action,
		}

		if body[i].Content != nil {
			diffs[i].Content = &data.BigMapUpdateContent{
				Hash:  body[i].Content.Hash,
				Key:   body[i].Content.Key,
				Value: body[i].Content.Value,
			}
		}
	}

	scanner.diffs <- Message{
		Type:  msg.Type,
		Body:  diffs,
		Level: msg.State,
	}
	return nil
}
