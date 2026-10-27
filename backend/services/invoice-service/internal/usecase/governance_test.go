package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// TestCrossPeriodWindow 釘住 §8.2.3 M20 的窗口判定。
//
// 規格逐字：「條件是**台北時間的日**（單月 3–5 日 ⇒ closing、6 日起至下一期開始
// ⇒ closed、1–2 日 ⇒ open）」。
//
// 🔴 為什麼判定不能放進 PromQL（規格自己寫的理由）：PromQL 沒有時區安全的日曆
// 函式，寫成 hour／day_of_month 會用 UTC 算，**每兩個月的邊界日剛好錯 8 小時**
// ——而那正是本指標唯一有意義的那幾天。下面兩格 UTC 月底的 case 就是在守這件事。
func TestCrossPeriodWindow(t *testing.T) {
	utc := func(s string) time.Time {
		ts, err := time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			t.Fatalf("bad fixture %q: %v", s, err)
		}
		return ts
	}
	cases := []struct {
		name string
		at   string
		want CrossPeriodWindow
	}{
		{"單月 1 日 → open", "2026-09-01T04:00:00Z", WindowOpen},
		{"單月 2 日 → open", "2026-09-02T04:00:00Z", WindowOpen},
		{"單月 3 日 → closing", "2026-09-03T04:00:00Z", WindowClosing},
		{"單月 5 日 → closing", "2026-09-05T04:00:00Z", WindowClosing},
		{"單月 6 日 → closed", "2026-09-06T04:00:00Z", WindowClosed},
		{"單月 20 日 → closed", "2026-09-20T04:00:00Z", WindowClosed},
		// 雙月一律 closed：上一期的關帳日早就過了。
		{"雙月 1 日 → closed（不是 open）", "2026-10-01T04:00:00Z", WindowClosed},
		{"雙月 4 日 → closed（不是 closing）", "2026-10-04T04:00:00Z", WindowClosed},
		// 🔴 UTC 08-31 17:00 ＝ 台北 09-01 01:00 ⇒ 單月 1 日 ⇒ open。
		// 用 UTC 判定會得到「8 月 31 日」＝雙月 ⇒ closed，**整個 open 窗口消失**。
		{"UTC 08-31T17:00 ＝ 台北 09-01 → open", "2026-08-31T17:00:00Z", WindowOpen},
		// 同理右邊界：UTC 09-05 17:00 ＝ 台北 09-06 01:00 ⇒ 已 closed。
		{"UTC 09-05T17:00 ＝ 台北 09-06 → closed", "2026-09-05T17:00:00Z", WindowClosed},
	}
	for _, c := range cases {
		at := utc(c.at)
		if got := crossPeriodWindow(at); got != c.want {
			t.Errorf("%s: got=%s want=%s（台北 %s）", c.name, got, c.want,
				at.In(timeutil.LocationTaipei).Format("2006-01-02 15:04"))
		}
	}
}

// ---------------------------------------------------------------------------
// 行事曆鏡像每日同步（§5.1.14；#3089 第 17 項）
//
// 這一組在 2026-09-04 之前一支都不存在——`BusinessDayRepository` 的兩個寫入方法
// 零呼叫端，鏡像自建表以來一列都沒有，於是每一筆申訴的 SLA 都退到 14 自然日的
// 硬上限。它的退化**完全靜默**（ComplaintSlaDueAt 恆有回傳值），所以測試的重點
// 不在「同步成功」，在「失敗時不要把東西弄壞」。
// ---------------------------------------------------------------------------

type fakeHolidaySource struct {
	rows []time.Time
	err  error
	// from／to 記下呼叫端要的區間——它必須與 BusinessCalendar 宣稱涵蓋的區間一致，
	// 窄一天就會讓那一天被當成假日而沒有任何東西轉紅。
	from, to time.Time
	calls    int
}

func (f *fakeHolidaySource) GetHolidays(_ context.Context, from, to time.Time) ([]time.Time, error) {
	f.calls++
	f.from, f.to = from, to
	return f.rows, f.err
}

type fakeBusinessDayRepo struct {
	domain.BusinessDayRepository

	deletes              int
	inserted             []time.Time
	syncedAt             []time.Time
	deleteErr, insertErr error
}

func (f *fakeBusinessDayRepo) DeleteAll(context.Context) (int64, error) {
	f.deletes++
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	n := int64(len(f.inserted))
	f.inserted = nil
	f.syncedAt = nil
	return n, nil
}

func (f *fakeBusinessDayRepo) Insert(_ context.Context, holidayDate, syncedAt time.Time) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, holidayDate)
	f.syncedAt = append(f.syncedAt, syncedAt)
	return nil
}

// govFixture 只裝行事曆同步要用到的那幾樣；C4 與 M20 兩件事各自回空。
type govFixture struct {
	uc     *Governance
	source *fakeHolidaySource
	mirror *fakeBusinessDayRepo
}

func newGovFixture(source *fakeHolidaySource, mirror *fakeBusinessDayRepo, now time.Time) *govFixture {
	uc := NewGovernance(
		noopGovStore{}, noopCrossPeriodCounter{}, noopGovObserver{},
		mirror, source, directTx{}, nil, func() time.Time { return now },
	)
	return &govFixture{uc: uc, source: source, mirror: mirror}
}

type noopGovStore struct{}

func (noopGovStore) ExpirePending(context.Context, time.Time, time.Time, int32) ([]domain.SellerTaxProfile, error) {
	return nil, nil
}

func (noopGovStore) CountApprovalRecheckDue(context.Context, time.Time) (int64, error) { return 0, nil }

type noopCrossPeriodCounter struct{}

func (noopCrossPeriodCounter) CountCrossPeriodPending(context.Context, time.Time) (int64, error) {
	return 0, nil
}

type noopGovObserver struct{}

func (noopGovObserver) ObserveCrossPeriodPending(CrossPeriodWindow, int64) {}
func (noopGovObserver) ObserveApprovalRecheckDue(int64)                    {}

// govRetentionFixture 是 §5.4.6 兩支清理的替身（#3261 ①②）。
//
// 🔴 **記下 cutoff 不只記次數**：保留期算錯（90d 寫成 90 分鐘、或正負號相反）
// 只有 cutoff 抓得到，而算錯的後果是把還在保留期內的 PII／事件提前清掉。
type govRetentionFixture struct {
	purgeCalls   int
	purgeCutoff  time.Time
	purgeN       int64
	purgeErr     error
	deleteCalls  int
	deleteCutoff time.Time
	deleteN      int64
	deleteErr    error
}

func (f *govRetentionFixture) PurgeProcessedPayloads(_ context.Context, _, cutoff time.Time, _ int32) (int64, error) {
	f.purgeCalls++
	f.purgeCutoff = cutoff
	return f.purgeN, f.purgeErr
}

func (f *govRetentionFixture) DeleteDoneBefore(_ context.Context, cutoff time.Time, _ int32) (int64, error) {
	f.deleteCalls++
	f.deleteCutoff = cutoff
	return f.deleteN, f.deleteErr
}

// #3261 ①②③：INV.gov 每日 tick 要帶跑保留期清理與屆期待複審計數。
//
// 🔴 三者在 2026-09-08 之前都零呼叫端：inbox 的 payload 永久保留（含 buyer_id／
// store_id／金額）、outbox 的 done 列永不刪除、而「屆期待複審」連 metric 都不存在。
func TestGovernanceRunOnce_RunsRetentionAndRecheckCount(t *testing.T) {
	now := govNow(t)
	ret := &govRetentionFixture{purgeN: 3, deleteN: 7}
	obs := &recordingGovObserver{}
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, obs, nil, nil,
		directTx{}, zap.NewNop(), func() time.Time { return now }).
		WithRetention(ret, ret)

	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if ret.purgeCalls != 1 || ret.deleteCalls != 1 {
		t.Fatalf("兩支清理都該跑到，實得 purge=%d delete=%d", ret.purgeCalls, ret.deleteCalls)
	}
	// 🔴 保留期硬寫，**不引用 inboxPayloadRetention／outboxDoneRetention**：
	// 期望值取自被測物的常數等於「它改成多少都會過」。90／30 天的來源是設計書 §5.4.6。
	if want := now.Add(-90 * 24 * time.Hour); !ret.purgeCutoff.Equal(want) {
		t.Fatalf("inbox cutoff = %s，want %s（90 天）", ret.purgeCutoff, want)
	}
	if want := now.Add(-30 * 24 * time.Hour); !ret.deleteCutoff.Equal(want) {
		t.Fatalf("outbox cutoff = %s，want %s（30 天）", ret.deleteCutoff, want)
	}
	if !obs.recheckSet {
		t.Fatal("屆期待複審家數必須每一輪都 Set（0 也要），否則 Prometheus 分不出「是 0」與「沒更新」")
	}
}

// 清理失敗不得讓 RunOnce 回錯誤 —— 它們是盡力而為的維護工作，而同一個 tick 還帶著
// 行事曆同步與跨期計數。失敗記 log、下一輪再試。
func TestGovernanceRunOnce_RetentionFailureDoesNotFailTheTick(t *testing.T) {
	now := govNow(t)
	ret := &govRetentionFixture{purgeErr: errors.New("boom"), deleteErr: errors.New("boom")}
	obs := &recordingGovObserver{}
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, obs, nil, nil,
		directTx{}, zap.NewNop(), func() time.Time { return now }).
		WithRetention(ret, ret)

	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("清理失敗不該讓整個 tick 失敗：%v", err)
	}
	if ret.purgeCalls != 1 || ret.deleteCalls != 1 {
		t.Fatalf("兩支都該被嘗試，實得 purge=%d delete=%d", ret.purgeCalls, ret.deleteCalls)
	}
}

// 沒 wire 清理時安靜跳過（本地 wiring 缺件），不得 panic。
func TestGovernanceRunOnce_NilRetentionIsSkipped(t *testing.T) {
	now := govNow(t)
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, &recordingGovObserver{}, nil, nil,
		directTx{}, zap.NewNop(), func() time.Time { return now })
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("未掛清理時應照常跑完：%v", err)
	}
}

// recordingGovObserver 記下屆期待複審有沒有被 Set。
type recordingGovObserver struct {
	recheckSet   bool
	recheckCount int64
}

func (o *recordingGovObserver) ObserveCrossPeriodPending(CrossPeriodWindow, int64) {}
func (o *recordingGovObserver) ObserveApprovalRecheckDue(count int64) {
	o.recheckSet = true
	o.recheckCount = count
}

func govNow(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, "2026-09-04T04:00:00Z")
	if err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return ts
}

func TestGovernance_CalendarSync_ReplacesMirror(t *testing.T) {
	now := govNow(t)
	d1 := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	fx := newGovFixture(&fakeHolidaySource{rows: []time.Time{d1, d2}}, &fakeBusinessDayRepo{}, now)

	if err := fx.uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce 失敗：%v", err)
	}
	if fx.mirror.deletes != 1 {
		t.Fatalf("整段覆蓋要先 DeleteAll 一次，實得 %d", fx.mirror.deletes)
	}
	if len(fx.mirror.inserted) != 2 {
		t.Fatalf("應寫入 2 列，實得 %d", len(fx.mirror.inserted))
	}
	for i, s := range fx.mirror.syncedAt {
		if !s.Equal(now) {
			t.Fatalf("第 %d 列的 synced_at 應為同一個 now（整段覆蓋 ⇒ 同批同值），實得 %s", i, s)
		}
	}

	// 🔴 同步窗口必須與 NewBusinessCalendar 宣稱涵蓋的區間逐日對齊。
	//
	// 期望值**硬寫**、不呼叫 BusinessDayMirrorWindow：拿被測函式的回傳值當期望值
	// 是自己跟自己對，常數改壞照樣全綠（2026-09-04 實測 +400 → +399 時 exit=0）。
	// now=2026-09-04T04:00Z ＝ 台北 09-04 12:00 ⇒ 台北日界 09-04 00:00
	// ⇒ −30 天 ＝ 08-05、+400 天 ＝ 2027-10-09（台北時間，UTC 為前一日 16:00）。
	wantFrom := time.Date(2026, 8, 5, 0, 0, 0, 0, timeutil.LocationTaipei)
	wantTo := time.Date(2027, 10, 9, 0, 0, 0, 0, timeutil.LocationTaipei)
	if !fx.source.from.Equal(wantFrom) || !fx.source.to.Equal(wantTo) {
		t.Fatalf("同步區間 [%s, %s]，want [%s, %s]",
			fx.source.from.In(timeutil.LocationTaipei), fx.source.to.In(timeutil.LocationTaipei),
			wantFrom, wantTo)
	}
}

// 🔴 本組最重要的一支：RPC 失敗**不得動到鏡像**。
//
// 先 DeleteAll 再打 RPC 的實作會讓一次網路抖動把整張表清空，而空表 = 零涵蓋
// = 每一筆申訴的 SLA 都落在 14 天硬上限。昨天的行事曆比空表準得多。
func TestGovernance_CalendarSync_KeepsMirrorWhenFetchFails(t *testing.T) {
	now := govNow(t)
	mirror := &fakeBusinessDayRepo{inserted: []time.Time{time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}}
	fx := newGovFixture(&fakeHolidaySource{err: errors.New("boom")}, mirror, now)

	if err := fx.uc.RunOnce(context.Background()); err == nil {
		t.Fatal("RPC 失敗必須讓 RunOnce 回 error（排程器那層要 log 得到）")
	}
	if mirror.deletes != 0 {
		t.Fatalf("RPC 失敗時不得 DeleteAll，實得 %d 次", mirror.deletes)
	}
	if len(mirror.inserted) != 1 {
		t.Fatalf("舊鏡像必須原封不動，實得 %d 列", len(mirror.inserted))
	}
}

// 區間內沒有假日是合法結果（例如只涵蓋暑假的一段），照樣整段覆蓋成空。
// 它與「RPC 失敗」的差別正是上一支要守的東西。
func TestGovernance_CalendarSync_EmptyResultStillReplaces(t *testing.T) {
	now := govNow(t)
	mirror := &fakeBusinessDayRepo{inserted: []time.Time{time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}
	fx := newGovFixture(&fakeHolidaySource{rows: nil}, mirror, now)

	if err := fx.uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("空清單不是錯誤：%v", err)
	}
	if mirror.deletes != 1 || len(mirror.inserted) != 0 {
		t.Fatalf("空清單應整段覆蓋成空，實得 deletes=%d rows=%d", mirror.deletes, len(mirror.inserted))
	}
}

// 依賴缺席（本地開發起不了 shipping-service）時整段跳過，其餘治理工作照跑。
func TestGovernance_CalendarSync_SkipsWhenDependenciesMissing(t *testing.T) {
	now := govNow(t)
	uc := NewGovernance(noopGovStore{}, noopCrossPeriodCounter{}, noopGovObserver{},
		nil, nil, nil, nil, func() time.Time { return now })
	if err := uc.RunOnce(context.Background()); err != nil {
		t.Fatalf("缺依賴時應跳過而不是失敗：%v", err)
	}
}
