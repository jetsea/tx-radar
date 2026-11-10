package processor

import (
	"strings"

	"github.com/jetsea/tx-radar/internal/models"
)

// Function signature constants (hex of the first 4 bytes of Data, without the 0x prefix)
const (
	// Uniswap V2
	SigSwapExactTokensForTokens = "38ed1739"
	SigSwapExactETHForTokens    = "7ff36ab5"
	SigSwapExactTokensForETH    = "18cbafe5"
	SigSwapETHForExactTokens    = "fb3bdb41"

	// Uniswap V3
	SigUniswapV3Swap       = "128acb08"
	SigUniswapV3ExactInput = "c04b8d59"

	// SushiSwap
	SigSushiSwap = "38ed1739" // same as Uniswap V2

	// Aave
	SigLiquidationCall = "00f714ce"

	// Compound
	SigLiquidateBorrow = "02c42981"

	// NFT
	SigMint   = "1249c58b"
	SigMintTo = "449a52f8"

	// ERC-20
	SigTransfer = "a9059cbb"
	SigApprove  = "095ea7b3"
)

// TxType is the transaction type
type TxType string

const (
	TxTypeDexSwap     TxType = "DEX_SWAP"
	TxTypeLiquidation TxType = "LIQUIDATION"
	TxTypeNFTMint     TxType = "NFT_MINT"
	TxTypeTransfer    TxType = "TRANSFER"
	TxTypeApprove     TxType = "APPROVE"
	TxTypeUnknown     TxType = "UNKNOWN"
)

// Known high-value contract addresses (examples, extensible)
var highValueContracts = map[string]string{
	"0x7a250d5630b4cf539739df2c5dacb4c659f2488d": "Uniswap V2 Router",
	"0xe592427a0aece92de3edee1f18e0157c05861564": "Uniswap V3 Router",
	"0xd9e1ce17f2641f24ae83637ab66a2cca9c378b9f": "SushiSwap Router",
	"0x7d2768de32b0b80b7a3454c06bdac94a69ddc7a9": "Aave Lending Pool",
	"0x3d9819210a31b4961b30ef54be2aed79b9c9cd3b": "Compound Comptroller",
}

// IdentifyTxType identifies the transaction type based on Data and To
func IdentifyTxType(tx *models.Transaction) string {
	if len(tx.Data) < 4 {
		// no Data or fewer than 4 bytes → plain transfer or contract deployment
		if tx.To == nil {
			return "CONTRACT_DEPLOY"
		}
		return string(TxTypeTransfer)
	}

	switch tx.MethodID {
	case SigSwapExactTokensForTokens, SigSwapExactETHForTokens, SigSwapExactTokensForETH, SigSwapETHForExactTokens:
		return string(TxTypeDexSwap)
	case SigUniswapV3Swap, SigUniswapV3ExactInput:
		return string(TxTypeDexSwap)
	case SigLiquidationCall, SigLiquidateBorrow: //SigLiquidationCall-Aave, SigLiquidateBorrow-Compound
		return string(TxTypeLiquidation)
	case SigMint, SigMintTo: //SigMint-for msg.sender, SigMintTo-for other address
		return string(TxTypeNFTMint)
	case SigTransfer: //SigTransfer-ERC-20 transfer
		return string(TxTypeTransfer)
	case SigApprove: //SigApprove-ERC-20 approve
		return string(TxTypeApprove)
	}

	return string(TxTypeUnknown)
}

// IsDexSwap reports whether the transaction is a DEX swap
func IsDexSwap(tx *models.Transaction) bool {
	return strings.Contains(tx.TxType, "DEX_SWAP")
}

// IsLiquidation reports whether the transaction is a liquidation
func IsLiquidation(tx *models.Transaction) bool {
	return strings.Contains(tx.TxType, "LIQUIDATION")
}

// IsHighValueContract reports whether the To address is a known high-value contract
func IsHighValueContract(tx *models.Transaction) (string, bool) {
	if tx.To == nil {
		return "", false
	}
	name, ok := highValueContracts[strings.ToLower(tx.To.Hex())]
	return name, ok
}
