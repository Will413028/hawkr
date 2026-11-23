package charge

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/riverqueue/river"
	"uuid"
)

type recordingEnqueuer struct{ opts []river.InsertOpts }

func (r *recordingEnqueuer) Enqueue(_ context.Context, _ river.JobArgs, opts *river.InsertOpts) error {
	r.opts = append(r.opts, *opts)
	return nil
}

// 補做 job 可以延後執行：請求內建立的上限之後才跑；沒有指定時刻（掃描）立刻執行。
func TestEnqueueCreateAtDelaysTheJobOnlyWhenAskedTo(t *testing.T) {
	t.Parallel()
	rec := &recordingEnqueuer{}
	at := time.Date(2026, 11, 23, 4, 0, 15, 0, time.UTC)
	if err := enqueueCreateAt(context.Background(), rec, uuid.NewV7(), at); err != nil {
		t.Fatal(err)
	}
	if err := enqueueCreate(context.Background(), rec, uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	if len(rec.opts) != 2 || !rec.opts[0].ScheduledAt.Equal(at) || !rec.opts[1].ScheduledAt.IsZero() {
		t.Fatalf("ScheduledAt=%v", rec.opts)
	}
	if createJobDelay <= requestCallTimeout+2*writeTimeout {
		t.Fatalf("補做的延後 %v 必須長於請求內的全部上限（記呼叫時刻、呼叫 provider、寫結果）%v", createJobDelay, requestCallTimeout+2*writeTimeout)
	}
}

// 付款說明：訂單的商品名稱（去頭尾空白、超過上限以 rune 截斷），沒有就用訂單編號；永遠不是空字串（payments_description_check）。
func TestPaymentDescriptionTrimsFallsBackToTheOrderNoAndCapsByRunes(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		in   Order
		want string
	}{
		"商品名稱":  {Order{OrderNo: "O-1", Description: "  藍色 T 恤 "}, "藍色 T 恤"},
		"空白":    {Order{OrderNo: "O-2", Description: " \t "}, "O-2"},
		"沒有":    {Order{OrderNo: "O-3"}, "O-3"},
		"截斷以字計": {Order{OrderNo: "O-4", Description: strings.Repeat("長", 250)}, strings.Repeat("長", descriptionMax)},
	} {
		if got := paymentDescription(tc.in); got != tc.want || utf8.RuneCountInString(got) < 1 || utf8.RuneCountInString(got) > descriptionMax {
			t.Errorf("%s：%q，want %q", name, got, tc.want)
		}
	}
}
