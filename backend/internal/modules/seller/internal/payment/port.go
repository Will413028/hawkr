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
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// Capabilities 是 provider 的能力描述：誰收身分與銀行資料、用什麼介面開通。兩軸互相獨立。
type Capabilities struct {
	RequirementCollection model.RequirementCollection
	OnboardingSurface     model.OnboardingSurface
}

// Snapshot 是外部帳戶某一刻的狀態，由 adapter 從 provider 的回應整理成中立的欄位。Requirements 是 provider 回報的待補條件原始 JSON
// （物件），原樣保存給後台看。
type Snapshot struct {
	// TransfersActive 是轉帳能力（開通閘門）已啟用；PayoutsActive 是撥款能力已啟用。
	TransfersActive bool
	PayoutsActive   bool
	// DetailsSubmitted 是賣家已送出開通資料。
	DetailsSubmitted bool
	// RequirementsDue 是有待補或逾期的條件（currently_due 或 past_due 非空）。
	RequirementsDue bool
	// DisabledReason 非空表示帳戶被 provider 拒絕或暫停（adapter 只在這兩類原因時設值）。
	DisabledReason string
	Requirements   json.RawMessage
}

// EnsureInput 是建立外部帳戶的輸入。IdempotencyKey 是 store_id 的十進位文字：同一家店重送建立得到同一個帳戶。
type EnsureInput struct {
	StoreID        int64
	IdempotencyKey string
}

// SessionInput 是開通 session 的輸入；ReturnURL 是賣家完成或離開後回到的頁面，RefreshURL 是連結過期時重新取得的頁面。
type SessionInput struct {
	ExternalAccountID string
	ReturnURL         string
	RefreshURL        string
}

// Event 是驗過簽章的 provider 事件：事件 ID（inbox 的唯一鍵）與它指的外部帳戶。
type Event struct {
	ID                string
	ExternalAccountID string
}

// Provider 是收款開通的 port。真正的實作（Stripe）在後續的 adapter，測試與本機用 Fake。傳輸錯誤（連線、逾時、5xx、429）回一般錯誤，
// job 重試；provider 以業務理由拒絕、重試也不會成功時包 ErrPermanent。實作不得把傳輸錯誤轉成業務結果。每個呼叫都有 deadline。
type Provider interface {
	// Name 是 provider 的代碼（小寫英數與底線），存在帳戶列的 provider 欄。
	Name() string
	Capabilities() Capabilities
	// EnsureAccount 建立外部帳戶，以 IdempotencyKey 冪等。回傳外部帳戶 ID。
	EnsureAccount(ctx context.Context, in EnsureInput) (externalID string, err error)
	// CreateOnboardingSession 產生開通的連結或內嵌元件的 session。
	CreateOnboardingSession(ctx context.Context, in SessionInput) (model.OnboardingSession, error)
	// FetchAccount 讀外部帳戶目前的狀態。
	FetchAccount(ctx context.Context, externalID string) (Snapshot, error)
	// ParseEvent 驗簽並取出事件 ID 與外部帳戶 ID；簽章不符回 ErrInvalidEvent。
	ParseEvent(header http.Header, body []byte) (Event, error)
}

var (
	// ErrPermanent 標示重試也不會成功的失敗：建立 job 記錄後不再重試。
	ErrPermanent = errors.New("payment: permanent failure")
	// ErrInvalidEvent 標示事件的簽章或格式不合，不處理。
	ErrInvalidEvent = errors.New("payment: invalid event")
)

// StatusOf 由快照推導中立的狀態（Stripe 計畫 D13）。順序：被拒絕或暫停 → 轉帳能力已啟用 → 還沒送出資料 → 有待補的條件 → 審核中。
// 只有已啟用的狀態的 transfers_active 為真（帳戶被暫停時，能力即使仍標示啟用也不算）。
func StatusOf(s Snapshot) model.PaymentAccountStatus {
	switch {
	case s.DisabledReason != "":
		return model.PaymentDisabled
	case s.TransfersActive:
		return model.PaymentEnabled
	case !s.DetailsSubmitted:
		return model.PaymentOnboarding
	case s.RequirementsDue:
		return model.PaymentActionRequired
	default:
		return model.PaymentUnderReview
	}
}
