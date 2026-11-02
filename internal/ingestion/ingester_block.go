package ingestion

import (
	"context"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jetsea/tx-radar/internal/models"
)

// BlockIngester listens for new blocks
type BlockIngester struct {
}

// Name returns the pipeline name
func (b *BlockIngester) Name() models.Source {
	return models.SourceBlock
}

// Connect connects and listens for new blocks
func (b *BlockIngester) Connect(ctx context.Context, rpc *RPCClient, emit func(*models.Transaction), onSucceed func(), onFail func()) error {

	headers := make(chan *types.Header, 10)

	sub, err := rpc.Eth.SubscribeNewHead(ctx, headers)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	log.Println("📡 [Block] Start listening for new block headers...")
	onSucceed()
	for {
		select {
		case <-ctx.Done():
			log.Println("🔍 [Block] ctx cancelled:", ctx.Err())
			return nil
		case err := <-sub.Err():
			return err
		case header := <-headers:
			// fetch the full block (with retry)
			var block *types.Block
			var err error
			for i := 0; i < 3; i++ {
				block, err = rpc.Eth.BlockByNumber(ctx, header.Number)
				if err == nil {
					break
				}
				select {
				case <-ctx.Done():
					log.Println("🔍 [Block] ctx cancelled:", ctx.Err())
					return nil
				case <-time.After(200 * time.Millisecond):
				}
			}

			if err != nil {
				log.Printf("⚠️ [Block] Failed to fetch full block: %v", err)
				continue
			}

			txs := block.Transactions()
			log.Printf("📦 [Block] New block #%d contains %d transactions", block.NumberU64(), len(txs))

			// emit after normalization
			for _, tx := range txs {
				internalTx, err := models.FromRawTx(tx, models.SourceBlock, block.NumberU64())
				if err != nil {
					onFail()
					continue
				}
				log.Printf("📦 [Block] New transaction %s", internalTx.Hash)
				emit(internalTx)
			}
		}
	}
}
