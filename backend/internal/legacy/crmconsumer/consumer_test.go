package crmconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
)

type recordedJobs struct{ args []river.JobArgs }

func (r *recordedJobs) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	r.args = append(r.args, args)
	return nil
}

type fakeCRM struct {
	orders  []crm.OrderCompleted
	refunds []crm.Refund
	err     error
}

func (f *fakeCRM) RecordOrderCompleted(_ context.Context, e crm.OrderCompleted) (bool, error) {
	f.orders = append(f.orders, e)
	return f.err == nil, f.err
}

func (f *fakeCRM) RecordRefund(_ context.Context, r crm.Refund) (bool, error) {
	f.refunds = append(f.refunds, r)
	return f.err == nil, f.err
}

type fakeParties struct {
	p   orderservice.OrderParties
	err error
	got string
}

func (f *fakeParties) OrderParties(_ context.Context, orderNo string) (orderservice.OrderParties, error) {
	f.got = orderNo
	return f.p, f.err
}

const goodOrder = `{"event_id":"evt-1","order_no":"O1","buyer_id":"71","store_id":"5","total_actual_amount":"1200","notify_buyer_account_id":"9","completed_at":"x"}`
const goodPartial = `{"event_id":"e-1","case_no":"C1","order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"300","external_ref_id":"REF-1"}`

func field(body, name, value string) string {
	// 取代 JSON 字串欄位的值（欄位在 body 內一定存在）
	marker := `"` + name + `":"`
	i := strings.Index(body, marker)
	if i < 0 {
		panic("no field " + name)
	}
	start := i + len(marker)
	end := start + strings.Index(body[start:], `"`)
	return body[:start] + value + body[end:]
}

func without(body, name string) string {
	marker := `"` + name + `":"`
	i := strings.Index(body, marker)
	start := i + len(marker)
	end := start + strings.Index(body[start:], `"`) + 1
	cut := body[:i] + body[end:]
	return strings.Replace(strings.Replace(cut, ",,", ",", 1), "{,", "{", 1)
}

// CD6：兩個事件對同一種壞輸入分類一致，壞輸入全部是 permanent；好的輸入排一個 job。
func TestMalformedInputsArePermanentForBothEvents(t *testing.T) {
	t.Parallel()
	type input struct {
		name   string
		order  string
		refund string
	}
	long := strings.Repeat("k", crm.MaxEventKeyLen+1)
	inputs := []input{
		{"壞 JSON", `{`, `{`},
		{"空 body", ``, ``},
		{"缺 event_id", without(goodOrder, "event_id"), without(goodPartial, "event_id")},
		{"缺 order_no", without(goodOrder, "order_no"), without(goodPartial, "order_no")},
		{"order_no 超過字元上限", field(goodOrder, "order_no", strings.Repeat("單", crm.MaxOrderNoLen+1)), field(goodPartial, "order_no", strings.Repeat("單", crm.MaxOrderNoLen+1))},
		{"event_id 或 external_ref_id 過長", field(goodOrder, "event_id", long), field(goodPartial, "external_ref_id", long)},
	}
	jobs := &recordedJobs{}
	c := Consumer{Jobs: jobs}
	// 輔助函式造出的「缺欄位」輸入必須仍是合法 JSON，否則測的是壞 JSON 而不是缺欄位。
	for _, in := range inputs[2:] {
		if !json.Valid([]byte(in.order)) || !json.Valid([]byte(in.refund)) {
			t.Fatalf("%s：測試輸入不是合法 JSON", in.name)
		}
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			t.Parallel()
			if err := c.OrderCompleted(t.Context(), []byte(in.order)); !isPermanent(err) {
				t.Errorf("order.completed %q：err=%v，預期 permanent", in.name, err)
			}
			if err := c.RefundCompleted(t.Context(), []byte(in.refund)); !isPermanent(err) {
				t.Errorf("refund.completed %q：err=%v，預期 permanent", in.name, err)
			}
		})
	}
	if len(jobs.args) != 0 {
		t.Fatalf("壞輸入排入了 %d 個 job", len(jobs.args))
	}
}

func TestOrderCompletedFieldValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"buyer_id 非數字", field(goodOrder, "buyer_id", "abc")},
		{"buyer_id 為 0（舊碼丟棄，現為 permanent）", field(goodOrder, "buyer_id", "0")},
		{"store_id 為負", field(goodOrder, "store_id", "-3")},
		{"store_id 空字串", field(goodOrder, "store_id", "")},
		{"金額非整數", field(goodOrder, "total_actual_amount", "12.5")},
		{"金額為負", field(goodOrder, "total_actual_amount", "-1")},
		{"金額空字串", field(goodOrder, "total_actual_amount", "")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			jobs := &recordedJobs{}
			if err := (Consumer{Jobs: jobs}).OrderCompleted(t.Context(), []byte(tc.body)); !isPermanent(err) || len(jobs.args) != 0 {
				t.Fatalf("err=%v jobs=%d，預期 permanent 且不排 job", err, len(jobs.args))
			}
		})
	}
}

func TestRefundFieldValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{"未知的 outcome", field(goodPartial, "refund_outcome", "SOMETHING")},
		{"空 outcome", field(goodPartial, "refund_outcome", "")},
		{"部分退款金額非整數", field(goodPartial, "refund_amount", "3.5")},
		{"部分退款金額為 0", field(goodPartial, "refund_amount", "0")},
		{"部分退款金額為負", field(goodPartial, "refund_amount", "-5")},
		{"部分退款缺金額", field(goodPartial, "refund_amount", "")},
		{"external_ref_id 與 case_no 都缺", field(field(goodPartial, "external_ref_id", ""), "case_no", "")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			jobs := &recordedJobs{}
			if err := (Consumer{Jobs: jobs}).RefundCompleted(t.Context(), []byte(tc.body)); !isPermanent(err) || len(jobs.args) != 0 {
				t.Fatalf("err=%v jobs=%d，預期 permanent 且不排 job", err, len(jobs.args))
			}
		})
	}
}

// 全額退款不信、也不解析 refund_amount：壞的金額不影響全額退款。
func TestFullRefundIgnoresRefundAmount(t *testing.T) {
	t.Parallel()
	jobs := &recordedJobs{}
	body := field(field(goodPartial, "refund_outcome", "FULL_REFUND"), "refund_amount", "garbage")
	if err := (Consumer{Jobs: jobs}).RefundCompleted(t.Context(), []byte(body)); err != nil {
		t.Fatal(err)
	}
	a := jobs.args[0].(RefundCompletedArgs)
	if a.Outcome != "FULL_REFUND" || a.Amount != 0 {
		t.Fatalf("args=%+v", a)
	}
}

func TestRefundDedupKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, ext, caseNo, want string
		ok                      bool
	}{
		{"用 external_ref_id", "REF-1", "C1", "REF-1", true},
		{"缺 external_ref_id 用 case_no", "", "C1", "AS_REFUND_C1", true},
		{"兩者都缺", "", "", "", false},
		{"external_ref_id 剛好上限", strings.Repeat("a", crm.MaxEventKeyLen), "", strings.Repeat("a", crm.MaxEventKeyLen), true},
		{"external_ref_id 超過上限", strings.Repeat("a", crm.MaxEventKeyLen+1), "", "", false},
		{"case_no 加前綴後超過上限", "", strings.Repeat("a", crm.MaxEventKeyLen-len("AS_REFUND_")+1), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := refundDedupKey(tc.ext, tc.caseNo)
			if tc.ok != (err == nil) || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
			if err != nil && !isPermanent(err) {
				t.Fatalf("err 不是 permanent：%v", err)
			}
		})
	}
}

// 冪等鍵：同一筆退款重發時 event_id 每次都不同，鍵只看 external_ref_id（case_no），所以 River 的唯一鍵也一樣。
func TestRefundKeyIgnoresEventID(t *testing.T) {
	t.Parallel()
	jobs := &recordedJobs{}
	c := Consumer{Jobs: jobs}
	for _, id := range []string{"e-1", "e-2", "e-3"} {
		if err := c.RefundCompleted(t.Context(), []byte(field(goodPartial, "event_id", id))); err != nil {
			t.Fatal(err)
		}
	}
	first := jobs.args[0].(RefundCompletedArgs)
	for i, a := range jobs.args {
		if a.(RefundCompletedArgs) != first {
			t.Fatalf("第 %d 個 job %+v 與第一個 %+v 不同", i, a, first)
		}
	}
	if first.Key != "REF-1" {
		t.Fatalf("key = %q", first.Key)
	}
}

func TestOrderCompletedEnqueuesDecodedFieldsOnly(t *testing.T) {
	t.Parallel()
	jobs := &recordedJobs{}
	if err := (Consumer{Jobs: jobs}).OrderCompleted(t.Context(), []byte(goodOrder)); err != nil {
		t.Fatal(err)
	}
	got := jobs.args[0].(OrderCompletedArgs)
	want := OrderCompletedArgs{EventID: "evt-1", OrderNo: "O1", BuyerID: 71, StoreID: 5, Amount: 1200}
	if got != want {
		t.Fatalf("args = %+v，預期 %+v", got, want)
	}
}

func TestOrderWorkerPassesFieldsAndCancelsHopelessErrors(t *testing.T) {
	t.Parallel()
	args := OrderCompletedArgs{EventID: "evt-1", OrderNo: "O1", BuyerID: 71, StoreID: 5, Amount: 1200}
	f := &fakeCRM{}
	w := &orderCompletedWorker{crm: f}
	if err := w.Work(t.Context(), &river.Job[OrderCompletedArgs]{Args: args}); err != nil {
		t.Fatal(err)
	}
	if len(f.orders) != 1 || f.orders[0] != (crm.OrderCompleted{EventID: "evt-1", OrderNo: "O1", StoreID: 5, BuyerID: 71, Amount: 1200}) {
		t.Fatalf("orders = %+v", f.orders)
	}
	for _, hopeless := range []error{crm.ErrInvalidEvent, fmt.Errorf("x：%w", crm.ErrPartiesMismatch)} {
		w.crm = &fakeCRM{err: hopeless}
		var cancel *river.JobCancelError
		if err := w.Work(t.Context(), &river.Job[OrderCompletedArgs]{Args: args}); !errors.As(err, &cancel) {
			t.Errorf("%v 應取消 job，得到 %v", hopeless, err)
		}
	}
	w.crm = &fakeCRM{err: errors.New("db down")}
	err := w.Work(t.Context(), &river.Job[OrderCompletedArgs]{Args: args})
	var cancel *river.JobCancelError
	if err == nil || errors.As(err, &cancel) {
		t.Fatalf("DB 錯誤應重試（回一般錯誤），得到 %v", err)
	}
}

// CD10：退款的店家與買家來自訂單解析（tx 外）；訂單不存在取消，其他錯誤重試。
func TestRefundWorkerResolvesPartiesAndClassifiesErrors(t *testing.T) {
	t.Parallel()
	args := RefundCompletedArgs{Key: "REF-1", OrderNo: "O1", Outcome: "PARTIAL_REFUND", Amount: 300}
	f := &fakeCRM{}
	parties := &fakeParties{p: orderservice.OrderParties{OrderNo: "O1", BuyerID: 71, StoreID: 5, Status: "50"}}
	w := &refundCompletedWorker{crm: f, orders: parties}
	if err := w.Work(t.Context(), &river.Job[RefundCompletedArgs]{Args: args}); err != nil {
		t.Fatal(err)
	}
	want := crm.Refund{Key: "REF-1", OrderNo: "O1", StoreID: 5, BuyerID: 71, Outcome: crm.RefundPartial, Amount: 300}
	if parties.got != "O1" || len(f.refunds) != 1 || f.refunds[0] != want {
		t.Fatalf("parties.got=%q refunds=%+v", parties.got, f.refunds)
	}

	var cancel *river.JobCancelError
	w2 := &refundCompletedWorker{crm: &fakeCRM{}, orders: &fakeParties{err: orderservice.ErrOrderNotFound}}
	if err := w2.Work(t.Context(), &river.Job[RefundCompletedArgs]{Args: args}); !errors.As(err, &cancel) {
		t.Fatalf("訂單不存在應取消，得到 %v", err)
	}
	crm2 := &fakeCRM{}
	w3 := &refundCompletedWorker{crm: crm2, orders: &fakeParties{err: errors.New("order-service 逾時")}}
	err := w3.Work(t.Context(), &river.Job[RefundCompletedArgs]{Args: args})
	if err == nil || errors.As(err, &cancel) || len(crm2.refunds) != 0 {
		t.Fatalf("訂單服務失敗應重試且不投影：err=%v refunds=%d", err, len(crm2.refunds))
	}
}
