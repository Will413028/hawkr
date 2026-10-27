package config

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("config",
	fx.Provide(Load),
	fx.Provide(func(cfg *Config) *zap.Logger { return logger.NewLogger(cfg.Env) }),
	fx.Invoke(func(l *zap.Logger) { zap.ReplaceGlobals(l) }),
	fx.Provide(newPool),
	fx.Provide(newTxManager),
	fx.Provide(newCipher),
)

// newCipher builds the AES-256-GCM cipher for at-rest PII encryption
// (certification id_card_no, #608). Config.Validate() already guarantees
// EncryptionMasterKey is non-empty, so crypto.New never fails-closed here.
func newCipher(cfg *Config) (*crypto.Cipher, error) {
	return crypto.New(cfg.EncryptionMasterKey)
}

func newPool(lc fx.Lifecycle, cfg *Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := database.NewPool(context.Background(), cfg.Database.DSN())
	if err != nil {
		return nil, err
	}
	logger.Info("Database connection established")
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error { logger.Info("Closing database connection"); pool.Close(); return nil },
	})
	return pool, nil
}

func newTxManager(pool *pgxpool.Pool) database.TxManager {
	return database.NewTxManager(pool)
}
