package watchdog

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("worker-1")

	if cfg.ScanInterval != 60*time.Second {
		t.Errorf("ScanInterval = %v, want 60s", cfg.ScanInterval)
	}
	if cfg.VisibilityTimeout != 5*time.Minute {
		t.Errorf("VisibilityTimeout = %v, want 5m", cfg.VisibilityTimeout)
	}
	if cfg.MaxRecoverPerScan != 100 {
		t.Errorf("MaxRecoverPerScan = %v, want 100", cfg.MaxRecoverPerScan)
	}
	if cfg.LeaderConfig.LeaderID != "worker-1" {
		t.Errorf("LeaderID = %v, want worker-1", cfg.LeaderConfig.LeaderID)
	}
}

func TestDefaultLeaderConfig(t *testing.T) {
	cfg := DefaultLeaderConfig("worker-2")

	if cfg.LockKey != "titan:watchdog:leader" {
		t.Errorf("LockKey = %v, want titan:watchdog:leader", cfg.LockKey)
	}
	if cfg.LeaderID != "worker-2" {
		t.Errorf("LeaderID = %v, want worker-2", cfg.LeaderID)
	}
	if cfg.TTL != 30*time.Second {
		t.Errorf("TTL = %v, want 30s", cfg.TTL)
	}
	if cfg.RenewInterval != 10*time.Second {
		t.Errorf("RenewInterval = %v, want 10s", cfg.RenewInterval)
	}
}
