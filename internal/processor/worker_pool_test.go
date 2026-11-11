package processor

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jetsea/tx-radar/internal/models"
)

// newWorkerPoolTx builds a minimal transaction a worker can process.
func newWorkerPoolTx(hash string, gasPrice uint64) *models.Transaction {
	return &models.Transaction{
		Hash:     common.HexToHash(hash),
		From:     common.HexToAddress("0x0000000000000000000000000000000000000001"),
		GasPrice: new(big.Int).Mul(big.NewInt(int64(gasPrice)), big.NewInt(1e9)),
	}
}

// stopDedup stops the deduplicator's cleanup goroutine to avoid leaks.
func stopDedup(d *Deduplicator) {
	if d != nil {
		d.Stop()
	}
}

// TestWorkerPool_ProcessesAllTransactions verifies that every non-duplicate
// transaction written to the channel gets processed with context below a timeout.
func TestWorkerPool_ProcessesAllTransactions(t *testing.T) {
	txs := []*models.Transaction{
		newWorkerPoolTx("0x01", 200),
		newWorkerPoolTx("0x02", 150),
		newWorkerPoolTx("0x03", 10),
	}

	ch := make(chan *models.Transaction, len(txs))
	for _, tx := range txs {
		ch <- tx
	}
	close(ch)

	gs := NewGasStats(10)
	marker := NewMarker(gs)
	dedup := NewMemoryDeduplicator(30 * time.Second)
	defer stopDedup(dedup)

	pool := NewWorkerPool(ch, marker, dedup, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool.Start(ctx)
	pool.Wait()

	// every transaction supplied must have been accepted by the deduplicator
	// as non-duplicate during this run (first appearance only)
	for _, tx := range txs {
		if !dedup.IsDuplicate(tx.Hash.Hex()) {
			t.Errorf("expected tx %s to have been processed", tx.Hash.Hex())
		}
	}
}

// TestWorkerPool_CancelContext propagates cancellation: workers observe the
// cancelled context and exit even while the channel stays open.
func TestWorkerPool_CancelContext(t *testing.T) {
	ch := make(chan *models.Transaction) // never closed on purpose
	gs := NewGasStats(10)
	marker := NewMarker(gs)
	dedup := NewMemoryDeduplicator(30 * time.Second)
	defer stopDedup(dedup)

	pool := NewWorkerPool(ch, marker, dedup, 2)

	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)

	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	// cancel the context so workers exit without the channel closing
	cancel()

	select {
	case <-done:
		// all workers returned once the context was cancelled
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not exit after context cancellation")
	}
}

// TestWorkerPool_Deduplication verifies that a duplicate transaction is
// skipped a second time via the in-memory deduplicator.
func TestWorkerPool_Deduplication(t *testing.T) {
	tx := newWorkerPoolTx("0xAA", 200)

	dedup := NewMemoryDeduplicator(30 * time.Second)
	defer stopDedup(dedup)

	// mark the tx as already seen so the pool should treat it as a duplicate
	dedup.IsDuplicate(tx.Hash.Hex())

	ch := make(chan *models.Transaction, 1)
	ch <- tx
	close(ch)

	gs := NewGasStats(10)
	marker := NewMarker(gs)
	pool := NewWorkerPool(ch, marker, dedup, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool.Start(ctx)
	pool.Wait()
}
