// internal/config/config.go
package config

import (
	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	LevelDB  LevelDBConfig  `mapstructure:"leveldb"`
	Worker   WorkerConfig   `mapstructure:"worker"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"readTimeout"`
	WriteTimeout int    `mapstructure:"writeTimeout"`
}

// PostgresConfig holds PostgreSQL configuration
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// RabbitMQConfig holds RabbitMQ configuration
type RabbitMQConfig struct {
	URL          string `mapstructure:"url"`
	Exchange     string `mapstructure:"exchange"`
	Queue        string `mapstructure:"queue"`
	JobsQueue    string `mapstructure:"jobsQueue"`
	StatusQueue  string `mapstructure:"statusQueue"`
	ExchangeType string `mapstructure:"exchangeType"`
}

// LevelDBConfig holds LevelDB configuration
type LevelDBConfig struct {
	Path string `mapstructure:"path"`
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	MaxWorkers      int `mapstructure:"maxWorkers"`
	MaxRetries      int `mapstructure:"maxRetries"`
	RetryDelay      int `mapstructure:"retryDelay"`
	ShutdownTimeout int `mapstructure:"shutdownTimeout"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	var config Config

	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
