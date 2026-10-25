package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the global configuration
type Config struct {
	Ingestion     IngestionConfig     `mapstructure:"ingestion"`
	Processor     ProcessorConfig     `mapstructure:"processor"`
	Storage       StorageConfig       `mapstructure:"storage"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	Relayer       RelayerConfig       `mapstructure:"relayer"`
}

// ============================================================
// Module 1: data ingestion layer
// ============================================================

type IngestionConfig struct {
	RPCURLs               []string      `mapstructure:"rpc_urls"`
	ReconnectWaitPerNode  time.Duration `mapstructure:"reconnect_wait_per_node"`
	ReconnectWaitAllNodes time.Duration `mapstructure:"reconnect_wait_all_nodes"`
	ChannelBufferSize     int           `mapstructure:"channel_buffer_size"`
}

// ============================================================
// Module 2: data processing layer
// ============================================================

type ProcessorConfig struct {
	WorkerCount int            `mapstructure:"worker_count"`
	GasStats    GasStatsConfig `mapstructure:"gas_stats"`
	Dedup       DedupConfig    `mapstructure:"dedup"`
	Marker      MarkerConfig   `mapstructure:"marker"`
}

type GasStatsConfig struct {
	WindowSize int `mapstructure:"window_size"`
}

type DedupConfig struct {
	Mode  string        `mapstructure:"mode"` // "memory" or "redis"
	TTL   time.Duration `mapstructure:"ttl"`
	Redis RedisConfig   `mapstructure:"redis"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type MarkerConfig struct {
	MinHighGasGwei   uint64  `mapstructure:"min_high_gas_gwei"`
	MinMediumGasGwei uint64  `mapstructure:"min_medium_gas_gwei"`
	MinHighValueETH  float64 `mapstructure:"min_high_value_eth"`
}

// ============================================================
// Module 3: data storage layer
// ============================================================

type StorageConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Influx   InfluxConfig   `mapstructure:"influx"`
	Batch    BatchConfig    `mapstructure:"batch"`
}

type PostgresConfig struct {
	DSN            string `mapstructure:"dsn"`
	MaxConnections int    `mapstructure:"max_connections"`
}

type InfluxConfig struct {
	URL    string `mapstructure:"url"`
	Token  string `mapstructure:"token"`
	Org    string `mapstructure:"org"`
	Bucket string `mapstructure:"bucket"`
}

// ============================================================
// Module 5: observability
// ============================================================

type ObservabilityConfig struct {
	LogLevel    string `mapstructure:"log_level"`    // debug / info / warn / error
	LogFormat   string `mapstructure:"log_format"`   // json / text
	MetricsPort int    `mapstructure:"metrics_port"` // Prometheus port, default 9090
	HealthPort  int    `mapstructure:"health_port"`  // health check port, default 8080
	EnablePprof bool   `mapstructure:"enable_pprof"` // whether to enable pprof
}

// BatchConfig is the batch writing configuration
type BatchConfig struct {
	MaxSize       int           `mapstructure:"max_size"`       // max records per batch
	FlushInterval time.Duration `mapstructure:"flush_interval"` // max wait time
}

type RelayerConfig struct {
	WebSocket WebSocketConfig `mapstructure:"websocket"`
	Webhook   WebhookConfig   `mapstructure:"webhook"`
	MQ        MQConfig        `mapstructure:"mq"`
}

// WebSocketConfig is the WebSocket configuration
type WebSocketConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`      // listen address, e.g. ":8081"
	MinLevel string `mapstructure:"min_level"` // minimum push level, e.g. "MEDIUM"
}

// WebhookConfig is the Webhook configuration
type WebhookConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries int           `mapstructure:"max_retries"`
	MinLevel   string        `mapstructure:"min_level"` // minimum push level, e.g. "MEDIUM"
}

// MQConfig is the message queue configuration
type MQConfig struct {
	Enabled  bool     `mapstructure:"enabled"`
	Brokers  []string `mapstructure:"brokers"`
	Topic    string   `mapstructure:"topic"`
	MinLevel string   `mapstructure:"min_level"` // minimum push level, e.g. "MEDIUM"
}

// deal with array fields
func expandConfig(cfg *Config) {
	// ingestion
	for i := range cfg.Ingestion.RPCURLs {
		cfg.Ingestion.RPCURLs[i] = os.ExpandEnv(cfg.Ingestion.RPCURLs[i])
	}
	// relayer.mq.brokers
	for i := range cfg.Relayer.MQ.Brokers {
		cfg.Relayer.MQ.Brokers[i] = os.ExpandEnv(cfg.Relayer.MQ.Brokers[i])
	}
}

// ============================================================
// Loading configuration
// ============================================================

// Load loads the configuration from a file
// path can be a relative or absolute path; if relative, several candidate locations are tried in order
func Load(path string) (*Config, error) {
	resolvedPath := resolveConfigPath(path)
	v := viper.New()
	v.SetConfigFile(resolvedPath)
	v.SetConfigType("yaml")

	// Support environment variable overrides (e.g. TXRADAR_PROCESSOR_DEDUP_MODE=redis)
	v.SetEnvPrefix("")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	expandConfig(&cfg)

	return &cfg, nil
}

// resolveConfigPath tries to find the config file at several candidate locations
// candidates: 1) the path explicitly passed by the user  2) two levels up (compatible with running under cmd/)  3) the directory of the executable
func resolveConfigPath(path string) string {
	candidates := []string{
		path,                            // use the passed-in path directly
		filepath.Join("..", path),       // parent directory (cmd/ → project root)
		filepath.Join("..", "..", path), // two levels up
	}

	// Also check the directory of the executable (for post-compile deployment scenarios)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), path),
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// If none is found, return the original path and let viper report the clear error
	return path
}
