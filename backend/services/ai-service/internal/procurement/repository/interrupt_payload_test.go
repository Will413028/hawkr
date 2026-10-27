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

// TestRepo_CreateInterrupt_RoundTripsPayloadAndFactsAsOf 對真實 Postgres 驗證
// 「這個中斷正在問什麼」跟中斷本體同一次 INSERT 落地、也讀得回來。
//
// 這是刀 2 的核心結構性主張的釘子：待答狀態住 procurement_requests 的時代，
// 「候選落庫」與「建立中斷」是兩次各自 log-and-swallow 的寫入，因此存在兩個不
// 對稱的中間狀態（有卡片沒中斷／有中斷沒白名單）。合成一次 INSERT 之後這兩個
// 狀態結構上不存在——而「結構上不存在」的前提是這一次 INSERT 真的兩者都寫進去
// 了，那正是這支測試唯一要證明的事。
func TestRepo_CreateInterrupt_RoundTripsPayloadAndFactsAsOf(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	rev, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-interrupt-payload",
		PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: testBuyerID, ClientRequestID: "cr-interrupt-payload",
		CommandKind: "SUBMIT_TURN", PayloadHash: "h", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindGraph, RequestID: req.ID,
	})
	require.NoError(t, err)
	_, err = repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: testBuyerID, Version: run.Version,
		Status: domain.RunStatusWaiting(), ResultRevisionID: rev.ID,
		TerminalEventType: "CONFIRMATION_REQUIRED", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	// 逐字比照 usecase.CropDisambiguationInterruptPayload 的 wire 形狀。這一層
	// 刻意不 import usecase（依賴方向是 usecase → repository），所以用同形狀的
	// 匿名結構——欄位名對不上的話下面的反序列化斷言會抓到。
	factsAsOf := time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(map[string]any{
		"kind":          "crop_disambiguation",
		"pendingReason": string(domain.CropPendingReasonAmbiguous),
		"candidates": []map[string]any{
			{"label": "甘藍-初秋", "cropCode": "LA1", "fallbackGroupCode": "moa-group:abc"},
		},
		"factsAsOf": factsAsOf,
	})
	require.NoError(t, err)

	created, err := repo.CreateInterrupt(ctx, CreateInterruptInput{
		RunID: run.ID, InterruptKind: "CLARIFICATION", ResponseSchemaVersion: "v1",
		PayloadJSON: payload, FactsAsOf: factsAsOf,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	got, ok, err := repo.GetOpenInterruptForRun(ctx, run.ID, testBuyerID)
	require.NoError(t, err)
	require.True(t, ok, "剛建立的中斷必須查得到")
	require.Equal(t, created.ID, got.ID)
	require.True(t, factsAsOf.Equal(got.FactsAsOf),
		"facts_as_of 必須以 UTC 原值往返，不被 server local timezone 改寫")

	var decoded struct {
		Kind          string                            `json:"kind"`
		PendingReason domain.CropPendingReason          `json:"pendingReason"`
		Candidates    []domain.CropDisambiguationOption `json:"candidates"`
	}
	require.NoError(t, json.Unmarshal(got.PayloadJSON, &decoded))
	require.Equal(t, "crop_disambiguation", decoded.Kind)
	require.Equal(t, domain.CropPendingReasonAmbiguous, decoded.PendingReason,
		"待處理原因必須是顯式欄位——0 候選的 UNAVAILABLE 靠候選數量推不出來")
	require.Len(t, decoded.Candidates, 1)
	require.Equal(t, "LA1", *decoded.Candidates[0].CropCode)
}

// TestRepo_CreateInterrupt_WithoutPayload_StoresEmptyObjectNotNull 釘住
// payload_json 的 NOT NULL 契約：沒有 payload 的 kind 存空物件而不是 SQL NULL，
// 讀取端因此永遠 unmarshal 得動，不需要每一處都先判 nil。
//
// 空物件同時是白名單的 fail-closed 狀態：它解得開，但 kind 不是
// crop_disambiguation，resume 的 ResolveInput 因此拒絕它——一個 payload 沒寫成功
// 的 CLARIFICATION 中斷不會變成「買家可以自填任意品項與價格」的缺口。
func TestRepo_CreateInterrupt_WithoutPayload_StoresEmptyObjectNotNull(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)
	rev, err := repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: testBuyerID, Slots: domain.Slots{},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-interrupt-empty",
		PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	run, _, err := repo.ClaimRun(ctx, ClaimRunInput{
		BuyerID: testBuyerID, ClientRequestID: "cr-interrupt-empty",
		CommandKind: "SUBMIT_TURN", PayloadHash: "h", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindGraph, RequestID: req.ID,
	})
	require.NoError(t, err)
	_, err = repo.TerminalizeRun(ctx, TerminalizeRunInput{
		RunID: run.ID, BuyerID: testBuyerID, Version: run.Version,
		Status: domain.RunStatusWaiting(), ResultRevisionID: rev.ID,
		TerminalEventType: "CONFIRMATION_REQUIRED", PublicPayloadJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	created, err := repo.CreateInterrupt(ctx, CreateInterruptInput{
		RunID: run.ID, InterruptKind: "QUOTE_APPROVAL", ResponseSchemaVersion: "v1",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(created.PayloadJSON))
	require.True(t, created.FactsAsOf.IsZero(), "沒有外部事實的 kind，facts_as_of 是 SQL NULL")

	var isNull bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payload_json IS NULL FROM procurement_agent_run_interrupts WHERE id = $1`,
		created.ID).Scan(&isNull))
	require.False(t, isNull, "payload_json 是 NOT NULL，未提供時存 '{}' 不是 NULL")
}

// TestMigrationBackfill_MovesPendingStateOntoOpenInterrupts 補
// 20260905030000 的 backfill 在 testcontainers 上結構性測不到的那一半：空的 DB
// 讓那支 UPDATE 匹配 0 列，SQL 語法被驗證了（語法錯的話 migration 當場失敗），
// 但它**產出的 payload 形狀**沒有。
//
// 跟刀 1 的同型測試一樣，把 migration 裡逐字相同的 jsonb_build_object 拿出來餵
// 真實的舊欄位組合，再反序列化回讀取端真正使用的形狀。形狀對不上時 Go 端不會
// 報錯（未知 key 直接忽略），只會得到一個 Kind 為空的 payload——而那會讓 resume
// 的白名單檢查把買家所有的答案都擋掉，沒有任何錯誤訊息指向 backfill。
func TestMigrationBackfill_MovesPendingStateOntoOpenInterrupts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	// 逐字取自 20260905030000_move_crop_pending_state_into_interrupts.sql，
	// 欄位改成參數。這份重複是刻意的：migration 已經套用完、來源欄位也 DROP 了，
	// 沒有辦法再從 DB 那一側觸發它一次。
	const backfillExpr = `
		SELECT jsonb_build_object(
			'kind', 'crop_disambiguation',
			'pendingReason', coalesce($1::text, 'AMBIGUOUS'),
			'candidates', coalesce($2::jsonb, '[]'::jsonb))::text`

	cases := []struct {
		name              string
		pendingReason     *string
		optionsJSON       *string
		wantReason        domain.CropPendingReason
		wantCandidateCode string
		wantCandidates    int
	}{
		{
			name:          "AMBIGUOUS＋候選清單",
			pendingReason: ptrStr("AMBIGUOUS"),
			optionsJSON: ptrStr(`[{"label":"甘藍-初秋","cropCode":"LA1",` +
				`"fallbackGroupCode":"moa-group:abc"}]`),
			wantReason: domain.CropPendingReasonAmbiguous, wantCandidates: 1, wantCandidateCode: "LA1",
		},
		{
			name:          "UNAVAILABLE＋零候選（Cut 1 起刻意不落 options）",
			pendingReason: ptrStr("UNAVAILABLE"),
			wantReason:    domain.CropPendingReasonUnavailable, wantCandidates: 0,
		},
		{
			// Cut 1 部署當下的在途 request：pending_reason 還沒回填，options 已經
			// 是舊碼寫入的候選清單。migration 的 coalesce 預設值就是為了它。
			name: "相容窗：只有 options、沒有 pending_reason",
			optionsJSON: ptrStr(`[{"label":"整個甘藍都可以","groupCode":"moa-group:abc",` +
				`"fallbackGroupCode":"moa-group:abc"}]`),
			wantReason: domain.CropPendingReasonAmbiguous, wantCandidates: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw string
			require.NoError(t, pool.QueryRow(ctx, backfillExpr, tc.pendingReason, tc.optionsJSON).Scan(&raw))

			var decoded struct {
				Kind          string                            `json:"kind"`
				PendingReason domain.CropPendingReason          `json:"pendingReason"`
				Candidates    []domain.CropDisambiguationOption `json:"candidates"`
			}
			require.NoError(t, json.Unmarshal([]byte(raw), &decoded), "backfill 產出的 JSON 必須解得開")

			require.Equal(t, "crop_disambiguation", decoded.Kind,
				"kind 必須落成消歧 payload——resume 的白名單檢查認的就是這個值，錯了會把買家所有答案擋掉")
			require.Equal(t, tc.wantReason, decoded.PendingReason)
			require.Len(t, decoded.Candidates, tc.wantCandidates)
			if tc.wantCandidateCode != "" {
				require.NotNil(t, decoded.Candidates[0].CropCode)
				require.Equal(t, tc.wantCandidateCode, *decoded.Candidates[0].CropCode)
			}
		})
	}
}

func ptrStr(s string) *string { return &s }
