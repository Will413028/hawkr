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
	// CancelSchedules 取消這些 session 發起的待發布排程（原因授權失效）；空清單是 no-op。它鎖 social 的列，所以每個呼叫點都在鎖或改
	// delegation 的列之前呼叫（單體規則 4）。
	CancelSchedules func(ctx context.Context, sessionIDs []int64) error
	// PendingScheduleSessionIDs 回傳 social 目前有待發布排程的代行 session ID（不鎖的讀取）。
	PendingScheduleSessionIDs func(ctx context.Context) ([]int64, error)
	Now                       func() time.Time
	Logger                    *slog.Logger
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

func (m *Module) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
