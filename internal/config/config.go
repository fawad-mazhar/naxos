// internal/config/config.go
package config

import (
	"fmt"
	"os"

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
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
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

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}
