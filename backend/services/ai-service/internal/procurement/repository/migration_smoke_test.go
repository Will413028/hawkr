//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMigration_ProcurementQuoteTables_Dropped 原本是 Task 1 的 smoke test，證的是
// 20260814121016_add_procurement_quote_tables.sql 套得乾淨、三張表建得出來。
// 那三張表已隨 #2543 C3 刀 4 的 20260917064637_drop_legacy_quote_tables DROP 掉，
// 所以兩段斷言現在都是「必須不存在」。
// 它不驗任何 repository 行為（本來就沒有可呼叫的東西）。
//
// 第二段原本斷言 procurement_requests 有 resolved_crop_code。那個欄位已由
// 20260904230000_move_crop_resolution_into_slots.sql 移除（解析結果搬進
// slots_json，ADR 2026-09-04-crop-resolution-into-slots），斷言因此**反轉**：
// 五個解析欄位必須全部不存在。這不是把測試改到通過——它現在守的是「那支 drop
// migration 真的跑過了」，而那正是這個檔案原本在守的東西（schema 到位與否），
// 只是到位的定義變了。
//
// 這也是這支 migration 唯一會被真實 Postgres 執行到的地方：backfill 的
// jsonb_build_object／jsonb_strip_nulls 語法有錯的話，setupProcurementTestDB
// 套用 migration 時就會失敗，本測試連斷言都跑不到。
func TestMigration_ProcurementQuoteTables_Dropped(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	// 三張 legacy quote 表必須**不存在**——它們已隨 #2543 C3 刀 4 的
	// 20260917064637_drop_legacy_quote_tables DROP 掉。
	//
	// 這一條原本斷言的是「三張新表都應該存在」（它們當年是這支 migration smoke
	// test 的主角）。翻面而不是刪掉：這個檔守的是「schema 到位與否」，到位的
	// 定義變了、守的東西沒變。刪掉的話，某天有人把那三張表加回來就沒有東西會響。
	var legacyCount int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name IN ('procurement_quote_batches', 'procurement_quote_invites', 'procurement_quote_outbox')
	`).Scan(&legacyCount)
	require.NoError(t, err)
	require.Zero(t, legacyCount, "三張 legacy quote 表必須已被 20260917064637 drop")

	var droppedCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'procurement_requests'
		  AND column_name IN (
			'resolved_crop_code', 'resolved_group_code', 'resolved_fallback_group_code',
			'crop_resolution_self_priced', 'crop_resolution_self_priced_price_per_kg')
	`).Scan(&droppedCount)
	require.NoError(t, err)
	require.Zero(t, droppedCount, "五個解析欄位必須已被 20260904230000 移除——解析結果現在住 slots_json")

	// 待處理狀態的兩欄也已離開這張表（20260905030000，刀 2）：它們描述「現在正在
	// 問什麼」，生命週期是那個中斷的生命週期，不是需求內容。
	var pendingCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'procurement_requests'
		  AND column_name IN ('crop_disambiguation_options_json', 'crop_resolution_pending_reason')
	`).Scan(&pendingCount)
	require.NoError(t, err)
	require.Zero(t, pendingCount, "待處理狀態必須已搬進 procurement_agent_run_interrupts.payload_json")

	// 搬去的目的地：中斷列上的 payload_json（NOT NULL，預設空物件）與 facts_as_of。
	var interruptCols int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'procurement_agent_run_interrupts'
		  AND column_name IN ('payload_json', 'facts_as_of')
	`).Scan(&interruptCols)
	require.NoError(t, err)
	require.Equal(t, 2, interruptCols)

	var payloadNullable string
	err = pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'procurement_agent_run_interrupts' AND column_name = 'payload_json'
	`).Scan(&payloadNullable)
	require.NoError(t, err)
	require.Equal(t, "NO", payloadNullable,
		"payload_json 是 NOT NULL——讀取端因此永遠 unmarshal 得動，不需要每一處都先判 nil")
}
