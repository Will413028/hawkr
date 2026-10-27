package grpc

import (
	"sort"
	"testing"

	"google.golang.org/grpc/codes"
)

// 58 號《電子發票》§C.8 的 7 ＋ 7 列登記。
//
// **為什麼需要這一支**：§C.8 自己就標了「不是 CodeMeta 生成的，最容易漏」，而 2026-08-29
// 的獨立驗收實測——把 `envelopeCodeToGRPC` 的 9301 整列刪掉，`go build` 過、gateway 全部
// 16 個 package 的 `go test` 零 FAIL。也就是說這兩張表在此之前**沒有任何機械保證**。
//
// 漏一列的後果分兩類，都靜默：
//   - `envelopeCodeToGRPC` 少一列 ⇒ 該碼從 404 塌成 fallback 的 409，
//     §8.1.4 要求的 Tier A「越權與不存在不可區分」當場破掉
//   - `envelopeCodePreserveData` 少一列 ⇒ `data` 被 `return nil` 丟掉，
//     9331 的 providerRequestId、9347 的 documentId、9344 的 unlockId 靜默消失
//
// 本檔只釘 93xx 段（發票領域）。其餘領域的碼不在此斷言——那會讓別的領域每加一個碼
// 都要回頭改這裡，而它們各有自己的 owner。

// §C.8：需要 404 的 not-found 家族 ＋ 一個需要 429 的節流碼。
var invoiceCodeToGRPCExpected = map[int32]codes.Code{
	9301: codes.NotFound,          // INVOICE_DOCUMENT_NOT_FOUND
	9302: codes.NotFound,          // INVOICE_PROFILE_NOT_FOUND
	9309: codes.NotFound,          // INVOICE_COMPLAINT_NOT_FOUND
	9320: codes.NotFound,          // SELLER_TAX_PROFILE_NOT_FOUND
	9330: codes.NotFound,          // INVOICE_PROVIDER_ACCOUNT_NOT_FOUND
	9340: codes.NotFound,          // INVOICE_JOB_NOT_FOUND
	9352: codes.ResourceExhausted, // INVOICE_NOTIFICATION_RESEND_TOO_FREQUENT
}

// §C.8：非零業務碼仍要帶 data 的七個。
var invoicePreserveDataExpected = []int32{9310, 9324, 9326, 9331, 9342, 9344, 9347}

func invoiceCodesIn(m map[int32]envelopeMapping) []int32 {
	var out []int32
	for c := range m {
		if c >= 9300 && c <= 9399 {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestInvoiceEnvelopeCodeToGRPC_ExactSet(t *testing.T) {
	got := invoiceCodesIn(envelopeCodeToGRPC)
	want := make([]int32, 0, len(invoiceCodeToGRPCExpected))
	for c := range invoiceCodeToGRPCExpected {
		want = append(want, c)
	}
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })

	if len(got) != len(want) {
		t.Fatalf("93xx 登記數不符：got %v (%d), want %v (%d)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("93xx 碼集合不符：got %v, want %v", got, want)
		}
	}
	for code, wantGRPC := range invoiceCodeToGRPCExpected {
		m := envelopeCodeToGRPC[code]
		if m.grpcCode != wantGRPC {
			t.Errorf("%d: grpcCode = %v, want %v", code, m.grpcCode, wantGRPC)
		}
		// preserveBizCode 一律 true，否則 envelope 的 code 會被 HTTP status 覆蓋，
		// 業務碼在邊界上蒸發——那個寫法不會讓任何既有測試轉紅。
		if !m.preserveBizCode {
			t.Errorf("%d: preserveBizCode 必須是 true", code)
		}
	}
}

func TestInvoiceEnvelopeCodePreserveData_ExactSet(t *testing.T) {
	var got []int32
	for c := range envelopeCodePreserveData {
		if c >= 9300 && c <= 9399 {
			got = append(got, c)
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

	if len(got) != len(invoicePreserveDataExpected) {
		t.Fatalf("93xx preserveData 數不符：got %v (%d), want %v (%d)",
			got, len(got), invoicePreserveDataExpected, len(invoicePreserveDataExpected))
	}
	for i := range got {
		if got[i] != invoicePreserveDataExpected[i] {
			t.Fatalf("93xx preserveData 集合不符：got %v, want %v", got, invoicePreserveDataExpected)
		}
	}
}

// §C.8：兩張表的交集必須為空。`preserveData` 命中會 `return nil`，
// 同碼在另一張表的那一列永遠不執行——重複登記不會報錯，只會讓其中一份成為死碼。
func TestEnvelopeGuardTablesAreDisjoint(t *testing.T) {
	for code := range envelopeCodeToGRPC {
		if envelopeCodePreserveData[code] {
			t.Errorf("業務碼 %d 同時出現在 envelopeCodeToGRPC 與 envelopeCodePreserveData", code)
		}
	}
}

// §C.8 逐字：「`9332` 一個字都不寫進去」。
//
// 憑證測試端點的整個設計建立在「9331 憑證錯 → 改 AppKey」與「9332 暫時不可用 →
// 不要改 AppKey」可區分之上。把 9332 登記成 codes.Unavailable 會讓業務碼被吃掉、
// code 恆為 503，兩者再也分不出來。
func TestInvoiceCode9332IsNotRegistered(t *testing.T) {
	if _, ok := envelopeCodeToGRPC[9332]; ok {
		t.Error("9332 不得登記進 envelopeCodeToGRPC（會讓它與 9331 無法區分）")
	}
	if envelopeCodePreserveData[9332] {
		t.Error("9332 不得登記進 envelopeCodePreserveData")
	}
}
