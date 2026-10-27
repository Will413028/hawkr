//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	code, fallback := "LA1", "LA"
	resolution := &domain.ItemResolution{
		Kind: domain.ItemResolvedCrop, Code: &code, Label: "甘藍-初秋",
		FallbackGroupCode: &fallback, Provenance: domain.ItemResolutionByBuyerChoice,
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
	require.Equal(t, string(domain.ItemResolvedCrop), kind)
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

// TestMigrationBackfill_ProducesDecodableItemResolution 補一個 migration
// smoke test 結構上測不到的缺口：testcontainers 的 DB 是空的，
// 20260904230000 的 backfill UPDATE 因此匹配 0 列——SQL **語法**被驗證了
// （語法錯的話 migration 套用當場就失敗），但它**產出的 JSON 形狀**沒有。
//
// 這裡把那支 migration 裡逐字相同的 CASE 運算式拿出來，餵三種真實的舊欄位組合，
// 再把結果反序列化成 domain.ItemResolution——形狀對不上的話這裡會抓到。
//
// 為什麼形狀錯了很難從別處發現：Go 端反序列化不會失敗（未知 key 直接忽略），
// 只會得到一個 Kind 為空的 ItemResolution，而那在讀取端一律被當成「尚未解析」
// ——買家的報價按鈕安靜地不出現，沒有任何錯誤訊息指向 backfill。
func TestMigrationBackfill_ProducesDecodableItemResolution(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	// 逐字取自 20260904230000_move_crop_resolution_into_slots.sql 的 CASE 運算式，
	// 欄位改成參數。這份重複是刻意的：migration 已經套用完、欄位也 DROP 了，沒有
	// 辦法再從 DB 那一側觸發它一次。
	const backfillExpr = `
		SELECT CASE
			WHEN $1::boolean THEN
				jsonb_strip_nulls(jsonb_build_object(
					'kind', 'SELF_PRICED', 'price_per_kg', $2::bigint, 'provenance', 'SYSTEM'))
			WHEN $3::text IS NOT NULL THEN
				jsonb_strip_nulls(jsonb_build_object(
					'kind', 'CROP', 'code', $3::text,
					'fallback_group_code', $5::text, 'provenance', 'SYSTEM'))
			ELSE
				jsonb_strip_nulls(jsonb_build_object(
					'kind', 'GROUP', 'code', $4::text,
					'fallback_group_code', $5::text, 'provenance', 'SYSTEM'))
		END::text`

	cases := []struct {
		name                                   string
		selfPriced                             bool
		price                                  *int64
		cropCode, groupCode, fallbackGroupCode *string
		want                                   domain.ItemResolution
	}{
		{
			name: "crop 命中（含 fallback）", cropCode: ptr("LA1"), fallbackGroupCode: ptr("LA"),
			want: domain.ItemResolution{
				Kind: domain.ItemResolvedCrop, Code: ptr("LA1"),
				FallbackGroupCode: ptr("LA"), Provenance: domain.ItemResolutionBySystem,
			},
		},
		{
			name: "group 命中", groupCode: ptr("LA"), fallbackGroupCode: ptr("LA"),
			want: domain.ItemResolution{
				Kind: domain.ItemResolvedGroup, Code: ptr("LA"),
				FallbackGroupCode: ptr("LA"), Provenance: domain.ItemResolutionBySystem,
			},
		},
		{
			name: "自填價", selfPriced: true, price: ptr(int64(150)),
			want: domain.ItemResolution{
				Kind: domain.ItemResolvedSelfPriced, PricePerKg: ptr(150),
				Provenance: domain.ItemResolutionBySystem,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw string
			require.NoError(t, pool.QueryRow(ctx, backfillExpr,
				tc.selfPriced, tc.price, tc.cropCode, tc.groupCode, tc.fallbackGroupCode,
			).Scan(&raw))

			var got domain.ItemResolution
			require.NoError(t, json.Unmarshal([]byte(raw), &got))
			require.Equal(t, tc.want, got)

			// 搬過去的值必須是 Slots.Apply 認得的合法解析結果——backfill 產出一筆
			// domain 層自己會拒絕的資料，等於把不一致寫進 DB。
			item := "高麗菜"
			after, rejected := domain.Slots{Item: &item}.Apply(
				domain.SlotPatch{Item: &item, ItemResolution: &got}, time.Now())
			require.Empty(t, rejected, "backfill 產出的解析結果必須通過 domain 驗證")
			require.Equal(t, &got, after.ItemResolution)
		})
	}
}
