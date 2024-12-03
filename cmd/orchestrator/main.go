// cmd/orchestrator/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/fawad-mazhar/naxos/internal/orchestrator"
	"github.com/fawad-mazhar/naxos/internal/queue"
	"github.com/fawad-mazhar/naxos/internal/storage/leveldb"
	"github.com/fawad-mazhar/naxos/internal/storage/postgres"
	"github.com/fawad-mazhar/naxos/internal/worker"
)

// loadTaskFunctions discovers and loads task functions using reflection
// It examines the task_functions package for compatible method signatures
// Returns a map of function names to their implementations
func loadTaskFunctions() (map[string]worker.TaskFunction, error) {
	taskFunctions := make(map[string]worker.TaskFunction)

	// Get the type information for the TaskFunctions interface
	// This is used to find all available task function implementations
	pkgType := reflect.TypeOf((*worker.TaskFunctions)(nil)).Elem()

	// Iterate through all methods in the interface
	// Check each method for compatibility with the TaskFunction signature
	for i := 0; i < pkgType.NumMethod(); i++ {
		method := pkgType.Method(i)

		// Verify the method signature matches our TaskFunction type:
		// - Takes context.Context and map[string]interface{}
		// - Returns error
		if method.Type.NumIn() == 2 &&
			method.Type.In(0).String() == "context.Context" &&
			method.Type.In(1).String() == "map[string]interface {}" &&
			method.Type.NumOut() == 1 &&
			method.Type.Out(0).String() == "error" {

			// Convert method name to expected function name in job definitions
			// For example: "Process" becomes "processFunction"
			functionName := fmt.Sprintf("%sFunction", strings.ToLower(method.Name))

			// Get the actual function implementation
			fn := reflect.ValueOf(worker.GetTaskFunction(method.Name)).Interface().(func(context.Context, map[string]interface{}) error)

			// Store the function in our map
			taskFunctions[functionName] = fn
			fmt.Printf("Loaded task function: %s\n", functionName)
		}
	}

	// Ensure at least one task function was loaded
	if len(taskFunctions) == 0 {
		return nil, fmt.Errorf("no task functions found")
	}

	return taskFunctions, nil
}

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize PostgreSQL client
	db, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize LevelDB client
	cache, err := leveldb.NewClient(cfg.LevelDB, 24*time.Hour) // 24-hour TTL
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cache.Close()

	// Initialize RabbitMQ client
	queue, err := queue.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer queue.Close()

	// Create task registry
	registry := worker.NewRegistry()

	// Load task functions dynamically
	taskFunctions, err := loadTaskFunctions()
	if err != nil {
		log.Fatalf("Failed to load task functions: %v", err)
	}

	// Register discovered task functions
	for name, fn := range taskFunctions {
		if err := registry.Register(name, fn); err != nil {
			log.Fatalf("Failed to register task function %s: %v", name, err)
		}
	}

	// Create task executor
	executor := worker.NewTaskExecutor(registry, cfg.Worker.MaxWorkers, time.Duration(cfg.Worker.ShutdownTimeout)*time.Second)

	// Create and start orchestrator
	orch := orchestrator.NewOrchestrator(cfg, db, cache, queue)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start task executor
	executor.Start(ctx)

	// Start orchestrator
	go func() {
		if err := orch.Start(ctx); err != nil {
			log.Printf("Orchestrator stopped with error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("Received shutdown signal: %v", sig)

	// Initiate shutdown
	shutdownTimeout := time.Duration(cfg.Worker.ShutdownTimeout) * time.Second
	if err := orch.Shutdown(shutdownTimeout); err != nil {
		log.Printf("Error during orchestrator shutdown: %v", err)
	}

	// Stop task executor
	executor.Stop()

	log.Println("Orchestrator shutdown complete")
}
