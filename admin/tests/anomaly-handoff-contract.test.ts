import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	EMPTY_ANOMALY_HANDOFF,
	parseAnomalyHandoff,
} from "../src/features/delegated-operations/anomaly-handoff";

/**
 * 代行工作區 → 平台異常處置的交接（產品需求 §6.7）。
 *
 * 上游是 `hawkr-website` 的 `delegatedAnomalyUrl`，它組出
 * `/admin/anomalies?tab=orders&store_id=…&order_no=…&status=…`。**鍵名是跨 repo
 * 契約**：任何一個對不上，這裡照樣 200、照樣渲染，只是帶過來的訂單完全沒有落點，
 * 而畫面上看不出來（與 website `delegated-adm-rail-audit-entry.test.ts` 修掉的死
 * fragment 是同一種壞法）。兩邊各釘一支測試。
 */

const PAGE = "src/app/admin/anomalies/page.tsx";

function source(path: string): string {
	return readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
}

function q(search: string): URLSearchParams {
	return new URLSearchParams(search);
}

/* ============================================================
   一、parser
   ============================================================ */

test("四個鍵逐字讀進來", () => {
	assert.deepEqual(
		parseAnomalyHandoff(
			q("tab=orders&store_id=2685914682&order_no=HK260728001&status=35"),
		),
		{
			tab: "orders",
			storeId: "2685914682",
			orderNo: "HK260728001",
			status: "35",
		},
	);
});

test("沒有參數就是一張乾淨的異常監控頁", () => {
	assert.deepEqual(parseAnomalyHandoff(q("")), EMPTY_ANOMALY_HANDOFF);
	assert.deepEqual(parseAnomalyHandoff(null), EMPTY_ANOMALY_HANDOFF);
	assert.deepEqual(parseAnomalyHandoff(undefined), EMPTY_ANOMALY_HANDOFF);
});

test("tab 只收五個合法值，其餘退回 null 讓頁面用自己的預設", () => {
	for (const key of ["shipping", "orders", "payments", "refunds", "grants"]) {
		assert.equal(parseAnomalyHandoff(q(`tab=${key}`)).tab, key);
	}
	// 猜一個近似值（例如把 orders 寫成 order）會落在一個操作者沒有要求的 tab 上。
	for (const bad of ["order", "ORDERS", "", "../"]) {
		assert.equal(
			parseAnomalyHandoff(q(`tab=${encodeURIComponent(bad)}`)).tab,
			null,
		);
	}
});

test("status 只收純數字 —— 它會原樣送進後端的 status 參數", () => {
	assert.equal(parseAnomalyHandoff(q("status=35")).status, "35");
	for (const bad of ["35;drop", "abc", "35,40", " "]) {
		assert.equal(
			parseAnomalyHandoff(q(`status=${encodeURIComponent(bad)}`)).status,
			null,
			`status=${bad} 不該被接受`,
		);
	}
});

test("空白字串與純空白一律當成沒帶", () => {
	const parsed = parseAnomalyHandoff(q("store_id=%20&order_no="));
	assert.equal(parsed.storeId, null);
	assert.equal(parsed.orderNo, null);
});

/* ============================================================
   二、頁面真的用了它（parser 對了但沒接線＝白做）
   ============================================================ */

test("異常監控頁把交接參數接進 tab、查詢與畫面", () => {
	const text = source(PAGE);
	assert.match(
		text,
		/parseAnomalyHandoff\(searchParams\)/u,
		"頁面沒有解析交接參數 ⇒ 連結帶什麼過來都一樣",
	);
	assert.match(
		text,
		/useState<TabKey>\(handoff\.tab \?\? "shipping"\)/u,
		"tab 沒有被套用 ⇒ 從訂單抽屜點過來會落在配送異常那一頁",
	);
	assert.match(
		text,
		/if \(storeId\) params\.set\("store_id", storeId\);/u,
		"store_id 沒有進 server 查詢 ⇒ 落在全平台的異常清單上，等於沒帶上下文",
	);
	assert.match(
		text,
		/useState\(handoff\.status \?\? "35"\)/u,
		"status 沒有被套用 ⇒ 後端預設只回 35，從付款異常的訂單點過來會查無此單",
	);
	assert.match(
		text,
		/handoff\.orderNo && r\.orderNo === handoff\.orderNo/u,
		"帶了訂單編號卻不標出來，操作者仍要一列一列對",
	);
});

test("交接橫幅只在真的帶了訂單時出現", () => {
	assert.match(
		source(PAGE),
		/\{handoff\.orderNo \? \(/u,
		"沒帶訂單時也掛一條「自代行工作區帶入」＝對著空氣宣告一個沒發生的事",
	);
});
