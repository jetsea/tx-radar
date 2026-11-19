package storage

import (
	"context"

	"github.com/jetsea/tx-radar/internal/models"
)

// Storage is the unified storage layer interface.
// All storage backends (PostgreSQL, InfluxDB) implement this interface.
type Storage interface {
	// Save stores a single transaction.
	Save(ctx context.Context, tx *models.Transaction) error

	// SaveBatch stores transactions in bulk.
	SaveBatch(ctx context.Context, txs []*models.Transaction) error

	// Close closes the storage connection.
	Close() error
}
