package database

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration

	ConnectRetryBudget time.Duration
}

func DefaultPoolConfig(dsn string) *PoolConfig {
	maxConns := int32(10)
	if v := os.Getenv("DB_POOL_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConns = int32(n)
		}
	}
	return &PoolConfig{
		DSN:               dsn,
		MaxConns:          maxConns,
		MinConns:          0,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,

		ConnectRetryBudget: connectRetryBudget(),
	}
}

func connectRetryBudget() time.Duration {
	const defaultBudget = 60 * time.Second
	v := os.Getenv("DB_CONNECT_RETRY_BUDGET")
	if v == "" {
		return defaultBudget
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultBudget
	}
	return time.Duration(n) * time.Second
}

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if !strings.Contains(dsn, "connect_timeout") {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {

			if strings.Contains(dsn, "?") {
				dsn += "&connect_timeout=10"
			} else {
				dsn += "?connect_timeout=10"
			}
		} else {

			dsn += " connect_timeout=10"
		}
	}
	return NewPoolWithConfig(ctx, DefaultPoolConfig(dsn))
}

func NewPoolWithConfig(ctx context.Context, cfg *PoolConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, err
	}

	config.MaxConns = cfg.MaxConns
	config.MinConns = cfg.MinConns
	config.MaxConnLifetime = cfg.MaxConnLifetime
	config.MaxConnIdleTime = cfg.MaxConnIdleTime
	config.HealthCheckPeriod = cfg.HealthCheckPeriod

	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err := pingWithRetry(ctx, pool.Ping, cfg.ConnectRetryBudget, defaultRetryPolicy); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func pingWithRetry(ctx context.Context, ping func(context.Context) error, budget time.Duration, p retryPolicy) error {
	err := ping(ctx)
	if err == nil || budget <= 0 || !retryableConnectError(err) {
		return err
	}

	deadline := time.Now().Add(budget)
	backoff := p.base

	for attempt := 2; ; attempt++ {
		wait := backoff/2 + time.Duration(rand.Int64N(int64(backoff/2)))
		if time.Now().Add(wait).After(deadline) {
			return fmt.Errorf("資料庫連線在 %s 重試預算內未成功（嘗試 %d 次）：%w", budget, attempt-1, err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("資料庫連線重試被中止（嘗試 %d 次）：%w", attempt-1, ctx.Err())
		case <-time.After(wait):
		}

		err = ping(ctx)
		if err == nil {
			return nil
		}
		if !retryableConnectError(err) {
			return err
		}
		if backoff < p.max {
			backoff *= 2
		}
	}
}

type retryPolicy struct {
	base time.Duration
	max  time.Duration
}

var defaultRetryPolicy = retryPolicy{base: time.Second, max: 8 * time.Second}

var nonRetryableSQLStates = map[string]string{
	"28P01": "密碼認證失敗",
	"28000": "認證方式不被接受",
	"3D000": "資料庫不存在",
	"42501": "權限不足",
}

func retryableConnectError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if _, stop := nonRetryableSQLStates[pgErr.Code]; stop {
			return false
		}
	}
	return true
}
