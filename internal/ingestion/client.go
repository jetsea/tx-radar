package ingestion

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// RPCClient wraps three clients sharing the same underlying connection.
type RPCClient struct {
	RPC    *rpc.Client        // underlying JSON-RPC connection
	Eth    *ethclient.Client  // standard Ethereum API for transactions in blocks
	Geth   *gethclient.Client // Geth-specific API for transactions in mempool
	RawURL string             // URL for the RPC connection
}

// NewRPCClient creates an RPC client (with timeout)
func NewRPCClient(ctx context.Context, rawURL string) (*RPCClient, error) {
	rpcClient, err := rpc.DialContext(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	return &RPCClient{
		RPC:    rpcClient,
		Eth:    ethclient.NewClient(rpcClient),
		Geth:   gethclient.New(rpcClient),
		RawURL: rawURL,
	}, nil
}

// Close closes the underlying connection
func (c *RPCClient) Close() {
	if c.RPC != nil {
		c.RPC.Close()
	}
}
