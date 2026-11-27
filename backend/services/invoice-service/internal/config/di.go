package config

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/logger"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module is Layer 0（58 號設計書 §4.2.2）：Config / Logger / pool / TxManager / OTel / cipher。
var Module = fx.Module("config",
	fx.Provide(Load),
	fx.Provide(func(cfg *Config) *zap.Logger { return logger.NewLogger(cfg.Env) }),
	fx.Invoke(func(l *zap.Logger) { zap.ReplaceGlobals(l) }),
	fx.Provide(newPool),
	fx.Provide(newTxManager),
	fx.Provide(newCipher),
	fx.Invoke(initOTel),
)

func newCipher(cfg *Config) (*crypto.Cipher, error) {
	return crypto.New(cfg.Encryption.MasterKey)
}

func newPool(lc fx.Lifecycle, cfg *Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := database.NewPool(context.Background(), cfg.Database.DSN())
	if err != nil {
		return nil, err
	}
	logger.Info("Database connection established")
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing database connection")
			pool.Close()
			return nil
		},
	})
	return pool, nil
}

func newTxManager(pool *pgxpool.Pool) database.TxManager {
	return database.NewTxManager(pool)
}

func initOTel(lc fx.Lifecycle, logger *zap.Logger) {
	shutdown := pkgotel.InitTracer("invoice-service", logger)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down tracer provider")
			return shutdown(ctx)
		},
	})
}
