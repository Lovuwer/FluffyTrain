package redis

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Host != "localhost" {
		t.Errorf("Host = %v, want localhost", opts.Host)
	}
	if opts.Port != 6379 {
		t.Errorf("Port = %v, want 6379", opts.Port)
	}
	if opts.PoolSize != 10 {
		t.Errorf("PoolSize = %v, want 10", opts.PoolSize)
	}
	if opts.CircuitBreakerThreshold != 5 {
		t.Errorf("CircuitBreakerThreshold = %v, want 5", opts.CircuitBreakerThreshold)
	}
}

func TestOptionsAddr(t *testing.T) {
	opts := Options{Host: "redis.example.com", Port: 6380}
	addr := opts.Addr()
	if addr != "redis.example.com:6380" {
		t.Errorf("Addr() = %v, want redis.example.com:6380", addr)
	}
}

func TestCircuitBreaker(t *testing.T) {
	opts := DefaultOptions()
	opts.CircuitBreakerThreshold = 3
	opts.CircuitBreakerTimeout = 100 * time.Millisecond

	c := &client{opts: opts}
	c.state.Store(int32(circuitClosed))

	// Simulate failures
	for i := 0; i < 3; i++ {
		c.recordFailure()
	}

	// Circuit should be open
	if c.IsHealthy() {
		t.Error("Circuit should be open after threshold failures")
	}

	err := c.checkCircuit()
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("checkCircuit() = %v, want ErrCircuitOpen", err)
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Circuit should transition to half-open
	err = c.checkCircuit()
	if err != nil {
		t.Errorf("checkCircuit() after timeout = %v, want nil", err)
	}

	// Success should close the circuit
	c.recordSuccess()
	if !c.IsHealthy() {
		t.Error("Circuit should be closed after success")
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		err     error
		wantNil bool
	}{
		{"nil error", "test", nil, true},
		{"wrapped error", "get", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapError(tt.op, tt.err)
			if tt.wantNil && got != nil {
				t.Errorf("wrapError() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("wrapError() = nil, want error")
			}
		})
	}
}

// MockClient is a mock implementation of the Client interface for testing.
type MockClient struct {
	PingFunc     func(ctx context.Context) error
	GetFunc      func(ctx context.Context, key string) (string, error)
	SetFunc      func(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	DelFunc      func(ctx context.Context, keys ...string) error
	ExistsFunc   func(ctx context.Context, keys ...string) (int64, error)
	ExpireFunc   func(ctx context.Context, key string, expiration time.Duration) (bool, error)
	LPushFunc    func(ctx context.Context, key string, values ...interface{}) error
	RPushFunc    func(ctx context.Context, key string, values ...interface{}) error
	LPopFunc     func(ctx context.Context, key string) (string, error)
	RPopFunc     func(ctx context.Context, key string) (string, error)
	BLMoveFunc   func(ctx context.Context, source, dest, srcpos, destpos string, timeout time.Duration) (string, error)
	LMoveFunc    func(ctx context.Context, source, dest, srcpos, destpos string) (string, error)
	LLenFunc     func(ctx context.Context, key string) (int64, error)
	LRangeFunc   func(ctx context.Context, key string, start, stop int64) ([]string, error)
	LRemFunc     func(ctx context.Context, key string, count int64, value interface{}) error
	HSetFunc     func(ctx context.Context, key string, values ...interface{}) error
	HGetFunc     func(ctx context.Context, key, field string) (string, error)
	HGetAllFunc  func(ctx context.Context, key string) (map[string]string, error)
	HDelFunc     func(ctx context.Context, key string, fields ...string) error
	ZAddFunc     func(ctx context.Context, key string, members ...Z) error
	ZRemFunc     func(ctx context.Context, key string, members ...interface{}) error
	ZRangeByScoreFunc func(ctx context.Context, key string, opt *ZRangeBy) ([]string, error)
	ZCardFunc    func(ctx context.Context, key string) (int64, error)
	SetNXFunc    func(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	EvalFunc     func(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	EvalShaFunc  func(ctx context.Context, sha string, keys []string, args ...interface{}) (interface{}, error)
	ScriptLoadFunc func(ctx context.Context, script string) (string, error)
	TxPipelineFunc func() Pipeline
	CloseFunc    func() error
	IsHealthyFunc func() bool
}

func (m *MockClient) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

func (m *MockClient) Get(ctx context.Context, key string) (string, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return "", nil
}

func (m *MockClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, expiration)
	}
	return nil
}

func (m *MockClient) Del(ctx context.Context, keys ...string) error {
	if m.DelFunc != nil {
		return m.DelFunc(ctx, keys...)
	}
	return nil
}

func (m *MockClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, keys...)
	}
	return 0, nil
}

func (m *MockClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	if m.ExpireFunc != nil {
		return m.ExpireFunc(ctx, key, expiration)
	}
	return true, nil
}

func (m *MockClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	if m.LPushFunc != nil {
		return m.LPushFunc(ctx, key, values...)
	}
	return nil
}

func (m *MockClient) RPush(ctx context.Context, key string, values ...interface{}) error {
	if m.RPushFunc != nil {
		return m.RPushFunc(ctx, key, values...)
	}
	return nil
}

func (m *MockClient) LPop(ctx context.Context, key string) (string, error) {
	if m.LPopFunc != nil {
		return m.LPopFunc(ctx, key)
	}
	return "", nil
}

func (m *MockClient) RPop(ctx context.Context, key string) (string, error) {
	if m.RPopFunc != nil {
		return m.RPopFunc(ctx, key)
	}
	return "", nil
}

func (m *MockClient) BLMove(ctx context.Context, source, dest, srcpos, destpos string, timeout time.Duration) (string, error) {
	if m.BLMoveFunc != nil {
		return m.BLMoveFunc(ctx, source, dest, srcpos, destpos, timeout)
	}
	return "", nil
}

func (m *MockClient) LMove(ctx context.Context, source, dest, srcpos, destpos string) (string, error) {
	if m.LMoveFunc != nil {
		return m.LMoveFunc(ctx, source, dest, srcpos, destpos)
	}
	return "", nil
}

func (m *MockClient) LLen(ctx context.Context, key string) (int64, error) {
	if m.LLenFunc != nil {
		return m.LLenFunc(ctx, key)
	}
	return 0, nil
}

func (m *MockClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if m.LRangeFunc != nil {
		return m.LRangeFunc(ctx, key, start, stop)
	}
	return nil, nil
}

func (m *MockClient) LRem(ctx context.Context, key string, count int64, value interface{}) error {
	if m.LRemFunc != nil {
		return m.LRemFunc(ctx, key, count, value)
	}
	return nil
}

func (m *MockClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	if m.HSetFunc != nil {
		return m.HSetFunc(ctx, key, values...)
	}
	return nil
}

func (m *MockClient) HGet(ctx context.Context, key, field string) (string, error) {
	if m.HGetFunc != nil {
		return m.HGetFunc(ctx, key, field)
	}
	return "", nil
}

func (m *MockClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if m.HGetAllFunc != nil {
		return m.HGetAllFunc(ctx, key)
	}
	return nil, nil
}

func (m *MockClient) HDel(ctx context.Context, key string, fields ...string) error {
	if m.HDelFunc != nil {
		return m.HDelFunc(ctx, key, fields...)
	}
	return nil
}

func (m *MockClient) ZAdd(ctx context.Context, key string, members ...Z) error {
	if m.ZAddFunc != nil {
		return m.ZAddFunc(ctx, key, members...)
	}
	return nil
}

func (m *MockClient) ZRem(ctx context.Context, key string, members ...interface{}) error {
	if m.ZRemFunc != nil {
		return m.ZRemFunc(ctx, key, members...)
	}
	return nil
}

func (m *MockClient) ZRangeByScore(ctx context.Context, key string, opt *ZRangeBy) ([]string, error) {
	if m.ZRangeByScoreFunc != nil {
		return m.ZRangeByScoreFunc(ctx, key, opt)
	}
	return nil, nil
}

func (m *MockClient) ZCard(ctx context.Context, key string) (int64, error) {
	if m.ZCardFunc != nil {
		return m.ZCardFunc(ctx, key)
	}
	return 0, nil
}

func (m *MockClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if m.SetNXFunc != nil {
		return m.SetNXFunc(ctx, key, value, expiration)
	}
	return true, nil
}

func (m *MockClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if m.EvalFunc != nil {
		return m.EvalFunc(ctx, script, keys, args...)
	}
	return nil, nil
}

func (m *MockClient) EvalSha(ctx context.Context, sha string, keys []string, args ...interface{}) (interface{}, error) {
	if m.EvalShaFunc != nil {
		return m.EvalShaFunc(ctx, sha, keys, args...)
	}
	return nil, nil
}

func (m *MockClient) ScriptLoad(ctx context.Context, script string) (string, error) {
	if m.ScriptLoadFunc != nil {
		return m.ScriptLoadFunc(ctx, script)
	}
	return "", nil
}

func (m *MockClient) TxPipeline() Pipeline {
	if m.TxPipelineFunc != nil {
		return m.TxPipelineFunc()
	}
	return nil
}

func (m *MockClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockClient) IsHealthy() bool {
	if m.IsHealthyFunc != nil {
		return m.IsHealthyFunc()
	}
	return true
}

// Verify MockClient implements Client interface
var _ Client = (*MockClient)(nil)
