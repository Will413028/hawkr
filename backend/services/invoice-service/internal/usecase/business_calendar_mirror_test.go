package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// mirrorRepo 是會交易回滾的記憶體鏡像：mirrorRollbackTx 在 fn 失敗時把狀態還原成進交易前。
type mirrorState struct {
	holidays []time.Time
	coverage *domain.BusinessDayCoverage
}

type mirrorRepo struct {
	domain.BusinessDayRepository
	state     mirrorState
	insertErr error
	deletes   int
}

func (r *mirrorRepo) DeleteAll(context.Context) (int64, error) {
	r.deletes++
	n := int64(len(r.state.holidays))
	r.state.holidays = nil
	return n, nil
}

func (r *mirrorRepo) Insert(_ context.Context, d, _ time.Time) error {
	if r.insertErr != nil {
		return r.insertErr
	}
	r.state.holidays = append(r.state.holidays, d)
	return nil
}

func (r *mirrorRepo) SetCoverage(_ context.Context, from, to, syncedAt time.Time) error {
	r.state.coverage = &domain.BusinessDayCoverage{From: from, To: to, SyncedAt: syncedAt}
	return nil
}

type mirrorRollbackTx struct{ repo *mirrorRepo }

func (t mirrorRollbackTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	before := mirrorState{holidays: append([]time.Time(nil), t.repo.state.holidays...)}
	if t.repo.state.coverage != nil {
		c := *t.repo.state.coverage
		before.coverage = &c
	}
	if err := fn(ctx); err != nil {
		t.repo.state = before
		return err
	}
	return nil
}

func newMirrorUC(repo *mirrorRepo) *BusinessCalendarMirror {
	now := time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC)
	return NewBusinessCalendarMirror(repo, mirrorRollbackTx{repo}, nil, func() time.Time { return now })
}

func seededMirror() *mirrorRepo {
	return &mirrorRepo{state: mirrorState{
		holidays: []time.Time{time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)},
		coverage: &domain.BusinessDayCoverage{
			From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		},
	}}
}

func TestBusinessCalendarMirror_ReplaceStoresRangeAndHolidays(t *testing.T) {
	repo := seededMirror()
	n, err := newMirrorUC(repo).Replace(context.Background(), "2026-10-23", "2027-12-31",
		[]string{"2027-01-01", "2026-12-25", "2027-01-01"})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if n != 2 || len(repo.state.holidays) != 2 {
		t.Fatalf("重複的日期應視為一天：n=%d rows=%d", n, len(repo.state.holidays))
	}
	c := repo.state.coverage
	if c.From.Format("2006-01-02") != "2026-10-23" || c.To.Format("2006-01-02") != "2027-12-31" {
		t.Fatalf("涵蓋區間要原樣存下推送端給的，實得 %+v", *c)
	}
}

// 冪等：同一批重送，結果相同。
func TestBusinessCalendarMirror_ReplaceIsIdempotent(t *testing.T) {
	repo := seededMirror()
	uc := newMirrorUC(repo)
	for range 2 {
		if _, err := uc.Replace(context.Background(), "2026-10-23", "2026-12-31", []string{"2026-12-25"}); err != nil {
			t.Fatalf("Replace: %v", err)
		}
	}
	if len(repo.state.holidays) != 1 {
		t.Fatalf("重送不得累加：%d 列", len(repo.state.holidays))
	}
}

// 區間內沒有假日是合法的：整段覆蓋成空、涵蓋區間照存。
func TestBusinessCalendarMirror_EmptyHolidaysStillReplaces(t *testing.T) {
	repo := seededMirror()
	if _, err := newMirrorUC(repo).Replace(context.Background(), "2026-10-23", "2026-10-30", nil); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if len(repo.state.holidays) != 0 || repo.state.coverage.To.Format("2006-01-02") != "2026-10-30" {
		t.Fatalf("空清單應覆蓋成空並存下新區間：%+v", repo.state)
	}
}

// 🔴 任何一個日期不合就整批拒絕，舊鏡像原封不動（連 DeleteAll 都不能發生）。
func TestBusinessCalendarMirror_BadBatchRejectedWholeAndMirrorUntouched(t *testing.T) {
	cases := map[string]struct {
		from, to string
		holidays []string
	}{
		"holiday not a date":    {"2026-10-23", "2026-12-31", []string{"2026-12-25", "2026-13-01"}},
		"holiday format":        {"2026-10-23", "2026-12-31", []string{"20261225"}},
		"holiday before range":  {"2026-10-23", "2026-12-31", []string{"2026-10-22"}},
		"holiday after range":   {"2026-10-23", "2026-12-31", []string{"2027-01-01"}},
		"from after to":         {"2026-12-31", "2026-10-23", nil},
		"from unparsable":       {"2026/10/23", "2026-12-31", nil},
		"to unparsable":         {"2026-10-23", "", nil},
		"valid then bad second": {"2026-10-23", "2026-12-31", []string{"2026-11-01", "x"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			repo := seededMirror()
			_, err := newMirrorUC(repo).Replace(context.Background(), c.from, c.to, c.holidays)
			if !errors.Is(err, ErrInvalidBusinessDayMirror) {
				t.Fatalf("應回 ErrInvalidBusinessDayMirror，實得 %v", err)
			}
			if repo.deletes != 0 || len(repo.state.holidays) != 1 || repo.state.coverage.To.Format("2006-01-02") != "2026-12-31" {
				t.Fatalf("拒絕時不得動到鏡像：deletes=%d state=%+v", repo.deletes, repo.state)
			}
		})
	}
}

// 🔴 交易中途失敗：刪除與已寫入的假日全部回滾，涵蓋區間也不變（整段覆蓋是原子的）。
func TestBusinessCalendarMirror_InsertFailureRollsBackEverything(t *testing.T) {
	repo := seededMirror()
	repo.insertErr = errors.New("boom")
	_, err := newMirrorUC(repo).Replace(context.Background(), "2026-10-23", "2027-12-31", []string{"2026-12-25", "2027-01-01"})
	if err == nil || errors.Is(err, ErrInvalidBusinessDayMirror) {
		t.Fatalf("應回非驗證類的錯誤，實得 %v", err)
	}
	if len(repo.state.holidays) != 1 || repo.state.coverage.To.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("失敗後舊鏡像必須原封不動：%+v", repo.state)
	}
}
