package runner

import (
	"fmt"
)

// topologicalSort performs a topological sort on services based on dependencies.
// Returns the sorted service names in startup order, or an error if there's a cycle.
func topologicalSort(services map[string]*managedService) ([]string, error) {
	// Build adjacency list and in-degree count
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)

	// Initialize all services with in-degree 0
	for name := range services {
		inDegree[name] = 0
		adjList[name] = []string{}
	}

	// Build dependency graph
	for name, svc := range services {
		for _, dep := range svc.config.DependsOn {
			// Verify dependency exists
			if _, exists := services[dep]; !exists {
				return nil, fmt.Errorf("service %q depends on non-existent service %q", name, dep)
			}

			// Add edge from dependency to dependent
			adjList[dep] = append(adjList[dep], name)
			inDegree[name]++
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		// Reduce in-degree of neighbors
		for _, neighbor := range adjList[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Check for cycles
	if len(sorted) != len(services) {
		return nil, fmt.Errorf("circular dependency detected in services")
	}

	return sorted, nil
}

// reverseOrder returns a reversed copy of the slice.
func reverseOrder(order []string) []string {
	reversed := make([]string, len(order))
	for i, name := range order {
		reversed[len(order)-1-i] = name
	}
	return reversed
}
