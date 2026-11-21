package jobs

import (
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// UnfinishedStates 是 River 唯一鍵要涵蓋的「還沒結束」的狀態。running 不能排除（River 的規定）；不含 completed：完成的 job 在
// River 清掉之前會擋下之後同一個鍵的排入。
func UnfinishedStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}
}

// UniqueByArgs 回傳「同一組參數、尚未結束的 job 合併」的 InsertOpts。maxAttempts 為 0 時用 River 的預設。
func UniqueByArgs(maxAttempts int) river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: maxAttempts,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: UnfinishedStates()},
	}
}

// SweepOpts 回傳「同一個 interval 內的排程與重啟合併」的 InsertOpts。參數固定的週期 job 用 ByPeriod，不用 ByArgs 加預設
// ByState：後者讓完成的列在 River 清掉前持續擋住之後的排入。呼叫端需要 MaxAttempts 時在回傳值上設定。
func SweepOpts(interval time.Duration) river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: interval}}
}
