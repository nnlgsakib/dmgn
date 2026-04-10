package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	AppName    = "dmgn"
	ConfigFile = "config.json"
)

type Config struct {
	DataDir           string   `json:"data_dir"`
	ListenAddr        string   `json:"listen_addr"`
	APIPort           int      `json:"api_port"`
	MaxRecentMemories int      `json:"max_recent_memories"`
	LogLevel          string   `json:"log_level"`
	Version           string   `json:"version"`
	BootstrapPeers    []string `json:"bootstrap_peers"`
	MDNSService       string   `json:"mdns_service"`
	MaxPeersLow       int      `json:"max_peers_low"`
	MaxPeersHigh      int      `json:"max_peers_high"`
	ShardThreshold    int      `json:"shard_threshold"`
	ShardCount        int      `json:"shard_count"`
	EmbeddingDim      int      `json:"embedding_dim"`
	HybridScoreAlpha  float64  `json:"hybrid_score_alpha"`
	QueryTimeout      string   `json:"query_timeout"`
	SyncInterval      string   `json:"sync_interval"`
	GossipTopic       string   `json:"gossip_topic"`
	OTLPEndpoint      string   `json:"otlp_endpoint"`
	MCPIPCPort        int      `json:"mcp_ipc_port"`
}

func DefaultConfig() *Config {
	return &Config{
		DataDir:           DefaultDataDir(),
		ListenAddr:        "/ip4/0.0.0.0/tcp/0",
		APIPort:           8080,
		MaxRecentMemories: 1000,
		LogLevel:          "info",
		Version:           "0.1.0",
		BootstrapPeers:    []string{},
		MDNSService:       "_dmgn._tcp",
		MaxPeersLow:       15,
		MaxPeersHigh:      25,
		ShardThreshold:    3,
		ShardCount:        5,
		EmbeddingDim:      0,
		HybridScoreAlpha:  0.7,
		QueryTimeout:      "2s",
		SyncInterval:      "60s",
		GossipTopic:       "dmgn/memories/1.0.0",
		MCPIPCPort:        0,
	}
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", AppName)
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", AppName)
	default:
		return filepath.Join(home, ".config", AppName)
	}
}

func Load(dataDir string) (*Config, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}

	configPath := filepath.Join(dataDir, ConfigFile)
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.DataDir = dataDir
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.DataDir == "" {
		cfg.DataDir = dataDir
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	configPath := filepath.Join(c.DataDir, ConfigFile)
	
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

func (c *Config) IdentityDir() string {
	return filepath.Join(c.DataDir, "identity")
}

func (c *Config) StorageDir() string {
	return filepath.Join(c.DataDir, "storage")
}

func (c *Config) BackupDir() string {
	return filepath.Join(c.DataDir, "backups")
}

func (c *Config) LogDir() string {
	return filepath.Join(c.DataDir, "logs")
}

func (c *Config) VectorIndexPath() string {
	return filepath.Join(c.DataDir, "vector-index.enc")
}

func (c *Config) PIDFile() string {
	return filepath.Join(c.DataDir, "daemon.pid")
}

func (c *Config) PortFile() string {
	return filepath.Join(c.DataDir, "daemon.port")
}

func (c *Config) QueryTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.QueryTimeout)
	if err != nil {
		return 2 * time.Second
	}
	return d
}

func (c *Config) SyncIntervalDuration() time.Duration {
	d, err := time.ParseDuration(c.SyncInterval)
	if err != nil {
		return 60 * time.Second
	}
	return d
}

func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.DataDir,
		c.IdentityDir(),
		c.StorageDir(),
		c.BackupDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
