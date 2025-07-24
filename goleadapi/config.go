package goleadapi

import (
	"fmt"
	"time"

	"github.com/gosom/google-maps-scraper/runner"
)

// BuildConfigFromRunner creates a go-lead-api client configuration from runner configuration
func BuildConfigFromRunner(runnerConfig *runner.Config) *Config {
	if !runnerConfig.LeadAPIEnabled {
		return nil
	}

	return &Config{
		BaseURL: runnerConfig.LeadAPIURL,
		APIKey:  runnerConfig.LeadAPIKey,
		Timeout: runnerConfig.LeadAPITimeout,
	}
}

// DefaultConfig returns a default configuration for development/testing
func DefaultConfig() Config {
	return Config{
		BaseURL: "http://localhost:3001", // Default go-lead-api URL
		APIKey:  "test-api-key",
		Timeout: 30 * time.Second,
	}
}

// ValidateConfig checks if the client configuration is valid
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if config.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if config.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second")
	}

	return nil
}