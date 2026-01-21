package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the root configuration structure
type Config struct {
	Defaults     Defaults      `yaml:"defaults"`
	Groups       Groups        `yaml:"groups"`
	Destinations []Destination `yaml:"destinations"`
}

// Defaults represents global default settings
type Defaults struct {
	User string `yaml:"user"`
}

// Groups represents a map of group names to their settings
type Groups map[string]GroupSettings

// GroupSettings represents settings for a specific group
type GroupSettings struct {
	User string `yaml:"user"`
}

// Status represents the connectivity and fingerprint status
type Status int

const (
	StatusUnknown Status = iota
	StatusOk
	StatusOffline
	StatusKeyError
)

// Destination represents a single SSH target
type Destination struct {
	Alias           string    `yaml:"alias"`
	Hostname        string    `yaml:"hostname"`
	Port            int       `yaml:"port,omitempty"`
	User            string    `yaml:"user,omitempty"`
	Key             string    `yaml:"key"`
	Group           string    `yaml:"group,omitempty"`
	LastConnectedAt time.Time `yaml:"last_connected_at"`
	ConnectionCount int       `yaml:"connection_count"`
	Status          Status    `yaml:"-"` // Runtime status, not persisted
}

// Load reads the configuration from ~/.config/sshx/config.yaml
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default empty config if file doesn't exist
		return &Config{
			Destinations: []Destination{},
			Groups:       make(Groups),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Groups == nil {
		cfg.Groups = make(Groups)
	}

	return &cfg, nil
}

// Save writes the configuration to ~/.config/sshx/config.yaml
func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// CalculateFrecency computes the score for a destination
func (d *Destination) CalculateFrecency() float64 {
	if d.ConnectionCount == 0 {
		return 0
	}

	now := time.Now()
	diff := now.Sub(d.LastConnectedAt)

	var weight float64
	switch {
	case diff < 4*time.Hour:
		weight = 100
	case diff < 24*time.Hour:
		weight = 70
	case diff < 7*24*time.Hour:
		weight = 50
	case diff < 30*24*time.Hour:
		weight = 30
	default:
		weight = 10
	}

	return float64(d.ConnectionCount) * weight
}

// Matches returns true if the destination matches the query using fuzzy matching
func (d *Destination) Matches(query string) bool {
	if query == "" {
		return true
	}

	// fast path: exact substring
	query = strings.ToLower(query)
	target := strings.ToLower(d.Alias)
	if strings.Contains(target, query) {
		return true
	}

	// Check Hostname as well
	if strings.Contains(strings.ToLower(d.Hostname), query) {
		return true
	}

	// fuzzy subsequence match on Alias
	// "apd" matches "app-prod-01"
	qIdx := 0
	qRunes := []rune(query)
	for _, r := range target {
		if qIdx < len(qRunes) && r == qRunes[qIdx] {
			qIdx++
		}
	}
	return qIdx == len(qRunes)
}

// SortDestinations sorts the destinations by frecency (descending)
func (c *Config) SortDestinations() {
	sort.Slice(c.Destinations, func(i, j int) bool {
		return c.Destinations[i].CalculateFrecency() > c.Destinations[j].CalculateFrecency()
	})
}

// ResolveUser determines the effective user for a destination
func (c *Config) ResolveUser(d *Destination) string {
	// 1. Explicit destination override
	if d.User != "" {
		return d.User
	}

	// 2. Group default
	if d.Group != "" {
		if group, ok := c.Groups[d.Group]; ok && group.User != "" {
			return group.User
		}
	}

	// 3. Global default
	if c.Defaults.User != "" {
		return c.Defaults.User
	}

	// Fallback
	return "root"
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "sshx", "config.yaml"), nil
}
