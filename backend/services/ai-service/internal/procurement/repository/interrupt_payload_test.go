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
			{"label": "高麗菜", "categoryId": "17", "categoryLevel": 3},
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
	require.Equal(t, "17", decoded.Candidates[0].CategoryID)
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
