package postgres

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// Module Postgres Repository Layer
var Module = fx.Module("repository.postgres",
	fx.Provide(newPool),
	fx.Provide(newTxManager),
	fx.Provide(
		fx.Annotate(NewConversationRepository, fx.As(new(domain.ConversationRepository))),
		fx.Annotate(NewMessageRepository, fx.As(new(domain.MessageRepository))),
	),
)

func newPool(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := database.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

func newTxManager(pool *pgxpool.Pool) database.TxManager {
	return database.NewTxManager(pool)
}
