package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Load loads configuration from a file and environment variables.
// The prefix parameter is used for environment variable names (e.g., "CQI" -> CQI_DATABASE_HOST).
// If configPath is empty, only environment variables will be used.
func Load(configPath, envPrefix string) (*Config, error) {
	v := viper.New()

	// Configure environment variable handling
	if envPrefix != "" {
		v.SetEnvPrefix(envPrefix)
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Load from file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into Config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate configuration
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// MustLoad loads configuration and panics on error.
// This is useful in main() where configuration errors should be fatal.
func MustLoad(configPath, envPrefix string) *Config {
	cfg, err := Load(configPath, envPrefix)
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}

// LoadFromEnv loads configuration only from environment variables (no config file).
func LoadFromEnv(envPrefix string) (*Config, error) {
	return Load("", envPrefix)
}

// MustLoadFromEnv loads configuration from environment variables and panics on error.
func MustLoadFromEnv(envPrefix string) *Config {
	return MustLoad("", envPrefix)
}
