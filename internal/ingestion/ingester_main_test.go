package ingestion

import (
	"context"
	"testing"

	"github.com/jetsea/tx-radar/internal/config"
)

// ============================================================
// Ingestion loops — configuration guards
// ============================================================

func TestRunMempoolLoop_EmptyRPCURLs(t *testing.T) {
	cfg := config.IngestionConfig{RPCURLs: []string{}}
	err := RunMempoolLoop(context.Background(), cfg, &MempoolIngester{}, NewPipeline(10))
	if err == nil {
		t.Fatal("expected error for empty RPCURLs, got nil")
	}
}

func TestRunBlockLoop_EmptyRPCURLs(t *testing.T) {
	cfg := config.IngestionConfig{RPCURLs: []string{}}
	err := RunBlockLoop(context.Background(), cfg, &BlockIngester{}, NewPipeline(10))
	if err == nil {
		t.Fatal("expected error for empty RPCURLs, got nil")
	}
}
