// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/fawad-mazhar/naxos/internal/models"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the application
type Config struct {
	Server         ServerConfig           `yaml:"server"`
	Postgres       PostgresConfig         `yaml:"postgres"`
	RabbitMQ       RabbitMQConfig         `yaml:"rabbitmq"`
	LevelDB        LevelDBConfig          `yaml:"leveldb"`
	Worker         WorkerConfig           `yaml:"worker"`
	JobDefinitions []models.JobDefinition `yaml:"job_definitions"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string `yaml:"port"`
	ReadTimeout  int    `yaml:"readTimeout"`
	WriteTimeout int    `yaml:"writeTimeout"`
}

// PostgresConfig holds PostgreSQL configuration
type PostgresConfig struct {
	URL string `yaml:"-"`
}

// RabbitMQConfig holds RabbitMQ configuration
type RabbitMQConfig struct {
	URL          string `yaml:"url"`
	Exchange     string `yaml:"exchange"`
	JobsQueue    string `yaml:"jobsQueue"`
	StatusQueue  string `yaml:"statusQueue"`
	ExchangeType string `yaml:"exchangeType"`
}

// LevelDBConfig holds LevelDB configuration
type LevelDBConfig struct {
	Path string `yaml:"path"`
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	MaxWorkers      int `yaml:"maxWorkers"`
	ShutdownTimeout int `yaml:"shutdownTimeout"`
}

// Default configuration values
const (
	DefaultServerPort         = "8080"
	DefaultServerReadTimeout  = 30
	DefaultServerWriteTimeout = 30
	DefaultMaxWorkers         = 10
	DefaultMaxRetries         = 3
	DefaultRetryDelay         = 5
	DefaultShutdownTimeout    = 30
	DefaultLevelDBPath        = "./data/leveldb"
	DefaultRabbitMQExchange   = "naxos"
	DefaultJobsQueue          = "naxos.jobs"
	DefaultStatusQueue        = "naxos.status"
	DefaultExchangeType       = "direct"
)

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an environment variable as integer or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// Load creates a new configuration with environment variables and job definitions from YAML file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Check mandatory environment variables
	postgresURL := os.Getenv("NAXOS_POSTGRES_URL")
	if postgresURL == "" {
		return nil, fmt.Errorf("NAXOS_POSTGRES_URL environment variable is required")
	}

	rabbitmqURL := os.Getenv("NAXOS_RABBITMQ_URL")
	if rabbitmqURL == "" {
		return nil, fmt.Errorf("NAXOS_RABBITMQ_URL environment variable is required")
	}

	// Override/set configuration with environment variables and defaults
	config.Server = ServerConfig{
		Port:         getEnv("NAXOS_SERVER_PORT", DefaultServerPort),
		ReadTimeout:  getEnvInt("NAXOS_SERVER_READ_TIMEOUT", DefaultServerReadTimeout),
		WriteTimeout: getEnvInt("NAXOS_SERVER_WRITE_TIMEOUT", DefaultServerWriteTimeout),
	}

	config.Postgres = PostgresConfig{
		URL: postgresURL,
	}

	config.RabbitMQ = RabbitMQConfig{
		URL:          rabbitmqURL,
		Exchange:     getEnv("NAXOS_RABBITMQ_EXCHANGE", DefaultRabbitMQExchange),
		JobsQueue:    getEnv("NAXOS_RABBITMQ_JOBS_QUEUE", DefaultJobsQueue),
		StatusQueue:  getEnv("NAXOS_RABBITMQ_STATUS_QUEUE", DefaultStatusQueue),
		ExchangeType: getEnv("NAXOS_RABBITMQ_EXCHANGE_TYPE", DefaultExchangeType),
	}

	config.LevelDB = LevelDBConfig{
		Path: getEnv("NAXOS_LEVELDB_PATH", DefaultLevelDBPath),
	}

	config.Worker = WorkerConfig{
		MaxWorkers:      getEnvInt("NAXOS_WORKER_MAX_WORKERS", DefaultMaxWorkers),
		ShutdownTimeout: getEnvInt("NAXOS_WORKER_SHUTDOWN_TIMEOUT", DefaultShutdownTimeout),
	}

	// Initialize empty job definitions slice if none were loaded from file
	if config.JobDefinitions == nil {
		config.JobDefinitions = make([]models.JobDefinition, 0)
	}

	return &config, nil
}
