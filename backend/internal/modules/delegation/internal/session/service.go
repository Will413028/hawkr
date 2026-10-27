package session

import (
	"context"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

type Deps struct {
	DB       *db.DB
	Identity *identity.Module
	// StoreState 讀店家此刻的狀態；讀取失敗回 seller 的錯誤，不是拒絕。
	StoreState func(ctx context.Context, storeID int64) (model.StoreState, error)
	Now        func() time.Time
	Logger     *slog.Logger
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

func (m *Module) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
