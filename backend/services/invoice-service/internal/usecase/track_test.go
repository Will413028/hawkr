package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// INV.trk 的判定邏輯。fixture 的形狀取自 §4.5.10 的規格碼與 track_all 的巢狀說明
// （三層：1 財政部配給 → 2 給光貿用 → 3 發票字軌列表；自動配號＝layer 3 且 category 1），
// **不是**我對 AMEGO 回傳的想像。
// ---------------------------------------------------------------------------

func trk(layer, category int, start, end, now string, status, booklets int) amego.Track {
	return amego.Track{
		Layer: layer, Category: category,
		Start: start, End: end, Now: now,
		Status: status, TotalBooklet: booklets,
	}
}

func TestRemainingNumbers(t *testing.T) {
	cases := []struct {
		name string
		in   amego.Track
		want int64
	}{
		// 🔴 now == "" 是「尚未配過號」（§4.5.10 逐字「實測會出現」），整段都還沒用。
		// 少了這一格會把整段可用號碼算成 0-start ＝ 負數，於是新賣家一律被判為
		// 「字軌用盡」——而那正是最不該誤報的一群人（他們剛配好號、完全健康）。
		{"尚未配號 ⇒ 整段可用", trk(3, 1, "10000000", "10000049", "", 0, 1), 50},
		{"已配到一半", trk(3, 1, "10000000", "10000049", "10000020", 0, 1), 29},
		{"剛好用完", trk(3, 1, "10000000", "10000049", "10000049", 0, 1), 0},
	}
	for _, c := range cases {
		if got := remainingNumbers(c.in); got != c.want {
			t.Errorf("%s: remainingNumbers()=%d want %d", c.name, got, c.want)
		}
	}
}

func TestAutoAssignedTracks_OnlyLayer3Category1(t *testing.T) {
	roots := []amego.Track{{
		Layer: 1, Category: 1, // 財政部配給（總量），**不是**可用字軌
		Data: []amego.Track{{
			Layer: 2, Category: 1, // 給光貿用
			Data: []amego.Track{
				trk(3, 1, "10000000", "10000049", "", 0, 1), // ✅ 自動配號
				trk(3, 2, "20000000", "20000049", "", 0, 1), // ❌ category 2（非自動配號）
			},
		}},
	}}
	got := autoAssignedTracks(roots)
	if len(got) != 1 {
		t.Fatalf("應只挑出 1 段（layer 3 且 category 1），實得 %d 段", len(got))
	}
	if got[0].Start != "10000000" {
		t.Errorf("挑錯段：start=%s", got[0].Start)
	}
	// 🔴 這一格是關鍵：把頂層當清單用會拿到「財政部配給總量」，
	// 於是餘量被高估一個數量級，InvoiceTrackBalanceLow 永遠不會亮。
	for _, g := range got {
		if g.Layer != 3 {
			t.Errorf("挑到 layer %d 的列", g.Layer)
		}
	}
}

func TestExhausted(t *testing.T) {
	cases := []struct {
		name string
		in   amego.Track
		want bool
	}{
		{"還有餘量", trk(3, 1, "10000000", "10000049", "10000020", 0, 1), false},
		{"餘量 0", trk(3, 1, "10000000", "10000049", "10000049", 0, 1), true},
		// status 3 過期 / 9 用畢：即使算出來還有號，也開不了票（§8.2.3 M21 逐字）。
		{"status 3 過期（仍有餘量）", trk(3, 1, "10000000", "10000049", "10000000", 3, 1), true},
		{"status 9 用畢（仍有餘量）", trk(3, 1, "10000000", "10000049", "10000000", 9, 1), true},
	}
	for _, c := range cases {
		if got := exhausted(c.in); got != c.want {
			t.Errorf("%s: exhausted()=%v want %v", c.name, got, c.want)
		}
	}
}

// ---- RunOnce 的聚合 --------------------------------------------------------

type fakeAccounts struct {
	list []domain.ProviderAccount
	// 讓某一家的憑證解密失敗，驗「單一賣家失敗不中斷整輪」。
	failOn int64
}

func (f *fakeAccounts) ListActive(context.Context) ([]domain.ProviderAccount, error) {
	return f.list, nil
}

func (f *fakeAccounts) WithDecryptedCredential(_ context.Context, id int64, fn func(string, string) error) error {
	if id == f.failOn {
		return errors.New("credential decrypt failed")
	}
	return fn("12345678", "appkey")
}

// fakeScanner 的鑑別力來自它**拒絕什麼**：只認登記過的 (issuerID, year, period)，
// 其餘一律回 error。若 usecase 把期別算錯、或把本期當下期送，測試會失敗而不是靜默通過。
type fakeScanner struct {
	byKey map[[3]int64][]amego.Track
}

func (f *fakeScanner) ListTracks(_ context.Context, c amego.Credential, year, period int) ([]amego.Track, error) {
	v, ok := f.byKey[[3]int64{c.IssuerID, int64(year), int64(period)}]
	if !ok {
		return nil, errors.New("fakeScanner: 未登記的 (issuer,year,period) 組合")
	}
	return v, nil
}

type capturedScan struct {
	remainingMin, low, exhausted, nextInsufficient int64
	calls                                          int
}

func (c *capturedScan) ObserveTrackScan(a, b, d, e int64) {
	c.remainingMin, c.low, c.exhausted, c.nextInsufficient = a, b, d, e
	c.calls++
}

func nested(inner ...amego.Track) []amego.Track {
	return []amego.Track{{Layer: 1, Category: 1, Data: []amego.Track{{Layer: 2, Category: 1, Data: inner}}}}
}

func TestTrack_RunOnce_Aggregates(t *testing.T) {
	// 2026-08-20（台北）⇒ 本期 = 2026 期別 3（07-08月）、下期 = 2026 期別 4（09-10月）
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	accounts := &fakeAccounts{
		list: []domain.ProviderAccount{
			{ID: 1, IssuerID: 101}, // 健康：餘量 500
			{ID: 2, IssuerID: 102}, // 低於門檻 200：餘量 50
			{ID: 3, IssuerID: 103}, // 已用盡
			{ID: 4, IssuerID: 104}, // 憑證失敗 ⇒ 不計入任何一格
		},
		failOn: 4,
	}
	sc := &fakeScanner{byKey: map[[3]int64][]amego.Track{
		// issuer 101：本期還剩 500，下期 20 本 ×50 = 1000 ≥ 已用 0 ×1.5 ⇒ 足夠
		{101, 2026, 3}: nested(trk(3, 1, "10000000", "10000499", "", 0, 10)),
		{101, 2026, 4}: nested(trk(3, 1, "20000000", "20000999", "", 0, 20)),
		// issuer 102：本期剩 50（低於 200），下期 1 本 ×50 = 50 < 已用 450 ×1.5 ⇒ 不足
		{102, 2026, 3}: nested(trk(3, 1, "10001000", "10001499", "10001449", 0, 10)),
		{102, 2026, 4}: nested(trk(3, 1, "20001000", "20001049", "", 0, 1)),
		// issuer 103：用盡
		{103, 2026, 3}: nested(trk(3, 1, "10002000", "10002499", "10002499", 0, 10)),
		{103, 2026, 4}: nested(trk(3, 1, "20002000", "20002999", "", 0, 20)),
	}}
	obs := &capturedScan{}

	uc := NewTrack(accounts, sc, obs, &fakePromoter{}, zap.NewNop(), func() time.Time { return now }, 200, 0.5)
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if obs.calls != 1 {
		t.Fatalf("ObserveTrackScan 應被呼叫一次，實得 %d", obs.calls)
	}
	// 最小值取自成功量到的三家：500 / 50 / 0 ⇒ 0
	if obs.remainingMin != 0 {
		t.Errorf("remainingMin=%d want 0", obs.remainingMin)
	}
	// 低於 200 的：102（50）與 103（0）
	if obs.low != 2 {
		t.Errorf("lowSellers=%d want 2", obs.low)
	}
	// 用盡的：103
	if obs.exhausted != 1 {
		t.Errorf("exhaustedSellers=%d want 1", obs.exhausted)
	}
	// 下期不足的：102
	if obs.nextInsufficient != 1 {
		t.Errorf("nextPeriodInsufficient=%d want 1", obs.nextInsufficient)
	}
	// 🔴 issuer 104 憑證失敗，四個計數器都不該把它算進去——把它當成「餘量 0」
	// 會產生假的 InvoiceTrackExhausted，當成「充足」會遮住真問題。
	// low=2 而不是 3 就是這件事的證據。
}

// ---------------------------------------------------------------------------
// J33：字軌餘量恢復 ⇒ 把該 issuer 卡在 SELLER_ACTION_REQUIRED 的票推回 READY
//
// #3089 A 組第 3 項：這條邊在 2026-09-04 之前**零執行者**，而 CodeMeta 對該狀態的
// description 逐字寫著「由 INV.trk 的 RunOnce 前置步驟 promote 回 READY」。
// 加上 J34 的讀端閘門也壞著（同票第 8 項），那個狀態的兩個出口都不通。
// ---------------------------------------------------------------------------

// fakePromoter 的鑑別力來自它**記下打給誰**：只斷言「有沒有被呼叫」的替身，
// 對「打到不該打的賣家」完全沒有反應——而 J33 最危險的失誤正是漏掉 issuer 述詞
// （一個賣家配到字軌，把全平台卡住的票一起放行）。
type fakePromoter struct {
	rows   map[int64]int64
	called []int64
	nows   []time.Time
}

func (f *fakePromoter) PromoteSellerActionRequired(_ context.Context, issuerID int64, now time.Time) (int64, error) {
	f.called = append(f.called, issuerID)
	f.nows = append(f.nows, now)
	return f.rows[issuerID], nil
}

func TestTrack_RunOnce_PromotesOnlyRecoveredSellers(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	accounts := &fakeAccounts{
		list: []domain.ProviderAccount{
			{ID: 1, IssuerID: 101}, // 餘量 500 ⇒ 該 promote
			{ID: 2, IssuerID: 102}, // 餘量 50（低於門檻但仍 > 0）⇒ 仍該 promote
			{ID: 3, IssuerID: 103}, // 用盡 ⇒ 不該 promote
			{ID: 4, IssuerID: 104}, // 憑證失敗、量不到 ⇒ 不該 promote
		},
		failOn: 4,
	}
	sc := &fakeScanner{byKey: map[[3]int64][]amego.Track{
		{101, 2026, 3}: nested(trk(3, 1, "10000000", "10000499", "", 0, 10)),
		{101, 2026, 4}: nested(trk(3, 1, "20000000", "20000999", "", 0, 20)),
		{102, 2026, 3}: nested(trk(3, 1, "10001000", "10001499", "10001449", 0, 10)),
		{102, 2026, 4}: nested(trk(3, 1, "20001000", "20001049", "", 0, 1)),
		{103, 2026, 3}: nested(trk(3, 1, "10002000", "10002499", "10002499", 0, 10)),
		{103, 2026, 4}: nested(trk(3, 1, "20002000", "20002999", "", 0, 20)),
	}}
	pr := &fakePromoter{rows: map[int64]int64{101: 2, 102: 1}}

	uc := NewTrack(accounts, sc, &capturedScan{}, pr, zap.NewNop(), func() time.Time { return now }, 200, 0.5)
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// 🔴 斷言的是**精確集合**不是「有被呼叫」：後者對「多打了 103／104」毫無反應。
	want := []int64{101, 102}
	if len(pr.called) != len(want) {
		t.Fatalf("J33 應只對餘量恢復的賣家發動，期望 %v，實得 %v", want, pr.called)
	}
	for i, v := range want {
		if pr.called[i] != v {
			t.Fatalf("J33 打到的 issuer 不對，期望 %v，實得 %v", want, pr.called)
		}
	}
	// 用盡（103）與量不到（104）都不得被 promote——後者尤其重要：
	// 「量不到」不等於「量到 0」，也不等於「量到有餘量」（memory feedback_no_blind_measurement）。
	for _, v := range pr.called {
		if v == 103 || v == 104 {
			t.Errorf("issuer %d 不該被 promote（用盡／掃描失敗）", v)
		}
	}
	for _, n := range pr.nows {
		if !n.Equal(now) {
			t.Errorf("J33 的 now 應取本輪掃描的時鐘，期望 %v，實得 %v", now, n)
		}
	}
}

// TestTrack_RunOnce_NilPromoterIsTolerated 守的是「依賴沒接上時不要 panic」——
// 既有測試與 DI 之外的呼叫端會傳 nil（同 advanceAdjustmentForAdmin 的慣例）。
func TestTrack_RunOnce_NilPromoterIsTolerated(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	accounts := &fakeAccounts{list: []domain.ProviderAccount{{ID: 1, IssuerID: 101}}}
	sc := &fakeScanner{byKey: map[[3]int64][]amego.Track{
		{101, 2026, 3}: nested(trk(3, 1, "10000000", "10000499", "", 0, 10)),
		{101, 2026, 4}: nested(trk(3, 1, "20000000", "20000999", "", 0, 20)),
	}}
	uc := NewTrack(accounts, sc, &capturedScan{}, nil, zap.NewNop(), func() time.Time { return now }, 200, 0.5)
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce with nil promoter: %v", err)
	}
}
