package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// TestFinalizeCandidateSet_RejectsZeroRequestRevisionID 跟
// TestAppendRevision_RejectsEmptyRequestStatus 是同一個模式（見
// revision_validation_test.go 的完整說明）：這道檢查是 FinalizeCandidateSet 的第一行，
// 早於任何交易／連線操作，所以可以用 nil pool/txManager 建構 Repo 來測——不需要 Docker，
// 也因此刻意放在沒有 `integration` build tag 的檔案裡，是這個 guard 最快的回歸防線。
// 檢查若被誤刪，測試會在觸碰 nil txManager 的那一刻 panic，而不是安靜地跑進交易。
//
// 被擋下的行為本身是沉默的：RequestRevisionID 零值 0 會讓
// SetProcurementRequestStatusIfActiveRevision 的守衛條件永遠對不上任何真實的列，那句
// UPDATE 靜默 no-op，candidate_set 照樣落地終態，只有 request.status 永遠停在 MATCHING。
// 沒有 error、沒有 panic、沒有 log——正是最難從線上症狀反推回成因的那一種。
func TestFinalizeCandidateSet_RejectsZeroRequestRevisionID(t *testing.T) {
	repo := New(nil, nil, func() int64 { return 1 })

	wrote, finalStatus, failureReason, err := repo.FinalizeCandidateSet(context.Background(), FinalizeCandidateSetInput{
		CandidateSetID: 1, FencingToken: 1, RequestID: 1,
		// RequestRevisionID 刻意留 0。
		Platform: LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
		External: LaneResult{Status: codemeta.ProcurementLaneStatusSucceeded.Value},
	})

	require.ErrorIs(t, err, ErrMissingRequestRevisionID)
	require.False(t, wrote, "被輸入驗證擋下時什麼都沒寫，wrote 必須是 false")
	require.Empty(t, finalStatus,
		"什麼都沒寫時 finalStatus 必須是空字串——呼叫端拿它當 metric label，"+
			"回一個看似合法的終態會讓一次被擋下的呼叫在 procurement_run_total 上長出一筆假 run")
	require.Empty(t, failureReason,
		"什麼都沒寫時 failureReason 也必須是空字串——呼叫端拿它組 RUN_FAILED 的 "+
			"ledger payload，回一個原因等於對外宣稱這個 run 失敗過")
}
