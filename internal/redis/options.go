// Package redis provides a Redis client wrapper with connection pooling,
// circuit breaker, and error handling for the Titan job queue system.
package redis

import (
	"time"
)

// Options configures the Redis client connection.
type Options struct {
	// Host is the Redis server hostname.
	Host string

	// Port is the Redis server port.
	Port int

	// Password is the Redis password (optional).
	Password string

	// DB is the Redis database number.
	DB int

	// PoolSize is the maximum number of connections in the pool.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int

	// MaxRetries is the maximum number of retries before giving up.
	MaxRetries int

	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes.
	WriteTimeout time.Duration

	// CircuitBreakerThreshold is the number of consecutive failures before opening the circuit.
	CircuitBreakerThreshold int

	// CircuitBreakerTimeout is the time to wait before trying again after circuit opens.
	CircuitBreakerTimeout time.Duration

	// ReconnectBackoffInitial is the initial backoff duration for reconnection attempts.
	ReconnectBackoffInitial time.Duration

	// ReconnectBackoffMax is the maximum backoff duration for reconnection attempts.
	ReconnectBackoffMax time.Duration
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		Host:                    "localhost",
		Port:                    6379,
		Password:                "",
		DB:                      0,
		PoolSize:                10,
		MinIdleConns:            2,
		MaxRetries:              3,
		DialTimeout:             5 * time.Second,
		ReadTimeout:             3 * time.Second,
		WriteTimeout:            3 * time.Second,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		ReconnectBackoffInitial: 1 * time.Second,
		ReconnectBackoffMax:     30 * time.Second,
	}
}

// Addr returns the Redis server address in host:port format.
func (o Options) Addr() string {
	return o.Host + ":" + itoa(o.Port)
}

// itoa converts an int to a string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var s string
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		return "-" + s
	}
	return s
}
