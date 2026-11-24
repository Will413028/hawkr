package intake

import (
	"context"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

// fan-out 的排頁 job：同一頁尚未結束的合併；River 的唯一性選項要求 ByState 含 available、pending、running、scheduled（缺任何一個，Insert 就回
// 選項錯誤），Completed 排除（完成的頁不擋之後的重跑）。
func TestFanoutPageJobStatesFollowRiverRulesAndExcludeCompleted(t *testing.T) {
	t.Parallel()
	o := fanoutOpts().UniqueOpts
	if !o.ByArgs {
		t.Error("排頁 job 要以參數唯一")
	}
	for _, st := range []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
	} {
		if !slices.Contains(o.ByState, st) {
			t.Errorf("ByState 缺 %s", st)
		}
	}
	if slices.Contains(o.ByState, rivertype.JobStateCompleted) {
		t.Error("ByState 不能含 completed：完成的頁會擋住之後的重跑")
	}
}

// 用真的 River client 排入：選項驗證在碰資料庫之前，連不上資料庫的錯誤代表驗證已通過；假的 enqueuer 會吞掉選項錯誤。
func TestRiverAcceptsTheFanoutInsertOpts(t *testing.T) {
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
	_, err = c.Insert(context.Background(), FanoutArgs{IntentID: uuid.NewV7()}, fanoutOpts())
	if err != nil && (strings.Contains(err.Error(), "UniqueOpts") || strings.Contains(err.Error(), "InsertOpts")) {
		t.Fatalf("River 拒絕選項：%v", err)
	}
}
