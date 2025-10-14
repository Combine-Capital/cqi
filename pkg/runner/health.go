package runner

import (
	"fmt"
	"strings"

	"github.com/Combine-Capital/cqi/pkg/service"
)

// HealthStatus represents the aggregate health status of the runner.
type HealthStatus struct {
	// Healthy indicates if all services are healthy.
	Healthy bool

	// Services maps service names to their individual health status.
	Services map[string]ServiceHealth
}

// ServiceHealth represents the health status of a single service.
type ServiceHealth struct {
	// Healthy indicates if the service is healthy.
	Healthy bool

	// Error is the error message if the service is unhealthy.
	Error string

	// Running indicates if the service is currently running.
	Running bool
}

// aggregateHealth performs a health check across all managed services.
// Returns the aggregate health status with per-service details.
func aggregateHealth(services map[string]*managedService) HealthStatus {
	status := HealthStatus{
		Healthy:  true,
		Services: make(map[string]ServiceHealth),
	}

	for name, svc := range services {
		svcHealth := checkServiceHealth(svc)
		status.Services[name] = svcHealth

		// If any service is unhealthy, the overall status is unhealthy
		if !svcHealth.Healthy {
			status.Healthy = false
		}
	}

	return status
}

// checkServiceHealth checks the health of a single managed service.
func checkServiceHealth(svc *managedService) ServiceHealth {
	svc.mu.RLock()
	running := svc.running
	svc.mu.RUnlock()

	health := ServiceHealth{
		Running: running,
		Healthy: true,
	}

	if !running {
		health.Healthy = false
		health.Error = "service not running"
		return health
	}

	// Check service health
	if err := svc.service.Health(); err != nil {
		health.Healthy = false
		health.Error = err.Error()
	}

	return health
}

// Health implements a health check that can be integrated with the health package.
// It returns nil if all services are healthy, or an error with details about unhealthy services.
func (r *Runner) Health() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := aggregateHealth(r.services)
	if status.Healthy {
		return nil
	}

	// Build error message with details about unhealthy services
	var unhealthy []string
	for name, svc := range status.Services {
		if !svc.Healthy {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", name, svc.Error))
		}
	}

	return fmt.Errorf("unhealthy services: %s", strings.Join(unhealthy, ", "))
}

// HealthStatus returns detailed health status for all managed services.
func (r *Runner) HealthStatus() HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return aggregateHealth(r.services)
}

// Ensure Runner implements the Service interface's Health method.
var _ service.Service = (*Runner)(nil)
