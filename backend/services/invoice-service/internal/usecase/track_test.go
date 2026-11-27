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
		Layer: 1, Category: 1,
		Data: []amego.Track{{
			Layer: 2, Category: 1,
			Data: []amego.Track{
				trk(3, 1, "10000000", "10000049", "", 0, 1),
				trk(3, 2, "20000000", "20000049", "", 0, 1),
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
		{"status 3 過期（仍有餘量）", trk(3, 1, "10000000", "10000049", "10000000", 3, 1), true},
		{"status 9 用畢（仍有餘量）", trk(3, 1, "10000000", "10000049", "10000000", 9, 1), true},
	}
	for _, c := range cases {
		if got := exhausted(c.in); got != c.want {
			t.Errorf("%s: exhausted()=%v want %v", c.name, got, c.want)
		}
	}
}


type fakeAccounts struct {
	list []domain.ProviderAccount
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
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	accounts := &fakeAccounts{
		list: []domain.ProviderAccount{
			{ID: 1, IssuerID: 101},
			{ID: 2, IssuerID: 102},
			{ID: 3, IssuerID: 103},
			{ID: 4, IssuerID: 104},
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
	obs := &capturedScan{}

	uc := NewTrack(accounts, sc, obs, &fakePromoter{}, zap.NewNop(), func() time.Time { return now }, 200, 0.5)
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if obs.calls != 1 {
		t.Fatalf("ObserveTrackScan 應被呼叫一次，實得 %d", obs.calls)
	}
	if obs.remainingMin != 0 {
		t.Errorf("remainingMin=%d want 0", obs.remainingMin)
	}
	if obs.low != 2 {
		t.Errorf("lowSellers=%d want 2", obs.low)
	}
	if obs.exhausted != 1 {
		t.Errorf("exhaustedSellers=%d want 1", obs.exhausted)
	}
	if obs.nextInsufficient != 1 {
		t.Errorf("nextPeriodInsufficient=%d want 1", obs.nextInsufficient)
	}
}


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
			{ID: 1, IssuerID: 101},
			{ID: 2, IssuerID: 102},
			{ID: 3, IssuerID: 103},
			{ID: 4, IssuerID: 104},
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

	want := []int64{101, 102}
	if len(pr.called) != len(want) {
		t.Fatalf("J33 應只對餘量恢復的賣家發動，期望 %v，實得 %v", want, pr.called)
	}
	for i, v := range want {
		if pr.called[i] != v {
			t.Fatalf("J33 打到的 issuer 不對，期望 %v，實得 %v", want, pr.called)
		}
	}
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
