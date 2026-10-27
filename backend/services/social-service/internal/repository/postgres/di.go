package postgres

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("repository",
	fx.Provide(
		newDBPool,
		fx.Annotate(NewPostRepository, fx.As(new(domain.PostRepository))),
		fx.Annotate(NewCommentRepository, fx.As(new(domain.CommentRepository))),
		fx.Annotate(NewForumRepository, fx.As(new(domain.ForumRepository))),
		fx.Annotate(NewReportRepository, fx.As(new(domain.ReportRepository))),
		fx.Annotate(NewBlockRepository, fx.As(new(domain.BlockRepository))),
		fx.Annotate(NewModerationActionRepository, fx.As(new(domain.ModerationActionRepository))),
		fx.Annotate(NewHashtagRepository, fx.As(new(domain.HashtagRepository))),
		fx.Annotate(NewScheduledPostRepository, fx.As(new(domain.ScheduledPostRepository))),
	),
)

func newDBPool(cfg *config.Config, lc fx.Lifecycle, log *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := database.NewPool(context.Background(), cfg.DB.DSN())
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("db ping: %w", err)
			}
			log.Info("Connected to PostgreSQL", zap.String("db", cfg.DB.Name))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})
	return pool, nil
}
