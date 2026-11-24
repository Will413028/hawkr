// Package suppress 是 email suppression 名單的寫入端：Resend 的退信與投訴 webhook 驗簽後，把事件縮成「類型加位址雜湊」（不含位址、主旨或其他欄位）寫進 platform/inbox
// （commit 後才回 2xx），派送 job 再呼叫 Handle 把雜湊寫進 notification.email_suppressions。名單只存位址的 HMAC-SHA256（D22），email 渠道 job 送出前以同一把密鑰雜湊後查。
package suppress

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/emailaddr"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

// 名單的原因（email_suppressions.reason）。
const (
	ReasonHardBounce = "HARD_BOUNCE"
	ReasonComplaint  = "COMPLAINT"
)

// MinKeyBytes 是 HMAC 密鑰的最短長度（SHA-256 的區塊安全下限）。
const MinKeyBytes = 32

// Keys 是位址雜湊的密鑰：Current 寫入與查詢都用；Previous 只在輪替期間查詢用（兩把一起查，新的列都以 Current 寫）。
type Keys struct {
	Current  []byte
	Previous []byte
}

// Hasher 把正規化後的位址轉成名單裡的雜湊。零值（沒有密鑰）的 Hash 與 Lookup 回錯誤：沒有密鑰時不能判斷位址在不在名單，寧可不寄。
type Hasher struct{ keys Keys }

// NewHasher 建立 Hasher。
func NewHasher(k Keys) Hasher { return Hasher{keys: k} }

var errNoKey = errors.New("email suppression 的 HMAC 密鑰沒有設定")

func mac(key []byte, normalized string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(normalized))
	return m.Sum(nil)
}

// Hash 回傳位址（先經 emailaddr.Normalize，不合格式回 false）在目前密鑰下的雜湊。
func (h Hasher) Hash(address string) ([]byte, bool, error) {
	norm, ok := emailaddr.Normalize(address)
	if !ok {
		return nil, false, nil
	}
	if len(h.keys.Current) == 0 {
		return nil, false, errNoKey
	}
	return mac(h.keys.Current, norm), true, nil
}

// Lookup 回傳位址在目前與舊密鑰下的雜湊（查名單用）；傳入的位址須已正規化。
func (h Hasher) Lookup(normalized string) ([][]byte, error) {
	if len(h.keys.Current) == 0 {
		return nil, errNoKey
	}
	out := [][]byte{mac(h.keys.Current, normalized)}
	if len(h.keys.Previous) > 0 {
		out = append(out, mac(h.keys.Previous, normalized))
	}
	return out, nil
}

// Provider 是 inbox 裡 Resend 事件的 provider 名稱。
const Provider = "resend"

// EventRetention 是已處理事件在 inbox 的保存期（由 inbox 的每日清除依 purge_after 刪除）。事件 payload 本來就只有類型與位址雜湊，沒有位址。
const EventRetention = 14 * 24 * time.Hour

const routeKey = "email"

// Deps 是 Service 的依賴。WebhookSecret 為空表示沒有設定 webhook：Receive 回 FailedPrecondition。
type Deps struct {
	DB            *db.DB
	Inbox         *inbox.Inbox
	WebhookSecret string
	Hasher        Hasher
	Now           func() time.Time
}

// Service 接收退信與投訴事件。
type Service struct{ d Deps }

// New 建立 Service；有 Inbox 時登記 Resend 的 provider（同一個 inbox 登記兩次會 panic，每個行程只建一個模組）。
func New(d Deps) *Service {
	s := &Service{d: d}
	if d.Inbox != nil {
		d.Inbox.Register(inbox.Source{Provider: Provider, Retention: EventRetention, Handle: s.Handle})
	}
	return s
}

// Receive 是 webhook 入口：驗簽、只收會讓位址進名單的事件（永久退信、投訴），在一個 transaction 內寫 inbox 並排入派送 job。
// 沒有設定密鑰或 inbox 回 FailedPrecondition；簽章或格式不合回 InvalidInput。其他事件型別（送達、延遲、暫時退信）直接接受、不寫入。
func (s *Service) Receive(ctx context.Context, header http.Header, body []byte) error {
	if s.d.WebhookSecret == "" || s.d.Inbox == nil {
		return apperr.FailedPrecondition("Resend webhook 沒有設定")
	}
	ev, err := resend.Verify(s.d.WebhookSecret, header, body, s.d.Now())
	if errors.Is(err, resend.ErrSignature) || errors.Is(err, resend.ErrMalformed) {
		return apperr.InvalidInput("Resend webhook 簽章或格式不合")
	}
	if err != nil {
		return err
	}
	if !ev.Suppresses() {
		return nil
	}
	payload, err := s.redact(ev)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := s.d.Inbox.Record(ctx, inbox.Event{Provider: Provider, EventID: ev.ID, RouteKey: routeKey, Payload: payload})
		return err
	})
}

// event 是寫進 inbox 的縮減 payload：沒有位址、郵件 ID、主旨與寄件人。
type event struct {
	Reason        string   `json:"reason"`
	AddressHashes []string `json:"address_hashes"`
}

// redact 把驗過簽章的事件縮成 event 的 JSON：位址在這裡雜湊，原始 payload 不進 inbox。沒有任何合格位址時回 nil（沒有東西可進名單）。
func (s *Service) redact(ev resend.Event) ([]byte, error) {
	reason := ReasonHardBounce
	if ev.Type == resend.EventComplained {
		reason = ReasonComplaint
	}
	out := event{Reason: reason}
	for _, to := range ev.To {
		h, ok, err := s.d.Hasher.Hash(to)
		if err != nil {
			return nil, err
		}
		if ok {
			out.AddressHashes = append(out.AddressHashes, hex.EncodeToString(h))
		}
	}
	if len(out.AddressHashes) == 0 {
		return nil, nil
	}
	return json.Marshal(out)
}

// Handle 是 inbox 的 handler（派送 job 的 transaction 內）：把事件的位址雜湊寫進 suppression 名單並標記已處理。可重複執行。
func (s *Service) Handle(ctx context.Context, e inbox.Event) error {
	var ev event
	if err := json.Unmarshal(e.Payload, &ev); err != nil {
		return fmt.Errorf("解析 suppression 事件：%w", err)
	}
	if ev.Reason != ReasonHardBounce && ev.Reason != ReasonComplaint {
		return fmt.Errorf("suppression 事件的原因 %q 不認得", ev.Reason)
	}
	q := store.New(s.d.DB.Conn(ctx))
	for _, hx := range ev.AddressHashes {
		h, err := hex.DecodeString(hx)
		if err != nil || len(h) != sha256.Size {
			return fmt.Errorf("suppression 事件的位址雜湊格式不合")
		}
		if err := q.InsertEmailSuppression(ctx, store.InsertEmailSuppressionParams{AddressHash: h, Reason: ev.Reason, Now: s.d.Now()}); err != nil {
			return fmt.Errorf("寫入 suppression 名單：%w", err)
		}
	}
	return s.d.Inbox.MarkProcessed(ctx, e.Provider, e.EventID, "notification.suppress")
}

// RemoveHardBounce 把位址從名單移除（管理員處理「退信原因已排除」；只有內部方法，沒有 RPC）：只移除永久退信；位址是投訴時回 FailedPrecondition（投訴永遠不可移除）；
// 位址不在名單、或不合格式回 false。回傳是否有移除。目前與舊密鑰下的列都處理。
func (s *Service) RemoveHardBounce(ctx context.Context, address string) (bool, error) {
	norm, ok := emailaddr.Normalize(address)
	if !ok {
		return false, nil
	}
	hashes, err := s.d.Hasher.Lookup(norm)
	if err != nil {
		return false, err
	}
	removed := false
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		n, err := q.DeleteHardBounceSuppressions(ctx, hashes)
		if err != nil {
			return fmt.Errorf("移除 suppression：%w", err)
		}
		removed = n > 0
		reasons, err := q.EmailSuppressionReasons(ctx, hashes)
		if err != nil {
			return fmt.Errorf("讀取 suppression 原因：%w", err)
		}
		for _, r := range reasons {
			if r == ReasonComplaint {
				return apperr.FailedPrecondition("這個位址是投訴，不可從名單移除")
			}
		}
		return nil
	})
	return removed && err == nil, err
}
