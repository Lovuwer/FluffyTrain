// Package config provides configuration management for the Titan job queue system.
// It supports loading configuration from environment variables, YAML files, and command-line flags.
package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	flagsOnce sync.Once
	flagSet   *pflag.FlagSet
)

// Config holds all configuration for the Titan application.
type Config struct {
	Redis   RedisConfig   `mapstructure:"redis"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	API     APIConfig     `mapstructure:"api"`
	Worker  WorkerConfig  `mapstructure:"worker"`
	Logging LoggingConfig `mapstructure:"logging"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

// APIConfig holds API server settings.
type APIConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// WorkerConfig holds worker process settings.
type WorkerConfig struct {
	Concurrency     int           `mapstructure:"concurrency"`
	PollInterval    time.Duration `mapstructure:"poll_interval"`
	MaxRetries      int           `mapstructure:"max_retries"`
	BackoffInitial  time.Duration `mapstructure:"backoff_initial"`
	BackoffMax      time.Duration `mapstructure:"backoff_max"`
	BackoffFactor   float64       `mapstructure:"backoff_factor"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load loads configuration from environment variables, config file, and command-line flags.
// Priority order (highest to lowest): environment variables > config file > defaults.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Bind command-line flags
	bindFlags(v)

	// Configure config file loading
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./config")
		v.AddConfigPath(".")
	}

	// Read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Configure environment variable loading
	v.SetEnvPrefix("TITAN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific environment variables for secrets
	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for all configuration options.
func setDefaults(v *viper.Viper) {
	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)

	// Postgres defaults
	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.user", "titan")
	v.SetDefault("postgres.database", "titan")
	v.SetDefault("postgres.ssl_mode", "disable")

	// API defaults
	v.SetDefault("api.port", 8080)
	v.SetDefault("api.read_timeout", 30*time.Second)
	v.SetDefault("api.write_timeout", 30*time.Second)

	// Worker defaults
	v.SetDefault("worker.concurrency", 10)
	v.SetDefault("worker.poll_interval", 1*time.Second)
	v.SetDefault("worker.max_retries", 3)
	v.SetDefault("worker.backoff_initial", 1*time.Second)
	v.SetDefault("worker.backoff_max", 1*time.Minute)
	v.SetDefault("worker.backoff_factor", 2.0)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// bindFlags binds command-line flags to configuration options.
// Flags are only registered once to avoid redefinition errors in tests.
func bindFlags(v *viper.Viper) {
	flagsOnce.Do(func() {
		flagSet = pflag.NewFlagSet("titan", pflag.ContinueOnError)
		flagSet.Int("api.port", 8080, "API server port")
		flagSet.String("logging.level", "info", "Log level (debug/info/warn/error)")
		flagSet.String("logging.format", "json", "Log format (json/text)")
		flagSet.Int("worker.concurrency", 10, "Worker concurrency")
	})

	_ = v.BindPFlags(flagSet)
}

// bindEnvVars binds specific environment variables, especially for secrets.
// Secrets MUST come from environment variables, not config files.
func bindEnvVars(v *viper.Viper) {
	// Redis password (secret - must be from env)
	_ = v.BindEnv("redis.password", "TITAN_REDIS_PASSWORD")

	// Postgres password (secret - must be from env)
	_ = v.BindEnv("postgres.password", "TITAN_POSTGRES_PASSWORD")

	// Other overridable config values
	_ = v.BindEnv("redis.host", "TITAN_REDIS_HOST")
	_ = v.BindEnv("redis.port", "TITAN_REDIS_PORT")
	_ = v.BindEnv("redis.db", "TITAN_REDIS_DB")
	_ = v.BindEnv("redis.pool_size", "TITAN_REDIS_POOL_SIZE")

	_ = v.BindEnv("postgres.host", "TITAN_POSTGRES_HOST")
	_ = v.BindEnv("postgres.port", "TITAN_POSTGRES_PORT")
	_ = v.BindEnv("postgres.user", "TITAN_POSTGRES_USER")
	_ = v.BindEnv("postgres.database", "TITAN_POSTGRES_DATABASE")
	_ = v.BindEnv("postgres.ssl_mode", "TITAN_POSTGRES_SSL_MODE")

	_ = v.BindEnv("api.port", "TITAN_API_PORT")
	_ = v.BindEnv("api.read_timeout", "TITAN_API_READ_TIMEOUT")
	_ = v.BindEnv("api.write_timeout", "TITAN_API_WRITE_TIMEOUT")

	_ = v.BindEnv("worker.concurrency", "TITAN_WORKER_CONCURRENCY")
	_ = v.BindEnv("worker.poll_interval", "TITAN_WORKER_POLL_INTERVAL")
	_ = v.BindEnv("worker.max_retries", "TITAN_WORKER_MAX_RETRIES")
	_ = v.BindEnv("worker.backoff_initial", "TITAN_WORKER_BACKOFF_INITIAL")
	_ = v.BindEnv("worker.backoff_max", "TITAN_WORKER_BACKOFF_MAX")
	_ = v.BindEnv("worker.backoff_factor", "TITAN_WORKER_BACKOFF_FACTOR")

	_ = v.BindEnv("logging.level", "TITAN_LOGGING_LEVEL")
	_ = v.BindEnv("logging.format", "TITAN_LOGGING_FORMAT")
}
