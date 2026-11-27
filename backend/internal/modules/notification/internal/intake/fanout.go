package intake

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// DefaultFanoutChunk 是 fan-out 一頁的預設收件人數（不超過 registry.MaxFollowerPage）。
const DefaultFanoutChunk = 200

func (s *Service) fanoutChunk() int {
	if s.d.FanoutChunk > 0 {
		return min(s.d.FanoutChunk, registry.MaxFollowerPage)
	}
	return DefaultFanoutChunk
}

// FanoutArgs 是 fan-out 母意圖的一頁：母意圖與上一頁最後一個收件人的 ID（0 是從頭）。
type FanoutArgs struct {
	IntentID uuid.UUID `json:"intent_id"`
	AfterID  int64     `json:"after_id"`
}

// Kind 是 River 的 job 種類。
func (FanoutArgs) Kind() string { return "notification.fanout_intent" }

func fanoutOpts() *river.InsertOpts {
	o := jobs.UniqueByArgs(0)
	return &o
}

func (s *Service) startFanout(ctx context.Context, intent store.NotificationIntent) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return s.d.Jobs.Enqueue(ctx, FanoutArgs{IntentID: intent.ID}, fanoutOpts())
	})
}

// Fanout 展開母意圖的一頁收件人：每位收件人一個子意圖（冪等鍵含收件人）與一個 notification.process_intent job，子意圖與下一頁的 job 同一個 transaction。
// 母意圖已被清除時什麼都不做。型別壞掉（子型別沒註冊、不是 Spawned、母型別不是 Broadcast）是永久錯誤。
func (s *Service) Fanout(ctx context.Context, a FanoutArgs) error {
	parent, err := store.New(s.d.DB.Conn(ctx)).IntentByID(ctx, a.IntentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("讀取母意圖：%w", err)
	}
	e, ok := s.d.Registry.Lookup(parent.Type)
	if !ok || !e.Spec().Broadcast {
		err := fmt.Errorf("通知型別 %q 沒有註冊或不是 fan-out 母意圖", parent.Type)
		s.log().WarnContext(ctx, "fan-out 的母意圖型別不對，取消 job", "intent_id", parent.ID, "type", parent.Type)
		return river.JobCancel(err)
	}
	child := e.Spec().Child
	if c, ok := s.d.Registry.Lookup(child); !ok || !c.Spec().Spawned {
		s.log().WarnContext(ctx, "fan-out 的子型別沒有註冊或不是 Spawned，取消 job", "intent_id", parent.ID, "type", parent.Type, "child", child)
		return river.JobCancel(fmt.Errorf("通知型別 %q 的子型別 %q 沒有註冊或不是 Spawned", parent.Type, child))
	}
	page, err := e.Spawn(ctx, s.d.Resolver, parent.Params, registry.PageRequest{After: a.AfterID, Limit: s.fanoutChunk(), OccurredAt: parent.OccurredAt})
	if errors.Is(err, registry.ErrBadParams) {
		s.log().WarnContext(ctx, "fan-out 母意圖的參數無法展開，取消 job", "intent_id", parent.ID, "type", parent.Type, "error", err)
		return river.JobCancel(err)
	}
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		for _, t := range page.Items {
			id := s.d.NewID()
			_, err := q.InsertIntent(ctx, store.InsertIntentParams{
				ID: id, Type: child, IdempotencyKey: registry.SpawnedKey(child, parent.IdempotencyKey, t.ID),
				Params: t.Params, OccurredAt: parent.OccurredAt, CreatedAt: s.d.Now(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return fmt.Errorf("寫入子意圖：%w", err)
			}
			if err := s.d.Jobs.Enqueue(ctx, ProcessArgs{IntentID: id}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}); err != nil {
				return fmt.Errorf("排入子意圖 job：%w", err)
			}
		}
		if page.Done {
			return nil
		}
		return s.d.Jobs.Enqueue(ctx, FanoutArgs{IntentID: parent.ID, AfterID: page.Next}, fanoutOpts())
	})
}

type fanoutWorker struct {
	river.WorkerDefaults[FanoutArgs]
	service *Service
}

func (w *fanoutWorker) Work(ctx context.Context, job *river.Job[FanoutArgs]) error {
	return w.service.Fanout(ctx, job.Args)
}
