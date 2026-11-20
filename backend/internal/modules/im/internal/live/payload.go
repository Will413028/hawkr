// Package live 是 im 的即時通道：以 Postgres LISTEN/NOTIFY 通知、進程內扇出，訂閱者的內容一律經資料庫讀取。
//
// 通知只是「對話 C 在 seq q 有東西」的提示；訊息內容不經 NOTIFY，也不在進程間以別的形式傳遞。
package live

import (
	"encoding/json"
	"errors"
	"fmt"
	"uuid"
)

// Channel 是 NOTIFY 的頻道名稱。
const Channel = "im_events"

// payloadVersion 是目前的 payload 版本；監聽端遇到不認得的版本丟棄並計數。
const payloadVersion = 1

// MaxPayloadBytes 是 PostgreSQL NOTIFY payload 的長度上限。
const MaxPayloadBytes = 8000

// 通知的種類。
const (
	KindMessage   = "m" // 新訊息
	KindVisible   = "v" // 訊息可見度改變（隱藏、還原）
	KindRead      = "r" // 某一方的已讀游標前進
	KindReadBatch = "u" // 某一方的批次已讀
)

// 已讀游標前進的一方（Payload.Side）。
const (
	SideBuyer = "B"
	SideStore = "S"
)

// Payload 是 NOTIFY 的內容：只有業務鍵與 seq，沒有訊息文字、名稱或個資。
type Payload struct {
	V    int    `json:"v"`
	Kind string `json:"k"`
	Conv string `json:"c,omitempty"`
	// Buyer、Store 是對話的 buyer_id、store_id；批次已讀只有被標已讀的那一方。
	Buyer int64 `json:"b,omitempty"`
	Store int64 `json:"s,omitempty"`
	// Seq 是相關的 seq：新訊息的 seq、被改可見度那則的 seq、新的游標值；批次已讀沒有。
	Seq int64 `json:"q,omitempty"`
	// Side 只有 KindRead：哪一方的游標前進。
	Side string `json:"d,omitempty"`
}

// ErrPayload 表示 payload 不是合法的通知（非 JSON、版本不認得、缺欄位）。
var ErrPayload = errors.New("im live：不合法的通知")

// Message 是新訊息的通知。
func Message(conv uuid.UUID, buyer, store, seq int64) Payload {
	return Payload{V: payloadVersion, Kind: KindMessage, Conv: conv.String(), Buyer: buyer, Store: store, Seq: seq}
}

// Visible 是可見度改變的通知。
func Visible(conv uuid.UUID, buyer, store, seq int64) Payload {
	return Payload{V: payloadVersion, Kind: KindVisible, Conv: conv.String(), Buyer: buyer, Store: store, Seq: seq}
}

// Read 是一方的已讀游標前進到 seq 的通知。
func Read(conv uuid.UUID, buyer, store, seq int64, side string) Payload {
	return Payload{V: payloadVersion, Kind: KindRead, Conv: conv.String(), Buyer: buyer, Store: store, Seq: seq, Side: side}
}

// ReadBatch 是一方批次已讀的通知；buyer 與 store 只有一個大於零（被標已讀的那一方）。
func ReadBatch(buyer, store int64) Payload {
	return Payload{V: payloadVersion, Kind: KindReadBatch, Buyer: buyer, Store: store}
}

// Encode 回傳要交給 pg_notify 的字串。
func (p Payload) Encode() string {
	b, err := json.Marshal(p)
	if err != nil {
		panic(fmt.Sprintf("im live：編碼通知：%v", err)) // 欄位全是字串與整數，不會失敗
	}
	return string(b)
}

// Decode 解析並驗證一則通知；任何不合法都回 ErrPayload。
func Decode(s string) (Payload, uuid.UUID, error) {
	var p Payload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return Payload{}, uuid.UUID{}, fmt.Errorf("%w：%w", ErrPayload, err)
	}
	if p.V != payloadVersion {
		return Payload{}, uuid.UUID{}, fmt.Errorf("%w：版本 %d", ErrPayload, p.V)
	}
	switch p.Kind {
	case KindMessage, KindVisible:
		if p.Buyer <= 0 || p.Store <= 0 || p.Seq <= 0 {
			return Payload{}, uuid.UUID{}, fmt.Errorf("%w：缺欄位", ErrPayload)
		}
	case KindRead:
		if p.Buyer <= 0 || p.Store <= 0 || p.Seq <= 0 || (p.Side != SideBuyer && p.Side != SideStore) {
			return Payload{}, uuid.UUID{}, fmt.Errorf("%w：缺欄位", ErrPayload)
		}
	case KindReadBatch:
		if (p.Buyer > 0) == (p.Store > 0) || p.Buyer < 0 || p.Store < 0 {
			return Payload{}, uuid.UUID{}, fmt.Errorf("%w：批次已讀要剛好一方", ErrPayload)
		}
		return p, uuid.UUID{}, nil
	default:
		return Payload{}, uuid.UUID{}, fmt.Errorf("%w：種類 %q", ErrPayload, p.Kind)
	}
	conv, err := uuid.Parse(p.Conv)
	if err != nil {
		return Payload{}, uuid.UUID{}, fmt.Errorf("%w：對話 ID", ErrPayload)
	}
	return p, conv, nil
}
