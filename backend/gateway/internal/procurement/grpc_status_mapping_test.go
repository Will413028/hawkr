package procurement

import (
	"testing"

	"google.golang.org/grpc/codes"
)

// GRPCCodeToHTTPStatus 決定 procurement 全部 18 支端點對外的 HTTP status，兩條路徑
// 共用：unary 的 error envelope（handler/procurement_errors.go）與 SSE 的 AG-UI
// RUN_ERROR frame（proxy/ai_assistant_stream.go）。它先前沒有任何測試。
//
// 這裡刻意列出 gRPC 的**全部** 17 個標準 code，而不只是有 case 的那幾個。落在
// default 的 code 也要在這張表裡寫明預期值，這樣新增或改動一個 case 時，會被迫
// 回答「其他 code 是否也該跟著變」，而不是靜默地讓某個 code 換一個 HTTP status。
func TestGRPCCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
		why  string
	}{
		{codes.OK, 500, "不該走到錯誤映射；真的走到就是上游漏判 err，回 500 才會被注意到"},
		{codes.Canceled, 500, "未分類，落 default"},
		{codes.Unknown, 500, "未分類，落 default"},
		{codes.InvalidArgument, 400, "參數/schema 驗證失敗"},
		{codes.DeadlineExceeded, 500, "未分類，落 default"},
		{codes.NotFound, 404, "資源不存在；cross-buyer 與亂數 ID 共用此外形"},
		{codes.AlreadyExists, 500, "未分類，落 default"},
		{codes.PermissionDenied, 403, "無權操作"},
		{codes.ResourceExhausted, 429, "額度/速率上限。與 gateway 其他 AI 端點一致（4291 → ResourceExhausted → 429）"},
		{codes.FailedPrecondition, 422, "語意驗證失敗（例如答案不符問題卡 schema）"},
		{codes.Aborted, 409, "衝突（冪等鍵、版本過期、已有進行中的批次）"},
		{codes.OutOfRange, 410, "Task 12：GET replay 的 Last-Event-ID 早於 minAvailableSeq（REPLAY_CURSOR_EXPIRED），該段 ledger 已不在保留視窗內，重試同一個 cursor 永遠不會成功"},
		{codes.Unimplemented, 500, "未分類，落 default"},
		{codes.Internal, 500, "系統錯誤"},
		{codes.Unavailable, 503, "下游暫時不可用"},
		{codes.DataLoss, 500, "未分類，落 default"},
		{codes.Unauthenticated, 401, "未登入"},
	}

	// gRPC 的 canonical code 是 0..16。這裡斷言「每個 code 恰好出現一次」而不是只數
	// 長度——只檢查長度的話，把某一列複製成第二個 codes.OK 仍然是 17 列、仍然全綠，
	// 而被擠掉的那個 code 從此無人測（subtest 同名會自動變成 OK#01，也不會報錯）。
	covered := map[codes.Code]bool{}
	for _, tt := range tests {
		if covered[tt.code] {
			t.Fatalf("code %s 在表格中重複出現", tt.code)
		}
		covered[tt.code] = true
	}
	for c := codes.Code(0); c <= codes.Unauthenticated; c++ {
		if !covered[c] {
			t.Fatalf("gRPC code %s(%d) 沒有列在表格裡——新增或改動 case 時必須一併決定它的 HTTP status", c, c)
		}
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if got := GRPCCodeToHTTPStatus(tt.code); got != tt.want {
				t.Fatalf("GRPCCodeToHTTPStatus(%s) = %d, want %d\n理由：%s",
					tt.code, got, tt.want, tt.why)
			}
		})
	}
}
