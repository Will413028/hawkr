package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	client *redis.Client
	once   sync.Once
)

// Config holds Redis configuration
type Config struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// GetConfigFromEnv reads Redis configuration from environment variables
func GetConfigFromEnv() Config {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	password := os.Getenv("REDIS_PASSWORD")

	db := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		db, _ = strconv.Atoi(dbStr)
	}

	return Config{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
	}
}

// OptionsFromEnv builds redis.Options from REDIS_HOST/REDIS_PORT/REDIS_PASSWORD/
// REDIS_DB — the single source of truth for where Redis lives.
//
// There is deliberately no REDIS_URL override. A URL that silently outranks
// REDIS_HOST is invisible in the repo (it only ever exists inside a k8s Secret),
// and that invisibility caused the 2026-07-29 dev outage: every service had been
// talking to a managed provider for 12 days while k8s/values still read
// "REDIS_HOST: redis", so nothing in git — and no test — could reveal the drift.
// If a managed/TLS provider is ever needed again, add an explicit REDIS_TLS flag
// rather than a URL that overrides the declared host.
func OptionsFromEnv() *redis.Options {
	cfg := GetConfigFromEnv()
	return &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	}
}

// NewClient creates a new Redis client with the given configuration
func NewClient(cfg Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return rdb, nil
}

// GetClient returns a singleton Redis client
func GetClient() (*redis.Client, error) {
	var initErr error
	once.Do(func() {
		cfg := GetConfigFromEnv()
		client, initErr = NewClient(cfg)
	})
	return client, initErr
}

// MustGetClient returns the singleton Redis client or panics on error
func MustGetClient() *redis.Client {
	rdb, err := GetClient()
	if err != nil {
		panic(err)
	}
	return rdb
}
