package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Config holds all configuration for CLAWNET
type Config struct {
	mu     sync.RWMutex
	viper  *viper.Viper
	Node   NodeConfig   `mapstructure:"node"`
	Network NetworkConfig `mapstructure:"network"`
	Market MarketConfig `mapstructure:"market"`
	Memory MemoryConfig `mapstructure:"memory"`
	OpenClaw OpenClawConfig `mapstructure:"openclaw"`
	TUI    TUIConfig    `mapstructure:"tui"`
	Log    LogConfig    `mapstructure:"log"`
}

// NodeConfig contains node-specific settings
type NodeConfig struct {
	Name        string   `mapstructure:"name"`
	DataDir     string   `mapstructure:"data_dir"`
	ListenAddrs []string `mapstructure:"listen_addrs"`
	BootstrapPeers []string `mapstructure:"bootstrap_peers"`
	Capabilities []string `mapstructure:"capabilities"`
	MaxPeers    int      `mapstructure:"max_peers"`
	MinPeers    int      `mapstructure:"min_peers"`
}

// NetworkConfig contains network settings
type NetworkConfig struct {
	EnableQUIC      bool          `mapstructure:"enable_quic"`
	EnableTCP       bool          `mapstructure:"enable_tcp"`
	QUICPort        int           `mapstructure:"quic_port"`
	TCPPort         int           `mapstructure:"tcp_port"`
	EnableMDNS      bool          `mapstructure:"enable_mdns"`
	EnableDHT       bool          `mapstructure:"enable_dht"`
	DHTMode         string        `mapstructure:"dht_mode"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	PingInterval    time.Duration `mapstructure:"ping_interval"`
	EnableRelay     bool          `mapstructure:"enable_relay"`
	EnableNATPortMap bool         `mapstructure:"enable_nat_portmap"`
}

// MarketConfig contains market mechanism settings
type MarketConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	InitialWalletBalance float64       `mapstructure:"initial_wallet_balance"`
	MinBidIncrement      float64       `mapstructure:"min_bid_increment"`
	MaxConcurrentAuctions int          `mapstructure:"max_concurrent_auctions"`
	BidTimeout           time.Duration `mapstructure:"bid_timeout"`
	TaskTimeout          time.Duration `mapstructure:"task_timeout"`
	Weights              MarketWeights `mapstructure:"weights"`
	EscrowRequired       bool          `mapstructure:"escrow_required"`
	ReputationDecayRate  float64       `mapstructure:"reputation_decay_rate"`
	EnableSwarmMode      bool          `mapstructure:"enable_swarm_mode"`
	AntiCollusionChecks  bool          `mapstructure:"anti_collusion_checks"`
	RateLimitBids        int           `mapstructure:"rate_limit_bids"`
	FraudDetection       bool          `mapstructure:"fraud_detection"`
	ConsensusMode        bool          `mapstructure:"consensus_mode"`
	ConsensusThreshold   int           `mapstructure:"consensus_threshold"`
}

// MarketWeights defines scoring weights for winner selection
type MarketWeights struct {
	Price      float64 `mapstructure:"price"`
	Reputation float64 `mapstructure:"reputation"`
	Latency    float64 `mapstructure:"latency"`
	Confidence float64 `mapstructure:"confidence"`
}

// MemoryConfig contains memory synchronization settings
type MemoryConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	SyncInterval      time.Duration `mapstructure:"sync_interval"`
	MaxMemorySize     int64         `mapstructure:"max_memory_size"`
	CompressionLevel  int           `mapstructure:"compression_level"`
	EncryptionEnabled bool          `mapstructure:"encryption_enabled"`
	GCInterval        time.Duration `mapstructure:"gc_interval"`
}

// OpenClawConfig contains OpenClaw integration settings
type OpenClawConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	APIEndpoint     string        `mapstructure:"api_endpoint"`
	APITimeout      time.Duration `mapstructure:"api_timeout"`
	MaxTokens       int           `mapstructure:"max_tokens"`
	Temperature     float64       `mapstructure:"temperature"`
	Model           string        `mapstructure:"model"`
	EnableStreaming bool          `mapstructure:"enable_streaming"`
}

// TUIConfig contains terminal UI settings
type TUIConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Theme           string `mapstructure:"theme"`
	RefreshRate     int    `mapstructure:"refresh_rate"`
	ShowPeersPanel  bool   `mapstructure:"show_peers_panel"`
	ShowMarketPanel bool   `mapstructure:"show_market_panel"`
	ShowMemoryPanel bool   `mapstructure:"show_memory_panel"`
	ShowLogPanel    bool   `mapstructure:"show_log_panel"`
}

// LogConfig contains logging settings
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			Name:        "clawnet-node",
			DataDir:     ".clawnet",
			ListenAddrs: []string{"/ip4/0.0.0.0/udp/0/quic-v1", "/ip4/0.0.0.0/tcp/0"},
			BootstrapPeers: []string{},
			Capabilities: []string{"compute", "storage", "ai-inference"},
			MaxPeers:    100,
			MinPeers:    5,
		},
		Network: NetworkConfig{
			EnableQUIC:        true,
			EnableTCP:         true,
			QUICPort:          0,
			TCPPort:           0,
			EnableMDNS:        true,
			EnableDHT:         true,
			DHTMode:           "auto",
			ConnectionTimeout: 30 * time.Second,
			PingInterval:      15 * time.Second,
			EnableRelay:       true,
			EnableNATPortMap:  true,
		},
		Market: MarketConfig{
			Enabled:               true,
			InitialWalletBalance:  1000.0,
			MinBidIncrement:       0.1,
			MaxConcurrentAuctions: 10,
			BidTimeout:            5 * time.Second,
			TaskTimeout:           60 * time.Second,
			Weights: MarketWeights{
				Price:      0.4,
				Reputation: 0.3,
				Latency:    0.1,
				Confidence: 0.2,
			},
			EscrowRequired:      true,
			ReputationDecayRate: 0.01,
			EnableSwarmMode:     true,
			AntiCollusionChecks: true,
			RateLimitBids:       100,
			FraudDetection:      true,
			ConsensusMode:       false,
			ConsensusThreshold:  2,
		},
		Memory: MemoryConfig{
			Enabled:           true,
			SyncInterval:      30 * time.Second,
			MaxMemorySize:     1024 * 1024 * 1024, // 1GB
			CompressionLevel:  6,
			EncryptionEnabled: true,
			GCInterval:        5 * time.Minute,
		},
		OpenClaw: OpenClawConfig{
			Enabled:         true,
			APIEndpoint:     "http://localhost:8080",
			APITimeout:      30 * time.Second,
			MaxTokens:       4096,
			Temperature:     0.7,
			Model:           "default",
			EnableStreaming: true,
		},
		TUI: TUIConfig{
			Enabled:         true,
			Theme:           "dark",
			RefreshRate:     10,
			ShowPeersPanel:  true,
			ShowMarketPanel: true,
			ShowMemoryPanel: true,
			ShowLogPanel:    true,
		},
		Log: LogConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
		},
	}
}

// Load loads configuration from file and environment
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.viper = viper.New()

	// Set defaults
	cfg.setDefaults()

	// Read from file if specified
	if configPath != "" {
		cfg.viper.SetConfigFile(configPath)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}

		cfgDir := filepath.Join(home, ".clawnet")
		cfg.viper.AddConfigPath(cfgDir)
		cfg.viper.SetConfigName("config")
		cfg.viper.SetConfigType("yaml")
	}

	// Read environment variables
	cfg.viper.SetEnvPrefix("CLAWNET")
	cfg.viper.AutomaticEnv()

	// Read config file (ignore error if not found)
	if err := cfg.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into struct
	if err := cfg.viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default values in viper
func (c *Config) setDefaults() {
	c.viper.SetDefault("node.name", c.Node.Name)
	c.viper.SetDefault("node.data_dir", c.Node.DataDir)
	c.viper.SetDefault("node.listen_addrs", c.Node.ListenAddrs)
	c.viper.SetDefault("node.max_peers", c.Node.MaxPeers)
	c.viper.SetDefault("node.min_peers", c.Node.MinPeers)

	c.viper.SetDefault("network.enable_quic", c.Network.EnableQUIC)
	c.viper.SetDefault("network.enable_tcp", c.Network.EnableTCP)
	c.viper.SetDefault("network.enable_mdns", c.Network.EnableMDNS)
	c.viper.SetDefault("network.enable_dht", c.Network.EnableDHT)
	c.viper.SetDefault("network.connection_timeout", c.Network.ConnectionTimeout)
	c.viper.SetDefault("network.ping_interval", c.Network.PingInterval)

	c.viper.SetDefault("market.enabled", c.Market.Enabled)
	c.viper.SetDefault("market.initial_wallet_balance", c.Market.InitialWalletBalance)
	c.viper.SetDefault("market.bid_timeout", c.Market.BidTimeout)
	c.viper.SetDefault("market.task_timeout", c.Market.TaskTimeout)
	c.viper.SetDefault("market.weights.price", c.Market.Weights.Price)
	c.viper.SetDefault("market.weights.reputation", c.Market.Weights.Reputation)
	c.viper.SetDefault("market.weights.latency", c.Market.Weights.Latency)
	c.viper.SetDefault("market.weights.confidence", c.Market.Weights.Confidence)

	c.viper.SetDefault("memory.enabled", c.Memory.Enabled)
	c.viper.SetDefault("memory.sync_interval", c.Memory.SyncInterval)

	c.viper.SetDefault("openclaw.enabled", c.OpenClaw.Enabled)
	c.viper.SetDefault("openclaw.api_endpoint", c.OpenClaw.APIEndpoint)

	c.viper.SetDefault("tui.enabled", c.TUI.Enabled)
	c.viper.SetDefault("log.level", c.Log.Level)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Validate market weights sum to 1.0
	if c.Market.Enabled {
		totalWeight := c.Market.Weights.Price + c.Market.Weights.Reputation +
			c.Market.Weights.Latency + c.Market.Weights.Confidence
		if totalWeight < 0.99 || totalWeight > 1.01 {
			return fmt.Errorf("market weights must sum to 1.0, got %f", totalWeight)
		}
	}

	// Validate timeouts
	if c.Market.BidTimeout <= 0 {
		return fmt.Errorf("bid_timeout must be positive")
	}
	if c.Market.TaskTimeout <= 0 {
		return fmt.Errorf("task_timeout must be positive")
	}

	// Validate peer counts
	if c.Node.MaxPeers <= 0 {
		return fmt.Errorf("max_peers must be positive")
	}
	if c.Node.MinPeers < 0 {
		return fmt.Errorf("min_peers must be non-negative")
	}
	if c.Node.MinPeers > c.Node.MaxPeers {
		return fmt.Errorf("min_peers cannot exceed max_peers")
	}

	return nil
}

// Save saves the current configuration to file
func (c *Config) Save(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if path == "" {
		home, err := homedir.Dir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".clawnet", "config.yaml")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	c.viper.SetConfigFile(path)
	return c.viper.WriteConfigAs(path)
}

// GetConfigDir returns the configuration directory
func (c *Config) GetConfigDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	home, _ := homedir.Dir()
	return filepath.Join(home, c.Node.DataDir)
}

// Reload reloads configuration from file
func (c *Config) Reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.viper.ReadInConfig(); err != nil {
		return err
	}
	return c.viper.Unmarshal(c)
}

// Set sets a configuration value
func (c *Config) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.viper.Set(key, value)
}

// Get gets a configuration value
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.Get(key)
}
