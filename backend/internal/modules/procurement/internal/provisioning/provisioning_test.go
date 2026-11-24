package provisioning

import (
	"errors"
	"testing"
	"time"
)

func TestItemName(t *testing.T) {
	cases := []struct {
		name                        string
		item, label, quantity, unit string
		want                        string
		wantErr                     bool
	}{
		{name: "解析後的標籤優先", item: "高麗菜", label: "甘藍", quantity: "12.5", unit: "公斤", want: "甘藍 12.5 公斤"},
		{name: "沒有標籤用買家原話", item: "高麗菜", quantity: "3", unit: "KG", want: "高麗菜 3 KG"},
		{name: "空白標籤用買家原話", item: " 高麗菜 ", label: "  ", quantity: " 3 ", unit: " KG ", want: "高麗菜 3 KG"},
		{name: "沒有品項", quantity: "3", unit: "KG", wantErr: true},
		{name: "沒有數量", item: "高麗菜", unit: "KG", wantErr: true},
		{name: "沒有單位", item: "高麗菜", quantity: "3", wantErr: true},
	}
	for _, c := range cases {
		got, err := ItemName(c.item, c.label, c.quantity, c.unit)
		if c.wantErr {
			if !errors.Is(err, ErrItemNameUnavailable) {
				t.Errorf("%s：要 ErrItemNameUnavailable，得到 %v", c.name, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s：得到 %q %v，要 %q", c.name, got, err, c.want)
		}
	}
}

func TestClassify(t *testing.T) {
	deadline := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   Result
		want Verdict
	}{
		{"成功", Result{OrderNo: "HK1", PaymentDeadline: deadline}, Verdict{Outcome: OutcomeCreated}},
		{"500 可重試", Result{Code: 500}, Verdict{Outcome: OutcomeRetry}},
		{"其他碼終態拒絕", Result{Code: 9504}, Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}},
		{"一般業務碼終態拒絕", Result{Code: 400}, Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}},
		{"501 不是暫時性", Result{Code: 501}, Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}},
		{"成功缺訂單號", Result{PaymentDeadline: deadline}, Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}},
		{"成功缺付款期限", Result{OrderNo: "HK1"}, Verdict{Outcome: OutcomeRejected, Reason: ReasonRejected}},
	}
	for _, c := range cases {
		if got := Classify(c.in); got != c.want {
			t.Errorf("%s：得到 %+v，要 %+v", c.name, got, c.want)
		}
	}
}
