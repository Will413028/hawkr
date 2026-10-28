//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/stretchr/testify/require"
)

func TestRepo_ItemResolution_RoundTripsThroughSlotsJSON(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	item := "高麗菜"
	resolution := &domain.ItemResolution{
		Kind: domain.ItemResolvedCategoryConfirmed, Label: "高麗菜", CategoryID: "17",
		Provenance: domain.ItemResolutionByBuyerChoice,
	}
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID,
		Slots:  domain.Slots{Item: &item, ItemResolution: resolution},
		Source: "ANSWER", ClientRequestID: "cr-item-resolution-roundtrip", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	_, rev, err := repo.GetSnapshot(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	var slots domain.Slots
	require.NoError(t, json.Unmarshal(rev.SlotsJSON, &slots))
	require.Equal(t, resolution, slots.ItemResolution)

	// 真的存成 jsonb：用 jsonb 運算子直接取巢狀欄位。裸 []byte 被當成 bytea 送出時
	// 這一段會失敗，而 Go 端的 Unmarshal 仍然會過——所以這個斷言不是重複。
	var kind, provenance string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT slots_json -> 'item_resolution' ->> 'kind',
		        slots_json -> 'item_resolution' ->> 'provenance'
		   FROM procurement_request_revisions WHERE id = $1`, rev.ID,
	).Scan(&kind, &provenance))
	require.Equal(t, string(domain.ItemResolvedCategoryConfirmed), kind)
	require.Equal(t, string(domain.ItemResolutionByBuyerChoice), provenance)
}

// TestRepo_SlotsWithoutItemResolution_OmitsTheKey 釘住「兄弟 key 而非巢狀」這個決定
// 在 DB 這一端也成立：沒有解析結果時 slots_json 不得出現 item_resolution 這個 key。
// 出現的話（omitempty 掉了）既有 revision 的 slots_hash 會全體漂移。
func TestRepo_SlotsWithoutItemResolution_OmitsTheKey(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	item := "高麗菜"
	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{Item: &item},
		Source: "ANSWER", ClientRequestID: "cr-no-item-resolution", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	_, rev, err := repo.GetSnapshot(ctx, req.ID, testBuyerID)
	require.NoError(t, err)

	var hasKey bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT slots_json ? 'item_resolution' FROM procurement_request_revisions WHERE id = $1`, rev.ID,
	).Scan(&hasKey))
	require.False(t, hasKey, "沒有解析結果時不得寫入 item_resolution key")
}
