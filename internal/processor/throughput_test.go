package processor

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jetsea/tx-radar/internal/ingestion"
	"github.com/jetsea/tx-radar/internal/models"
)

func synthTx(i int) *models.Transaction {
	h := common.BytesToHash([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24), 0xAA, 0xBB, 0xCC})
	return &models.Transaction{
		Hash:     h,
		Value:    big.NewInt(1e17), // 0.1 ETH
		GasPrice: big.NewInt(50e9), // 50 Gwei -> LOW level, avoids log spam
		GasLimit: 21000,
		Source:   models.SourceMempool,
		Data:     []byte{0x11, 0x22, 0x33, 0x44},
	}
}

// newConsumer assembles the consumer side: deduplicator + marker + worker pool.
// It wires the pipeline's read channel into a worker pool so that transactions
// emitted upstream are deduplicated, tagged, and processed concurrently.
func newConsumer(pipe *ingestion.Pipeline) *WorkerPool {
	// 30-minute TTL window: drops duplicate hashes seen within the last 30 min.
	dedup := NewMemoryDeduplicator(30 * time.Minute)
	// Marker scores each tx; GasStats tracks the latest 10,000 gas prices
	// so the marker can classify tx by gas price level.
	marker := NewMarker(NewGasStats(10_000))
	// workers=0 signals the pool to default to NumCPU*2 goroutines.
	// pipe.TxChan is the buffered ingest channel the workers drain from.
	return NewWorkerPool(pipe.TxChan, marker, dedup, 0)
}

// throughputReport reports ingest and consume TPS separately
type throughputReport struct {
	ingestTPS  float64 // production ingest throughput (synchronous send)
	consumeTPS float64 // worker pool consume throughput
	sent       uint64  // successfully enqueued
	dropped    uint64  // dropped
	failed     uint64  // failed
}

// runThroughput ingests pre-built TXs and times the ingest and consume sides separately
// returns (report, drop rate)
func runThroughput(txes []*models.Transaction, producers int) (throughputReport, float64) {
	// --- Production phase: large buffer + no workers, purely tests ingest cap ---
	total := len(txes)
	pipe := ingestion.NewPipeline(total) // cap >= total, guarantees no drop
	var wg sync.WaitGroup
	injectStart := time.Now()
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := base; i < total; i += producers {
				pipe.Emit(txes[i])
			}
		}(p)
	}
	wg.Wait()
	injectElapsed := time.Since(injectStart)

	// --- Consume phase: start workers after closing the channel, drain the buffer ---
	pipe.Close() // closed channel buffer is still readable; workers exit after draining
	wp := newConsumer(pipe)
	consumeStart := time.Now()
	wp.Start(context.Background())
	wp.Wait()
	consumeElapsed := time.Since(consumeStart)

	_, sent, dropped, failed := pipe.Stats()
	ingestTPS := float64(total) / injectElapsed.Seconds()
	consumeTPS := float64(sent) / consumeElapsed.Seconds() // tx consumed during drain / drain time
	produced := sent + dropped
	dropRate := 0.0
	if produced > 0 {
		dropRate = float64(dropped) / float64(produced) * 100
	}

	return throughputReport{
		ingestTPS:  ingestTPS,
		consumeTPS: consumeTPS,
		sent:       sent,
		dropped:    dropped,
		failed:     failed,
	}, dropRate
}

// TestThroughput5000TPS ingests N transactions and verifies pipeline throughput and drop rate
func TestThroughput5000TPS(t *testing.T) {
	const total = 100_000

	// Pre-build TXs; construction cost is not counted in the timing
	txes := make([]*models.Transaction, total)
	for i := range txes {
		txes[i] = synthTx(i)
	}

	rep, dropRate := runThroughput(txes, 8)
	consumeTPS := rep.consumeTPS
	fmt.Printf("=> enqueued=%d dropped=%d failed=%d dropRate=%.2f%%\n",
		rep.sent, rep.dropped, rep.failed, dropRate)
	fmt.Printf("=> ingest throughput: %.0f TPS | consume throughput: %.0f TPS (workers=%d)\n",
		rep.ingestTPS, consumeTPS, runtime.NumCPU()*2)

	if rep.dropped > 0 {
		t.Logf("⚠️ dropped %d (%.2f%%): buffer backpressure, production outpaces consumption", rep.dropped, dropRate)
	}
	if consumeTPS < 5000 {
		t.Errorf("consume throughput below 5000 TPS (%.0f)", consumeTPS)
	}
}

// 24 core cpu, 48 goroutines, 70k tx/s, 1.4us/tx, 0.4 Heap MB/tx, 10 allocs/tx
func BenchmarkWorkerPoolConsume(b *testing.B) {
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(os.Stderr) })

	const batch = 10_000

	for b.Loop() {
		b.StopTimer()
		pipe := ingestion.NewPipeline(batch)
		for j := 0; j < batch; j++ { // 10k per op
			pipe.Emit(synthTx(j))
		}
		pipe.Close()

		dedup := NewMemoryDeduplicator(30 * time.Minute)
		wp := NewWorkerPool(pipe.TxChan, NewMarker(NewGasStats(10_000)), dedup, 0)

		b.StartTimer()
		wp.Start(context.Background())
		wp.Wait()

		b.StopTimer()
		dedup.Stop()
		b.StartTimer()
	}

	total := uint64(b.N) * batch
	elapsedNs := b.Elapsed().Nanoseconds()
	fmt.Printf("=> consume throughput: %.0f tx/s | per tx %.1f ns\n",
		float64(total)*1e9/float64(elapsedNs),
		float64(elapsedNs)/float64(total))
}
