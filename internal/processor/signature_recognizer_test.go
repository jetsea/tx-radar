package processor

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jetsea/tx-radar/internal/models"
)

func newTestTx(to *common.Address, methodID string, valueWei int64, gasPriceWei int64) *models.Transaction {
	data, _ := hex.DecodeString(methodID)
	return &models.Transaction{
		To:       to,
		MethodID: methodID,
		Data:     data, //make data length >= 4
		Value:    big.NewInt(valueWei),
		GasPrice: big.NewInt(gasPriceWei),
		TxType:   string(TxTypeUnknown),
	}
}

func TestIdentifyTxType_SwapSelectors(t *testing.T) {
	cases := []struct {
		name     string
		methodID string
		want     string
	}{
		{"v2 exact tokens for tokens", SigSwapExactTokensForTokens, string(TxTypeDexSwap)},
		{"v2 exact ETH for tokens", SigSwapExactETHForTokens, string(TxTypeDexSwap)},
		{"v2 exact tokens for ETH", SigSwapExactTokensForETH, string(TxTypeDexSwap)},
		{"v2 ETH for exact tokens", SigSwapETHForExactTokens, string(TxTypeDexSwap)},
		{"v3 swap", SigUniswapV3Swap, string(TxTypeDexSwap)},
		{"v3 exact input", SigUniswapV3ExactInput, string(TxTypeDexSwap)},
		{"sushiswap reuse v2 sig", SigSushiSwap, string(TxTypeDexSwap)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tx := newTestTx(&common.Address{}, c.methodID, 0, 0)
			if got := IdentifyTxType(tx); got != c.want {
				t.Fatalf("got %q, want %q, methodID: %q", got, c.want, tx.MethodID)
			}
		})
	}
}

func TestIdentifyTxType_LiquidationNFTTransfer(t *testing.T) {
	cases := []struct {
		name     string
		methodID string
		want     string
	}{
		{"aave liquidation", SigLiquidationCall, string(TxTypeLiquidation)},
		{"compound liquidation", SigLiquidateBorrow, string(TxTypeLiquidation)},
		{"sushi sig maps to swap not liquidation", "38ed1739", string(TxTypeDexSwap)},
		{"erc20 transfer", SigTransfer, string(TxTypeTransfer)},
		{"erc20 approve", SigApprove, string(TxTypeApprove)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tx := newTestTx(&common.Address{}, c.methodID, 0, 0)
			if got := IdentifyTxType(tx); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestIdentifyTxType_UnknownSelector(t *testing.T) {
	tx := newTestTx(&common.Address{}, "unknownmethodID", 0, 0)
	if got := IdentifyTxType(tx); got != string(TxTypeUnknown) {
		t.Fatalf("got %q, want %q", got, string(TxTypeUnknown))
	}
}

func TestIdentifyTxType_ContractDeployment(t *testing.T) {
	// no Data and To == nil => CONTRACT_DEPLOY
	tx := newTestTx(nil, "", 0, 0)
	if got := IdentifyTxType(tx); got != "CONTRACT_DEPLOY" {
		t.Fatalf("got %q, want CONTRACT_DEPLOY", got)
	}
}

func TestIdentifyTxType_PlainTransfer(t *testing.T) {
	// no Data but To != nil => plain TRANSFER
	tx := newTestTx(&common.Address{}, "", 0, 0)
	if got := IdentifyTxType(tx); got != string(TxTypeTransfer) {
		t.Fatalf("got %q, want %q", got, string(TxTypeTransfer))
	}
}

func TestIsDexSwap(t *testing.T) {
	if !IsDexSwap(&models.Transaction{TxType: "DEX_SWAP"}) {
		t.Fatal("expected DEX_SWAP to be recognized")
	}
	// substring check, so suffixed types still match
	if !IsDexSwap(&models.Transaction{TxType: "DEX_SWAP [Uniswap V2 Router]"}) {
		t.Fatal("expected suffixed DEX_SWAP to be recognized")
	}
	if IsDexSwap(&models.Transaction{TxType: "TRANSFER"}) {
		t.Fatal("expected TRANSFER not to be a swap")
	}
	if IsDexSwap(&models.Transaction{}) {
		t.Fatal("expected empty tx not to be a swap")
	}
}

func TestIsLiquidation(t *testing.T) {
	if !IsLiquidation(&models.Transaction{TxType: "LIQUIDATION"}) {
		t.Fatal("expected LIQUIDATION to be recognized")
	}
	if IsLiquidation(&models.Transaction{TxType: "DEX_SWAP"}) {
		t.Fatal("expected non-liquidation not to match")
	}
}

func TestIsHighValueContract(t *testing.T) {
	router := common.HexToAddress("0x7a250d5630b4cf539739df2c5dacb4c659f2488d")
	name, ok := IsHighValueContract(&models.Transaction{To: &router})
	if !ok || name != "Uniswap V2 Router" {
		t.Fatalf("got %q,%v", name, ok)
	}
}

func TestIsHighValueContract_CaseInsensitive(t *testing.T) {
	// synthesized address must exist in map after ToLower
	router := common.HexToAddress("0xd9e1ce17f2641f24ae83637ab66a2cca9c378b9f")
	if _, ok := IsHighValueContract(&models.Transaction{To: &router}); !ok {
		t.Fatal("expected SushiSwap Router to be recognized")
	}
}

func TestIsHighValueContract_UnknownAndNil(t *testing.T) {
	if _, ok := IsHighValueContract(&models.Transaction{}); ok {
		t.Fatal("expected nil To to return ok=false")
	}
	unknown := common.HexToAddress("0x0000000000000000000000000000000000000123")
	if _, ok := IsHighValueContract(&models.Transaction{To: &unknown}); ok {
		t.Fatal("expected unknown address to return ok=false")
	}
}

var _ = strings.Contains // keep strings import if unused elsewhere
