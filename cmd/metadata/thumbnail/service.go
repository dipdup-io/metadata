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
	log "github.com/sirupsen/logrus"
)

// Service -
type Service struct {
	cursor   uint64
	limit    int
	gateways []string
	storage  storage.Storage
	db       models.Database
	prom     *prometheus.Service
	network  string

	workersCount int
	tasks        chan models.TokenMetadata
	wg           sync.WaitGroup
}

// New -
func New(storage storage.Storage, db models.Database, prom *prometheus.Service, network string, gateways []string, workersCount int) *Service {
	return &Service{
		cursor:       0,
		limit:        50,
		storage:      storage,
		gateways:     gateways,
		prom:         prom,
		db:           db,
		workersCount: workersCount,
		network:      network,
		tasks:        make(chan models.TokenMetadata, 1024),
	}
}

// Start -
func (s *Service) Start(ctx context.Context) {
	if s.storage == nil || s.db == nil {
		return
	}

	s.wg.Add(1)
	go s.dispatch(ctx)

	for i := 0; i < s.workersCount; i++ {
		s.wg.Add(1)
		go s.work(ctx)
	}
}

func (s *Service) dispatch(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(s.tasks) > 0 {
				continue
			}

			metadata, err := s.db.GetUnprocessedImage(s.cursor, s.limit)
			if err != nil {
				log.Error(err)
				continue
			}

			for _, one := range metadata {
				s.tasks <- one
				s.cursor = one.ID
			}
		}
	}
}

func (s *Service) work(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case one := <-s.tasks:
			filename := fmt.Sprintf("%s/%d.png", one.Contract, one.TokenID)
			if s.storage.Exists(filename) {
				continue
			}

			var raw Metadata
			if err := json.Unmarshal(one.Metadata, &raw); err != nil {
				log.Warn(err.Error())
				continue
			}

			var found bool
			for _, format := range raw.Formats {
				s.incrementMimeCounter(format.MimeType)

				if _, ok := validMimes[format.MimeType]; !ok {
					continue
				}
				found = true

				reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				defer cancel()

				if err := s.resolve(reqCtx, format.URI, format.MimeType, filename); err != nil {
					log.Error(err)
					continue
				}

				one.ImageProcessed = true
				break
			}

			if !found {
				reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				defer cancel()

				if err := s.fallback(reqCtx, raw.ThumbnailURI, filename); err != nil {
					log.Error(err)
					continue
				}
				one.ImageProcessed = true
			}

			if one.ImageProcessed {
				if err := s.db.SetImageProcessed(one); err != nil {
					log.Error(err)
					continue
				}
			}
		}
	}
}

// Close -
func (s *Service) Close() error {
	s.wg.Wait()

	close(s.tasks)
	return nil
}

func processLink(ctx context.Context, thumbnailStorage storage.Storage, link, mime, filename string) error {
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

	reader := io.LimitReader(resp.Body, maxFileSize)
	return createThumbnail(thumbnailStorage, reader, mime, filename)
}

func createThumbnail(thumbnailStorage storage.Storage, reader io.Reader, mime, filename string) error {
	switch mime {
	case MimeTypePNG, MimeTypeJPEG, MimeTypeGIF:
		img, _, err := image.Decode(reader)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, imaging.Thumbnail(img, thumbnailSize, thumbnailSize, imaging.NearestNeighbor)); err != nil {
			return err
		}
		return thumbnailStorage.Upload(&buf, filename)
	}
	return nil
}

func (s *Service) fallback(ctx context.Context, link, filename string) error {
	if link == "" {
		return nil
	}

	log.WithField("link", link).WithField("filename", filename).Info("Fallback thumbnail")
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
			if err := processLink(ctx, s.storage, link, mime, filename); err != nil {
				log.WithField("link", link).WithField("mime", mime).WithField("ipfs", gateway).Error(err)
				continue
			}
			return nil
		}
		return errors.Wrapf(ErrThumbnailCreating, "link=%s mime=%s", link, mime)

	case strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://"):
		return processLink(ctx, s.storage, link, mime, filename)

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
