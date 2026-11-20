package purge

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

func hasState(by []rivertype.JobState, want rivertype.JobState) bool {
	for _, st := range by {
		if st == want {
			return true
		}
	}
	return false
}

// River 要求 ByState 含 available、pending、running、scheduled；少了任何一個排入就失敗，連同排入的 transaction 一起回滾。
func TestPurgeUniquenessHasRiverRequiredStates(t *testing.T) {
	t.Parallel()
	by := (PurgeAuthorArgs{}).InsertOpts().UniqueOpts.ByState
	for _, st := range []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
	} {
		if !hasState(by, st) {
			t.Fatalf("ByState 缺 %s", st)
		}
	}
}

// 用真的 River client 排入：選項驗證在碰資料庫之前，連不上資料庫的錯誤代表驗證已通過。
func TestRiverAcceptsInsertOpts(t *testing.T) {
	t.Parallel()
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/none?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string]river.JobArgs{"purge": PurgeAuthorArgs{BuyerID: 1}, "sweep": SweepArgs{}} {
		_, err := c.Insert(context.Background(), args, nil)
		if err != nil && (strings.Contains(err.Error(), "UniqueOpts") || strings.Contains(err.Error(), "InsertOpts")) {
			t.Fatalf("%s：River 拒絕排入選項：%v", name, err)
		}
		if err == nil {
			t.Fatalf("%s：預期連不上資料庫", name)
		}
	}
}

// 保底掃描的唯一性以週期為時間窗，不看已完成的 job，否則完成後約一天內掃描排不進去。
func TestSweepUniquenessIsPeriodWindow(t *testing.T) {
	t.Parallel()
	u := (SweepArgs{}).InsertOpts().UniqueOpts
	if u.ByPeriod != SweepInterval {
		t.Fatalf("ByPeriod = %v", u.ByPeriod)
	}
	if u.ByArgs || len(u.ByState) != 0 {
		t.Fatal("掃描的唯一性不該依參數或狀態")
	}
}
