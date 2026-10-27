package grpc

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// procurementStatus 建立一個帶 BusinessError detail 的 gRPC error——所有
// AIProcurementService handler 共用同一個 helper，不各自 hand-roll
// status.WithDetails。bucket 是粗粒度 gRPC code（見下方 bucket 對照表），
// reason 是既有 HTTP handler 已經在用的機器碼（例如 "QUOTE_NOT_READY"），
// message 逐字沿用既有中文文案，不含代碼前綴（reason/message 分離設計見
// spec §6）。
func procurementStatus(bucket codes.Code, reason, message string) error {
	st := status.New(bucket, message)
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{Reason: reason, Message: message})
	if err != nil {
		// WithDetails 只在 detail message 本身編碼失敗時出錯（例如不是合法
		// proto.Message）——BusinessError 是我們自己定義的簡單型別，不會
		// 發生，這裡保底退回沒有 detail 的原始 status 而不是整個 panic。
		return st.Err()
	}
	return stWithDetails.Err()
}

// procurementQuotaExceededStatus 是「本月 Agent 額度已用完」（doc 59）專用的
// status——不共用 procurementStatus，因為這個 reason 是 BusinessError 上
// remaining／reset_at 兩個欄位唯一的使用者（見 aipb.BusinessError 兩個欄位的
// proto 註解："only set when reason == AGENT_MONTHLY_QUOTA_EXCEEDED"），而
// procurementStatus 的簽名刻意只有 reason/message，不該為了一個 reason 就長出
// 兩個對其餘 20 幾個呼叫端永遠是空值的參數。
//
// ResourceExhausted 對應 gateway 的 429（見 gateway/internal/procurement 的
// GRPCCodeToHTTPStatus 對照表），與 CodeMeta 這個 reason 的描述一致。
//
// Remaining 固定 "0"：走到這個分支就是 ReserveQuota 回 reserved=false，也就是
// 這個期間的已確認用量已經等於上限，餘量必然是 0。字串而不是數字是 repo 全域的
// 對外數值慣例（proto 欄位本身就是 string）。
//
// limit 目前不上 wire：BusinessError 沒有對應欄位，而 message（呼叫端傳進來的
// QuotaExceededError.Error()）本身就含「0/上限」。它仍留在簽名上，讓這個 helper
// 的輸入等於 QuotaExceededError 攜帶的全部資訊——契約日後補上 limit 欄位時只要
// 改這裡，不必回頭改呼叫端。
func procurementQuotaExceededStatus(_ int, resetAt time.Time, message string) error {
	st := status.New(codes.ResourceExhausted, message)
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{
		Reason:    codemeta.ProcurementErrorReasonAgentMonthlyQuotaExceeded.Value,
		Message:   message,
		Remaining: "0",
		ResetAt:   timeutil.FormatISO8601(resetAt),
	})
	if err != nil {
		// 同 procurementStatus：detail 編碼失敗時保底退回沒有 detail 的 status。
		return st.Err()
	}
	return stWithDetails.Err()
}
