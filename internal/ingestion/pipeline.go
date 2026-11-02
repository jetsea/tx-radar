package ingestion

import (
	"log"
	"sync/atomic"

	"github.com/jetsea/tx-radar/internal/models"
)

// Pipeline is the data ingestion pipeline
// responsible for managing the Channel and transaction normalization
type Pipeline struct {
	// TxChan is the channel for normalized transactions (buffer 10000)
	TxChan chan *models.Transaction

	// statistics counters
	totalReceived atomic.Uint64 // number of raw transactions received
	totalSent     atomic.Uint64 // number successfully sent to the Channel
	totalDropped  atomic.Uint64 // number dropped because the Channel was full
	totalFailed   atomic.Uint64 // number that failed normalization

}

// NewPipeline creates a data ingestion pipeline
func NewPipeline(bufferSize int) *Pipeline {
	return &Pipeline{
		TxChan: make(chan *models.Transaction, bufferSize),
	}
}

// Emit normalizes the raw transaction and writes it to the Channel
// rawTx: the raw go-ethereum transaction
// source: "mempool" or "block"
// blockNumber: block number (pass 0 for Mempool sources)
func (p *Pipeline) Emit(internalTx *models.Transaction) {
	p.totalReceived.Add(1)
	// 2. Non-blocking write to the Channel
	select {
	case p.TxChan <- internalTx:
		p.totalSent.Add(1)
		count := p.totalSent.Load()
		if count%1000 == 0 {
			log.Printf("📊 Total transactions processed: %d (received: %d, dropped: %d, failed: %d)",
				count,
				p.totalReceived.Load(),
				p.totalDropped.Load(),
				p.totalFailed.Load(),
			)
		}
	default:
		// Channel is full, drop it and count
		p.totalDropped.Add(1)
		if p.totalDropped.Load()%100 == 0 {
			log.Printf("⚠️ Channel is full, total dropped: %d", p.totalDropped.Load())
		}
	}
}
func (p *Pipeline) RecordFailure() {
	p.totalFailed.Add(1)
}

// Close closes the Channel (called by main on exit)
func (p *Pipeline) Close() {
	close(p.TxChan)
}

// Stats returns the current statistics
func (p *Pipeline) Stats() (received, sent, dropped, failed uint64) {
	return p.totalReceived.Load(),
		p.totalSent.Load(),
		p.totalDropped.Load(),
		p.totalFailed.Load()
}
