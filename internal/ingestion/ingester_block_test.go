package ingestion

import (
	"testing"

	"github.com/jetsea/tx-radar/internal/models"
)

func TestBlockIngester_Name(t *testing.T) {
	if got := (&BlockIngester{}).Name(); got != models.SourceBlock {
		t.Fatalf("BlockIngester.Name() = %q, want %q", got, models.SourceBlock)
	}
}
