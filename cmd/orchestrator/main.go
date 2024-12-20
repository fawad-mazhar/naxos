// cmd/orchestrator/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
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
	pkgType := reflect.TypeOf((*worker.TaskFunctions)(nil)).Elem()

	// Iterate through all methods in the interface
	for i := 0; i < pkgType.NumMethod(); i++ {
		method := pkgType.Method(i)

		// Verify the method signature matches our TaskFunction type
		if method.Type.NumIn() == 2 &&
			method.Type.In(0).String() == "context.Context" &&
			method.Type.In(1).String() == "map[string]interface {}" &&
			method.Type.NumOut() == 1 &&
			method.Type.Out(0).String() == "error" {

			// Use the method name directly instead of converting to functionName
			// This will match the task names in job definitions
			functionName := method.Name

			// Get the actual function implementation
			fn := reflect.ValueOf(worker.GetTaskFunction(method.Name)).Interface().(func(context.Context, map[string]interface{}) error)

			taskFunctions[functionName] = fn
			fmt.Printf("Loaded task function: %s\n", functionName)
		}
	}

	if len(taskFunctions) == 0 {
		return nil, fmt.Errorf("no task functions found")
	}

	return taskFunctions, nil
}

func main() {
	// Load configuration
	cfg, err := config.Load("naxos.yaml")
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
	cache, err := leveldb.NewClient(cfg.LevelDB, 24*time.Hour)
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

	// Load task functions dynamically
	taskFunctions, err := loadTaskFunctions()
	if err != nil {
		log.Fatalf("Failed to load task functions: %v", err)
	}

	// Create and start orchestrator with task functions
	orch := orchestrator.NewOrchestrator(cfg, db, cache, queue, taskFunctions)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Initiate graceful shutdown
	shutdownTimeout := time.Duration(cfg.Worker.ShutdownTimeout) * time.Second
	if err := orch.Shutdown(shutdownTimeout); err != nil {
		log.Printf("Error during orchestrator shutdown: %v", err)
	}

	log.Println("Orchestrator shutdown complete")
}
