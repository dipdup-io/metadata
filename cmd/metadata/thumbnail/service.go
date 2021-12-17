package thumbnail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dipdup-net/go-lib/prometheus"
	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/storage"
	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Service -
type Service struct {
	cursor   uint64
	limit    int
	gateways []string
	storage  storage.Storage
	db       models.Database
	prom     *prometheus.Service

	maxFileSizeMB int64
	size          int

	network string
	workers chan struct{}
	wg      sync.WaitGroup
}

// New -
func New(storage storage.Storage, db models.Database, network string, gateways []string, opts ...ThumbnailOption) *Service {
	service := &Service{
		cursor:        0,
		limit:         50,
		maxFileSizeMB: defaultMaxFileSize,
		size:          defaultThumbnailSize,
		storage:       storage,
		gateways:      gateways,
		db:            db,
		network:       network,
	}

	for i := range opts {
		opts[i](service)
	}

	if service.workers == nil {
		service.workers = make(chan struct{}, 10)
	}

	return service
}

// Start -
func (s *Service) Start(ctx context.Context) {
	if s.storage == nil || s.db == nil {
		return
	}

	s.wg.Add(1)
	go s.dispatch(ctx)
}

func (s *Service) dispatch(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			metadata, err := s.db.GetUnprocessedImage(s.cursor, s.limit)
			if err != nil {
				log.Err(err).Msg("")
				continue
			}

			if len(metadata) == 0 {
				time.Sleep(time.Second)
				continue
			}

			for _, one := range metadata {
				s.workers <- struct{}{}
				s.cursor = one.ID
				s.wg.Add(1)

				go func(metadata models.TokenMetadata) {
					defer func() {
						<-s.workers
						s.wg.Done()
					}()

					if err := s.work(ctx, metadata); err != nil {
						log.Err(err).Msg("")
					}
				}(one)
			}
		}
	}
}

func (s *Service) work(ctx context.Context, one models.TokenMetadata) error {
	filename := fmt.Sprintf("%s/%d.png", one.Contract, one.TokenID)
	if s.storage.Exists(filename) {
		return nil
	}

	var raw Metadata
	if err := json.Unmarshal(one.Metadata, &raw); err != nil {
		return err
	}

	var found bool
	for _, format := range raw.Formats {
		s.incrementMimeCounter(format.MimeType)

		if _, ok := validMimes[format.MimeType]; !ok {
			continue
		}
		found = true

		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := s.resolve(reqCtx, format.URI, format.MimeType, filename); err != nil {
			return err
		}

		one.ImageProcessed = true
		break
	}

	if !found {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := s.fallback(reqCtx, raw.ThumbnailURI, filename); err != nil {
			return err
		}
		one.ImageProcessed = true
	}

	if one.ImageProcessed {
		return s.db.SetImageProcessed(one)
	}
	return nil
}

// Close -
func (s *Service) Close() error {
	s.wg.Wait()

	close(s.workers)
	return nil
}

func (s *Service) processLink(ctx context.Context, link, mime, filename string) error {
	if _, err := url.ParseRequestURI(link); err != nil {
		return errors.Errorf("Invalid file link: %s", link)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("Invalid status code: %s", resp.Status)
	}

	reader := io.LimitReader(resp.Body, s.maxFileSizeMB*1048576)
	return s.createThumbnail(reader, mime, filename)
}

func (s *Service) createThumbnail(reader io.Reader, mime, filename string) error {
	switch mime {
	case MimeTypePNG, MimeTypeJPEG, MimeTypeGIF:
		img, _, err := image.Decode(reader)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, imaging.Thumbnail(img, s.size, s.size, imaging.NearestNeighbor)); err != nil {
			return err
		}
		return s.storage.Upload(&buf, filename)
	}
	return nil
}

func (s *Service) fallback(ctx context.Context, link, filename string) error {
	if link == "" {
		return nil
	}

	log.Info().Fields(map[string]interface{}{
		"link":     link,
		"filename": filename,
	}).Msg("Fallback thumbnail")
	return s.resolve(ctx, link, MimeTypePNG, filename)
}

func (s *Service) resolve(ctx context.Context, link, mime, filename string) error {
	switch {
	case strings.HasPrefix(link, "ipfs://"):
		hash, err := helpers.IPFSHash(link)
		if err != nil {
			return err
		}

		gateways := helpers.ShuffleGateways(s.gateways)
		for _, gateway := range gateways {
			link := helpers.IPFSLink(gateway, hash)
			if err := s.processLink(ctx, link, mime, filename); err != nil {
				log.Err(err).Fields(map[string]interface{}{
					"link": link,
					"mime": mime,
					"ipfs": gateway,
				}).Msg("")
				continue
			}
			return nil
		}
		return errors.Wrapf(ErrThumbnailCreating, "link=%s mime=%s", link, mime)

	case strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://"):
		return s.processLink(ctx, link, mime, filename)

	default:
		return errors.Wrapf(ErrInvalidThumbnailLink, "link=%s", link)
	}
}

func (service *Service) incrementMimeCounter(mime string) {
	if service.prom == nil {
		return
	}
	service.prom.IncrementCounter("metadata_mime_type", map[string]string{
		"network": service.network,
		"mime":    mime,
	})
}
