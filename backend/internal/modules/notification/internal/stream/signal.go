package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Channel 是 NOTIFY 的頻道名稱。
const Channel = "notification_signals"

const payloadVersion = 1

// 通知的種類。
const (
	kindInbox     = "i" // 某 (帳號, 受眾) 的收件匣有變化
	kindWorkspace = "w" // 某家店的賣家工作區有變化
)

// payload 是 NOTIFY 的內容：只有業務鍵與提示，沒有未讀數、通知文字或個資；內容由 serve 端對每個訂閱者讀取（授權在讀取）。
type payload struct {
	V    int    `json:"v"`
	Kind string `json:"k"`
	// Account、Audience、Store、Hint 屬於 kindInbox（BUYER 沒有 Store）；kindWorkspace 只有 Store 與 Change。
	Account  int64  `json:"a,omitempty"`
	Audience string `json:"u,omitempty"`
	Store    int64  `json:"s,omitempty"`
	Hint     string `json:"h,omitempty"`
	// Item 是這則通知新建或聚合更新的收件匣列 ID（kindInbox 可選；一則 NOTIFY 至多一個）。
	Item   string `json:"i,omitempty"`
	Change int    `json:"c,omitempty"`
}

var errPayload = errors.New("notification/stream：不合法的通知")

func (p payload) encode() string {
	b, err := json.Marshal(p)
	if err != nil {
		panic(fmt.Sprintf("notification/stream：編碼通知：%v", err)) // 欄位全是字串與整數，不會失敗
	}
	return string(b)
}

func decode(s string) (payload, error) {
	var p payload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return payload{}, fmt.Errorf("%w：%w", errPayload, err)
	}
	if p.V != payloadVersion {
		return payload{}, fmt.Errorf("%w：版本 %d", errPayload, p.V)
	}
	switch p.Kind {
	case kindInbox:
		if p.Account <= 0 || SellerChange(p.Change) != 0 {
			return payload{}, fmt.Errorf("%w：收件匣通知缺欄位", errPayload)
		}
		switch model.Audience(p.Audience) {
		case model.AudienceBuyer:
			if p.Store != 0 {
				return payload{}, fmt.Errorf("%w：BUYER 不帶店", errPayload)
			}
		case model.AudienceStore:
			if p.Store <= 0 {
				return payload{}, fmt.Errorf("%w：STORE 缺店", errPayload)
			}
		default:
			return payload{}, fmt.Errorf("%w：受眾 %q", errPayload, p.Audience)
		}
		if h := model.StateHint(p.Hint); h != model.StateHintNone && !isHint(h) {
			return payload{}, fmt.Errorf("%w：提示 %q", errPayload, p.Hint)
		}
		if p.Item != "" {
			if _, err := uuid.Parse(p.Item); err != nil {
				return payload{}, fmt.Errorf("%w：收件匣列 ID %q", errPayload, p.Item)
			}
		}
	case kindWorkspace:
		if p.Store <= 0 || !SellerChange(p.Change).valid() {
			return payload{}, fmt.Errorf("%w：賣家工作區通知缺欄位", errPayload)
		}
		if p.Item != "" {
			return payload{}, fmt.Errorf("%w：賣家工作區通知不帶收件匣列 ID", errPayload)
		}
	default:
		return payload{}, fmt.Errorf("%w：種類 %q", errPayload, p.Kind)
	}
	return p, nil
}

func isHint(h model.StateHint) bool {
	for _, k := range model.StateHints() {
		if k == h {
			return true
		}
	}
	return false
}

// NotifyInbox 在 ctx 帶的 transaction 內（寫入收件匣的同一個）發出 (帳號, 受眾) 的收件匣通知；沒有 transaction 時立即送出。
// BUYER 受眾 storeID 為 0。訊號可合併，重試重複發出無害。
// itemID 是這次新建或聚合更新的收件匣列（一則 NOTIFY 一個，不批次）；已讀、刪除等不新增通知的變化傳 uuid.Nil()。
func NotifyInbox(ctx context.Context, d *db.DB, key Key, storeID int64, hint model.StateHint, itemID uuid.UUID) error {
	p := payload{V: payloadVersion, Kind: kindInbox, Account: key.AccountID, Audience: string(key.Audience), Store: storeID, Hint: string(hint)}
	if itemID != uuid.Nil() {
		p.Item = itemID.String()
	}
	if _, err := decode(p.encode()); err != nil {
		return fmt.Errorf("notification/stream：收件匣通知不合法（帳號 %d、受眾 %q、店 %d）", key.AccountID, key.Audience, storeID)
	}
	return notify(ctx, d, p)
}

// NotifyWorkspace 發出這家店賣家工作區有變化的通知（可在呼叫端的 transaction 內，commit 後才送達）。不認得的 change 回錯誤。
func NotifyWorkspace(ctx context.Context, d *db.DB, storeID int64, change SellerChange) error {
	if storeID <= 0 || !change.valid() {
		return fmt.Errorf("notification/stream：賣家工作區事件不合法（店 %d、變更 %d）", storeID, int(change))
	}
	return notify(ctx, d, payload{V: payloadVersion, Kind: kindWorkspace, Store: storeID, Change: int(change)})
}

func notify(ctx context.Context, d *db.DB, p payload) error {
	if err := store.New(d.Conn(ctx)).NotifySignal(ctx, store.NotifySignalParams{Channel: Channel, Payload: p.encode()}); err != nil {
		return fmt.Errorf("notification/stream：發出通知：%w", err)
	}
	return nil
}
