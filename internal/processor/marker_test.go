package processor

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jetsea/tx-radar/internal/models"
)

func newMarkerTx(gasPrice uint64, valueETH int64, txType string, to *common.Address) *models.Transaction {
	return &models.Transaction{
		To:       to,
		Value:    new(big.Int).Mul(big.NewInt(valueETH), big.NewInt(1e18)),       //1ETH=1e18 wei
		GasPrice: new(big.Int).Mul(big.NewInt(int64(gasPrice)), big.NewInt(1e9)), //1Gwei=1e9 wei
		TxType:   txType,
	}
}

// evaluate runs Mark with enough samples so P95 is meaningful.
func evaluate(t *testing.T, tx *models.Transaction, window int) models.Level {
	t.Helper()
	gs := NewGasStats(window)
	// prefill a stable baseline so the target tx's P95 rule is comparable
	for i := 0; i < window; i++ {
		gs.Add(20) // baseline gas price 20 Gwei
	}
	m := NewMarker(gs)

	m.Mark(tx)
	return tx.Level
}

func TestMarker_LiquidationOnly(t *testing.T) {
	// liquidation (+4) => MEDIUM at low gas
	tx := newMarkerTx(10, 0, string(TxTypeLiquidation), nil)
	if lv := evaluate(t, tx, 10); lv != models.LevelMedium {
		t.Fatalf("got %v, want MEDIUM", lv)
	}
}

func TestMarker_DexSwapOnly(t *testing.T) {
	// DEX swap (+3) => MEDIUM at low gas
	tx := newMarkerTx(10, 0, string(TxTypeDexSwap), nil)
	if lv := evaluate(t, tx, 10); lv != models.LevelMedium {
		t.Fatalf("got %v, want MEDIUM", lv)
	}
}

func TestMarker_PlainTransferLow(t *testing.T) {
	// plain transfer, cheap gas, small amount => LOW
	tx := newMarkerTx(10, 5, string(TxTypeTransfer), &common.Address{})
	if lv := evaluate(t, tx, 10); lv != models.LevelLow {
		t.Fatalf("got %v, want LOW", lv)
	}
}

func TestMarker_HighGasDexSwap(t *testing.T) {
	// high gas 200 (>=100) => +3, plus a swap => +3 => >=6 => HIGH
	tx := newMarkerTx(200, 0, string(TxTypeDexSwap), nil)
	if lv := evaluate(t, tx, 10); lv != models.LevelHigh {
		t.Fatalf("got %v, want HIGH", lv)
	}
}

func TestMarker_HighValueTransfer(t *testing.T) {
	// value 500 ETH => +3, low gas => total 3 => MEDIUM
	tx := newMarkerTx(10, 500, string(TxTypeTransfer), &common.Address{})
	if lv := evaluate(t, tx, 10); lv != models.LevelMedium {
		t.Fatalf("got %v, want MEDIUM", lv)
	}
}

func TestMarker_HighValueContractBoost(t *testing.T) {
	// known high-value contract => +2, and a transfer with low value => total 2 => MEDIUM (>=3 required)
	router := common.HexToAddress("0x7a250d5630b4cf539739df2c5dacb4c659f2488d")
	tx := newMarkerTx(10, 0, string(TxTypeDexSwap), &router)
	// swap should dominate => >=3
	if lv := evaluate(t, tx, 10); lv != models.LevelMedium {
		t.Fatalf("got %v, want MEDIUM", lv)
	}
}
