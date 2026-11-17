package storage

import (
	"context"
	"fmt"
	"log"

	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jetsea/tx-radar/internal/config"
	"github.com/jetsea/tx-radar/internal/models"
)

type Cleaner interface {
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// PostgresStorage is the PostgreSQL storage backend.
type PostgresStorage struct {
	pool *pgxpool.Pool
}

// NewPostgresStorage creates a PostgreSQL storage backend.
func NewPostgresStorage(ctx context.Context, cfg config.PostgresConfig) (*PostgresStorage, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	if cfg.MaxConnections > 0 {
		poolCfg.MaxConns = int32(cfg.MaxConnections)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Verify the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("PostgreSQL ping failed: %w", err)
	}

	log.Println("✅ PostgreSQL connected")
	return &PostgresStorage{pool: pool}, nil
}

// Save stores a single transaction.
func (p *PostgresStorage) Save(ctx context.Context, tx *models.Transaction) error {
	query := `
		INSERT INTO transactions (
			hash, from_addr, to_addr, value, nonce, gas_price, gas_limit,
			gas_tip_cap, gas_fee_cap, data, chain_id, source, block_number,
			timestamp, tx_type, method_id, level, is_high_value
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18
		) ON CONFLICT (hash) DO NOTHING
	`

	toAddr := ""
	if tx.To != nil {
		toAddr = tx.To.Hex()
	}

	_, err := p.pool.Exec(ctx, query,
		tx.Hash.Hex(),
		tx.From.Hex(),
		toAddr,
		tx.Value.String(),
		tx.Nonce,
		tx.GasPrice.String(),
		tx.GasLimit,
		tx.GasTipCap.String(),
		tx.GasFeeCap.String(),
		tx.Data,
		tx.ChainID.String(),
		string(tx.Source),
		tx.BlockNumber,
		tx.Timestamp,
		tx.TxType,
		tx.MethodID,
		tx.Level,
		tx.IsHighValue,
	)

	return err
}

// SaveBatch stores transactions in bulk.
func (p *PostgresStorage) SaveBatch(ctx context.Context, txs []*models.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	// Use COPY or batched INSERT.
	// Simplified to per-row inserts here (pgx.CopyFrom is recommended in production).
	for _, tx := range txs {
		if err := p.Save(ctx, tx); err != nil {
			log.Printf("⚠️ PostgreSQL save failed: %v", err)
		}
	}
	return nil
}

// Close closes the connection pool.
func (p *PostgresStorage) Close() error {
	p.pool.Close()
	return nil
}

func (p *PostgresStorage) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	query := `DELETE FROM transactions WHERE timestamp < $1`
	result, err := p.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
