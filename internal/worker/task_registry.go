// internal/worker/task_registry.go
package worker

import (
	"context"
	"fmt"
	"sync"
)

// TaskFunction represents a function that can be executed as a task
type TaskFunction func(ctx context.Context, data map[string]interface{}) error

// Registry manages available task functions
type Registry struct {
	functions map[string]TaskFunction
	mu        sync.RWMutex
}

// NewRegistry creates a new task function registry
func NewRegistry() *Registry {
	return &Registry{
		functions: make(map[string]TaskFunction),
	}
}

// Register adds a new task function to the registry
func (r *Registry) Register(name string, fn TaskFunction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.functions[name]; exists {
		return fmt.Errorf("task function %s already registered", name)
	}

	r.functions[name] = fn
	return nil
}

// Get retrieves a task function from the registry
func (r *Registry) Get(name string) (TaskFunction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fn, exists := r.functions[name]
	if !exists {
		return nil, fmt.Errorf("task function %s not found", name)
	}

	return fn, nil
}
