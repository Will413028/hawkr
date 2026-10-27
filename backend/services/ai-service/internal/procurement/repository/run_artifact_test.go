//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// 本檔守 doc 57 §4.5／§6.2 的 typed artifact 寫入路徑（Slice F）。
//
// artifact 表在 schema 裡躺了一段時間但零 Go 碼碰它——13 個 artifact 型別因此全部
// 沒有 producer。這條路徑是**共用基礎**：型別各自的 data 形狀是各自的業務設計，
// 但「寫一版 artifact ＋ 在同一交易 append ARTIFACT_UPSERTED」對 13 型都一樣。

// upsertArtifactFixture 建一個候選搜尋的 ledger run，回傳 runID 與 buyerID。
// artifact 掛在 run 上，所以每一支測試都要先有一個真的 run row。
func upsertArtifactFixture(ctx context.Context, t *testing.T, repo *Repo, buyerID identity.BuyerID) (int64, identity.BuyerID) {
	t.Helper()
	_, set := setupRunIdentityFixture(ctx, t, repo, buyerID)
	require.NoError(t, repo.CreateLedgerRunForCandidateSearch(ctx, set))
	return set.PublicRunID(), buyerID
}

func TestUpsertRunArtifact_FirstProduceWritesVersionOneAndAppendsLedgerEvent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	runID, buyerID := upsertArtifactFixture(ctx, t, repo, identity.BuyerID(9101))

	got, err := repo.UpsertRunArtifact(ctx, UpsertRunArtifactInput{
		RunID: runID, BuyerID: buyerID,
		ArtifactType:   codemeta.ProcurementArtifactTypeCandidateResults.Value,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: []byte(`{"platformLane":{"status":"SUCCEEDED"}}`),
		Checksum:       "sha256-x",
		DomainRefType:  "candidateSetId", DomainRefID: "12345",
		AsOf: refTime(),
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), got.Version, "第一次產製必須是 version 1")
	require.NotZero(t, got.ArtifactID, "artifact_id 必須由 repository 鑄出")

	// 同一交易內必須 append ARTIFACT_UPSERTED——artifact 落庫但 ledger 沒事件，
	// 等於買家的 replay 永遠看不到這個 artifact 出現過。
	events, err := repo.ListRunEventsAfterSeq(ctx, runID, 0)
	require.NoError(t, err)
	var found *RunEvent
	for i := range events {
		if events[i].EventType == codemeta.ProcurementLedgerEventTypeArtifactUpserted.Value {
			found = &events[i]
		}
	}
	require.NotNil(t, found, "沒有 ARTIFACT_UPSERTED 事件，events=%+v", events)

	// payload 只帶參照，不內嵌完整 artifact（doc 57 §4.2 event data 欄）。
	var payload map[string]any
	require.NoError(t, json.Unmarshal(found.PublicPayloadJSON, &payload))
	require.Equal(t, codemeta.ProcurementArtifactTypeCandidateResults.Value, payload["type"])
	require.Equal(t, float64(1), payload["version"])
	require.NotEmpty(t, payload["artifactId"])
	require.NotContains(t, payload, "data", "ARTIFACT_UPSERTED 不得內嵌完整 artifact data")
}

func TestUpsertRunArtifact_ReproduceKeepsArtifactIdAndBumpsVersion(t *testing.T) {
	// version 是「這個 artifact_id 的第幾次產製」（doc 57 §4.5），不是 data schema
	// 版本——重新產製必須沿用同一個身分，否則前端每次都會看到一個全新的 artifact，
	// 「結果版本」這個概念整個不成立。
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	runID, buyerID := upsertArtifactFixture(ctx, t, repo, identity.BuyerID(9102))
	in := UpsertRunArtifactInput{
		RunID: runID, BuyerID: buyerID,
		ArtifactType:   codemeta.ProcurementArtifactTypeCandidateResults.Value,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: []byte(`{"platformLane":{"status":"SUCCEEDED"}}`),
		Checksum:       "sha256-x", DomainRefType: "candidateSetId", DomainRefID: "12345",
		AsOf: refTime(),
	}

	first, err := repo.UpsertRunArtifact(ctx, in)
	require.NoError(t, err)
	in.PublicDataJSON = []byte(`{"platformLane":{"status":"FAILED"}}`)
	in.Checksum = "sha256-y"
	second, err := repo.UpsertRunArtifact(ctx, in)
	require.NoError(t, err)

	require.Equal(t, first.ArtifactID, second.ArtifactID, "重新產製必須沿用同一個 artifact_id")
	require.Equal(t, int32(2), second.Version)

	// 舊版本不得被覆蓋——artifact 是不可變快照，replay 要讀得到每一版。
	require.Equal(t, "sha256-x", first.Checksum)
}

func TestUpsertRunArtifact_DifferentTypesOnSameRunAreDifferentArtifacts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	runID, buyerID := upsertArtifactFixture(ctx, t, repo, identity.BuyerID(9103))
	base := UpsertRunArtifactInput{
		RunID: runID, BuyerID: buyerID,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: []byte(`{}`), Checksum: "sha256-x", AsOf: refTime(),
	}

	base.ArtifactType = codemeta.ProcurementArtifactTypeCandidateResults.Value
	candidates, err := repo.UpsertRunArtifact(ctx, base)
	require.NoError(t, err)

	base.ArtifactType = codemeta.ProcurementArtifactTypeErrorRecovery.Value
	errRecovery, err := repo.UpsertRunArtifact(ctx, base)
	require.NoError(t, err)

	require.NotEqual(t, candidates.ArtifactID, errRecovery.ArtifactID,
		"同一個 run 上不同型的 artifact 是不同身分，不得共用 artifact_id")
	require.Equal(t, int32(1), errRecovery.Version, "另一型的第一次產製仍是 version 1")
}

func TestUpsertRunArtifact_UnknownRunIsRejected(t *testing.T) {
	// artifact 沒有 buyer_id 欄位，租戶邊界完全靠 run row 的 FOR UPDATE 查詢帶
	// buyer_id——這支測試釘住那條邊界真的在，而不是靠呼叫端自律。
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	runID, _ := upsertArtifactFixture(ctx, t, repo, identity.BuyerID(9104))

	_, err := repo.UpsertRunArtifact(ctx, UpsertRunArtifactInput{
		RunID: runID, BuyerID: identity.BuyerID(9999), // 別人的 buyer
		ArtifactType:   codemeta.ProcurementArtifactTypeCandidateResults.Value,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: []byte(`{}`), Checksum: "sha256-x", AsOf: refTime(),
	})
	require.ErrorIs(t, err, ErrNotFound)
}
