package adapter

import (
	"encoding/binary"
	"io/ioutil"
	"os"

	"github.com/dipdup-net/go-lib/config"
	"github.com/dipdup-net/metadata/cmd/metadata/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	stateFile = ".state"
)

// TzKT -
type TzKT struct {
	tzkt   *gorm.DB
	dipdup *gorm.DB

	updatedAt int
}

// NewTzKT -
func NewTzKT(connStringTzKT string, configDipDup config.Database) (*TzKT, error) {
	dipdup, err := models.OpenDatabaseConnection(configDipDup)
	if err != nil {
		return nil, err
	}

	tzkt, err := gorm.Open(postgres.Open(connStringTzKT), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	t := &TzKT{
		tzkt:   tzkt,
		dipdup: dipdup,
	}
	err = t.loadState()
	return t, err
}

// Do -
func (t *TzKT) Do() error {
	metadata, err := t.getContractMetadata()
	if err != nil {
		return err
	}

	if len(metadata) == 0 {
		return nil
	}

	return t.tzkt.Transaction(func(tx *gorm.DB) error {
		for i := range metadata {
			if err := tx.Table("Accounts").Where("Address = ?", metadata[i].Contract).Update("ContractMetadata", metadata[i].Metadata).Error; err != nil {
				return err
			}

			t.updatedAt = metadata[i].UpdatedAt
		}
		return nil
	})
}

// Close -
func (t *TzKT) Close() error {
	dd, err := t.dipdup.DB()
	if err != nil {
		return err
	}
	if err := dd.Close(); err != nil {
		return err
	}
	tzkt, err := t.tzkt.DB()
	if err != nil {
		return err
	}
	if err := tzkt.Close(); err != nil {
		return err
	}

	return t.dumpState()
}

func (t *TzKT) getContractMetadata() (all []models.ContractMetadata, err error) {
	query := t.dipdup.Model(&models.ContractMetadata{}).Where("status = ?", models.StatusApplied)
	if t.updatedAt > 0 {
		query.Where("updated_at > ?", t.updatedAt)
	}
	err = query.Order("updated_at asc").Limit(25).Find(&all).Error
	return
}

func (t *TzKT) dumpState() error {
	f, err := os.Open(stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		f, err = os.Create(stateFile)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	var data []byte
	binary.BigEndian.PutUint64(data, uint64(t.updatedAt))
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

func (t *TzKT) loadState() error {
	f, err := os.Open(stateFile)
	switch {
	case err == nil:
		defer f.Close()
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		t.updatedAt = int(binary.BigEndian.Uint64(data))
	case os.IsNotExist(err):
	default:
		return err
	}
	return nil
}
