package processor

import (
	"github.com/jetsea/tx-radar/internal/models"
)

// Level is the transaction level
// TxType is the transaction type
type Level string

const (
	LevelHigh   Level = "HIGH"
	LevelMedium Level = "MEDIUM"
	LevelLow    Level = "LOW"
)

// Marker marks transactions of high value
type Marker struct {
	gasStats *GasStats

	// threshold configuration
	minHighGasGwei   uint64  // minimum GasPrice for high value (Gwei)
	minMediumGasGwei uint64  // minimum GasPrice for medium value (Gwei)
	minHighValueETH  float64 // minimum transfer amount for high value (ETH)
}

// NewMarker creates a new marker
func NewMarker(gasStats *GasStats) *Marker {
	return &Marker{
		gasStats:         gasStats,
		minHighGasGwei:   100, // GasPrice above 100 Gwei is high value
		minMediumGasGwei: 30,  // GasPrice above 30 Gwei is medium value
		minHighValueETH:  100, // amount above 100 ETH is high value
	}
}

// Mark assigns a level to the transaction
func (m *Marker) Mark(tx *models.Transaction) {
	// 1. record the GasPrice into the sliding window
	m.gasStats.Add(tx.GasPriceGwei())

	// 2. compute the P95 percentile
	p95 := m.gasStats.P95()

	// 3. composite score
	score := 0

	// Rule 1: extremely high GasPrice
	gasPrice := tx.GasPriceGwei()
	if gasPrice > p95 && p95 > 0 {
		score += 3
	}
	if gasPrice >= m.minHighGasGwei {
		score += 3
	} else if gasPrice >= m.minMediumGasGwei {
		score += 1
	}

	// Rule 2: DEX swap (arbitrage signal)
	if IsDexSwap(tx) {
		score += 3
	}

	// Rule 3: liquidation transaction
	if IsLiquidation(tx) {
		score += 4
	}

	// Rule 4: NFT mint
	if tx.TxType == string(TxTypeNFTMint) {
		score += 2
	}

	// Rule 5: call to a known high-value contract
	if name, ok := IsHighValueContract(tx); ok {
		score += 2
		tx.TxType = tx.TxType + " [" + name + "]"
	}

	// Rule 6: large-value transfer
	if tx.ValueEther() >= m.minHighValueETH {
		score += 3
	}

	// 4. assign a level based on the score
	switch {
	case score >= 6:
		tx.Level = models.LevelHigh
		tx.IsHighValue = true
	case score >= 3:
		tx.Level = models.LevelMedium
	default:
		tx.Level = models.LevelLow
	}
}
