package handler

import (
	procurementwire "github.com/Will413028/hawkr/gateway/internal/procurement"
	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
)

// procurementErrorBody 是 procurement 18 支端點**唯一**的錯誤 JSON 形狀
// （spec §6/§7）。兩條錯誤路徑（RPC 回來的錯誤、Gateway 自己的 pre-flight 驗證
// 拒絕）必須交出同一個形狀：先前 pre-flight 走 pkg/envelope 的
// response.BadRequest／response.Err，產出 {code,message,traceId} 沒有 reason，
// 而 RPC 路徑產出 {code,reason,message} 沒有 traceId——client 讀 .reason 在相當
// 比例的 400/422 上會拿到 undefined，這次遷移的錯誤契約重設計等於只做了一半。
//
// traceId 是刻意保留而不是順手拿掉的：它是本 repo 所有錯誤回應的既有保證
// （pkg/envelope.Envelope），把 18 支端點從中挖掉會少掉線上追查的唯一線索。
// 對既有 client 而言 reason 與 traceId 都只是新增的 key，不刪不改型別。
// omitempty 與 envelope.Envelope.TraceID 對齊（取不到 trace 時不寫出空字串鍵）。
//
// Remaining／ResetAt 是 doc 57 §4.7 對 AGENT_MONTHLY_QUOTA_EXCEEDED 指定的兩個
// 附加欄位（回 `remaining=0` ＋ `resetAt`），來源是 aipb.BusinessError 同名的兩
// 個欄位——proto 註解已寫明「only set when reason == AGENT_MONTHLY_QUOTA_EXCEEDED」。
// 兩者都 omitempty：其餘 20 幾個 reason 的 BusinessError 這兩格是空字串，
// 不能因此在所有 procurement 錯誤上多長兩個永遠是 "" 的鍵（
// ai_procurement_error_envelope_test.go 對 18 支端點釘的正是「精確鍵集合」）。
type procurementErrorBody struct {
	Code      int    `json:"code"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Remaining string `json:"remaining,omitempty"`
	ResetAt   string `json:"resetAt,omitempty"`
	TraceID   string `json:"traceId,omitempty"`
}

// writeProcurementError 把 AIProcurementService 回傳的 gRPC error 轉成
// procurementErrorBody——18 支 RPC 共用同一個 helper，不 per-RPC 客製
// （spec §6/§7）。Gateway 在這裡自己組錯誤 JSON（Phase C 前的 HTTP proxy
// 是逐位元組透傳 ai-service 的 body，該路徑已移除）。
func writeProcurementError(c *gin.Context, err error) {
	st := status.Convert(err)
	body := procurementErrorBody{
		Code:    procurementwire.GRPCCodeToHTTPStatus(st.Code()),
		Message: st.Message(),
	}
	// Remaining／ResetAt 必須跟 Reason／Message 從**同一個** detail 取——它們是
	// 同一個 BusinessError 上的四個欄位，分開讀會在多 detail 時錯配。
	// procurementwire.BusinessErrorFromStatus 是與 SSE 那條路徑共用的唯一取出點
	// （proxy/ai_assistant_stream.go），兩條 wire 因此不可能只有一邊帶到額度資訊。
	if be := procurementwire.BusinessErrorFromStatus(st); be != nil {
		body.Reason = be.GetReason()
		body.Message = be.GetMessage()
		body.Remaining = be.GetRemaining()
		body.ResetAt = be.GetResetAt()
	}
	emitProcurementErrorBody(c, body)
}

// writeProcurementErrorBody 是 Gateway 自己的 pre-flight 驗證拒絕用的入口——跟
// writeProcurementError 走同一個 struct，所以「這個 400 是 Gateway 擋的還是
// ai-service 擋的」對 client 完全不可見（也不該可見）。reason 對齊 ai-service
// 端同一個檢查的 BusinessError.reason：多數 schema 級驗證在 ai-service 也是
// procurementStatus(..., "", msg)，故 reason 為 ""；有機器碼的（INPUT_TOO_LONG）
// 則兩邊帶同一個 token、message 同一句不含前綴的中文。
//
// pre-flight 永遠不帶 Remaining／ResetAt：Gateway 自己擋得下來的沒有一項是額度
// 用盡（那個判定只有 ai-service 做得到），所以這個入口刻意維持 4 個參數，不為了
// 一個 RPC-only 的欄位污染 17 個呼叫端。
func writeProcurementErrorBody(c *gin.Context, httpStatus int, reason, message string) {
	emitProcurementErrorBody(c, procurementErrorBody{Code: httpStatus, Reason: reason, Message: message})
}

// emitProcurementErrorBody 是 procurementErrorBody 唯一的實際輸出點——兩個入口
// （RPC 錯誤、pre-flight 拒絕）都經過這裡，所以 traceId 的填法與下面那條
// ruleguard 例外各自只存在一份。
//
// The inline gocritic suppression below is the one deliberate exception this
// migration already carried: the ruleguard rule (gateway/ruleguard/rules.go)
// bans direct gin JSON emitters in favour of response.*, but procurement's
// error contract has a `reason` field pkg/envelope.Envelope does not, so no
// response.* helper can produce this body — routing through response.Err
// instead is precisely the defect this file exists to fix.
func emitProcurementErrorBody(c *gin.Context, body procurementErrorBody) {
	body.TraceID = envelope.ExtractTraceID(c)
	c.JSON(body.Code, body) //nolint:gocritic
}

// badProcurementRequest 是最常見的那一種 pre-flight 拒絕（400、沒有機器碼）的
// 簡寫，對應被取代的 response.BadRequest 呼叫點。
func badProcurementRequest(c *gin.Context, message string) {
	writeProcurementErrorBody(c, 400, "", message)
}
