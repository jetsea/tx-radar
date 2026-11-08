package processor

import (
	"sort"
	"sync"
)

// GasStats maintains a sliding window of GasPrices
type GasStats struct {
	mu      sync.RWMutex
	prices  []uint64 // stores GasPrices (in Gwei)
	maxSize int
}

// NewGasStats creates a new stats instance
func NewGasStats(maxSize int) *GasStats {
	return &GasStats{
		prices:  make([]uint64, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add records a transaction's GasPrice (in Gwei)
func (g *GasStats) Add(priceGwei uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.prices = append(g.prices, priceGwei)
	if len(g.prices) > g.maxSize {
		g.prices = g.prices[len(g.prices)-g.maxSize:]
	}
}

// P95 returns the current P95 percentile
func (g *GasStats) P95() uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.prices) == 0 {
		return 0
	}

	sorted := make([]uint64, len(g.prices))
	// make a copy to avoid modifying the original slice, because original slice should be in order of arrival, not sorted order
	copy(sorted, g.prices)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(float64(len(sorted)) * 0.95)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Count returns the number of samples in the current window
func (g *GasStats) Count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.prices)
}
