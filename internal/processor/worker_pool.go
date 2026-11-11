package processor

import (
	"context"
	"log"
	"runtime"
	"sync"

	"github.com/jetsea/tx-radar/internal/models"
)

// WorkerPool manages a pool of consumer workers
type WorkerPool struct {
	txChan  <-chan *models.Transaction
	marker  *Marker
	dedup   *Deduplicator
	workers int
	wg      sync.WaitGroup
}

// NewWorkerPool creates a new worker pool
// when workers <= 0, defaults to runtime.NumCPU() * 2
func NewWorkerPool(txChan <-chan *models.Transaction, marker *Marker, dedup *Deduplicator, workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}
	return &WorkerPool{
		txChan:  txChan,
		marker:  marker,
		dedup:   dedup,
		workers: workers,
	}
}

// Start launches all workers
func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
	log.Printf("👷 Worker pool started with %d workers", wp.workers)
}

// Wait blocks until all workers exit
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

// worker is a single consumer goroutine
func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()
	log.Printf("👷 Worker #%d started", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("👷 Worker #%d exiting...", id)
			return
		case tx, ok := <-wp.txChan:
			if !ok {
				log.Printf("👷 Worker #%d channel closed, exiting", id)
				return
			}
			wp.process(id, tx)
		}
	}
}

// process handles a single transaction
func (wp *WorkerPool) process(workerID int, tx *models.Transaction) {
	// 1. deduplication check
	if wp.dedup != nil && wp.dedup.IsDuplicate(tx.Hash.Hex()) {
		log.Printf("Duplicate transaction %s detected, skipping", tx.Hash.Hex())
		return // already processed, skip
	}

	// 2. parse the transaction (fill derived TxType fields)
	tx.TxType = IdentifyTxType(tx)

	// 3. mark the transaction level
	wp.marker.Mark(tx)

	// 4. log based on level
	switch tx.Level {
	case models.LevelHigh:
		log.Printf("🔥 [HIGH] %s %s detected (gas=%d Gwei, from=%s)",
			tx.Hash.Hex(), tx.TxType, tx.GasPriceGwei(), tx.From.Hex())
	case models.LevelMedium:
		log.Printf("⚡ [MEDIUM] %s %s (gas=%d Gwei)",
			tx.Hash.Hex(), tx.TxType, tx.GasPriceGwei())
	case models.LevelLow:
		// low-value transactions are not logged to avoid spam
	}
}
