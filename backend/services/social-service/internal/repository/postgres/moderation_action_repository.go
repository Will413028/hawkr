package postgres

import (
	"context"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModerationActionRepository struct {
	q *gen.Queries
}

func NewModerationActionRepository(pool *pgxpool.Pool) *ModerationActionRepository {
	return &ModerationActionRepository{q: gen.New(pool)}
}

func (r *ModerationActionRepository) Create(ctx context.Context, p domain.ModerationActionParams) error {
	_, err := r.q.CreateModerationAction(ctx, gen.CreateModerationActionParams{
		ID:               p.ID,
		ReportID:         nullableInt64(p.ReportID),
		AdminID:          nullableInt64(p.AdminID),
		AffectedUserID:   nullableInt64(p.AffectedUserID),
		Action:           p.Action,
		Note:             nullableText(p.Note),
		AttemptedContent: nullableText(p.AttemptedContent),
		PreviousState:    p.PreviousState,
		NewState:         p.NewState,
	})
	return err
}
