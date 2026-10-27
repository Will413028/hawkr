package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// Admin 回傳管理員。
func (m *Service) Admin(ctx context.Context, id int64) (model.Admin, error) {
	a, err := m.q(ctx).GetAdmin(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Admin{}, apperr.NotFound("管理員 %d 不存在", id)
	}
	if err != nil {
		return model.Admin{}, fmt.Errorf("identity：讀取管理員：%w", err)
	}
	return model.Admin{ID: a.ID, Email: a.Email}, nil
}

// Admins 以一次查詢回傳存在的管理員；不存在的 ID 不會出現在結果中。
func (m *Service) Admins(ctx context.Context, ids []int64) (map[int64]model.Admin, error) {
	admins := make(map[int64]model.Admin, len(ids))
	if len(ids) == 0 {
		return admins, nil
	}
	rows, err := m.q(ctx).AdminsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity：批次讀取管理員：%w", err)
	}
	for _, row := range rows {
		admins[row.ID] = model.Admin{ID: row.ID, Email: row.Email}
	}
	return admins, nil
}

type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Now    func() time.Time
	Logger *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (m *Service) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
