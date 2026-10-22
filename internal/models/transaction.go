package models

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Source identifies the transaction source
type Source string
type Level string

const (
	SourceMempool Source = "mempool"
	SourceBlock   Source = "block"
)
const (
	LevelHigh   Level = "HIGH"
	LevelMedium Level = "MEDIUM"
	LevelLow    Level = "LOW"
)

// Transaction is the internal unified transaction structure
// Regardless of whether it comes from Mempool or Block, it is converted to this structure
type Transaction struct {
	// ===== base fields =====
	Hash      common.Hash     `json:"hash"`        // transaction hash
	From      common.Address  `json:"from"`        // sender address
	To        *common.Address `json:"to"`          // recipient address (nil means contract deployment)
	Value     *big.Int        `json:"value"`       // transfer amount (in Wei)
	Nonce     uint64          `json:"nonce"`       // sender transaction sequence number
	GasPrice  *big.Int        `json:"gas_price"`   // gas price (in Wei)
	GasLimit  uint64          `json:"gas_limit"`   // gas limit
	GasTipCap *big.Int        `json:"gas_tip_cap"` // priority fee (EIP-1559)
	GasFeeCap *big.Int        `json:"gas_fee_cap"` // max fee (EIP-1559)
	Data      []byte          `json:"data"`        // contract call data
	ChainID   *big.Int        `json:"chain_id"`    // chain ID

	// ===== metadata =====
	Source      Source    `json:"source"`       // source: mempool or block
	BlockNumber uint64    `json:"block_number"` // block number (0 for Mempool sources)
	Timestamp   time.Time `json:"timestamp"`    // capture time

	// ===== derived fields (filled after parsing) =====
	TxType      string `json:"tx_type"`       // transaction type: DEX_SWAP / NFT_MINT / TRANSFER / ...
	IsHighValue bool   `json:"is_high_value"` // whether it is a high-value transaction
	MethodID    string `json:"method_id"`     // function signature (hex of the first 4 bytes of Data)
	Level       Level  `json:"level"`         // HIGH / MEDIUM / LOW
}

// FromRawTx converts a raw go-ethereum transaction into the internal unified structure
// source: "mempool" or "block"
// blockNumber: block number (pass 0 for Mempool sources)
func FromRawTx(tx *types.Transaction, source Source, blockNumber uint64) (*Transaction, error) {
	// 1. Recover the sender address (requires the signature)
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, err
	}

	// 2. Extract the function signature (first 4 bytes of Data)
	methodID := ""
	data := tx.Data()
	if len(data) >= 4 {
		methodID = hex.EncodeToString(data[:4])
	}

	// 3. Construct the internal structure
	internalTx := &Transaction{
		Hash:        tx.Hash(),
		From:        from,
		To:          tx.To(),
		Value:       tx.Value(),
		Nonce:       tx.Nonce(),
		GasPrice:    tx.GasPrice(),
		GasLimit:    tx.Gas(),
		GasTipCap:   tx.GasTipCap(),
		GasFeeCap:   tx.GasFeeCap(),
		Data:        data,
		ChainID:     tx.ChainId(),
		Source:      source,
		BlockNumber: blockNumber,
		Timestamp:   time.Now(),
		MethodID:    methodID,
	}

	return internalTx, nil
}

// IsContractDeployment determines whether it is a contract deployment transaction
func (t *Transaction) IsContractDeployment() bool {
	return t.To == nil
}

// GasPriceGwei returns the GasPrice in Gwei
func (t *Transaction) GasPriceGwei() uint64 {
	if t.GasPrice == nil {
		return 0
	}
	return t.GasPrice.Uint64() / 1e9
}

// ValueEther returns the transfer amount in Ether
func (t *Transaction) ValueEther() float64 {
	if t.Value == nil {
		return 0
	}
	val, _ := new(big.Float).Quo(
		new(big.Float).SetInt(t.Value),
		big.NewFloat(1e18),
	).Float64()
	return val
}
