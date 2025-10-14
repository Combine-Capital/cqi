package health

import (
	"encoding/json"
	"net/http"
)

// LivenessHandler returns an HTTP handler that responds to liveness probes.
// Liveness probes verify that the service process is running and responsive.
// This handler always returns 200 OK with no dependency checks.
//
// Kubernetes liveness probes should use this endpoint. If this fails,
// Kubernetes will restart the pod.
//
// Example usage:
//
//	h := health.New()
//	http.HandleFunc("/health/live", h.LivenessHandler())
func (h *Health) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]string{
			"status": "alive",
		}

		// Encode response (ignore error - if encoding fails, empty response is sent)
		_ = json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns an HTTP handler that responds to readiness probes.
// Readiness probes verify that the service is ready to accept traffic by checking
// all registered infrastructure component health checkers.
//
// Returns 200 OK if all checks pass, 503 Service Unavailable if any check fails.
// The response includes detailed status for each registered component.
//
// Kubernetes readiness probes should use this endpoint. If this fails,
// Kubernetes will stop sending traffic to the pod.
//
// Example usage:
//
//	h := health.New()
//	h.RegisterChecker("database", dbPool)
//	h.RegisterChecker("cache", redisClient)
//	http.HandleFunc("/health/ready", h.ReadinessHandler())
func (h *Health) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Execute health checks
		result := h.Check(r.Context())

		// Set content type
		w.Header().Set("Content-Type", "application/json")

		// Set status code based on health
		if result.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Encode response (ignore error - if encoding fails, empty response is sent)
		_ = json.NewEncoder(w).Encode(result)
	}
}

// HealthHandler is a convenience handler that returns both liveness and readiness status.
// This is useful for simple services that don't need separate endpoints.
//
// Returns 200 OK if all checks pass, 503 Service Unavailable if any check fails.
// The response includes both liveness (always "alive") and readiness status.
//
// Example usage:
//
//	h := health.New()
//	h.RegisterChecker("database", dbPool)
//	http.HandleFunc("/health", h.HealthHandler())
func (h *Health) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Execute health checks
		result := h.Check(r.Context())

		// Set content type
		w.Header().Set("Content-Type", "application/json")

		// Set status code based on health
		if result.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Build combined response
		response := map[string]interface{}{
			"liveness":  "alive",
			"readiness": result,
		}

		// Encode response (ignore error - if encoding fails, empty response is sent)
		_ = json.NewEncoder(w).Encode(response)
	}
}
