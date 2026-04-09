package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	AppName    = "dmgn"
	ConfigFile = "config.json"
)

type Config struct {
	DataDir           string `json:"data_dir"`
	ListenAddr        string `json:"listen_addr"`
	APIPort           int    `json:"api_port"`
	MaxRecentMemories int    `json:"max_recent_memories"`
	LogLevel          string `json:"log_level"`
	Version           string `json:"version"`
}

func DefaultConfig() *Config {
	return &Config{
		DataDir:           DefaultDataDir(),
		ListenAddr:        "/ip4/0.0.0.0/tcp/0",
		APIPort:           8080,
		MaxRecentMemories: 1000,
		LogLevel:          "info",
		Version:           "0.1.0",
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
