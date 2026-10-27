package amego

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTimeSource struct {
	unix atomic.Int64
	err  error
	// calls 記錄 Resync 打了幾次。
	calls atomic.Int64
}

func (f *fakeTimeSource) ServerTime(context.Context) (time.Time, error) {
	f.calls.Add(1)
	if f.err != nil {
		return time.Time{}, f.err
	}
	return time.Unix(f.unix.Load(), 0).UTC(), nil
}

// TestClockZeroOffsetBehavesLikeNoLayer：offset 正常時恆為 0
// ⇒ 行為與沒有這一層完全相同。
func TestClockZeroOffsetBehavesLikeNoLayer(t *testing.T) {
	t.Parallel()
	base := time.Unix(1787577174, 0).UTC()
	c := newClockWithSource(func() time.Time { return base })
	if got := c.Unix(); got != base.Unix() {
		t.Fatalf("Unix = %d, want %d", got, base.Unix())
	}
	if c.OffsetSeconds() != 0 {
		t.Fatalf("OffsetSeconds = %d, want 0", c.OffsetSeconds())
	}
}

// TestClockResyncCompensatesOffset：pod 偏移超過 ±60 秒就全線 code 15
// ⇒ 補償後送出的 time 必須落回伺服器時間。
func TestClockResyncCompensatesOffset(t *testing.T) {
	t.Parallel()
	local := time.Unix(1787577174, 0).UTC() // 本機時間
	c := newClockWithSource(func() time.Time { return local })
	src := &fakeTimeSource{}
	src.unix.Store(local.Unix() + 300) // 伺服器快 5 分鐘

	if err := c.Resync(context.Background(), src); err != nil {
		t.Fatalf("Resync: %v", err)
	}
	if got := c.OffsetSeconds(); got != 300 {
		t.Fatalf("OffsetSeconds = %d, want 300", got)
	}
	if got := c.Unix(); got != local.Unix()+300 {
		t.Fatalf("補償後 Unix = %d, want %d", got, local.Unix()+300)
	}
}

// TestClockResyncFailureKeepsOldOffset。
//
// 🔴 失敗時保留舊 offset、**不歸零**：歸零等於宣稱「本機時間是對的」，
// 而我們剛剛才失去確認它的能力。採持續補償而非啟動時 fail-closed 的理由同源——
// 一次上游校時抖動不該讓整個服務起不來，而法定存證期限的時鐘還在跑。
func TestClockResyncFailureKeepsOldOffset(t *testing.T) {
	t.Parallel()
	local := time.Unix(1787577174, 0).UTC()
	c := newClockWithSource(func() time.Time { return local })
	good := &fakeTimeSource{}
	good.unix.Store(local.Unix() + 120)
	if err := c.Resync(context.Background(), good); err != nil {
		t.Fatalf("Resync: %v", err)
	}

	bad := &fakeTimeSource{err: errors.New("upstream down")}
	if err := c.Resync(context.Background(), bad); err == nil {
		t.Fatal("Resync 失敗必須回錯（呼叫端才知道要告警）")
	}
	if got := c.OffsetSeconds(); got != 120 {
		t.Fatalf("失敗後 OffsetSeconds = %d, want 120（舊值再壞也比在無資訊下歸零好）", got)
	}
}

// TestClientClockIsResyncableFromServerTime：校時來源是免簽章、免 Credential 的
// GET /json/time ⇒ 校時本身不會因憑證問題失敗。
func TestClientClockIsResyncableFromServerTime(t *testing.T) {
	t.Parallel()
	serverUnix := time.Now().Unix() + 240
	h := newHarness(t, Options{AllowRealInvoice: true}, func(url.Values) (int, string, string) {
		return 200, "text/plain", `{"timestamp":` + strconv.FormatInt(serverUnix, 10) + `}`
	})
	if err := h.client.Clock().Resync(context.Background(), h.client); err != nil {
		t.Fatalf("Resync: %v", err)
	}
	if off := h.client.Clock().OffsetSeconds(); off < 235 || off > 245 {
		t.Fatalf("OffsetSeconds = %d, want ≈240", off)
	}
}
