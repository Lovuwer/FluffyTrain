// Package health provides health check functionality for the Titan job queue system.
package health

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joek3softwares-boop/titan/internal/redis"
)

// RedisChecker creates a health checker for Redis.
func RedisChecker(client redis.Client) Checker {
	return func(ctx context.Context) ComponentStatus {
		start := time.Now()
		
		err := client.Ping(ctx)
		latency := time.Since(start)
		
		if err != nil {
			return ComponentStatus{
				Name:    "redis",
				Status:  StatusUnhealthy,
				Latency: latency.String(),
				Error:   err.Error(),
			}
		}
		
		return ComponentStatus{
			Name:    "redis",
			Status:  StatusHealthy,
			Latency: latency.String(),
		}
	}
}

// PostgresChecker creates a health checker for PostgreSQL.
func PostgresChecker(pool *pgxpool.Pool) Checker {
	return func(ctx context.Context) ComponentStatus {
		start := time.Now()
		
		err := pool.Ping(ctx)
		latency := time.Since(start)
		
		if err != nil {
			return ComponentStatus{
				Name:    "postgres",
				Status:  StatusUnhealthy,
				Latency: latency.String(),
				Error:   err.Error(),
			}
		}
		
		return ComponentStatus{
			Name:    "postgres",
			Status:  StatusHealthy,
			Latency: latency.String(),
		}
	}
}

// PostgresCheckerWithDSN creates a health checker for PostgreSQL using a connection string.
func PostgresCheckerWithDSN(dsn string) Checker {
	return func(ctx context.Context) ComponentStatus {
		start := time.Now()
		
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return ComponentStatus{
				Name:    "postgres",
				Status:  StatusUnhealthy,
				Latency: time.Since(start).String(),
				Error:   fmt.Sprintf("failed to connect: %v", err),
			}
		}
		defer pool.Close()
		
		err = pool.Ping(ctx)
		latency := time.Since(start)
		
		if err != nil {
			return ComponentStatus{
				Name:    "postgres",
				Status:  StatusUnhealthy,
				Latency: latency.String(),
				Error:   err.Error(),
			}
		}
		
		return ComponentStatus{
			Name:    "postgres",
			Status:  StatusHealthy,
			Latency: latency.String(),
		}
	}
}
