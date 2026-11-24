package couponconsumer

import (
	"context"
	"errors"
	"testing"
)

type recorded struct{ store, buyer int64 }

type fakeRecorder struct {
	calls []recorded
	err   error
}

func (f *fakeRecorder) RecordOrderCompleted(_ context.Context, storeID, buyerID int64) (bool, error) {
	f.calls = append(f.calls, recorded{storeID, buyerID})
	return true, f.err
}

func TestOrderCompletedRecordsBusinessKeysFromTheJavaPayload(t *testing.T) {
	t.Parallel()
	r := &fakeRecorder{}
	body := `{"event_id":"e1","order_no":"O1","buyer_id":"71","store_id":"5","notify_buyer_account_id":"9","total_actual_amount":"100"}`
	if err := (Consumer{Coupon: r}).OrderCompleted(context.Background(), []byte(body)); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || r.calls[0] != (recorded{store: 5, buyer: 71}) {
		t.Fatalf("calls = %+v，要 store 5、buyer 71（不是 account id）", r.calls)
	}
}

func TestOrderCompletedClassifiesBadPayloadsAsPermanentWithoutWriting(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"不是 JSON":       `{`,
		"缺 buyer_id":    `{"store_id":"5"}`,
		"缺 store_id":    `{"buyer_id":"71"}`,
		"buyer_id 空字串":  `{"buyer_id":"","store_id":"5"}`,
		"store_id 為零":   `{"buyer_id":"71","store_id":"0"}`,
		"buyer_id 為負":   `{"buyer_id":"-1","store_id":"5"}`,
		"buyer_id 不是數字": `{"buyer_id":"x","store_id":"5"}`,
		"數字型別":          `{"buyer_id":71,"store_id":5}`,
	} {
		r := &fakeRecorder{}
		err := (Consumer{Coupon: r}).OrderCompleted(context.Background(), []byte(body))
		if err == nil || !isPermanent(err) || len(r.calls) != 0 {
			t.Errorf("%s：err=%v permanent=%v calls=%d，要 permanent 且不寫入", name, err, err != nil && isPermanent(err), len(r.calls))
		}
	}
}

func TestOrderCompletedRetriesStorageFailures(t *testing.T) {
	t.Parallel()
	r := &fakeRecorder{err: errors.New("db down")}
	err := (Consumer{Coupon: r}).OrderCompleted(context.Background(), []byte(`{"buyer_id":"71","store_id":"5"}`))
	if err == nil || isPermanent(err) {
		t.Fatalf("資料庫錯誤要可重試：%v", err)
	}
}
