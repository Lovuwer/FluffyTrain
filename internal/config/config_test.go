package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars(t)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check Redis defaults
	if cfg.Redis.Host != "localhost" {
		t.Errorf("Redis.Host = %v, want localhost", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("Redis.Port = %v, want 6379", cfg.Redis.Port)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Redis.DB = %v, want 0", cfg.Redis.DB)
	}
	if cfg.Redis.PoolSize != 10 {
		t.Errorf("Redis.PoolSize = %v, want 10", cfg.Redis.PoolSize)
	}

	// Check Postgres defaults
	if cfg.Postgres.Host != "localhost" {
		t.Errorf("Postgres.Host = %v, want localhost", cfg.Postgres.Host)
	}
	if cfg.Postgres.Port != 5432 {
		t.Errorf("Postgres.Port = %v, want 5432", cfg.Postgres.Port)
	}
	if cfg.Postgres.User != "titan" {
		t.Errorf("Postgres.User = %v, want titan", cfg.Postgres.User)
	}
	if cfg.Postgres.Database != "titan" {
		t.Errorf("Postgres.Database = %v, want titan", cfg.Postgres.Database)
	}
	if cfg.Postgres.SSLMode != "disable" {
		t.Errorf("Postgres.SSLMode = %v, want disable", cfg.Postgres.SSLMode)
	}

	// Check API defaults
	if cfg.API.Port != 8080 {
		t.Errorf("API.Port = %v, want 8080", cfg.API.Port)
	}
	if cfg.API.ReadTimeout != 30*time.Second {
		t.Errorf("API.ReadTimeout = %v, want 30s", cfg.API.ReadTimeout)
	}
	if cfg.API.WriteTimeout != 30*time.Second {
		t.Errorf("API.WriteTimeout = %v, want 30s", cfg.API.WriteTimeout)
	}

	// Check Worker defaults
	if cfg.Worker.Concurrency != 10 {
		t.Errorf("Worker.Concurrency = %v, want 10", cfg.Worker.Concurrency)
	}
	if cfg.Worker.PollInterval != 1*time.Second {
		t.Errorf("Worker.PollInterval = %v, want 1s", cfg.Worker.PollInterval)
	}
	if cfg.Worker.MaxRetries != 3 {
		t.Errorf("Worker.MaxRetries = %v, want 3", cfg.Worker.MaxRetries)
	}
	if cfg.Worker.BackoffInitial != 1*time.Second {
		t.Errorf("Worker.BackoffInitial = %v, want 1s", cfg.Worker.BackoffInitial)
	}
	if cfg.Worker.BackoffMax != 1*time.Minute {
		t.Errorf("Worker.BackoffMax = %v, want 1m", cfg.Worker.BackoffMax)
	}
	if cfg.Worker.BackoffFactor != 2.0 {
		t.Errorf("Worker.BackoffFactor = %v, want 2.0", cfg.Worker.BackoffFactor)
	}

	// Check Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %v, want info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %v, want json", cfg.Logging.Format)
	}
}

func TestLoad_EnvVarsOverrideDefaults(t *testing.T) {
	// Clear and set specific environment variables
	clearEnvVars(t)

	t.Setenv("TITAN_REDIS_HOST", "redis.example.com")
	t.Setenv("TITAN_REDIS_PORT", "6380")
	t.Setenv("TITAN_REDIS_PASSWORD", "secret123")
	t.Setenv("TITAN_POSTGRES_HOST", "pg.example.com")
	t.Setenv("TITAN_POSTGRES_PASSWORD", "pgpass456")
	t.Setenv("TITAN_API_PORT", "9000")
	t.Setenv("TITAN_WORKER_CONCURRENCY", "20")
	t.Setenv("TITAN_LOGGING_LEVEL", "debug")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify environment variables override defaults
	if cfg.Redis.Host != "redis.example.com" {
		t.Errorf("Redis.Host = %v, want redis.example.com", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Errorf("Redis.Port = %v, want 6380", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "secret123" {
		t.Errorf("Redis.Password = %v, want secret123", cfg.Redis.Password)
	}
	if cfg.Postgres.Host != "pg.example.com" {
		t.Errorf("Postgres.Host = %v, want pg.example.com", cfg.Postgres.Host)
	}
	if cfg.Postgres.Password != "pgpass456" {
		t.Errorf("Postgres.Password = %v, want pgpass456", cfg.Postgres.Password)
	}
	if cfg.API.Port != 9000 {
		t.Errorf("API.Port = %v, want 9000", cfg.API.Port)
	}
	if cfg.Worker.Concurrency != 20 {
		t.Errorf("Worker.Concurrency = %v, want 20", cfg.Worker.Concurrency)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %v, want debug", cfg.Logging.Level)
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	// Clear environment variables
	clearEnvVars(t)

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
redis:
  host: redis-from-file.local
  port: 6381
  pool_size: 20
postgres:
  host: pg-from-file.local
  user: fileuser
api:
  port: 8888
logging:
  level: warn
  format: text
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify file values are loaded
	if cfg.Redis.Host != "redis-from-file.local" {
		t.Errorf("Redis.Host = %v, want redis-from-file.local", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6381 {
		t.Errorf("Redis.Port = %v, want 6381", cfg.Redis.Port)
	}
	if cfg.Redis.PoolSize != 20 {
		t.Errorf("Redis.PoolSize = %v, want 20", cfg.Redis.PoolSize)
	}
	if cfg.Postgres.Host != "pg-from-file.local" {
		t.Errorf("Postgres.Host = %v, want pg-from-file.local", cfg.Postgres.Host)
	}
	if cfg.Postgres.User != "fileuser" {
		t.Errorf("Postgres.User = %v, want fileuser", cfg.Postgres.User)
	}
	if cfg.API.Port != 8888 {
		t.Errorf("API.Port = %v, want 8888", cfg.API.Port)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %v, want warn", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %v, want text", cfg.Logging.Format)
	}
}

func TestLoad_EnvVarsOverrideConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
redis:
  host: redis-from-file.local
  port: 6381
api:
  port: 8888
logging:
  level: warn
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set environment variables that should override file values
	clearEnvVars(t)
	t.Setenv("TITAN_REDIS_HOST", "redis-from-env.local")
	t.Setenv("TITAN_API_PORT", "9999")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify env vars override file values
	if cfg.Redis.Host != "redis-from-env.local" {
		t.Errorf("Redis.Host = %v, want redis-from-env.local (env should override file)", cfg.Redis.Host)
	}
	if cfg.API.Port != 9999 {
		t.Errorf("API.Port = %v, want 9999 (env should override file)", cfg.API.Port)
	}

	// Verify file values still apply where no env var is set
	if cfg.Redis.Port != 6381 {
		t.Errorf("Redis.Port = %v, want 6381 (from file)", cfg.Redis.Port)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %v, want warn (from file)", cfg.Logging.Level)
	}
}

func TestLoad_SecretsFromEnvOnly(t *testing.T) {
	// Create a config file with passwords (which should be ignored in favor of env vars)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
redis:
  password: file-redis-pass
postgres:
  password: file-pg-pass
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set environment variables for passwords
	clearEnvVars(t)
	t.Setenv("TITAN_REDIS_PASSWORD", "env-redis-pass")
	t.Setenv("TITAN_POSTGRES_PASSWORD", "env-pg-pass")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify env passwords override file passwords
	if cfg.Redis.Password != "env-redis-pass" {
		t.Errorf("Redis.Password = %v, want env-redis-pass (env should override file for secrets)", cfg.Redis.Password)
	}
	if cfg.Postgres.Password != "env-pg-pass" {
		t.Errorf("Postgres.Password = %v, want env-pg-pass (env should override file for secrets)", cfg.Postgres.Password)
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Create an invalid YAML file
	invalidContent := `
redis:
  host: test
  port: "not-a-number"  # This is valid YAML but will fail unmarshaling
  invalid-yaml: [
`
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	clearEnvVars(t)
	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() should return error for invalid config file")
	}
}

// clearEnvVars clears all TITAN_ prefixed environment variables
func clearEnvVars(t *testing.T) {
	t.Helper()
	envVars := []string{
		"TITAN_REDIS_HOST",
		"TITAN_REDIS_PORT",
		"TITAN_REDIS_PASSWORD",
		"TITAN_REDIS_DB",
		"TITAN_REDIS_POOL_SIZE",
		"TITAN_POSTGRES_HOST",
		"TITAN_POSTGRES_PORT",
		"TITAN_POSTGRES_USER",
		"TITAN_POSTGRES_PASSWORD",
		"TITAN_POSTGRES_DATABASE",
		"TITAN_POSTGRES_SSL_MODE",
		"TITAN_API_PORT",
		"TITAN_API_READ_TIMEOUT",
		"TITAN_API_WRITE_TIMEOUT",
		"TITAN_WORKER_CONCURRENCY",
		"TITAN_WORKER_POLL_INTERVAL",
		"TITAN_WORKER_MAX_RETRIES",
		"TITAN_WORKER_BACKOFF_INITIAL",
		"TITAN_WORKER_BACKOFF_MAX",
		"TITAN_WORKER_BACKOFF_FACTOR",
		"TITAN_LOGGING_LEVEL",
		"TITAN_LOGGING_FORMAT",
	}
	for _, env := range envVars {
		t.Setenv(env, "")
		os.Unsetenv(env)
	}
}
