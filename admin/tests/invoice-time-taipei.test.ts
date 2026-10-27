import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { formatDateTime, formatTaipeiDateTime } from "../src/lib/format";

/**
 * ADM 的發票時間必須釘在 Asia/Taipei，不跟著看的人的瀏覽器時區跑（backend#3053）。
 *
 * 實測症狀：同一筆 `automation_cutover_at`（`2026-08-29T19:56:50Z`），ADM 在
 * `Asia/Taipei` 顯示 03:56:50、在 `Asia/Tokyo` 顯示 04:56:50，而賣家端固定顯示
 * 03:56:50 ⇒ 兩個畫面對不起來。使用者人在日本（UTC+9），這是預設情境不是假想。
 */

// 票裡逐欄實測的三個值，直接當 fixture（不是我自己編的例子）。
const CASES: Array<[string, string]> = [
	["2026-08-29T19:56:50Z", "2026/08/30 03:56:50"], // 自動開票啟用切點
	["2026-08-29T20:56:50Z", "2026/08/30 04:56:50"], // 最近驗證時間
	["2026-09-02T16:37:21Z", "2026/09/03 00:37:21"], // 憑證更新時間
];

test("台北換算逐值正確（三個值取自 #3053 的實測表）", () => {
	for (const [iso, want] of CASES) {
		assert.equal(formatDateTime(iso), want, `${iso} 的台北時間應為 ${want}`);
	}
});

test("輸出與 runtime 時區脫鉤：改 process.env.TZ 不影響結果", () => {
	const original = process.env.TZ;
	try {
		for (const tz of ["Asia/Tokyo", "Asia/Taipei", "UTC", "America/New_York"]) {
			process.env.TZ = tz;
			assert.equal(
				formatDateTime("2026-08-29T19:56:50Z"),
				"2026/08/30 03:56:50",
				`TZ=${tz} 下輸出改變了 ⇒ 又跟著 runtime 時區跑了`,
			);
		}
	} finally {
		process.env.TZ = original;
	}
});

test("跨日邊界：UTC 當天 16:00 之後在台北已是隔天", () => {
	// 這一格是「本機時區」與「台北」最容易分岔的地方，也是 #3053 症狀的來源。
	assert.equal(formatDateTime("2026-08-29T15:59:59Z"), "2026/08/29 23:59:59");
	assert.equal(formatDateTime("2026-08-29T16:00:00Z"), "2026/08/30 00:00:00");
});

test("空值與壞值：空回破折號、解析不出來回原字串（不得吞成空白）", () => {
	assert.equal(formatDateTime(null), "—");
	assert.equal(formatDateTime(""), "—");
	assert.equal(formatDateTime("not-a-date"), "not-a-date");
	assert.equal(formatTaipeiDateTime(null), "—", "空值不該被加上時區後綴");
});

test("formatTaipeiDateTime 只是加後綴，值與 formatDateTime 一致", () => {
	assert.equal(
		formatTaipeiDateTime("2026-08-29T19:56:50Z"),
		"2026/08/30 03:56:50 (台北)",
	);
});

/**
 * Ratchet：發票頁不得再出現裸的 `toLocale*` 或 local-time getter。
 *
 * 🔴 這一支才是防回歸的那一支：上面幾支只驗 `formatDateTime` 本身，而症狀的來源
 * 是「某一頁自己組時間字串」——那繞得過所有針對 formatter 的測試。
 */
/**
 * 🔴 觀測窗是**全 ADM**，不是只有發票模組（backend#3053 的稽查）。
 *
 * 第一版只掃 `src/app/admin/invoices` ＋ `src/features/invoices`，於是另外 14 處
 * 自己組時間字串的位置全在窗外 —— 交易管理、售後爭議、異常監控、代行營運、
 * 商品認證、貼文／商品／App 版本／會員方案／站台內容、頁首日期。
 * 那正是 #3053 票裡警告的「有些頁釘住、有些沒有，比全部一致地錯更難察覺」，
 * 只是邊界換了地方。
 *
 * 豁免（都必須是「顯示看的人自己的本機時刻」而不是後端資料）：
 *   - `src/app/admin/page.tsx` 的 `setLastUpdate(new Date().toLocaleTimeString())`
 *     —— 那是「你剛才重新整理的時刻」，本機時間才是對的
 * 新增豁免必須在這裡登記並附理由；沒有理由的一律修成走 lib/format。
 */
const TZ_RATCHET_EXEMPT = new Set([
	"src/app/admin/page.tsx",
	// `new Date(y, mo-1, d)` 建構後用同一組 local getter 反查，確認 2026/13/40 這類
	// 非法值沒有被 Date 進位吃掉。建構與讀取在同一個時區系統內，**輸出與時區無關**
	// ——它根本不產生給人看的時間字串。
	"src/app/admin/holiday-calendar/page.tsx",
]);

test("ADM 不得自己組時間字串（一律走 lib/format 的台北釘死版）", async () => {
	const roots = ["src/app", "src/features", "src/components", "src/lib"];
	const offenders: string[] = [];
	const banned =
		/\.(toLocaleString|toLocaleDateString|toLocaleTimeString|getFullYear|getMonth|getDate|getHours|getMinutes|getSeconds)\s*\(/;

	const walk = async (dir: string): Promise<void> => {
		for (const entry of await readdir(dir, { withFileTypes: true })) {
			const full = path.join(dir, entry.name);
			if (entry.isDirectory()) {
				await walk(full);
				continue;
			}
			if (!/\.tsx?$/.test(entry.name)) continue;
			if (TZ_RATCHET_EXEMPT.has(full)) continue;
			// lib/format.ts 自己就是那個釘死的實作，它必須用 getUTC*。
			if (full === "src/lib/format.ts") continue;
			const src = await readFile(full, "utf8");
			for (const [i, line] of src.split("\n").entries()) {
				// 金額千分位（`n.toLocaleString()` 無參數）不在此列：它與時區無關。
				if (/toLocaleString\(\s*\)/.test(line)) continue;
				if (line.trimStart().startsWith("*")) continue; // 註解
				if (banned.test(line))
					offenders.push(`${full}:${i + 1} ${line.trim()}`);
			}
		}
	};

	for (const root of roots) await walk(root);
	assert.deepEqual(
		offenders,
		[],
		`這些位置自己組時間字串，會跟著看的人的瀏覽器時區跑：\n${offenders.join("\n")}\n` +
			`修法：改用 @/lib/format 的 formatDateTime／formatDateTimeShort／formatDate／` +
			`formatDateLabel／formatWeekday。真的要顯示「看的人的本機時刻」才登記進 TZ_RATCHET_EXEMPT。`,
	);
});
