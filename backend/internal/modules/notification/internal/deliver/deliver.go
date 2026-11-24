// Package deliver 是推播與 email 的渠道 job：一個 job 處理一筆投遞紀錄（intake 寫入、狀態 PENDING）。
//
// 一次執行三個階段：先在 transaction 內鎖收件人並準備（確認收件人還在、依帳號 preferred_locale 組字、取 token 或信箱、查 suppression），
// 再在 transaction 外呼叫 provider（單體規則 5），最後在新的 transaction 內再鎖一次收件人並記下結果。兩次鎖用 intake.LockRecipient，與意圖 worker 同一組鎖與順序。
// 送達保證：推播是 at-least-once（FCM HTTP v1 沒有冪等鍵，provider 收下之後、帳本定案之前若程序死掉，重試會再推一次）；email 以投遞紀錄 ID 當 Resend 的 Idempotency-Key，重試不會重複寄。
// 送出之前收件人已刪除、已關閉或已退場，投遞紀錄標為永久失敗、不寫送出文字（D16、A8-1c 審查 R1-4）；送出之後才被刪除，紀錄不留文字。
package deliver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/emailaddr"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 投遞失敗的原因（deliveries.failure_reason）。
const (
	ReasonRecipientGone    = "recipient_gone"
	ReasonIntentMissing    = "intent_missing"
	ReasonTypeUnregistered = "type_unregistered"
	ReasonRenderFailed     = "render_failed"
	ReasonNoDeviceToken    = "no_device_token"
	ReasonTokenGone        = "token_unregistered"
	ReasonPushRejected     = "push_rejected"
	ReasonNoEmailAddress   = "no_email_address"
	ReasonSuppressed       = "email_suppressed"
	ReasonEmailRejected    = "email_rejected"
	ReasonRetriesExhausted = "retries_exhausted"
)

const (
	appBuyer  = "BUYER"
	appSeller = "SELLER"
)

// Contacts 是對 identity 的需求：帳號的聯絡信箱與來源（*identity.Module 滿足它）。
type Contacts interface {
	ContactEmail(ctx context.Context, accountID int64) (identity.ContactEmail, bool, error)
}

// Locales 回傳帳號的 preferred_locale（非同步訊息的語系，D1）。
type Locales interface {
	PreferredLocale(ctx context.Context, accountID int64) (uitext.Locale, error)
}

// Suppression 把正規化後的位址轉成 suppression 名單裡的雜湊（目前與輪替期間的舊密鑰各一個）；名單不存位址（D22）。沒有設定密鑰時回錯誤，email 不寄、job 重試。
type Suppression interface {
	Lookup(normalized string) ([][]byte, error)
}

// Registry 是型別註冊表。
type Registry interface {
	Lookup(name string) (registry.Entry, bool)
}

// Deps 是 Service 的依賴。
type Deps struct {
	DB       *db.DB
	Locks    intake.Locks
	Contacts Contacts
	Locales  Locales
	Registry Registry
	// Sources 解析讀取時才組字的型別（公告）的內容。
	Sources registry.Sources
	Push    fcm.Sender
	// ImageURL 把型別的圖片 key 轉成推播用的絕對網址；nil 表示沒有設定圖片網域，推播不帶圖。
	ImageURL func(key string) string
	Mail     resend.Sender
	// Suppression 是 email 查名單的雜湊；email 渠道必填。
	Suppression Suppression
	Now         func() time.Time
	Logger      *slog.Logger
}

// Service 執行渠道 job。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service { return &Service{d: d} }

// plan 是準備階段的結果：送出需要的一切，加上寫回結果時要重新比對的收件人。
type plan struct {
	delivery store.DeliveryForUpdateRow
	typeName string
	rendered registry.Rendered
	tokens   []string
	// inboxItemID 是這筆投遞對應的收件匣列（只推播的型別沒有）；imageURL 是推播圖片（型別沒有圖或沒有設定圖片網域時為空）。
	inboxItemID *uuid.UUID
	imageURL    string
	address     string
}

// Deliver 處理一筆投遞紀錄。暫時性失敗回錯誤（River 退避重試）；final 為 true（最後一次嘗試）時暫時性失敗改記永久失敗，不讓紀錄永遠停在 PENDING。
// 投遞紀錄不存在（已被保留期清除）或已是終態時什麼都不做，所以重複執行安全。
func (s *Service) Deliver(ctx context.Context, id uuid.UUID, final bool) error {
	head, err := store.New(s.d.DB.Conn(ctx)).DeliveryByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("讀取投遞紀錄：%w", err)
	}
	if head.Status != "PENDING" {
		return nil
	}
	var p plan
	var done bool
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		p, done, err = s.prepare(ctx, head)
		return err
	})
	if err != nil || done {
		return err
	}
	if model.Channel(head.Channel) == model.ChannelPush {
		return s.sendPush(ctx, p, final)
	}
	return s.sendEmail(ctx, p, final)
}

func recipientStoreID(d store.DeliveryByIDRow) int64 {
	if d.StoreID == nil {
		return 0
	}
	return *d.StoreID
}

// prepare 在 transaction 內鎖收件人、重讀並鎖投遞紀錄、組字、取目標。done 為 true 表示這筆已在這個 transaction 內定案（標成永久失敗或本來就不需要送）。
func (s *Service) prepare(ctx context.Context, head store.DeliveryByIDRow) (plan, bool, error) {
	q := store.New(s.d.DB.Conn(ctx))
	aud := model.Audience(head.Audience)
	gone, err := intake.LockRecipient(ctx, s.d.Locks, aud, head.AccountID, recipientStoreID(head))
	if err != nil {
		return plan{}, false, err
	}
	d, err := q.DeliveryForUpdate(ctx, head.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return plan{}, true, nil
	}
	if err != nil {
		return plan{}, false, fmt.Errorf("鎖定投遞紀錄：%w", err)
	}
	if d.Status != "PENDING" {
		return plan{}, true, nil
	}
	fail := func(reason string) (plan, bool, error) {
		return plan{}, true, s.markFailed(ctx, q, d.ID, reason)
	}
	if gone != intake.GoneNone {
		s.log().InfoContext(ctx, "收件人已不在，投遞標為永久失敗", "delivery_id", d.ID, "gone", string(gone))
		return fail(ReasonRecipientGone)
	}
	intent, err := q.IntentByID(ctx, d.IntentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ReasonIntentMissing)
	}
	if err != nil {
		return plan{}, false, fmt.Errorf("讀取通知意圖：%w", err)
	}
	entry, ok := s.d.Registry.Lookup(intent.Type)
	if !ok {
		return fail(ReasonTypeUnregistered)
	}
	locale, err := s.d.Locales.PreferredLocale(ctx, d.AccountID)
	if err != nil {
		return plan{}, false, fmt.Errorf("讀取帳號語系：%w", err)
	}
	loaded, rowErrs, err := entry.Load(ctx, s.d.Sources, []json.RawMessage{intent.Params})
	if err != nil {
		return plan{}, false, fmt.Errorf("解析通知內容：%w", err)
	}
	if rowErrs[0] != nil {
		s.log().WarnContext(ctx, "通知參數快照無法解析，投遞標為永久失敗", "delivery_id", d.ID, "type", intent.Type, "error", rowErrs[0])
		return fail(ReasonRenderFailed)
	}
	rendered, err := entry.RenderLoaded(locale, intent.Params, loaded[0])
	if err != nil {
		s.log().WarnContext(ctx, "通知組字失敗，投遞標為永久失敗", "delivery_id", d.ID, "type", intent.Type, "error", err)
		return fail(ReasonRenderFailed)
	}
	p := plan{delivery: d, typeName: intent.Type, rendered: rendered}
	switch model.Channel(d.Channel) {
	case model.ChannelPush:
		tokens, err := q.DeviceTokensForRecipient(ctx, store.DeviceTokensForRecipientParams{AccountID: d.AccountID, App: appFor(model.Audience(d.Audience))})
		if err != nil {
			return plan{}, false, fmt.Errorf("讀取裝置 token：%w", err)
		}
		if len(tokens) == 0 {
			return fail(ReasonNoDeviceToken)
		}
		p.tokens = tokens
		item, err := q.InboxItemForDelivery(ctx, store.InboxItemForDeliveryParams{AccountID: d.AccountID, Audience: d.Audience, StoreID: d.StoreID, IntentID: d.IntentID})
		switch {
		case err == nil:
			p.inboxItemID = &item
		case !errors.Is(err, pgx.ErrNoRows):
			return plan{}, false, fmt.Errorf("讀取收件匣列：%w", err)
		}
		if rendered.ImageKey != "" && s.d.ImageURL != nil {
			p.imageURL = s.d.ImageURL(rendered.ImageKey)
		}
	case model.ChannelEmail:
		address, reason, err := s.emailTarget(ctx, q, d.AccountID)
		if err != nil {
			return plan{}, false, err
		}
		if reason != "" {
			return fail(reason)
		}
		p.address = address
	case model.ChannelInbox:
		return fail("channel_unsupported")
	}
	return p, false, nil
}

// appFor 是受眾對應的 App：BUYER 受眾只推 BUYER App、STORE 受眾只推 SELLER App，不跨 App。
func appFor(a model.Audience) string {
	if a == model.AudienceStore {
		return appSeller
	}
	return appBuyer
}

// emailTarget 回傳寄件位址；沒有可寄的位址（沒有已驗證信箱、在 suppression 名單內）時回失敗原因。
func (s *Service) emailTarget(ctx context.Context, q *store.Queries, accountID int64) (address, reason string, err error) {
	c, ok, err := s.d.Contacts.ContactEmail(ctx, accountID)
	switch {
	case apperr.KindOf(err) == apperr.KindNotFound:
		return "", ReasonRecipientGone, nil
	case err != nil:
		return "", "", fmt.Errorf("讀取聯絡信箱：%w", err)
	case !ok || c.Source != identity.ContactEmailVerified:
		return "", ReasonNoEmailAddress, nil
	}
	norm, valid := emailaddr.Normalize(c.Address)
	if !valid {
		return "", ReasonNoEmailAddress, nil
	}
	if s.d.Suppression == nil {
		return "", "", errors.New("email suppression 的雜湊沒有設定")
	}
	hashes, err := s.d.Suppression.Lookup(norm)
	if err != nil {
		return "", "", err
	}
	suppressed, err := q.EmailSuppressed(ctx, hashes)
	if err != nil {
		return "", "", fmt.Errorf("查 suppression 名單：%w", err)
	}
	if suppressed {
		return "", ReasonSuppressed, nil
	}
	return norm, "", nil
}

func (s *Service) markFailed(ctx context.Context, q *store.Queries, id uuid.UUID, reason string) error {
	if _, err := q.MarkDeliveryFailed(ctx, store.MarkDeliveryFailedParams{ID: id, Reason: reason, Now: s.d.Now()}); err != nil {
		return fmt.Errorf("標記投遞永久失敗：%w", err)
	}
	return nil
}

func (s *Service) log() *slog.Logger {
	if s.d.Logger != nil {
		return s.d.Logger
	}
	return slog.Default()
}

// settle 在 transaction 內再鎖一次收件人並記下結果：sent 為 true 記已送出（收件人此時已不在則不留文字），否則標永久失敗（收件人已不在時原因是 recipient_gone）。
func (s *Service) settle(ctx context.Context, p plan, sent bool, failReason string) error {
	d := p.delivery
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		var storeID int64
		if d.StoreID != nil {
			storeID = *d.StoreID
		}
		gone, err := intake.LockRecipient(ctx, s.d.Locks, model.Audience(d.Audience), d.AccountID, storeID)
		if err != nil {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		cur, err := q.DeliveryForUpdate(ctx, d.ID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.Status != "PENDING") {
			return nil
		}
		if err != nil {
			return fmt.Errorf("鎖定投遞紀錄：%w", err)
		}
		switch {
		case sent && gone != intake.GoneNone:
			_, err = q.MarkDeliverySent(ctx, store.MarkDeliverySentParams{ID: d.ID, Now: s.d.Now()})
		case sent:
			locale, title, body := string(p.rendered.Locale), p.rendered.Title, p.rendered.Body
			_, err = q.MarkDeliverySent(ctx, store.MarkDeliverySentParams{ID: d.ID, Locale: &locale, Title: &title, Body: &body, Now: s.d.Now()})
		case gone != intake.GoneNone:
			return s.markFailed(ctx, q, d.ID, ReasonRecipientGone)
		default:
			return s.markFailed(ctx, q, d.ID, failReason)
		}
		if err != nil {
			return fmt.Errorf("標記投遞已送出：%w", err)
		}
		return nil
	})
}

// retryOrFail 處理暫時性失敗：還有嘗試次數就回錯誤讓 River 重試，最後一次嘗試改記永久失敗（紀錄不會永遠停在 PENDING）。
func (s *Service) retryOrFail(ctx context.Context, p plan, final bool, cause error) error {
	if final {
		s.log().ErrorContext(ctx, "渠道暫時性失敗用完重試，投遞標為永久失敗", "delivery_id", p.delivery.ID, "channel", p.delivery.Channel, "error", cause)
		return s.settle(ctx, p, false, ReasonRetriesExhausted)
	}
	return cause
}

// AddWorkers 登記渠道 job 的 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &worker{svc: s})
}

type worker struct {
	river.WorkerDefaults[intake.DeliverArgs]
	svc *Service
}

// Work 處理一筆投遞；最後一次嘗試時暫時性失敗改記永久失敗。
func (w *worker) Work(ctx context.Context, job *river.Job[intake.DeliverArgs]) error {
	return w.svc.Deliver(ctx, job.Args.DeliveryID, job.Attempt >= job.MaxAttempts)
}

// Timeout 涵蓋兩個 transaction 與一次 provider 呼叫。
func (w *worker) Timeout(*river.Job[intake.DeliverArgs]) time.Duration { return 2 * time.Minute }
