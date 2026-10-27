import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { RealtimeSyncNotice } from "../features/seller-center/orders/RealtimeSyncNotice";

/**
 * 訂單頁丟掉串流的連線狀態，SSE 被
 * 429 回絕 / 重連預算耗盡時賣家沒有任何提示，只會看到一個安靜地不再更新的訂單
 * 列。以下兩層釘住修復：狀態列本身在「收不到更新」時真的渲染得出來，以及頁面
 * 確實有把連線狀態（`useNotificationStreamStatus`）接出來並渲染（丟掉它時，
 * 第二個測試會紅）。
 */

test("realtime 不可用（429 / 重連預算耗盡）時 UI 有提示", () => {
	const html = renderToStaticMarkup(
		React.createElement(RealtimeSyncNotice, { status: "unavailable" }),
	);
	assert.ok(html.includes("即時更新已暫停"), "應告知賣家即時更新已停止");
	assert.ok(html.includes("重新整理"), "應給出可執行的下一步");
	assert.ok(html.includes('role="status"'), "應以狀態語意播報而非錯誤語意");
});

test("離線時同樣有提示，且措辭不叫使用者做沒用的重新整理", () => {
	const html = renderToStaticMarkup(
		React.createElement(RealtimeSyncNotice, { status: "offline" }),
	);
	assert.ok(html.includes("即時更新已暫停"));
	assert.ok(html.includes("網路連線中斷"));
});

test("連線正常 / 過渡狀態不顯示任何提示", () => {
	for (const status of ["connected", "connecting", "idle"] as const) {
		assert.equal(
			renderToStaticMarkup(React.createElement(RealtimeSyncNotice, { status })),
			"",
			`${status} 不該出現狀態列`,
		);
	}
});

test("SellerOrdersPage 消費 realtime 狀態並渲染狀態列", () => {
	const source = readFileSync(
		new URL(
			"../features/seller-center/orders/SellerOrdersPage.tsx",
			import.meta.url,
		),
		"utf8",
	);
	assert.match(
		source,
		/realtimeStatus\s*=\s*useNotificationStreamStatus\(\)/,
		"串流連線狀態必須被接出來，不能丟棄",
	);
	assert.match(
		source,
		/<RealtimeSyncNotice\s+status={realtimeStatus}\s*\/>/,
		"接出來的狀態必須真的渲染成使用者看得到的提示",
	);
});
