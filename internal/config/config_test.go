package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testdataConfigPath returns the path to testdata/config.yaml
func testdataConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "config.yaml")
}

// writeTempConfig writes a yaml file into a temp dir and returns its path.
// Used by tests that need non-standard content (invalid YAML, invalid duration, etc.)
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoad_ParsesTestdataYAML(t *testing.T) {
	cfg, err := Load(testdataConfigPath(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// ingestion
	if len(cfg.Ingestion.RPCURLs) != 2 {
		t.Errorf("RPCURLs len = %d, want 2", len(cfg.Ingestion.RPCURLs))
	}
	if cfg.Ingestion.RPCURLs[0] != "wss://a.example/ws" {
		t.Errorf("RPCURLs[0] = %q", cfg.Ingestion.RPCURLs[0])
	}
	if cfg.Ingestion.ReconnectWaitPerNode != 2*time.Second {
		t.Errorf("ReconnectWaitPerNode = %v, want 2s", cfg.Ingestion.ReconnectWaitPerNode)
	}
	if cfg.Ingestion.ReconnectWaitAllNodes != 10*time.Second {
		t.Errorf("ReconnectWaitAllNodes = %v, want 10s", cfg.Ingestion.ReconnectWaitAllNodes)
	}
	if cfg.Ingestion.ChannelBufferSize != 10000 {
		t.Errorf("ChannelBufferSize = %d, want 10000", cfg.Ingestion.ChannelBufferSize)
	}

	// processor
	if cfg.Processor.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d, want 4", cfg.Processor.WorkerCount)
	}
	if cfg.Processor.Dedup.Mode != "memory" {
		t.Errorf("Dedup.Mode = %q, want memory", cfg.Processor.Dedup.Mode)
	}
	if cfg.Processor.Dedup.TTL != 30*time.Minute {
		t.Errorf("Dedup.TTL = %v, want 30m", cfg.Processor.Dedup.TTL)
	}
	if cfg.Processor.Dedup.Redis.Addr != "localhost:6379" {
		t.Errorf("Redis.Addr = %q", cfg.Processor.Dedup.Redis.Addr)
	}
	if cfg.Processor.Marker.MinHighGasGwei != 100 {
		t.Errorf("MinHighGasGwei = %d, want 100", cfg.Processor.Marker.MinHighGasGwei)
	}
	if cfg.Processor.Marker.MinHighValueETH != 100 {
		t.Errorf("MinHighValueETH = %v, want 100", cfg.Processor.Marker.MinHighValueETH)
	}

	// storage
	if cfg.Storage.Postgres.MaxConnections != 10 {
		t.Errorf("MaxConnections = %d, want 10", cfg.Storage.Postgres.MaxConnections)
	}
	if cfg.Storage.Influx.Token != "dev-token" {
		t.Errorf("Influx.Token = %q", cfg.Storage.Influx.Token)
	}
	if cfg.Storage.Batch.FlushInterval != time.Second {
		t.Errorf("Batch.FlushInterval = %v, want 1s", cfg.Storage.Batch.FlushInterval)
	}

	// observability
	if cfg.Observability.LogLevel != "info" {
		t.Errorf("LogLevel = %q", cfg.Observability.LogLevel)
	}
	if !cfg.Observability.EnablePprof {
		t.Errorf("EnablePprof = false, want true")
	}

	// relayer
	if !cfg.Relayer.WebSocket.Enabled {
		t.Errorf("WebSocket.Enabled = false, want true")
	}
	if cfg.Relayer.Webhook.Timeout != 5*time.Second {
		t.Errorf("Webhook.Timeout = %v, want 5s", cfg.Relayer.Webhook.Timeout)
	}
	if len(cfg.Relayer.MQ.Brokers) != 1 || cfg.Relayer.MQ.Brokers[0] != "localhost:9092" {
		t.Errorf("MQ.Brokers = %v", cfg.Relayer.MQ.Brokers)
	}
}

// string override test
func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("TXRADAR_PROCESSOR_DEDUP_MODE", "redis")
	t.Setenv("TXRADAR_OBSERVABILITY_LOG_LEVEL", "debug")

	cfg, err := Load(testdataConfigPath(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Processor.Dedup.Mode != "redis" {
		t.Errorf("Dedup.Mode = %q, want redis (env override)", cfg.Processor.Dedup.Mode)
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug (env override)", cfg.Observability.LogLevel)
	}
}

// duration override test
func TestLoad_EnvOverrideDuration(t *testing.T) {
	t.Setenv("TXRADAR_INGESTION_RECONNECT_WAIT_PER_NODE", "5s")

	cfg, err := Load(testdataConfigPath(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Ingestion.ReconnectWaitPerNode != 5*time.Second {
		t.Errorf("ReconnectWaitPerNode = %v, want 5s (env override)", cfg.Ingestion.ReconnectWaitPerNode)
	}
}

// int override test
func TestLoad_EnvOverrideNestedInt(t *testing.T) {
	t.Setenv("TXRADAR_PROCESSOR_DEDUP_REDIS_DB", "2")

	cfg, err := Load(testdataConfigPath(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Processor.Dedup.Redis.DB != 2 {
		t.Errorf("Redis.DB = %d, want 2 (env override)", cfg.Processor.Dedup.Redis.DB)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempConfig(t, "ingestion: [unclosed")
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want error for invalid yaml")
	}
}

func TestLoad_DurationParseError(t *testing.T) {
	path := writeTempConfig(t, `
ingestion:
  reconnect_wait_per_node: "not-a-duration"
`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want error for invalid duration")
	}
}
