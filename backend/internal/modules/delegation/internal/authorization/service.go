package authorization

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

type Deps struct {
	DB       *db.DB
	Identity *identity.Module
	Session  Sessions
	// StoreState 讀店家此刻的狀態；讀取失敗回 seller 的錯誤，不是拒絕。
	StoreState func(ctx context.Context, storeID int64) (model.StoreState, error)
	NewID      func() uuid.UUID
	Now        func() time.Time
}

type Sessions interface {
	ActiveSessionsOfStore(ctx context.Context, storeID int64, now time.Time) (int64, error)
	RevokeActiveOfStore(ctx context.Context, in model.RevokeStoreSessionsInput) ([]int64, error)
	AdminEligible(ctx context.Context, adminID int64) (bool, error)
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

func (m *Module) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
