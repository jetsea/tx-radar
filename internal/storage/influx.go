package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/jetsea/tx-radar/internal/config"
	"github.com/jetsea/tx-radar/internal/models"
)

// InfluxStorage persists transaction metrics as time-series data in InfluxDB.
//
// Why InfluxDB? TxRadar ingests a continuous stream of transactions whose core
// analysis is time-based (gas price trends, high-value trading hotspots, rate
// aggregation over time windows). PostgreSQL stores each transaction as one row
// for detailed/relational queries, but aggregating thousands of rows over time
// at high ingestion rates is slow. InfluxDB is purpose-built for that: writes
// are batched and very fast, and queries like "avg gas price over the last hour"
// use native window functions. It complements, rather than replaces, PostgreSQL.
type InfluxStorage struct {
	client   influxdb2.Client     // HTTP client for the InfluxDB v2 API
	writeAPI api.WriteAPIBlocking // blocking writer: every WritePoint waits for the server ack
	org      string               // InfluxDB organization (tenant)
	bucket   string               // bucket = logical database with its own retention policy
}

// NewInfluxStorage creates an InfluxDB storage backend.
func NewInfluxStorage(cfg config.InfluxConfig) (*InfluxStorage, error) {
	client := influxdb2.NewClient(cfg.URL, cfg.Token)

	// check connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("InfluxDB connection failed: %w", err)
	}
	if health.Status != "pass" {
		return nil, fmt.Errorf("InfluxDB health check failed: %s", health.Status)
	}

	log.Println("InfluxDB connected")
	return &InfluxStorage{
		client:   client,
		writeAPI: client.WriteAPIBlocking(cfg.Org, cfg.Bucket),
		org:      cfg.Org,
		bucket:   cfg.Bucket,
	}, nil
}

// Save writes a single transaction as one time-series point.
//
// The point is serialized to the InfluxDB line protocol before sending, e.g.:
//
//	transaction,source=eth,tx_type=swap,level=high gas_price=52.3,value_eth=1.25,is_high_value=1 1700000000000000000
//
// - measurement:  "transaction"
// - tags:          source / tx_type / level (indexed, used in WHERE filters)
// - fields:        gas_price / value_eth / is_high_value (numeric values)
// - timestamp:     tx.Timestamp as nanoseconds
func (i *InfluxStorage) Save(ctx context.Context, tx *models.Transaction) error {
	point := influxdb2.NewPoint(
		"transaction", // measurement name (like a table name)
		map[string]string{ // tags (like indexed columns)
			"source":  string(tx.Source),
			"tx_type": tx.TxType,
			"level":   string(tx.Level),
		},
		map[string]interface{}{ // fields (like columns)
			"gas_price":     tx.GasPriceGwei(),
			"value_eth":     tx.ValueEther(),
			"is_high_value": boolToInt(tx.IsHighValue), // bool is encoded as 0/1 for the line protocol
		},
		tx.Timestamp,
	)

	return i.writeAPI.WritePoint(ctx, point)
}

// SaveBatch writes a batch of transactions.
//
// Each transaction is written as its own point in a loop; the batch allows
// write failures to be counted and reported as a single summary error instead
// of aborting on the first failure.
func (i *InfluxStorage) SaveBatch(ctx context.Context, txs []*models.Transaction) error {
	// points := make([]*write.Point, 0, len(txs))
	var failed int
	for _, tx := range txs {

		if err := i.Save(ctx, tx); err != nil {
			failed++
			log.Printf("InfluxDB write failed: %v", err)
		}

	}

	if failed > 0 {
		return fmt.Errorf("InfluxDB batch write completed with %d/%d failures", failed, len(txs))
	}
	return nil
}

// Close closes the InfluxDB client.
func (i *InfluxStorage) Close() error {
	i.client.Close()
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
