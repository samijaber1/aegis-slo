package config

import (
	"fmt"
	"time"
)

// Config holds server configuration
type Config struct {
	// Server settings
	Port int
	Host string

	// SLO settings
	SLODirectory string

	// Metrics adapter settings
	AdapterType     string // "prometheus" or "synthetic"
	PrometheusURL   string
	SyntheticFixDir string

	// Operational settings
	GracefulShutdownTimeout time.Duration
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if c.SLODirectory == "" {
		return fmt.Errorf("SLO directory is required")
	}

	if c.AdapterType != "prometheus" && c.AdapterType != "synthetic" {
		return fmt.Errorf("adapter type must be 'prometheus' or 'synthetic'")
	}

	if c.AdapterType == "prometheus" && c.PrometheusURL == "" {
		return fmt.Errorf("Prometheus URL required when adapter type is 'prometheus'")
	}

	return nil
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		Port:                    8080,
		Host:                    "0.0.0.0",
		AdapterType:             "synthetic",
		GracefulShutdownTimeout: 30 * time.Second,
	}
}
