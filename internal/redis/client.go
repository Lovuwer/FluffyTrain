// package redis - redis client wrapper with connection pooling and circuit breaker
package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client - interface for redis operations, use for mocking in tests
type Client interface {
	Ping(ctx context.Context) error
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	LPush(ctx context.Context, key string, values ...interface{}) error
	RPush(ctx context.Context, key string, values ...interface{}) error
	LPop(ctx context.Context, key string) (string, error)
	RPop(ctx context.Context, key string) (string, error)
	BLMove(ctx context.Context, source, dest, srcpos, destpos string, timeout time.Duration) (string, error)
	LMove(ctx context.Context, source, dest, srcpos, destpos string) (string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LRem(ctx context.Context, key string, count int64, value interface{}) error
	HSet(ctx context.Context, key string, values ...interface{}) error
	HGet(ctx context.Context, key, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error
	ZAdd(ctx context.Context, key string, members ...Z) error
	ZRem(ctx context.Context, key string, members ...interface{}) error
	ZRangeByScore(ctx context.Context, key string, opt *ZRangeBy) ([]string, error)
	ZCard(ctx context.Context, key string) (int64, error)
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	EvalSha(ctx context.Context, sha string, keys []string, args ...interface{}) (interface{}, error)
	ScriptLoad(ctx context.Context, script string) (string, error)
	TxPipeline() Pipeline
	Close() error
	IsHealthy() bool
}

// Pipeline - for transactional operations
type Pipeline interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	LRem(ctx context.Context, key string, count int64, value interface{}) *redis.IntCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	Exec(ctx context.Context) ([]redis.Cmder, error)
}

// Z - sorted set member
type Z struct {
	Score  float64
	Member interface{}
}

// ZRangeBy - range options for sorted sets
type ZRangeBy struct {
	Min, Max      string
	Offset, Count int64
}

type circuitState int32

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

type client struct {
	rdb         *redis.Client
	opts        Options
	mu          sync.RWMutex
	state       atomic.Int32
	failures    atomic.Int32
	lastFailure atomic.Int64
}

// New - create a redis client
func New(opts Options) (Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         opts.Addr(),
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
		MaxRetries:   opts.MaxRetries,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	})

	c := &client{
		rdb:  rdb,
		opts: opts,
	}
	c.state.Store(int32(circuitClosed))

	return c, nil
}

func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	if err == redis.Nil {
		return fmt.Errorf("redis %s: %w", op, ErrNotFound)
	}
	return fmt.Errorf("redis %s: %w", op, err)
}

var ErrNotFound = fmt.Errorf("key not found")
var ErrCircuitOpen = fmt.Errorf("circuit breaker is open, check redis connectivity")

func (c *client) checkCircuit() error {
	state := circuitState(c.state.Load())
	
	switch state {
	case circuitOpen:
		lastFail := c.lastFailure.Load()
		if time.Since(time.Unix(0, lastFail)) > c.opts.CircuitBreakerTimeout {
			c.state.CompareAndSwap(int32(circuitOpen), int32(circuitHalfOpen))
			return nil
		}
		return ErrCircuitOpen
	case circuitHalfOpen:
		return nil
	default:
		return nil
	}
}

func (c *client) recordSuccess() {
	c.failures.Store(0)
	c.state.Store(int32(circuitClosed))
}

func (c *client) recordFailure() {
	failures := c.failures.Add(1)
	c.lastFailure.Store(time.Now().UnixNano())

	if int(failures) >= c.opts.CircuitBreakerThreshold {
		c.state.Store(int32(circuitOpen))
	}
}

func (c *client) Ping(ctx context.Context) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.Ping(ctx).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("ping", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) Get(ctx context.Context, key string) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			c.recordFailure()
		}
		return "", wrapError("get", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.Set(ctx, key, value, expiration).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("set", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) Del(ctx context.Context, keys ...string) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.Del(ctx, keys...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("del", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if err := c.checkCircuit(); err != nil {
		return 0, err
	}
	
	val, err := c.rdb.Exists(ctx, keys...).Result()
	if err != nil {
		c.recordFailure()
		return 0, wrapError("exists", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	if err := c.checkCircuit(); err != nil {
		return false, err
	}
	
	val, err := c.rdb.Expire(ctx, key, expiration).Result()
	if err != nil {
		c.recordFailure()
		return false, wrapError("expire", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) LPush(ctx context.Context, key string, values ...interface{}) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.LPush(ctx, key, values...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("lpush", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) RPush(ctx context.Context, key string, values ...interface{}) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.RPush(ctx, key, values...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("rpush", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) LPop(ctx context.Context, key string) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.LPop(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			c.recordFailure()
		}
		return "", wrapError("lpop", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) RPop(ctx context.Context, key string) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.RPop(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			c.recordFailure()
		}
		return "", wrapError("rpop", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) BLMove(ctx context.Context, source, dest, srcpos, destpos string, timeout time.Duration) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.BLMove(ctx, source, dest, srcpos, destpos, timeout).Result()
	if err != nil {
		if err != redis.Nil {
			c.recordFailure()
		}
		return "", wrapError("blmove", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) LMove(ctx context.Context, source, dest, srcpos, destpos string) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.LMove(ctx, source, dest, srcpos, destpos).Result()
	if err != nil {
		if err != redis.Nil {
			c.recordFailure()
		}
		return "", wrapError("lmove", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) LLen(ctx context.Context, key string) (int64, error) {
	if err := c.checkCircuit(); err != nil {
		return 0, err
	}
	
	val, err := c.rdb.LLen(ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, wrapError("llen", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if err := c.checkCircuit(); err != nil {
		return nil, err
	}
	
	val, err := c.rdb.LRange(ctx, key, start, stop).Result()
	if err != nil {
		c.recordFailure()
		return nil, wrapError("lrange", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) LRem(ctx context.Context, key string, count int64, value interface{}) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.LRem(ctx, key, count, value).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("lrem", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) HSet(ctx context.Context, key string, values ...interface{}) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.HSet(ctx, key, values...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("hset", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) HGet(ctx context.Context, key, field string) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.HGet(ctx, key, field).Result()
	if err != nil {
		if err != redis.Nil {
			c.recordFailure()
		}
		return "", wrapError("hget", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if err := c.checkCircuit(); err != nil {
		return nil, err
	}
	
	val, err := c.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return nil, wrapError("hgetall", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) HDel(ctx context.Context, key string, fields ...string) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.HDel(ctx, key, fields...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("hdel", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) ZAdd(ctx context.Context, key string, members ...Z) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	zMembers := make([]redis.Z, len(members))
	for i, m := range members {
		zMembers[i] = redis.Z{Score: m.Score, Member: m.Member}
	}
	
	err := c.rdb.ZAdd(ctx, key, zMembers...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("zadd", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) ZRem(ctx context.Context, key string, members ...interface{}) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}
	
	err := c.rdb.ZRem(ctx, key, members...).Err()
	if err != nil {
		c.recordFailure()
		return wrapError("zrem", err)
	}
	c.recordSuccess()
	return nil
}

func (c *client) ZRangeByScore(ctx context.Context, key string, opt *ZRangeBy) ([]string, error) {
	if err := c.checkCircuit(); err != nil {
		return nil, err
	}
	
	val, err := c.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    opt.Min,
		Max:    opt.Max,
		Offset: opt.Offset,
		Count:  opt.Count,
	}).Result()
	if err != nil {
		c.recordFailure()
		return nil, wrapError("zrangebyscore", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) ZCard(ctx context.Context, key string) (int64, error) {
	if err := c.checkCircuit(); err != nil {
		return 0, err
	}
	
	val, err := c.rdb.ZCard(ctx, key).Result()
	if err != nil {
		c.recordFailure()
		return 0, wrapError("zcard", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if err := c.checkCircuit(); err != nil {
		return false, err
	}
	
	val, err := c.rdb.SetNX(ctx, key, value, expiration).Result()
	if err != nil {
		c.recordFailure()
		return false, wrapError("setnx", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if err := c.checkCircuit(); err != nil {
		return nil, err
	}
	
	val, err := c.rdb.Eval(ctx, script, keys, args...).Result()
	if err != nil {
		c.recordFailure()
		return nil, wrapError("eval", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) EvalSha(ctx context.Context, sha string, keys []string, args ...interface{}) (interface{}, error) {
	if err := c.checkCircuit(); err != nil {
		return nil, err
	}
	
	val, err := c.rdb.EvalSha(ctx, sha, keys, args...).Result()
	if err != nil {
		c.recordFailure()
		return nil, wrapError("evalsha", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) ScriptLoad(ctx context.Context, script string) (string, error) {
	if err := c.checkCircuit(); err != nil {
		return "", err
	}
	
	val, err := c.rdb.ScriptLoad(ctx, script).Result()
	if err != nil {
		c.recordFailure()
		return "", wrapError("scriptload", err)
	}
	c.recordSuccess()
	return val, nil
}

func (c *client) TxPipeline() Pipeline {
	return c.rdb.TxPipeline()
}

func (c *client) Close() error {
	return c.rdb.Close()
}

func (c *client) IsHealthy() bool {
	return circuitState(c.state.Load()) != circuitOpen
}
