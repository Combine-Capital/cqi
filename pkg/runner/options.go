package runner

import "time"

// Option configures a service within the runner.
type Option func(*serviceConfig)

// serviceConfig holds configuration for a service managed by the runner.
type serviceConfig struct {
	// DependsOn lists service names that must start before this service.
	DependsOn []string

	// StartDelay is the duration to wait before starting this service.
	StartDelay time.Duration

	// RestartConfig configures restart behavior on failure.
	RestartConfig RestartConfig
}

// defaultServiceConfig returns a service config with defaults.
func defaultServiceConfig() serviceConfig {
	return serviceConfig{
		DependsOn:     []string{},
		StartDelay:    0,
		RestartConfig: DefaultRestartConfig(),
	}
}

// WithDependsOn specifies services that must start before this service.
// The service will wait for all dependencies to be healthy before starting.
func WithDependsOn(names ...string) Option {
	return func(cfg *serviceConfig) {
		cfg.DependsOn = append(cfg.DependsOn, names...)
	}
}

// WithStartDelay adds a delay before starting the service.
// Useful for staggering service startups or waiting for external dependencies.
func WithStartDelay(duration time.Duration) Option {
	return func(cfg *serviceConfig) {
		cfg.StartDelay = duration
	}
}

// WithRestartPolicy sets the restart policy for the service.
func WithRestartPolicy(policy RestartPolicy) Option {
	return func(cfg *serviceConfig) {
		cfg.RestartConfig.Policy = policy
	}
}

// WithRestartConfig sets the full restart configuration for the service.
func WithRestartConfig(config RestartConfig) Option {
	return func(cfg *serviceConfig) {
		cfg.RestartConfig = config
	}
}

// WithMaxRetries sets the maximum number of restart attempts.
func WithMaxRetries(max int) Option {
	return func(cfg *serviceConfig) {
		cfg.RestartConfig.MaxRetries = max
	}
}

// WithBackoff configures the exponential backoff parameters.
func WithBackoff(initial, max time.Duration, multiplier float64) Option {
	return func(cfg *serviceConfig) {
		cfg.RestartConfig.InitialBackoff = initial
		cfg.RestartConfig.MaxBackoff = max
		cfg.RestartConfig.Multiplier = multiplier
	}
}
