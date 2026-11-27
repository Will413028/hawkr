package grpc

import (
	"sort"
	"testing"

	"google.golang.org/grpc/codes"
)


var invoiceCodeToGRPCExpected = map[int32]codes.Code{
	9301: codes.NotFound,
	9302: codes.NotFound,
	9309: codes.NotFound,
	9320: codes.NotFound,
	9330: codes.NotFound,
	9340: codes.NotFound,
	9352: codes.ResourceExhausted,
}

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
