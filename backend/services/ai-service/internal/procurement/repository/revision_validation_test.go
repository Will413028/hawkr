package repository

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/stretchr/testify/require"
)

// TestAppendRevision_RejectsEmptyRequestStatus 不需要真的 DB：ErrMissingRequestStatus
// 的檢查是 AppendRevision 的第一行，早於任何交易／連線操作（見 revision.go），所以
// 可以直接用 nil pool/txManager 建構 Repo 來測——若這個檢查被誤刪，測試會在觸碰到
// nil txManager 的那一刻 panic，而不是安靜地跑進交易，把
// procurement_requests.status 洗成空字串。這個測試特意放在沒有 `integration` build
// tag 的檔案：不用 Docker/Postgres 就能跑，是這個 guard 最快的回歸防線。
func TestAppendRevision_RejectsEmptyRequestStatus(t *testing.T) {
	repo := New(nil, nil, func() int64 { return 1 })

	_, err := repo.AppendRevision(context.Background(), AppendInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "cr-1",
		// RequestStatus 刻意留空。
	})

	require.ErrorIs(t, err, ErrMissingRequestStatus)
}
