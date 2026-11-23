package transform

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

var (
	png      = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	jpeg     = append([]byte("\xff\xd8\xff\xe0"), make([]byte, 64)...)
	week     = time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC)
	reset    = week.AddDate(0, 0, 7)
	resultPN = []byte("result")
)

// slots 是容量固定的名額，與模組的 limit 套件同一個語意（transform 不依賴它）。
type slots chan struct{}

func (s slots) TryAcquire() (func(), bool) {
	select {
	case s <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-s }) }, true
	default:
		return nil, false
	}
}

type fakeQuota struct {
	mu        sync.Mutex
	order     *[]string
	consumes  int
	refunds   []time.Time
	exhausted bool
	used      int32
}

func (q *fakeQuota) Consume(context.Context, int64) (model.Usage, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.consumes++
	*q.order = append(*q.order, "consume")
	u := model.Usage{Used: q.used + 1, Limit: 10, PeriodStart: week, ResetTime: reset}
	if q.exhausted {
		u.Used = u.Limit
		return u, model.ErrQuotaExhausted
	}
	return u, nil
}

func (q *fakeQuota) Refund(_ context.Context, _ int64, p time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refunds = append(q.refunds, p)
	return nil
}

type fakeGuard struct {
	err   error
	order *[]string
}

func (g fakeGuard) LockNotRetired(context.Context, int64) error {
	*g.order = append(*g.order, "guard")
	return g.err
}

type fakeGen struct {
	results []llm.ImageResult
	err     error
	calls   int
	order   *[]string
}

func (g *fakeGen) GenerateImage(ctx context.Context, _ llm.ImageRequest) (llm.ImageResult, error) {
	*g.order = append(*g.order, "generate")
	if _, ok := ctx.Deadline(); !ok {
		panic("模型呼叫沒有 deadline")
	}
	i := g.calls
	g.calls++
	if g.err != nil {
		return llm.ImageResult{}, g.err
	}
	return g.results[min(i, len(g.results)-1)], nil
}

type rig struct {
	svc     *Service
	quota   *fakeQuota
	gen     *fakeGen
	limiter slots
	order   []string
}

func newRig(guardErr error, results ...llm.ImageResult) *rig {
	r := &rig{limiter: make(slots, 1)}
	r.quota = &fakeQuota{order: &r.order}
	r.gen = &fakeGen{results: results, order: &r.order}
	r.svc = New(Deps{
		Slots: r.limiter, Quota: r.quota, Guard: fakeGuard{err: guardErr, order: &r.order}, Generator: r.gen,
		Tx: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	})
	return r
}

func image() llm.ImageResult { return llm.ImageResult{Image: resultPN, MimeType: "image/png"} }

func input() model.TransformInput {
	return model.TransformInput{StoreID: 7, Image: png, MimeType: "image/png", Prompt: "白色背景"}
}

func reasonOf(t *testing.T, err error) (apperr.Kind, string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("不是 apperr：%v", err)
	}
	return ae.Kind(), ae.Reason.String()
}

func violationFields(err error) []string {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		return nil
	}
	var out []string
	for _, v := range ae.FieldViolations() {
		out = append(out, v.Field)
	}
	return out
}

func TestSlotFullIsRejectedBeforeAnyQuotaIsCharged(t *testing.T) {
	r := newRig(nil, image())
	hold, ok := r.limiter.TryAcquire()
	if !ok {
		t.Fatal("名額應該有空")
	}
	defer hold()
	_, err := r.svc.Transform(context.Background(), input())
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindResourceExhausted || ae.Reason != (apperr.Reason{}) || ae.RetryAfter <= 0 {
		t.Fatalf("名額滿了要回 resource_exhausted 帶 RetryAfter、沒有 reason：%v", err)
	}
	if r.quota.consumes != 0 || len(r.order) != 0 || r.gen.calls != 0 {
		t.Fatalf("名額滿了不能扣額度、不能呼叫模型：consumes=%d order=%v", r.quota.consumes, r.order)
	}
}

func TestOrderIsGuardThenConsumeThenGenerate(t *testing.T) {
	r := newRig(nil, image())
	out, err := r.svc.Transform(context.Background(), input())
	if err != nil || string(out.Image) != "result" || out.MimeType != "image/png" || out.Usage.Used != 1 || out.Usage.Limit != 10 {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	if got := strings.Join(r.order, ","); got != "guard,consume,generate" {
		t.Fatalf("順序 = %s", got)
	}
	if len(r.quota.refunds) != 0 {
		t.Fatal("成功不能退額度")
	}
	if rel, ok := r.limiter.TryAcquire(); !ok {
		t.Fatal("成功後名額要釋放")
	} else {
		rel()
	}
}

func TestInvalidInputIsRejectedWithFieldAndNothingIsCharged(t *testing.T) {
	big := slices.Concat(png[:8], make([]byte, model.MaxImageBytes))
	cases := map[string]struct {
		mut   func(*model.TransformInput)
		field string
	}{
		"image over 2 MiB":      {func(in *model.TransformInput) { in.Image = big }, "Image"},
		"image empty":           {func(in *model.TransformInput) { in.Image = nil }, "Image"},
		"prompt over 500 runes": {func(in *model.TransformInput) { in.Prompt = strings.Repeat("字", 501) }, "Prompt"},
		"prompt empty":          {func(in *model.TransformInput) { in.Prompt = "" }, "Prompt"},
		"prompt only spaces":    {func(in *model.TransformInput) { in.Prompt = " \t　\n" }, "Prompt"},
		"mime empty":            {func(in *model.TransformInput) { in.MimeType = "" }, "MimeType"},
		"mime not allowed":      {func(in *model.TransformInput) { in.MimeType = "image/gif" }, "MimeType"},
		"bytes not an image":    {func(in *model.TransformInput) { in.Image = bytes.Repeat([]byte("x"), 100) }, "Image"},
		"mime lies about bytes": {func(in *model.TransformInput) { in.Image = jpeg }, "Image"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(nil, image())
			in := input()
			c.mut(&in)
			_, err := r.svc.Transform(context.Background(), in)
			if kind, _ := reasonOf(t, err); kind != apperr.KindInvalidInput {
				t.Fatalf("kind = %v err = %v", kind, err)
			}
			if f := violationFields(err); len(f) != 1 || f[0] != c.field {
				t.Fatalf("欄位 = %v，預期 %s", f, c.field)
			}
			if len(r.order) != 0 {
				t.Fatalf("輸入不合不能扣額度：%v", r.order)
			}
		})
	}
}

func TestPromptBoundariesCountRunesAfterTrim(t *testing.T) {
	for name, prompt := range map[string]string{
		"500 runes":           strings.Repeat("字", 500),
		"500 runes in spaces": "  " + strings.Repeat("字", 500) + "　",
		"1 rune":              "a",
	} {
		r := newRig(nil, image())
		in := input()
		in.Prompt = prompt
		if _, err := r.svc.Transform(context.Background(), in); err != nil {
			t.Errorf("%s：%v", name, err)
		}
	}
}

func TestQuotaExhaustedCarriesReasonAndMetadata(t *testing.T) {
	r := newRig(nil, image())
	r.quota.exhausted = true
	_, err := r.svc.Transform(context.Background(), input())
	kind, reason := reasonOf(t, err)
	if kind != apperr.KindResourceExhausted || reason != "IMAGE_QUOTA_EXCEEDED" {
		t.Fatalf("kind = %v reason = %s", kind, reason)
	}
	var ae *apperr.Error
	errors.As(err, &ae)
	if ae.Metadata["remaining"] != "0" || ae.Metadata["resetTime"] != reset.Format(time.RFC3339) {
		t.Fatalf("metadata = %v", ae.Metadata)
	}
	if r.gen.calls != 0 || len(r.quota.refunds) != 0 {
		t.Fatal("額度用完不呼叫模型、也沒有可退的")
	}
}

func TestRetiredStoreIsNeverCharged(t *testing.T) {
	r := newRig(apperr.NotFound("店家已退場"), image())
	_, err := r.svc.Transform(context.Background(), input())
	if kind, _ := reasonOf(t, err); kind != apperr.KindPermissionDenied {
		t.Fatalf("err = %v", err)
	}
	if r.quota.consumes != 0 || r.gen.calls != 0 {
		t.Fatalf("退場的店不能扣額度：consumes=%d", r.quota.consumes)
	}
}

func TestEachModelRefusalMapsToItsReasonAndRefunds(t *testing.T) {
	cases := map[string]struct {
		res    []llm.ImageResult
		kind   apperr.Kind
		reason string
		calls  int
	}{
		"prompt blocked":      {[]llm.ImageResult{{BlockReason: "PROHIBITED_CONTENT"}}, apperr.KindInvalidInput, "IMAGE_SAFETY_BLOCKED", 1},
		"finish image safety": {[]llm.ImageResult{{FinishReason: "IMAGE_SAFETY"}}, apperr.KindInvalidInput, "IMAGE_SAFETY_BLOCKED", 1},
		"finish prohibited":   {[]llm.ImageResult{{FinishReason: "IMAGE_PROHIBITED_CONTENT"}}, apperr.KindInvalidInput, "IMAGE_SAFETY_BLOCKED", 1},
		"finish recitation":   {[]llm.ImageResult{{FinishReason: "IMAGE_RECITATION"}}, apperr.KindInvalidInput, "IMAGE_RECITATION_BLOCKED", 1},
		"no image":            {[]llm.ImageResult{{FinishReason: "NO_IMAGE"}}, apperr.KindUnavailable, "IMAGE_NOT_PRODUCED", 2},
		"stop without image":  {[]llm.ImageResult{{FinishReason: "STOP"}}, apperr.KindUnavailable, "IMAGE_NOT_PRODUCED", 2},
		"empty response":      {[]llm.ImageResult{{}}, apperr.KindUnavailable, "IMAGE_NOT_PRODUCED", 2},
		"image other":         {[]llm.ImageResult{{FinishReason: "IMAGE_OTHER"}}, apperr.KindUnavailable, "IMAGE_NOT_PRODUCED", 2},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(nil, c.res...)
			_, err := r.svc.Transform(context.Background(), input())
			kind, reason := reasonOf(t, err)
			if kind != c.kind || reason != c.reason {
				t.Fatalf("kind = %v reason = %s err = %v", kind, reason, err)
			}
			if r.gen.calls != c.calls {
				t.Fatalf("模型呼叫 %d 次，預期 %d（只有沒有產出圖片才重試一次）", r.gen.calls, c.calls)
			}
			if len(r.quota.refunds) != 1 || !r.quota.refunds[0].Equal(week) {
				t.Fatalf("要以 Consume 回的 PeriodStart 退一次：%v", r.quota.refunds)
			}
		})
	}
}

func TestNotProducedThenImageOnRetrySucceedsWithoutRefund(t *testing.T) {
	r := newRig(nil, llm.ImageResult{FinishReason: "NO_IMAGE"}, image())
	out, err := r.svc.Transform(context.Background(), input())
	if err != nil || len(out.Image) == 0 || r.gen.calls != 2 || len(r.quota.refunds) != 0 || r.quota.consumes != 1 {
		t.Fatalf("out = %+v err = %v calls = %d refunds = %v consumes = %d", out, err, r.gen.calls, r.quota.refunds, r.quota.consumes)
	}
}

func TestProviderFailureRefundsAndIsUnavailableWithoutReason(t *testing.T) {
	for name, cause := range map[string]error{
		"call failed":    errors.New("connection reset"),
		"disabled":       llm.ErrImageDisabled,
		"deadline":       context.DeadlineExceeded,
		"wrapped llm.ok": errors.Join(errors.New("x"), llm.ErrDisabled),
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(nil)
			r.gen.err = cause
			_, err := r.svc.Transform(context.Background(), input())
			if kind, reason := reasonOf(t, err); kind != apperr.KindUnavailable || reason != "" {
				t.Fatalf("kind = %v reason = %q", kind, reason)
			}
			if len(r.quota.refunds) != 1 || r.gen.calls != 1 {
				t.Fatalf("refunds = %v calls = %d", r.quota.refunds, r.gen.calls)
			}
			if rel, ok := r.limiter.TryAcquire(); !ok {
				t.Fatal("失敗後名額要釋放")
			} else {
				rel()
			}
		})
	}
}

func TestCallerCancellationStillRefunds(t *testing.T) {
	r := newRig(nil)
	r.gen.err = context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.svc.Transform(ctx, input())
	if !errors.Is(err, context.Canceled) || len(r.quota.refunds) != 1 {
		t.Fatalf("err = %v refunds = %v", err, r.quota.refunds)
	}
}
