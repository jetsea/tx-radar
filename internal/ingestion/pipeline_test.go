package ingestion

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jetsea/tx-radar/internal/models"
)

func sampleTx(i int) *models.Transaction {
	return &models.Transaction{
		Hash:        common.BytesToHash([]byte{byte(i)}),
		Source:      models.SourceMempool,
		BlockNumber: 0,
	}
}

func TestPipeline_EmitAndStats(t *testing.T) {
	p := NewPipeline(10)

	const n = 5
	for i := 0; i < n; i++ {
		p.Emit(sampleTx(i))
	}

	received, sent, dropped, failed := p.Stats()
	if received != uint64(n) {
		t.Fatalf("received = %d, want %d", received, n)
	}
	if sent != uint64(n) {
		t.Fatalf("sent = %d, want %d", sent, n)
	}
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	if failed != 0 {
		t.Fatalf("failed = %d, want 0", failed)
	}
	if got := len(p.TxChan); got != n {
		t.Fatalf("TxChan len = %d, want %d", got, n)
	}
}

func TestPipeline_EmitDropsWhenFull(t *testing.T) {
	buffer := 2
	p := NewPipeline(buffer)

	const n = 5
	for i := 0; i < n; i++ {
		p.Emit(sampleTx(i))
	}

	received, sent, dropped, failed := p.Stats()
	if received != uint64(n) {
		t.Fatalf("received = %d, want %d", received, n)
	}
	if sent != uint64(buffer) {
		t.Fatalf("sent = %d, want %d (buffer size)", sent, buffer)
	}
	if dropped != uint64(n-buffer) {
		t.Fatalf("dropped = %d, want %d", dropped, n-buffer)
	}
	if failed != 0 {
		t.Fatalf("failed = %d, want 0", failed)
	}
}

func TestPipeline_RecordFailure(t *testing.T) {
	p := NewPipeline(8)
	p.RecordFailure()
	p.RecordFailure()

	_, _, _, failed := p.Stats()
	if failed != 2 {
		t.Fatalf("failed = %d, want 2", failed)
	}
}

func TestPipeline_Close(t *testing.T) {
	p := NewPipeline(4)
	p.Emit(sampleTx(1))
	p.Close()

	select {
	case tx, ok := <-p.TxChan:
		if !ok {
			t.Fatal("TxChan closed prematurely")
		}
		if tx.Hash != sampleTx(1).Hash {
			t.Fatalf("unexpected tx: %v", tx.Hash)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out reading from TxChan")
	}
}
