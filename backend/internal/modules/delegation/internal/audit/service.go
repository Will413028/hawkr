package audit

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type Deps struct {
	DB      *db.DB
	Events  *events.Bus
	Session Sessions
	NewID   func() uuid.UUID
	Now     func() time.Time
}

type Sessions interface {
	Session(ctx context.Context, id int64) (model.DelegationSession, error)
	SessionForAudit(ctx context.Context, in model.AuditSessionInput) (model.DelegationSession, error)
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

func (m *Module) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
