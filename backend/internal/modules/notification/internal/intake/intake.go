// Package intake 是通知意圖管線：來源 transaction 內的同步訂閱者只排意圖（Accept），worker（Process）在 transaction 外解析收件人、
// 套偏好、略過自己，再於一個 transaction 內寫收件匣與投遞紀錄（D10）。
package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Accounts 是 worker 對 identity 的需求：以共享鎖鎖住存活的帳號列（commit 前不會被刪），並回報帳號是否已刪除。identity.Module 實作它。
type Accounts interface {
	ShareLockAccount(ctx context.Context, id int64) (deleted bool, err error)
}

// Registry 是型別註冊表；正式組裝用 registry 套件的 Lookup，測試可以換成帶測試型別的版本。
type Registry interface {
	Lookup(name string) (registry.Entry, bool)
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB       *db.DB
	Accounts Accounts
	Registry Registry
	Jobs     jobs.Enqueuer
	Now      func() time.Time
	NewID    func() uuid.UUID
	Logger   *slog.Logger
}

// Service 是意圖管線。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

// ProcessArgs 是展開一個意圖的 job。
type ProcessArgs struct {
	IntentID uuid.UUID `json:"intent_id"`
}

// Kind 是 River 的 job 種類。
func (ProcessArgs) Kind() string { return "notification.process_intent" }

// DeliverArgs 是一筆投遞紀錄（推播或 email）的渠道 job；以投遞紀錄 ID 唯一。A8-3 把渠道 adapter 接進這個 job，這一步它不送出、
// 不改紀錄（紀錄留在 PENDING）。
type DeliverArgs struct {
	DeliveryID uuid.UUID `json:"delivery_id"`
}

// Kind 是 River 的 job 種類。
func (DeliverArgs) Kind() string { return "notification.deliver" }

// Accept 在 ctx 的 transaction 內登記一個意圖並排入 job：意圖冪等鍵是事件來源身分（型別名稱加來源身分），已存在時什麼都不排。
// 這是同步訂閱者唯一做的事。
func (s *Service) Accept(ctx context.Context, typeName string, params any) error {
	e, ok := s.d.Registry.Lookup(typeName)
	if !ok {
		return fmt.Errorf("通知型別 %q 沒有註冊", typeName)
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("通知型別 %s 的參數無法序列化：%w", typeName, err)
	}
	key, err := e.IdempotencyKey(raw)
	if err != nil {
		return err
	}
	occurredAt, err := e.OccurredAt(raw)
	if err != nil {
		return err
	}
	id := s.d.NewID()
	_, err = store.New(s.d.DB.Conn(ctx)).InsertIntent(ctx, store.InsertIntentParams{
		ID: id, Type: typeName, IdempotencyKey: key, Params: raw, OccurredAt: occurredAt, CreatedAt: s.d.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("寫入通知意圖：%w", err)
	}
	return s.d.Jobs.Enqueue(ctx, ProcessArgs{IntentID: id}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
}

// Process 展開一個意圖。意圖已被清除、收件帳號不存在或已刪除、收件人就是 actor 時什麼都不寫（回 nil）；意圖型別或快照壞掉是永久錯誤（取消，不重試）。
// 重試安全：意圖與收件匣列的連結（inbox_item_intents）判斷已寫過，聚合合併取 actor 聯集且只讓較新的事件換參數，
// 投遞紀錄有 (意圖, 收件人, 渠道) 唯一鍵、已存在時不重複排渠道 job。
func (s *Service) Process(ctx context.Context, intentID uuid.UUID) error {
	intent, err := store.New(s.d.DB.Conn(ctx)).IntentByID(ctx, intentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("讀取通知意圖：%w", err)
	}
	e, ok := s.d.Registry.Lookup(intent.Type)
	if !ok {
		err := fmt.Errorf("通知型別 %q 沒有註冊", intent.Type)
		s.log().WarnContext(ctx, "通知意圖的型別沒有註冊，取消 job", "intent_id", intent.ID, "type", intent.Type)
		return river.JobCancel(err)
	}
	resolved, err := e.Resolve(intent.Params)
	if err != nil {
		s.log().WarnContext(ctx, "通知意圖的參數快照無法解析，取消 job", "intent_id", intent.ID, "type", intent.Type, "error", err)
		return river.JobCancel(err)
	}
	if slices.Contains(resolved.ActorAccounts, resolved.Recipient.AccountID) {
		// 自己觸發的事不通知自己；比的是帳號，不是店或買家 ID。
		s.log().InfoContext(ctx, "收件人就是 actor，略過通知", "intent_id", intent.ID, "type", intent.Type)
		return nil
	}
	spec := e.Spec()
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		// 鎖順序：identity 帳號列最先，notification 的列最後（platform/lockorder）。以共享鎖鎖住收件帳號直到 commit：
		// 只需要帳號不在寫入途中被刪、不改帳號，所以用共享鎖：多個 worker 不互相等待；帳號已刪除時不寫。
		deleted, err := s.d.Accounts.ShareLockAccount(ctx, resolved.Recipient.AccountID)
		if err != nil {
			if apperr.KindOf(err) == apperr.KindNotFound {
				s.log().WarnContext(ctx, "收件帳號不存在，略過通知", "intent_id", intent.ID, "type", intent.Type)
				return nil
			}
			return fmt.Errorf("鎖定收件帳號：%w", err)
		}
		if deleted {
			s.log().InfoContext(ctx, "收件帳號已刪除，略過通知", "intent_id", intent.ID, "type", intent.Type)
			return nil
		}
		return s.deliver(ctx, intent, spec, resolved)
	})
}

func (s *Service) log() *slog.Logger {
	if s.d.Logger != nil {
		return s.d.Logger
	}
	return slog.Default()
}

func (s *Service) deliver(ctx context.Context, intent store.NotificationIntent, spec registry.Spec, r registry.Resolved) error {
	q := store.New(s.d.DB.Conn(ctx))
	var storeID *int64
	if r.Recipient.StoreID > 0 {
		storeID = &r.Recipient.StoreID
	}
	overrides, err := q.PreferenceOverrides(ctx, store.PreferenceOverridesParams{
		AccountID: r.Recipient.AccountID, Audience: string(spec.Audience), Category: string(spec.Category),
	})
	if err != nil {
		return fmt.Errorf("讀取偏好：%w", err)
	}
	override := make(map[model.Channel]bool, len(overrides))
	for _, o := range overrides {
		override[model.Channel(o.Channel)] = o.Enabled
	}
	enabled := func(ch model.Channel) bool {
		if !model.Disableable(spec.Audience, spec.Category, ch) {
			return true
		}
		if v, ok := override[ch]; ok {
			return v
		}
		return model.DefaultEnabled(spec.Category, ch)
	}
	now := s.d.Now()
	for _, ch := range spec.Channels {
		if !enabled(ch) {
			continue
		}
		if model.DeliveryChannel(ch) {
			// 投遞紀錄是推播與 email 的唯一帳本；每筆新紀錄在同一個 transaction 排一個渠道 job（A8-3 把 adapter 接進它，這一步 job 不送出）。
			// 紀錄已存在（重試）時 job 已隨第一次的 transaction 排過，不再排。
			id, err := q.InsertDelivery(ctx, store.InsertDeliveryParams{
				ID: s.d.NewID(), IntentID: intent.ID, AccountID: r.Recipient.AccountID, Audience: string(spec.Audience),
				StoreID: storeID, Channel: string(ch), Now: now,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return fmt.Errorf("寫入投遞紀錄：%w", err)
			}
			if err := s.d.Jobs.Enqueue(ctx, DeliverArgs{DeliveryID: id}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}); err != nil {
				return fmt.Errorf("排入渠道 job：%w", err)
			}
			continue
		}
		if err := s.writeInbox(ctx, q, intent, spec, r, storeID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) writeInbox(ctx context.Context, q *store.Queries, intent store.NotificationIntent, spec registry.Spec, r registry.Resolved, storeID *int64, now time.Time) error {
	linked, err := q.InboxItemIntentLinked(ctx, intent.ID)
	if err != nil {
		return fmt.Errorf("檢查收件匣連結：%w", err)
	}
	if linked {
		return nil
	}
	var aggKey *string
	if r.AggregationKey != "" {
		aggKey = &r.AggregationKey
	}
	actors := slices.Clone(r.ActorAccounts)
	if actors == nil {
		actors = []int64{}
	}
	itemID, err := q.UpsertInboxItem(ctx, store.UpsertInboxItemParams{
		ID: s.d.NewID(), AccountID: r.Recipient.AccountID, Audience: string(spec.Audience), StoreID: storeID, Type: intent.Type,
		Params: intent.Params, AggregationKey: aggKey, ActorAccountIds: actors, OccurredAt: intent.OccurredAt, Now: now,
	})
	if err != nil {
		return fmt.Errorf("寫入收件匣：%w", err)
	}
	if err := q.LinkInboxItemIntent(ctx, store.LinkInboxItemIntentParams{IntentID: intent.ID, InboxItemID: itemID}); err != nil {
		return fmt.Errorf("寫入收件匣連結：%w", err)
	}
	return nil
}
