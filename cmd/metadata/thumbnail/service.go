package thumbnail

import (
	"bytes"
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

	"github.com/dipdup-net/metadata/cmd/metadata/helpers"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"github.com/dipdup-net/metadata/cmd/metadata/storage"
	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Service -
type Service struct {
	cursor   uint
	limit    int
	gateways []string
	storage  storage.Storage
	db       *gorm.DB

	workersCount int
	tasks        chan models.TokenMetadata
	stop         chan struct{}
	wg           sync.WaitGroup
}

// New -
func New(storage storage.Storage, db *gorm.DB, gateways []string, workersCount int) *Service {
	return &Service{
		cursor:       0,
		limit:        50,
		storage:      storage,
		gateways:     gateways,
		db:           db,
		workersCount: workersCount,
		tasks:        make(chan models.TokenMetadata, 1024),
		stop:         make(chan struct{}, workersCount+1),
	}
}

// Start -
func (s *Service) Start() {
	if s.storage == nil || s.db == nil {
		return
	}

	s.wg.Add(1)
	go s.dispatch()

	for i := 0; i < s.workersCount; i++ {
		s.wg.Add(1)
		go s.work()
	}
}

func (s *Service) dispatch() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if len(s.tasks) > 0 {
				continue
			}

			metadata, err := s.unprocessedMetadata()
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

func (s *Service) work() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stop:
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
				if _, ok := validMimes[format.MimeType]; !ok {
					continue
				}
				found = true

				if err := s.resolve(format.URI, format.MimeType, filename); err != nil {
					log.Error(err)
					continue
				}

				one.ImageProcessed = true
				break
			}

			if !found {
				if err := s.fallback(raw.ThumbnailURI, filename); err != nil {
					log.Error(err)
					continue
				}
				one.ImageProcessed = true
			}

			if one.ImageProcessed {
				if err := s.db.Model(&one).Update("image_processed", true).Error; err != nil {
					log.Error(err)
					continue
				}
			}
		}
	}
}

// Close -
func (s *Service) Close() error {
	for i := 0; i < s.workersCount+1; i++ {
		s.stop <- struct{}{}
	}
	s.wg.Wait()

	close(s.stop)
	close(s.tasks)
	return nil
}

func processLink(thumbnailStorage storage.Storage, link, mime, filename string) error {
	if _, err := url.ParseRequestURI(link); err != nil {
		return errors.Errorf("Invalid file link: %s", link)
	}
	client := http.Client{
		Timeout: 20 * time.Second,
	}
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
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

func (s *Service) unprocessedMetadata() (all []models.TokenMetadata, err error) {
	query := s.db.Model(&models.TokenMetadata{}).Where("status = 3 AND image_processed = false")
	if s.cursor > 0 {
		query.Where("id > ?", s.cursor)
	}
	err = query.Limit(s.limit).Order("id asc").Find(&all).Error
	return
}

func (s *Service) fallback(link, filename string) error {
	if link == "" {
		return nil
	}

	log.WithField("link", link).WithField("filename", filename).Info("Fallback thumbnail")
	return s.resolve(link, MimeTypePNG, filename)
}

func (s *Service) resolve(link, mime, filename string) error {
	switch {
	case strings.HasPrefix(link, "ipfs://"):
		hash, err := helpers.IPFSHash(link)
		if err != nil {
			return err
		}

		gateways := helpers.ShuffleGateways(s.gateways)
		for _, gateway := range gateways {
			link := helpers.IPFSLink(gateway, hash)
			if err := processLink(s.storage, link, mime, filename); err != nil {
				log.WithField("link", link).WithField("mime", mime).WithField("ipfs", gateway).Error(err)
				continue
			}
			return nil
		}
		return errors.Wrapf(ErrThumbnailCreating, "link=%s mime=%s", link, mime)

	case strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://"):
		return processLink(s.storage, link, mime, filename)

	default:
		return errors.Wrapf(ErrInvalidThumbnailLink, "link=%s", link)
	}
}
