package storage

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jetsea/tx-radar/internal/config"
	"github.com/jetsea/tx-radar/internal/models"
)

// BatchWriter is a batch writer for writing transactions to both
// PostgreSQL and InfluxDB.
type BatchWriter struct {
	postgres *PostgresStorage
	influx   *InfluxStorage
	cfg      config.BatchConfig

	buffer []*models.Transaction
	mu     sync.Mutex

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewBatchWriter creates a new batch writer.
func NewBatchWriter(postgres *PostgresStorage, influx *InfluxStorage, cfg config.BatchConfig) *BatchWriter {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 1 * time.Second
	}

	return &BatchWriter{
		postgres: postgres,
		influx:   influx,
		cfg:      cfg,
		buffer:   make([]*models.Transaction, 0, cfg.MaxSize),
		stopChan: make(chan struct{}),
	}
}

// Start starts the background flush loop.
func (b *BatchWriter) Start(ctx context.Context) {
	b.wg.Add(1)
	go b.flushLoop(ctx)
	log.Printf("Batch writer started (maxSize=%d, flushInterval=%v)", b.cfg.MaxSize, b.cfg.FlushInterval)
}

// Add adds a transaction to the buffer.
func (b *BatchWriter) Add(tx *models.Transaction) {
	b.mu.Lock()
	b.buffer = append(b.buffer, tx)
	shouldFlush := len(b.buffer) >= b.cfg.MaxSize
	b.mu.Unlock()

	if shouldFlush {
		b.Flush(context.Background())
	}
}

// Flush flushes the buffer.
func (b *BatchWriter) Flush(ctx context.Context) {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return
	}
	txs := b.buffer
	b.buffer = make([]*models.Transaction, 0, b.cfg.MaxSize)
	b.mu.Unlock()

	// Write to PostgreSQL (cold data).
	if b.postgres != nil {
		if err := b.postgres.SaveBatch(ctx, txs); err != nil {
			log.Printf("Failed to write batch to PostgreSQL: %v", err)
		}
	}

	// Write to InfluxDB (hot data).
	if b.influx != nil {
		if err := b.influx.SaveBatch(ctx, txs); err != nil {
			log.Printf("Failed to write batch to InfluxDB: %v", err)
		}
	}
}

// flushLoop periodically flushes the buffer.
func (b *BatchWriter) flushLoop(ctx context.Context) {
	defer b.wg.Done()

	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.Flush(context.Background()) // final flush before exit
			return
		case <-b.stopChan:
			b.Flush(context.Background())
			return
		case <-ticker.C:
			b.Flush(ctx)
		}
	}
}

// Stop stops the batch writer.
func (b *BatchWriter) Stop() {
	close(b.stopChan)
	b.wg.Wait()
}
