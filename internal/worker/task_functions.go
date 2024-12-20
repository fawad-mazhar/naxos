// internal/worker/task_functions.go
package worker

import (
	"context"
	"log"
	"time"
)

// TaskFunction represents a function that can be executed as a task
type TaskFunction func(ctx context.Context, data map[string]interface{}) error

// TaskFunctions interface defines all available task operations
type TaskFunctions interface {
	Task1(ctx context.Context, data map[string]interface{}) error
	Task2(ctx context.Context, data map[string]interface{}) error
	Task3(ctx context.Context, data map[string]interface{}) error
	Task4(ctx context.Context, data map[string]interface{}) error
}

// Task1 implements a sample task operation
func Task1(ctx context.Context, data map[string]interface{}) error {
	log.Printf("Executing Task 1 with data %v", data)
	time.Sleep(10 * time.Second)
	return nil
}

// Task2 implements another sample task operation
func Task2(ctx context.Context, data map[string]interface{}) error {
	log.Println("Executing Task 2")
	time.Sleep(8 * time.Second)
	return nil
}

// Task3 implements a third sample task operation
func Task3(ctx context.Context, data map[string]interface{}) error {
	log.Println("Executing Task 3")
	time.Sleep(5 * time.Second)
	return nil
}

// Task4 implements a fourth sample task operation
func Task4(ctx context.Context, data map[string]interface{}) error {
	log.Println("Executing Task 4")
	time.Sleep(1 * time.Second)
	return nil
}

// GetTaskFunction returns the implementation for a given task name
func GetTaskFunction(name string) interface{} {
	switch name {
	case "Task1":
		return Task1
	case "Task2":
		return Task2
	case "Task3":
		return Task3
	case "Task4":
		return Task4
	default:
		return nil
	}
}
