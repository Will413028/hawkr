package sse

import (
	"context"
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module wires the SSE Hub + lifecycle hooks。
// Hub starts synchronously on app start (pure in-memory, no external deps)
// and shuts down by cancelling the GC goroutine on app stop。
var Module = fx.Module("sse",
	fx.Provide(NewHubFromEnv),
	fx.Invoke(startHub),
)

// NewHubFromEnv constructs a Hub using config from environment variables，
// falling back to DefaultConfig for any unset / invalid value。
// See 41 號設計書 §15 配置與環境變數。
func NewHubFromEnv(log *zap.Logger) *Hub {
	cfg := DefaultConfig()
	cfg.HeartbeatInterval = envDuration("SSE_HEARTBEAT_INTERVAL", cfg.HeartbeatInterval)
	cfg.RingBufferSize = envInt("SSE_RING_BUFFER_SIZE", cfg.RingBufferSize)
	cfg.RingBufferTTL = envDuration("SSE_RING_BUFFER_TTL", cfg.RingBufferTTL)
	cfg.BatchWindow = envDurationMS("SSE_BATCH_WINDOW_MS", cfg.BatchWindow)
	cfg.MaxConnPerPod = envInt("SSE_MAX_CONN_PER_POD", cfg.MaxConnPerPod)
	cfg.MaxConnPerSubject = envInt("SSE_MAX_CONN_PER_SUBJECT", cfg.MaxConnPerSubject)
	cfg.MaxConnPerUser = envInt("SSE_MAX_CONN_PER_USER", cfg.MaxConnPerUser)
	// issue #2040：連線最長存活與回收額度視窗。SSE_MAX_CONN_LIFETIME 允許 "0"
	// 停用（envDuration 對 0 會照收），其餘走一般 time.ParseDuration 格式。
	cfg.MaxConnLifetime = envDuration("SSE_MAX_CONN_LIFETIME", cfg.MaxConnLifetime)
	cfg.EvictWindow = envDuration("SSE_EVICT_WINDOW", cfg.EvictWindow)
	return NewHub(cfg, log)
}

func startHub(lc fx.Lifecycle, hub *Hub, log *zap.Logger) {
	var stop func()
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			stop = hub.Start(context.Background())
			log.Info("SSE.hub.started",
				zap.Int("ringBufferSize", hub.cfg.RingBufferSize),
				zap.Duration("ringBufferTTL", hub.cfg.RingBufferTTL),
				zap.Duration("heartbeatInterval", hub.cfg.HeartbeatInterval),
				zap.Int("maxConnPerPod", hub.cfg.MaxConnPerPod),
				zap.Int("maxConnPerUser", hub.cfg.MaxConnPerUser),
				zap.Duration("maxConnLifetime", hub.cfg.MaxConnLifetime),
				zap.Duration("evictWindow", hub.cfg.EvictWindow))
			return nil
		},
		OnStop: func(_ context.Context) error {
			if stop != nil {
				stop()
			}
			return nil
		},
	})
}

// ---------- env helpers ---------- //

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envDurationMS(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return time.Duration(n) * time.Millisecond
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
