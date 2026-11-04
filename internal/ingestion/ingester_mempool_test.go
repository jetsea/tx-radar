package ingestion

import (
	"testing"

	"github.com/jetsea/tx-radar/internal/models"
)

func TestMempoolIngester_Name(t *testing.T) {
	if got := (&MempoolIngester{}).Name(); got != models.SourceMempool {
		t.Fatalf("MempoolIngester.Name() = %q, want %q", got, models.SourceMempool)
	}
}
