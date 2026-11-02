package ingestion

import (
	"context"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jetsea/tx-radar/internal/models"
)

// MempoolIngester listens for pending Mempool transactions
type MempoolIngester struct {
}

// Name returns the pipeline name
func (m *MempoolIngester) Name() models.Source {
	return models.SourceMempool
}

// Connect connects and listens to the Mempool
func (m *MempoolIngester) Connect(ctx context.Context, rpc *RPCClient, emit func(*models.Transaction), onSucceed func(), onFail func()) error {
	hashes := make(chan common.Hash, 100)

	sub, err := rpc.Geth.SubscribePendingTransactions(ctx, hashes)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	log.Println("📡 [Mempool] Start listening for pending transactions...")
	onSucceed()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-sub.Err():
			return err
		case hash := <-hashes:
			tx, _, err := rpc.Eth.TransactionByHash(ctx, hash)
			if err != nil {
				// transaction not found, skip
				// 1. The transaction may have been dropped from the mempool before we could fetch it
				// 2. The transaction may be replaced by another transaction with the same nonce (e.g., a higher gas price).
				// 3. The transaction may come from  mempool A, but queried from mempool B， although they are both belong to INFURA.
				continue
			}

			// Emit after normalization (no block number for Mempool sources, pass 0)
			internalTx, err := models.FromRawTx(tx, models.SourceMempool, 0)
			if err != nil {
				onFail()
				continue
			}
			log.Printf("📦 [Mempool] New transaction %s", internalTx.Hash)
			emit(internalTx)
		}
	}
}
