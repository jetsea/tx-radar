package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jetsea/tx-radar/internal/config"
	"github.com/jetsea/tx-radar/internal/models"
)

// Ingester defines the interface for each data source.
type Ingester interface {
	Name() models.Source
	Connect(ctx context.Context, rpc *RPCClient, emit func(*models.Transaction), onSucceed func(), onFail func()) error
}

// RunMempoolLoop starts parallel listeners for all configured nodes (multi-active mode).
// Each RPC URL runs in its own independent goroutine to maximize Mempool coverage.
func RunMempoolLoop(ctx context.Context, cfg config.IngestionConfig, ingester Ingester, pipeline *Pipeline) error {
	name := ingester.Name()

	if len(cfg.RPCURLs) == 0 {
		log.Printf("❌ [%s] No RPC URLs configured", name)
		return fmt.Errorf("no RPC URLs configured for %s", name)
	}

	var g, gctx = errgroup.WithContext(ctx)

	// Spawn an independent listener for each RPC URL.
	for _, url := range cfg.RPCURLs {
		g.Go(func() error { return runSingleNode(gctx, cfg, ingester, pipeline, url) })
	}

	log.Printf("🚀 [%s] Started %d parallel ingesters (multi-active)", name, len(cfg.RPCURLs))
	// Wait for all ingesters to finish. If any ingester returns an error, the entire RunLoop will exit.
	// If entire RunLoop exits, the main program will exit gracefully.
	// g.Wait() will return the first error encountered, if any. If all ingesters exit cleanly (e.g., due to context cancellation), g.Wait() will return nil.
	return g.Wait()
}

// RunBlockLoop starts block ingestion in master-slave (active-standby) mode.
// Unlike RunMempoolLoop (multi-active), only ONE RPC node — the "master" —
// actively ingests blocks at a time; the rest act as hot standbys.
// It reuses runSingleNode per node: runSingleNode handles the master's dial,
// subscribe and exponential reconnection internally. It only returns a
// non-nil error when the current master has never connected (misconfigured /
// dead node); we then fail over and promote the next node, ensuring continuous
// block coverage without duplicate ingestion.
func RunBlockLoop(ctx context.Context, cfg config.IngestionConfig, ingester Ingester, pipeline *Pipeline) error {
	name := ingester.Name()

	if len(cfg.RPCURLs) == 0 {
		log.Printf("❌ [%s] No RPC URLs configured", name)
		return fmt.Errorf("no RPC URLs configured for %s", name)
	}

	log.Printf("👑 [%s] Master-slave block ingestion using %d node(s): master=%q",
		name, len(cfg.RPCURLs), cfg.RPCURLs[0])

	masterIdx := 0
	for {
		if ctx.Err() != nil {
			log.Printf("⏹️ [%s] Block ingester exited", name)
			return nil
		}

		masterURL := cfg.RPCURLs[masterIdx]
		log.Printf("👑 [%s] Running block ingester on master: %s", name, masterURL)

		// runSingleNode returns nil only on clean exit (ctx canceled).
		// A non-nil error means the master never connected — fail over.
		if err := runSingleNode(ctx, cfg, ingester, pipeline, masterURL); err != nil {
			nextURL := cfg.RPCURLs[(masterIdx+1)%len(cfg.RPCURLs)]
			log.Printf("🔄 [%s] Master %s unreachable (%v), failing over to %s",
				name, masterURL, err, nextURL)
			masterIdx = (masterIdx + 1) % len(cfg.RPCURLs)
			continue
		}
		return nil
	}
}

// runSingleNode handles the isolated reconnection loop for a single RPC endpoint.
func runSingleNode(ctx context.Context, cfg config.IngestionConfig, ingester Ingester, pipeline *Pipeline, targetURL string) error {
	name := ingester.Name()
	var (
		consecFails    int
		everSucceeded  bool
		currentBackoff time.Duration = cfg.ReconnectWaitPerNode
	)
	// loop-invariant closures — hoisted
	emit := func(tx *models.Transaction) { pipeline.Emit(tx) }
	onSucceed := func() { everSucceeded = true; consecFails = 0; currentBackoff = cfg.ReconnectWaitPerNode }
	onFail := func() { pipeline.RecordFailure() }

	for {
		if ctx.Err() != nil {
			log.Printf("⏹️ [%s] Ingester for %s exited", name, targetURL)
			return nil
		}

		log.Printf("🔗 [%s] Attempting to connect: %s", name, targetURL)
		rpcClient, err := NewRPCClient(ctx, targetURL)
		if err != nil {
			consecFails++
			if !everSucceeded && consecFails >= 3 {
				return fmt.Errorf("[%s] %s: %d consecutive dial failures, never succeeded — likely misconfigured",
					name, targetURL, consecFails)
			}
			log.Printf("❌ [%s] Connection failed (%s): %v (consecutive: %d)",
				name, targetURL, err, consecFails)

			if !waitForReconnect(ctx, name, targetURL, &currentBackoff) {
				return nil
			}
			continue
		}

		err = ingester.Connect(ctx, rpcClient, emit, onSucceed, onFail)
		rpcClient.Close()

		if err != nil {
			// graceful shutdown masquerading as an error — don't log it as one
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				log.Printf("⏹️ [%s] Ingester for %s exited", name, targetURL)
				return nil
			}
			consecFails++
			if !everSucceeded && consecFails >= 3 {
				return fmt.Errorf("[%s] %s: never established a subscription after %d attempts — %w",
					name, targetURL, consecFails, err)
			}
			log.Printf("⚠️ [%s] Connection lost (%s): %v", name, targetURL, err)
		} else {
			// Connect returned cleanly (ctx canceled) — clean exit
			log.Printf("⏹️ [%s] Ingester for %s exited", name, targetURL)
			return nil
		}

		if !waitForReconnect(ctx, name, targetURL, &currentBackoff) {
			return nil
		}
	}
}

// waitForReconnect handles exponential backoff for connection retries.
// It uses a select statement to allow immediate cancellation via context.
// Returns false if the context is canceled (program should exit),
// true if the wait completed successfully (ready to retry).
func waitForReconnect(ctx context.Context, name models.Source, targetURL string, currentBackoff *time.Duration) bool {
	const maxBackoff = 30 * time.Second

	log.Printf("⏳ [%s] Reconnecting to %s in %v...", name, targetURL, *currentBackoff)

	select {
	case <-ctx.Done():
		return false // Context canceled, abort the wait.
	case <-time.After(*currentBackoff):
		// Wait completed. Calculate the next backoff (double it) and enforce the max limit.
		*currentBackoff = max(2**currentBackoff, maxBackoff)

		return true
	}
}
