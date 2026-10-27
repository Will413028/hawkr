package grpc

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// envelopeMapping 決定一個 envelope 業務碼在 gRPC 邊界上的兩件**互相獨立**的事：
// 映射成哪個 gRPC status code，以及數字碼要不要靠 "[code] " 前綴活著送到 handler。
//
// 兩者曾被綁成同一件事（列表內 = 不加前綴），於是任何需要非預設 status 的業務碼
// 都得付出「數字碼被抹掉」的代價——4291 即因此在 gRPC 邊界消失，讓前端只剩 HTTP
// 429 可判，與限流器的 429 撞在一起（#2272）。
type envelopeMapping struct {
	grpcCode codes.Code
	// preserveBizCode 為 true 時，message 前綴 "[code] "，讓 response 層還原成
	// envelope Code。為 false 的碼是 HTTP 鏡射碼（400/401/…），前綴反而會破壞它們。
	preserveBizCode bool
}

// envelopeCodeToGRPC maps envelope business error codes to gRPC status codes.
var envelopeCodeToGRPC = map[int32]envelopeMapping{
	400: {grpcCode: codes.InvalidArgument},
	401: {grpcCode: codes.Unauthenticated},
	403: {grpcCode: codes.PermissionDenied},
	404: {grpcCode: codes.NotFound},
	409: {grpcCode: codes.AlreadyExists},
	412: {grpcCode: codes.FailedPrecondition},
	500: {grpcCode: codes.Internal},
	503: {grpcCode: codes.Unavailable},
	// Public product visibility is a business-specific 404 family. Preserve
	// the CodeMeta number so clients can distinguish absent, never published,
	// and permanently removed without changing the HTTP status.
	3021: {grpcCode: codes.NotFound, preserveBizCode: true},
	3022: {grpcCode: codes.NotFound, preserveBizCode: true},
	3023: {grpcCode: codes.NotFound, preserveBizCode: true},
	// AI 修圖本週額度已用完：需要 429（非預設的 409），同時數字碼必須活著——前端
	// 靠 code 4291 才能與限流器的 429 區分。
	4291: {grpcCode: codes.ResourceExhausted, preserveBizCode: true},
	// 會員方案代碼無效（9400）：輸入的方案代碼不存在，映射為 InvalidArgument (400)，
	// 且保留數字碼供前端識別業務錯誤（59 號設計書 §10）。
	9400: {grpcCode: codes.InvalidArgument, preserveBizCode: true},
	// 58 號《電子發票》9300~9399 業務碼段的 not-found 家族（§6.1.0(13)）。
	// 登記的理由是 Tier A：越權回應必須與「不存在」完全一致 ⇒ 需要 404 而不是預設的
	// 409。preserveBizCode 一律 true，否則 envelope 的 code 會變成 404，業務碼永遠不
	// 存在（那個寫法不會讓任何測試轉紅）。
	9301: {grpcCode: codes.NotFound, preserveBizCode: true}, // INVOICE_DOCUMENT_NOT_FOUND
	9302: {grpcCode: codes.NotFound, preserveBizCode: true}, // INVOICE_PROFILE_NOT_FOUND
	9309: {grpcCode: codes.NotFound, preserveBizCode: true}, // INVOICE_COMPLAINT_NOT_FOUND
	9320: {grpcCode: codes.NotFound, preserveBizCode: true}, // SELLER_TAX_PROFILE_NOT_FOUND
	9330: {grpcCode: codes.NotFound, preserveBizCode: true}, // INVOICE_PROVIDER_ACCOUNT_NOT_FOUND
	9340: {grpcCode: codes.NotFound, preserveBizCode: true}, // INVOICE_JOB_NOT_FOUND
	// 重寄通知節流：需要 429（非預設的 409），數字碼必須活著才與限流器的 429 區分。
	9352: {grpcCode: codes.ResourceExhausted, preserveBizCode: true}, // INVOICE_NOTIFICATION_RESEND_TOO_FREQUENT
	// ⚠️ 9300 段其餘 28 個碼**刻意兩張表都不登記**，走 fallback
	// {FailedPrecondition, preserveBizCode: true} ⇒ HTTP 409 ＋ 正確業務碼。那是多數碼
	// 的正解，不是遺漏。特別是 9332（provider 不可用）不得走 codes.Unavailable、
	// 9350（冪等鍵 payload 不符）不得走 codes.InvalidArgument——那兩條路上 envelope 的
	// code 會被 HTTP status 覆蓋成 503／400，業務碼在邊界上蒸發。503 只保留給
	// 「我方 invoice-service 不可達／逾時」。
}

// envelopeCodePreserveData：這些業務 code 雖然非 0，但其 data payload 含有
// 結構化資訊（如 unavailable_items[]、price_changed detail），前端需要原樣讀取。
// interceptor 對這些 code 不轉 error，handler 自行依 resp.Code + resp.Data 組裝
// envelope response 透傳給前端。issue #1127：4001 INSUFFICIENT_STOCK 帶
// data.unavailable_items[] 是核心契約，不可丟。
var envelopeCodePreserveData = map[int32]bool{
	4001: true, // INSUFFICIENT_STOCK / 商品不可購買 — data.unavailable_items[]
	4091: true, // CHECKOUT_PRICE_CHANGED — price_changed detail
	4092: true, // ORDER_ALREADY_CREATED — message 含 orderNo（前端引導付款）
	// 58 號《電子發票》帶 data 的業務碼（§6.1.0(13)）。這些碼 HTTP 恆為 200，handler
	// 走 response.Biz 把 data 原樣透傳。漏登記的話 data 整包在 gRPC 邊界被丟掉，前端
	// 拿不到 complaintId／documentId／cooldownUntil，症狀是「按鈕點了沒反應」。
	//
	// ⚠️ 與 envelopeCodeToGRPC 的交集必須為空：同時登記時 preserveData 先命中並
	// return nil，另一列永不執行。
	9310: true, // INVOICE_COMPLAINT_DUPLICATE — data.complaintId 導向既有案件
	9324: true, // SELLER_TAX_REAPPLY_COOLDOWN — data.cooldownUntil
	9326: true, // SELLER_INVOICE_INTAKE_LIMIT_REACHED — data.intakeLimit
	9331: true, // INVOICE_PROVIDER_CREDENTIAL_INVALID — data.providerRequestId
	9342: true, // INVOICE_JOB_CANCEL_BLOCKED_UNKNOWN — data 帶最後查證時間與 requestId
	9344: true, // INVOICE_MANUAL_UNLOCK_NEEDS_SECOND_APPROVAL — data.unlockId
	9347: true, // INVOICE_DUPLICATE — data.documentId
}

// EnvelopeGuardInterceptor 是 gRPC client interceptor，
// 自動將 envelope error（gRPC OK + resp.Code != 0）轉換為 gRPC error。
//
// 這樣 Gateway handler 只需檢查 err != nil，不需要手動檢查 resp.Code，
// 也不會因為忘記檢查而觸發 resp.Data nil dereference panic。
func EnvelopeGuardInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			return err
		}

		// 所有 proto response 都有 GetCode() int32
		type coder interface{ GetCode() int32 }
		resp, ok := reply.(coder)
		if !ok {
			// 無法判斷：這支 RPC 的 reply 沒有 envelope code。刻意**不** Disarm ——
			// 「我讀不出來」不是「上一個碼已經失效」的證據。
			return nil
		}
		if resp.GetCode() == 0 {
			// **同一支 method** 回報成功 ⇒ 解除它自己先前掛上的 arm（重試成功）。
			//
			// 🔴 逐 method 是重點。第一版是無條件 Disarm(ctx)，於是「RPC#1 回 9331、
			// RPC#2 成功」會讓 #1 的碼憑空消失 —— 拿「沒有觀察到危險」推論「沒有危險」，
			// 正是 bizcarrier 要消滅的那個根因換了個地方住。別支 RPC 的成功不是前一支
			// 失敗已被處理的證據；真要忽略請用 bizcarrier.Accept 指名那個碼。
			bizcarrier.Disarm(ctx, method)
			return nil
		}

		code := resp.GetCode()

		// 取得 message
		msg := fmt.Sprintf("service error (code=%d)", code)
		if m, ok := reply.(interface{ GetMessage() string }); ok && m.GetMessage() != "" {
			msg = m.GetMessage()
		}

		// 保留結構化 data 的業務碼：interceptor 不轉 error（data 過不了
		// status.Error），改由 bizcarrier 把碼帶到回應寫出的那一刻。
		//
		// 🔴 **Arm 失敗就不發這張特權票**。Arm 回 false 代表這條呼叫的 context 上沒有
		// carrier（handler 傳了 detached context，或這支呼叫不在 HTTP 請求路徑上），
		// 也就是「接下來沒有任何東西會替我把這個碼送出去」。此時落下去走一般錯誤
		// 映射：data 掉了（降級），但 handler 的 `if err != nil` 一定接得到，不會
		// 產生 code=0 的假成功。
		//
		// 這是與舊 AST gate 相反的預設：那道 gate 把「我看不到成功出口」當成安全的
		// 證明並靜默放行（per-site fail-OPEN），9 種繞道全部從那個洞進來。
		if envelopeCodePreserveData[code] {
			if bizcarrier.Arm(ctx, method, code, msg) {
				return nil
			}
		}

		// 對應 gRPC status code；未知業務碼走 FailedPrecondition 且一律保留數字碼
		mapping, found := envelopeCodeToGRPC[code]
		if !found {
			mapping = envelopeMapping{grpcCode: codes.FailedPrecondition, preserveBizCode: true}
		}
		if mapping.preserveBizCode {
			msg = fmt.Sprintf("[%d] %s", code, msg)
		}

		return status.Error(mapping.grpcCode, msg)
	}
}
