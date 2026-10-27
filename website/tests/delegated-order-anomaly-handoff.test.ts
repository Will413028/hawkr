import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { delegatedAnomalyUrl } from "../features/delegated-operations/state/session";

/**
 * 產品需求 §6.7（畫面 07：訂單唯讀）規定訂單面有兩個「安全動作」，並明寫
 * 「平台異常處置會**離開代行工作區並重新驗治理權限**」。
 *
 * 這一組守的是其中**做得成的那一顆**與**跟著它的那半句文案**：
 *
 * - 「平台異常處置」的落點是 29 號的 admin 異常監控（56 號 §2.2 Out-of-scope #5 把
 *   「異常訂單處置」歸給它），所以工作區這一側只有一條跨 host 連結，零新端點。
 * - 固定告知文案的後半句「與處理平台異常」曾與那顆鈕一起被刪掉。文案與功能一起
 *   消失是誠實的，但結果是 **ADM 發現訂單異常時畫面上沒有任何出口**，只能結束代行、
 *   換身分、重新找那筆單。
 *
 * ⚠️ 「通知賣家」那一顆**維持不渲染**：56 號 §2.2 Out-of-scope #15 逐字列它為本期
 * 不做（唯讀路徑沒有可依附的 mutation ⇒ 第 10 個 capability 的形狀），且實查後端沒有
 * 任何可複用路徑（notification 只有對 buyer／seller／all 的 broadcast、IM 無 admin
 * 入口）。最後一條就是釘住這件事，避免它被「兩顆一起補上」順手做成一顆假鈕。
 */

const ADM = "https://adm.example.com";
const PANEL =
	"features/delegated-operations/orders/DelegatedOrderDetailPanel.tsx";
const LIST = "features/delegated-operations/orders/DelegatedOrdersPage.tsx";

function source(path: string): string {
	return readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
}

/**
 * 剝掉註解再斷言。這一頁的註解本來就在解釋「為什麼不畫通知賣家」，不剝掉的話
 * 那條否定斷言永遠是紅的，而剝錯方向（只剝 JSX 註解）又會讓它永遠是綠的。
 */
function jsxOf(path: string): string {
	return source(path)
		.replace(/\/\*[\s\S]*?\*\//gu, "")
		.replace(/^\s*\/\/.*$/gmu, "");
}

/* ============================================================
   一、連結本身（跨 repo 契約）
   ============================================================ */

test("四個 query 鍵逐字固定：tab / store_id / order_no / status", () => {
	const url = new URL(
		delegatedAnomalyUrl(ADM, {
			storeId: "2685914682",
			orderNo: "HK260728001",
			status: "35",
		}),
	);
	assert.equal(url.origin + url.pathname, `${ADM}/admin/anomalies`);
	assert.deepEqual(
		[...url.searchParams.entries()],
		[
			["tab", "orders"],
			["store_id", "2685914682"],
			["order_no", "HK260728001"],
			["status", "35"],
		],
	);
});

test("store_id 一定要帶得出去 —— 少了它就是全平台的異常清單", () => {
	// `GET /admin/orders/abnormal` 沒有 order_no 參數（gateway admin_order.go 的
	// @Param 清單），伺服器端唯一收得窄的就是 store_id。它掉了，這條連結等於沒帶
	// 任何上下文，操作者仍要自己從全平台的清單裡找那一筆。
	const url = new URL(
		delegatedAnomalyUrl(ADM, {
			storeId: "2685914682",
			orderNo: "HK260728001",
		}),
	);
	assert.equal(url.searchParams.get("store_id"), "2685914682");
});

test("空值不組成 store_id= / order_no= 的空參數", () => {
	for (const empty of [null, undefined, ""] as const) {
		const url = new URL(
			delegatedAnomalyUrl(ADM, { storeId: empty, orderNo: empty }),
		);
		assert.equal(url.searchParams.has("store_id"), false);
		assert.equal(url.searchParams.has("order_no"), false);
		// tab 永遠在：目的地是五個 tab 的頁面，不指定就落在配送異常那一個。
		assert.equal(url.searchParams.get("tab"), "orders");
	}
});

test("值一律編碼，不得直接串進 query", () => {
	const url = delegatedAnomalyUrl(ADM, { storeId: "a&b", orderNo: "c d#e" });
	assert.ok(url.includes("store_id=a%26b"), url);
	assert.ok(!url.includes("c d#e"), url);
	assert.equal(
		new URL(url).searchParams.get("order_no"),
		"c d#e",
		"編碼後要還原得回來",
	);
});

/* ============================================================
   二、抽屜底部：出口存在，而且它是連結不是動作
   ============================================================ */

test("訂單抽屜渲染平台異常處置的出口，且是 <a href> 不是按鈕", () => {
	const text = source(PANEL);
	assert.match(
		text,
		/<a\s+className="adm-order-handoff"/u,
		"抽屜底部沒有平台異常處置的出口 ⇒ ADM 發現異常時只能結束代行、換身分、重找那筆單",
	);
	assert.match(
		text,
		/href=\{delegatedAnomalyUrl\(admOrigin, \{/u,
		"連結必須由 delegatedAnomalyUrl 組 —— 手寫 query 就繞過了上面那組跨 repo 契約",
	);
	// §6.7 明寫它會「離開代行工作區」；沿用 AdmRail 的既有形狀：純連結，跨 host，
	// 不得產生任何 session 狀態變更（動到 revision 會讓操作者永久回不來）。
	assert.ok(
		!/<button[^>]*adm-order-handoff/u.test(text),
		"做成 button 就會有 onClick，而這條路上任何 session 狀態變更都會讓人回不來",
	);
});

test("拿不到 storeId 就整顆不渲染", () => {
	assert.match(
		source(PANEL),
		/\{summary\?\.target\.storeId \? \(/u,
		"少了 store_id 的連結會把人丟到全平台的異常清單，那與沒有這顆鈕一樣，只是多騙一次點擊",
	);
});

test("按鈕字面不得承諾「建立」 —— 目的地沒有那個動作", () => {
	// 29 號自陳「純查詢聚合層，不新增 DB 表、不新增業務動作」；異常紀錄由
	// provider_status 推導，沒有任何「建立」端點。照抄 mockup 的字＝宣告一個不存在
	// 的功能，與同頁「查看權限說明」外連因目的地不存在而不渲染是同一條判準。
	const jsx = jsxOf(PANEL);
	assert.ok(
		!jsx.includes("建立平台異常紀錄"),
		"目的地是唯讀的異常監控，按不出「建立」這個動作",
	);
	assert.ok(jsx.includes("前往平台異常處置"));
	assert.ok(
		jsx.includes("將離開代行工作區"),
		"§6.7 的「會離開代行工作區並重新驗治理權限」要講給操作者聽，不能只寫在註解裡",
	);
});

/* ============================================================
   三、固定告知文案：與出口成對，不得再單方面砍半
   ============================================================ */

test("訂單頁的固定告知文案逐字等於 §6.7", () => {
	assert.ok(
		jsxOf(LIST).includes(
			"此區由賣家負責履約；ADM 僅可查看訂單與處理平台異常。",
		),
		"文案被砍成「僅可查看訂單」⇒ 畫面上不再有任何線索說異常有出口",
	);
});

/* ============================================================
   四、通知賣家：維持不渲染（56 號 §2.2 Out-of-scope #15）
   ============================================================ */

test("訂單抽屜不得出現「通知賣家」CTA，也不得做成 disabled 的假鈕", () => {
	assert.ok(
		!jsxOf(PANEL).includes("通知賣家"),
		[
			"訂單面唯讀，notifySeller 是依附於一筆 mutation 的旗標（§8.4 W5 只在 ",
			"result='SUCCEEDED' 時才建 outbox 列）—— 唯讀路徑沒有可依附的 mutation。",
			"後端也沒有任何可複用路徑：notification 只有 broadcast（對象是 buyer／seller／all ",
			"這個使用者類型），IM 沒有 admin 入口。要做＝第 10 個 capability，先改 56 號設計書。",
		].join(""),
	);
});

/* ============================================================
   五、剝註解的鑑別力（沒有這一條，上面兩條否定斷言可能是白過的）
   ============================================================ */

test("jsxOf 真的剝掉了註解，也真的沒有把 JSX 文字一起剝掉", () => {
	const raw = source(PANEL);
	assert.ok(raw.includes("通知賣家"), "檔頭註解本來就在解釋為什麼不畫它");
	assert.ok(!jsxOf(PANEL).includes("通知賣家"), "註解沒被剝乾淨");
	assert.ok(jsxOf(PANEL).includes("履約操作已鎖定"), "JSX 文字被誤剝了");
});
