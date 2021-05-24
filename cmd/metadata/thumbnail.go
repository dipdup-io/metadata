package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
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

var errThumbnailCreating = errors.New("Can't create thumbnail")

// Metadata -
type Metadata struct {
	Formats []Format `json:"formats,omitempty"`
}

// Format -
type Format struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
}

// Mime types
const (
	MimeTypePNG  = "image/png"
	MimeTypeJPEG = "image/jpeg"
	MimeTypeGIF  = "image/gif"
)

const (
	maxFileSize   = 52428800 // 50 MB
	thumbnailSize = 100
)

var validMimes = map[string]struct{}{
	MimeTypePNG:  {},
	MimeTypeJPEG: {},
	MimeTypeGIF:  {},
}

// ThumbnailCreator -
type ThumbnailCreator struct {
	gateways []string
	storage  storage.Storage
	db       *gorm.DB

	wg   sync.WaitGroup
	stop chan struct{}
}

// NewThumbnailCreator -
func NewThumbnailCreator(storage storage.Storage, db *gorm.DB, gateways []string) *ThumbnailCreator {
	return &ThumbnailCreator{
		storage:  storage,
		gateways: gateways,
		db:       db,
		stop:     make(chan struct{}, 1),
	}
}

// Start -
func (tc *ThumbnailCreator) Start() {
	if tc.storage == nil {
		return
	}

	tc.wg.Add(1)
	go tc.listen()
}

func (tc *ThumbnailCreator) listen() {
	defer tc.wg.Done()

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-tc.stop:
			return
		case <-ticker.C:
			err := tc.db.Transaction(func(tx *gorm.DB) error {
				metadata, err := models.GetTokenMetadataWithUnprocessedImages(tx)
				if err != nil {
					return err
				}

				for _, one := range metadata {
					var raw Metadata
					if err := json.Unmarshal(one.Metadata, &raw); err != nil {
						return err
					}
					filename := fmt.Sprintf("%s/%d.png", one.Contract, one.TokenID)

					var found bool
					for _, format := range raw.Formats {
						if _, ok := validMimes[format.MimeType]; !ok {
							continue
						}
						found = true

						if err := tc.processFormat(format, filename); err != nil {
							log.Error(err)
							continue
						}

						one.ImageProcessed = true
						break
					}

					if !found {
						one.ImageProcessed = true
					}

					if err := one.SetImageProcessed(tx); err != nil {
						return err
					}
				}

				return nil
			})
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
}

// Close -
func (tc *ThumbnailCreator) Close() error {
	tc.stop <- struct{}{}
	tc.wg.Wait()
	close(tc.stop)
	return nil
}

func (tc *ThumbnailCreator) processFormat(format Format, filename string) error {
	hash, err := helpers.IPFSHash(format.URI)
	if err != nil {
		return err
	}

	for _, gateway := range tc.gateways {
		link := helpers.IPFSLink(gateway, hash)
		if err := processLink(tc.storage, link, format.MimeType, filename); err != nil {
			log.WithField("link", format.URI).WithField("mime", format.MimeType).WithField("ipfs", gateway).Error(err)
			continue
		}
		return nil
	}
	return errors.Wrapf(errThumbnailCreating, "link=%s mime=%s", format.URI, format.MimeType)
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
