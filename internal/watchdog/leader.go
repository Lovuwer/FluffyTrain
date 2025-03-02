// Package watchdog implements job recovery for stuck/orphaned jobs.
package watchdog

import (
	"context"
	"fmt"
	"time"

	"github.com/joek3softwares-boop/titan/internal/redis"
)

// LeaderElection handles distributed leader election using Redis.
type LeaderElection interface {
	// TryAcquire attempts to become the leader.
	TryAcquire(ctx context.Context) (bool, error)

	// Renew renews the leadership lock.
	Renew(ctx context.Context) error

	// Release releases leadership.
	Release(ctx context.Context) error

	// IsLeader returns true if this instance is the leader.
	IsLeader() bool
}

// leaderElection implements LeaderElection using Redis.
type leaderElection struct {
	client   redis.Client
	lockKey  string
	leaderID string
	ttl      time.Duration
	isLeader bool
}

// LeaderConfig configures leader election.
type LeaderConfig struct {
	// LockKey is the Redis key for the leader lock.
	LockKey string

	// LeaderID is a unique identifier for this instance.
	LeaderID string

	// TTL is how long the leader lock is valid.
	TTL time.Duration

	// RenewInterval is how often to renew the lock.
	RenewInterval time.Duration
}

// DefaultLeaderConfig returns a LeaderConfig with sensible defaults.
func DefaultLeaderConfig(leaderID string) LeaderConfig {
	return LeaderConfig{
		LockKey:       "titan:watchdog:leader",
		LeaderID:      leaderID,
		TTL:           30 * time.Second,
		RenewInterval: 10 * time.Second,
	}
}

// NewLeaderElection creates a new LeaderElection instance.
func NewLeaderElection(client redis.Client, cfg LeaderConfig) LeaderElection {
	return &leaderElection{
		client:   client,
		lockKey:  cfg.LockKey,
		leaderID: cfg.LeaderID,
		ttl:      cfg.TTL,
	}
}

func (l *leaderElection) TryAcquire(ctx context.Context) (bool, error) {
	acquired, err := l.client.SetNX(ctx, l.lockKey, l.leaderID, l.ttl)
	if err != nil {
		return false, fmt.Errorf("leader acquire: %w", err)
	}

	l.isLeader = acquired
	return acquired, nil
}

func (l *leaderElection) Renew(ctx context.Context) error {
	if !l.isLeader {
		return fmt.Errorf("leader renew: not the leader")
	}

	// Check if we still own the lock
	currentLeader, err := l.client.Get(ctx, l.lockKey)
	if err != nil {
		l.isLeader = false
		return fmt.Errorf("leader renew: %w", err)
	}

	if currentLeader != l.leaderID {
		l.isLeader = false
		return fmt.Errorf("leader renew: lost leadership")
	}

	// Extend TTL
	_, err = l.client.Expire(ctx, l.lockKey, l.ttl)
	if err != nil {
		l.isLeader = false
		return fmt.Errorf("leader renew: %w", err)
	}

	return nil
}

func (l *leaderElection) Release(ctx context.Context) error {
	if !l.isLeader {
		return nil
	}

	// Only delete if we own the lock
	currentLeader, err := l.client.Get(ctx, l.lockKey)
	if err != nil {
		l.isLeader = false
		return nil // Already released
	}

	if currentLeader == l.leaderID {
		_ = l.client.Del(ctx, l.lockKey)
	}

	l.isLeader = false
	return nil
}

func (l *leaderElection) IsLeader() bool {
	return l.isLeader
}
