// Package payment 是賣家收款開通：provider 中立的 port（建立外部帳戶、開通 session、讀帳戶快照、驗事件）、帳戶的本地快取、
// 同步與建立的 River job，以及 webhook inbox。
//
// 結構（backend AGENTS.md 規則 4、5）：
//   - 平台審核核准的 transaction 內先寫「建立中」的列（沒有外部 ID）並排入以 store_id 為唯一鍵的建立 job；job 在 transaction 外呼叫
//     provider，再於自己的 transaction 填上外部 ID 並排入同步 job。
//   - 同步 job 在 transaction 外重讀外部帳戶，在 transaction 內整列覆寫快取；webhook、輪詢與管理員的重新整理都只排這個 job。
//   - 開通閘門只看快取的 transfers_active（db/objects.sql 的 store_requirement_flags）。
package payment

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// Capabilities 是 provider 的能力描述：誰收身分與銀行資料、用什麼介面開通。兩軸互相獨立。
type Capabilities struct {
	RequirementCollection model.RequirementCollection
	OnboardingSurface     model.OnboardingSurface
}

// CapabilityState 是轉帳能力的中立狀態，由 adapter 從 provider 的回應對應而來；不認得的值對應成 CapabilityUnknown（空值同）。
type CapabilityState string

// 轉帳能力的狀態。
const (
	// CapabilityActive 能力已啟用。
	CapabilityActive CapabilityState = "active"
	// CapabilityPending 資料已收齊或在處理，能力還沒啟用。
	CapabilityPending CapabilityState = "pending"
	// CapabilityRestricted 能力被限制，原因與誰能解除在 Details。
	CapabilityRestricted CapabilityState = "restricted"
	// CapabilityRejected 被 provider 拒絕，不會再啟用。
	CapabilityRejected CapabilityState = "rejected"
	// CapabilityUnsupported 這個帳戶不能使用此能力。
	CapabilityUnsupported CapabilityState = "unsupported"
	// CapabilityUnknown provider 回了不認得的狀態。
	CapabilityUnknown CapabilityState = "unknown"
)

// Valid 回傳 s 是否是 provider 已知的五個狀態之一（CapabilityUnknown 與空值都不是）。adapter 與 StatusOf 共用這一份清單。
func (s CapabilityState) Valid() bool {
	switch s {
	case CapabilityActive, CapabilityPending, CapabilityRestricted, CapabilityRejected, CapabilityUnsupported:
		return true
	case CapabilityUnknown:
		return false
	}
	return false
}

// Resolution 是狀態細節要由誰行動才能解開。
type Resolution string

// 狀態細節的行動方。
const (
	// ResolutionSellerAction 賣家補資料或完成開通。
	ResolutionSellerAction Resolution = "seller_action"
	// ResolutionPlatformAction 平台或 provider 處理，賣家補件解不開。
	ResolutionPlatformAction Resolution = "platform_action"
	// ResolutionNone 沒有可行的解法。
	ResolutionNone Resolution = "none"
	// ResolutionUnknown provider 回了不認得的行動方。
	ResolutionUnknown Resolution = "unknown"
)

// StatusDetail 是轉帳能力某個限制的行動方與 provider 的原因碼。
type StatusDetail struct {
	Resolution Resolution
	// Reason 是 provider 的原因碼，原樣保存。
	Reason string
}

// Snapshot 是外部帳戶某一刻的狀態，由 adapter 從 provider 的回應整理成中立的欄位。Transfers 與 Details 是轉帳能力（開通閘門）的狀態與
// 它的限制細節；PayoutsActive 是撥款能力已啟用；Requirements 是 provider 回報的待補條件原始 JSON（物件），原樣
// 保存給後台看。BankFingerprints 是目前掛在外部帳戶上的外部銀行帳號的 provider 識別碼（同一組銀行帳號在同一個 provider 內一致；沒有掛銀行帳號時
// 為空，不含空字串）：只供防詐比對，不是機密但不存明文。
type Snapshot struct {
	Transfers        CapabilityState
	Details          []StatusDetail
	PayoutsActive    bool
	Requirements     json.RawMessage
	BankFingerprints []string
}

// EnsureInput 是建立外部帳戶的輸入。IdempotencyKey 是「建立中」的列寫入時產生並保存的隨機 UUID：同一家店的每次重試都用它，得到同一個帳戶；
// 兩個資料庫共用同一個 provider 帳號時不會撞鍵。Collection 是帳戶列保存的收件方式（D2），不是行程設定。ContactEmail 是帳戶列保存的聯絡信箱（D23），金流商要求有收款設定的帳戶必須有。
type EnsureInput struct {
	StoreID        int64
	IdempotencyKey string
	ContactEmail   string
	Collection     model.RequirementCollection
}

// SessionInput 是開通 session 的輸入；Collection 是帳戶列保存的收件方式（D2），介面由它決定；ReturnURL 是賣家完成或離開後回到的頁面，RefreshURL 是連結過期時重新取得的頁面。
type SessionInput struct {
	ExternalAccountID string
	Collection        model.RequirementCollection
	ReturnURL         string
	RefreshURL        string
}

// Event 是驗過簽章的 provider 事件：事件 ID（inbox 的唯一鍵）與它指的外部帳戶。
type Event struct {
	ID                string
	ExternalAccountID string
}

// Provider 是收款開通的 port。真正的實作是 Stripe（stripe.go），測試與本機用 Fake。傳輸錯誤（連線、逾時、5xx、429）回一般錯誤，
// job 重試；provider 以業務理由拒絕、重試也不會成功時包 ErrPermanent。實作不得把傳輸錯誤轉成業務結果。每個呼叫都有 deadline。
type Provider interface {
	// Name 是 provider 的代碼（小寫英數與底線），存在帳戶列的 provider 欄。
	Name() string
	Capabilities() Capabilities
	// EnsureAccount 建立外部帳戶，以 IdempotencyKey 冪等，收件方式取 in.Collection。回傳外部帳戶 ID。
	EnsureAccount(ctx context.Context, in EnsureInput) (externalID string, err error)
	// CreateOnboardingSession 產生開通的連結或內嵌元件的 session。
	CreateOnboardingSession(ctx context.Context, in SessionInput) (model.OnboardingSession, error)
	// FetchAccount 讀外部帳戶目前的狀態與掛著的外部銀行帳號識別碼；任何一項讀取失敗整個呼叫失敗。
	FetchAccount(ctx context.Context, externalID string) (Snapshot, error)
	// ParseEvent 驗簽並取出事件 ID 與外部帳戶 ID；簽章不符或格式不合回 ErrInvalidEvent，驗過簽章但不是本平台要處理的事件回 ErrIgnoredEvent。
	ParseEvent(header http.Header, body []byte) (Event, error)
}

var (
	// ErrPermanent 標示重試也不會成功的失敗：建立 job 記錄後不再重試。
	ErrPermanent = errors.New("payment: permanent failure")
	// ErrInvalidEvent 標示事件的簽章或格式不合，不處理。
	ErrInvalidEvent = errors.New("payment: invalid event")
	// ErrIgnoredEvent 標示驗過簽章、但不是要處理的事件：回應 2xx，不寫 inbox、不排 job。
	ErrIgnoredEvent = errors.New("payment: ignored event")
)

// maxReasonLen 是 status_reason 的字數上限（payment_accounts_status_reason_check）。
const maxReasonLen = 200

// StatusNote 標示 StatusOf 遇到不認得或不完整的快照：狀態仍會推導（視為審核中），呼叫方要記警示（帶店家與外部帳戶 ID）。
type StatusNote string

// StatusOf 的註記。
const (
	// NoteNone 快照完整，沒有要記的事。
	NoteNone StatusNote = ""
	// NoteUnknownResolution 細節有不認得的行動方。
	NoteUnknownResolution StatusNote = "unknown_resolution"
	// NoteRestrictedWithoutDetail 轉帳能力被限制但沒有可判讀的細節。
	NoteRestrictedWithoutDetail StatusNote = "restricted_without_detail"
	// NoteUnknownState 轉帳能力的狀態不認得。
	NoteUnknownState StatusNote = "unknown_state"
)

// StatusOf 由快照推導中立的狀態、要存的原因碼與註記（Stripe 計畫 D13）。純函式，不記 log。順序：轉帳能力已啟用 → ENABLED；被拒絕或不支援 → DISABLED；
// 任何一筆細節要賣家行動（含 pending）→ ACTION_REQUIRED；pending → UNDER_REVIEW；restricted 而且有細節要平台處理或沒有解法 → DISABLED；
// 其餘（不認得的狀態、restricted 但沒有可判讀的細節）→ UNDER_REVIEW 並帶註記，由呼叫方記警示。細節有多筆時取最嚴重的：賣家能行動的優先，其次是 DISABLED。
// 原因碼是導出狀態的那一筆細節的 Reason（rejected、unsupported 沒有細節時用狀態名）；ENABLED 與沒有原因碼時是空字串。
func StatusOf(s Snapshot) (model.PaymentAccountStatus, string, StatusNote) {
	state := s.Transfers
	if !state.Valid() {
		state = CapabilityUnknown
	}
	sellerDetail, sellerCanAct := firstDetail(s.Details, ResolutionSellerAction)
	note := NoteNone
	switch state {
	case CapabilityActive:
		return model.PaymentEnabled, "", NoteNone
	case CapabilityRejected, CapabilityUnsupported:
		return model.PaymentDisabled, clipReason(cmp.Or(firstReason(s.Details), string(state))), NoteNone
	case CapabilityPending:
		if sellerCanAct {
			return model.PaymentActionRequired, clipReason(sellerDetail.Reason), NoteNone
		}
		if _, ok := firstDetail(s.Details, ResolutionUnknown); ok {
			note = NoteUnknownResolution
		}
	case CapabilityRestricted:
		if sellerCanAct {
			return model.PaymentActionRequired, clipReason(sellerDetail.Reason), NoteNone
		}
		if d, ok := firstDetail(s.Details, ResolutionPlatformAction, ResolutionNone); ok {
			return model.PaymentDisabled, clipReason(d.Reason), NoteNone
		}
		note = NoteRestrictedWithoutDetail
	case CapabilityUnknown:
		if sellerCanAct {
			return model.PaymentActionRequired, clipReason(sellerDetail.Reason), NoteNone
		}
		note = NoteUnknownState
	}
	return model.PaymentUnderReview, clipReason(firstReason(s.Details)), note
}

// firstDetail 回傳第一筆行動方在 want 之內的細節。
func firstDetail(details []StatusDetail, want ...Resolution) (StatusDetail, bool) {
	for _, d := range details {
		if slices.Contains(want, d.Resolution) {
			return d, true
		}
	}
	return StatusDetail{}, false
}

// firstReason 回傳第一筆有原因碼的細節的原因碼，沒有時是空字串。
func firstReason(details []StatusDetail) string {
	for _, d := range details {
		if d.Reason != "" {
			return d.Reason
		}
	}
	return ""
}

func clipReason(r string) string {
	if utf8.RuneCountInString(r) <= maxReasonLen {
		return r
	}
	return string([]rune(r)[:maxReasonLen])
}
