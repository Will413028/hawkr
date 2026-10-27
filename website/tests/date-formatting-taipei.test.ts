import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import type { DateLike } from "../lib/date/taipei";

/**
 * #391 — 日期／時間格式化必須走 `lib/date/taipei`，不得在呼叫點裸用 `toLocale*`。
 *
 * root cause 與 #374 同一條：`toLocaleDateString` / `toLocaleTimeString` /
 * `Date.prototype.toLocaleString` 的輸出同時吃 **runtime 時區** 與 **runtime ICU 版本**。
 * 站台 server 端跑在 Cloudflare Workers（UTC）、client 端跑在使用者的瀏覽器（多為
 * UTC+8），同一個 instant 在 16:00Z 之後就會產出差一天的文字：
 *   - 有 SSR 的畫面 → hydration mismatch（React #418，#374 當時把 Sentry ingest 打成 429）
 *   - 純 server 算完才下傳的畫面 → 沒有 mismatch，但**所有**使用者看到 UTC 日期，
 *     台北使用者在跨日窗口看到差一天 —— 是資料正確性缺陷，不只是視覺不一致。
 *
 * #374 只修掉當時炸掉的那幾處，剩下 16 個「日期＋時間」變體留在原地：它們現在多數
 * 是 client-only 渲染所以沒炸，但任何一處日後加上 SSR 就會複製一次 #374。本檔把
 * 「不准再長回來」變成機制，而不是靠人記得（#391 就是靠人記得的失敗案例：#374 的
 * 關票 comment 寫了「建議一次收尾」，然後沒有票、沒有 owner，懸空了整整一輪）。
 *
 * ── 規則 ────────────────────────────────────────────────────────────────
 * 1. `toLocaleDateString` / `toLocaleTimeString`：**一律禁止**（純日期 API，沒有
 *    「其實我在格式化數字」的可能）。
 * 2. `toLocaleString`：**數字用途放行、日期用途禁止**。判定為日期用途的條件（任一）：
 *      a. options 物件出現任何日期／時間欄位（`year` / `hour` / `timeZone` / `hour12` …）
 *      b. receiver 是 `Date(...)` 的回傳值（`new Date(x).toLocaleString(...)`）
 *      c. receiver 的識別字是日期形狀（`created` / `dt` / `xxxAt` / `xxxTime` …）
 *    金額千分位（`Number(x).toLocaleString()`、`amount.toLocaleString("zh-TW", {
 *    maximumFractionDigits: 0 })`）不受時區影響，維持放行。
 *
 * 掃描前會先把註解與字串**內容**抹掉（保留引號），所以說明文字裡出現這些 API 名稱
 * 不會誤判 —— `lib/date/taipei.ts` 的檔頭與本檔都在講它們。
 */

const repoRoot = fileURLToPath(new URL("..", import.meta.url));

/** 產品程式碼的根目錄。`tests/` 自己不掃（測試會需要示範舊寫法來做反向驗證）。 */
const SCAN_ROOTS = ["app", "components", "features", "lib"];

/**
 * 刻意要用「使用者裝置本地時區」而非台北時區的呼叫點。
 *
 * 目前是空的 —— 全站的日期都是業務時間（下單 / 評價 / 匯入工作 / 仲裁裁決 /
 * 最後訊息），一律以台北牆上時間呈現才正確。真的出現「顯示你裝置上的時間」這種
 * 需求時，把 `檔案:接近的識別字` 加進來，**並在該行程式碼寫明理由**——留白名單而
 * 不寫理由，下一個人只會看到一個沒人敢動的例外。
 */
const INTENTIONAL_DEVICE_TIMEZONE: readonly string[] = [];

/** `Intl.DateTimeFormat` 的日期／時間欄位。與數字格式化的欄位沒有交集。 */
const DATE_OPTION_KEYS = [
	"year",
	"month",
	"day",
	"hour",
	"minute",
	"second",
	"weekday",
	"era",
	"timeZone",
	"timeZoneName",
	"hour12",
	"hourCycle",
	"dateStyle",
	"timeStyle",
	"dayPeriod",
	"fractionalSecondDigits",
	"calendar",
];

/**
 * 把註解換成等長空白、把字串字面值的**內容**清空（保留引號），
 * 但**保留樣板字面值 `${…}` 裡的運算式**。
 *
 * 為什麼不用 regex 一次替換：`"https://…"` 裡的 `//` 會被當成行註解，整行後面
 * 都被吃掉 —— 那正好會把違規藏起來，是個會靜默失效的閘門。這裡逐字元掃描並追蹤
 * 引號狀態，換行一律保留，行號才對得上。
 *
 * ⚠️ `${…}` 必須留下來：日期渲染最常見的長相就是
 * `` `${d.getFullYear()}/${pad(d.getMonth() + 1)}` `` 與
 * `` `${date.toLocaleDateString("zh-TW")}` ``。把整段樣板內容抹掉的話，
 * **本檔全部三條閘門都會對這個最常見的形狀視而不見**（第一版就是這樣，
 * 加 local-time getter 帳本時才發現）。
 *
 * 用堆疊追蹤巢狀：樣板裡可以有 `${}`、`${}` 裡可以再有樣板／字串／物件字面值。
 */
function stripCommentsAndStringBodies(src: string): string {
	type Frame = { kind: "template" } | { kind: "expr"; depth: number };
	const stack: Frame[] = [];
	let out = "";
	let i = 0;
	const n = src.length;

	while (i < n) {
		const top = stack.length > 0 ? stack[stack.length - 1] : undefined;
		const c = src[i];
		const next = src[i + 1];

		// --- 樣板字面值的「文字」部分：抹掉，但要認得 `${` 與結尾的反引號 ---
		if (top?.kind === "template") {
			if (c === "\\") {
				i += 2;
				continue;
			}
			if (c === "`") {
				stack.pop();
				out += "`";
				i += 1;
				continue;
			}
			if (c === "$" && next === "{") {
				stack.push({ kind: "expr", depth: 0 });
				out += "${";
				i += 2;
				continue;
			}
			if (c === "\n") out += "\n";
			i += 1;
			continue;
		}

		// --- 以下是「程式碼」上下文（頂層，或 `${}` 內的運算式）---

		// 行註解
		if (c === "/" && next === "/") {
			while (i < n && src[i] !== "\n") {
				out += " ";
				i += 1;
			}
			continue;
		}

		// 區塊註解（換行保留，其餘換空白）
		if (c === "/" && next === "*") {
			out += "  ";
			i += 2;
			while (i < n && !(src[i] === "*" && src[i + 1] === "/")) {
				out += src[i] === "\n" ? "\n" : " ";
				i += 1;
			}
			out += "  ";
			i += 2;
			continue;
		}

		// 一般字串：保留引號，內容清空
		if (c === '"' || c === "'") {
			out += c;
			i += 1;
			while (i < n && src[i] !== c) {
				if (src[i] === "\\") {
					i += 2;
					continue;
				}
				if (src[i] === "\n") out += "\n";
				i += 1;
			}
			out += c;
			i += 1;
			continue;
		}

		// 樣板字面值開頭
		if (c === "`") {
			stack.push({ kind: "template" });
			out += "`";
			i += 1;
			continue;
		}

		// `${}` 內：追蹤大括號深度，深度回到 0 的 `}` 表示運算式結束
		if (top?.kind === "expr") {
			if (c === "{") {
				top.depth += 1;
			} else if (c === "}") {
				if (top.depth === 0) {
					stack.pop();
					out += "}";
					i += 1;
					continue;
				}
				top.depth -= 1;
			}
		}

		out += c;
		i += 1;
	}

	return out;
}

/** 從 `(` 開始取出配對的引數字串（不含外層括號）。 */
function argumentText(code: string, openParen: number): string {
	let depth = 0;
	for (let i = openParen; i < code.length; i += 1) {
		if (code[i] === "(") depth += 1;
		else if (code[i] === ")") {
			depth -= 1;
			if (depth === 0) return code.slice(openParen + 1, i);
		}
	}
	return code.slice(openParen + 1);
}

/**
 * 取出 `.toLocaleString` 左邊那個 receiver 的「最後一段」。
 *
 * `Number(x)` / `Math.round(n)` / `parsePriceValue(p)` 這種以 `)` 結尾的，往回配對
 * 括號後回傳被呼叫的函式名（`Number` / `round` / `parsePriceValue`），因為判定日期
 * 與否要看的是它 —— `new Date(iso).toLocaleString()` 的關鍵字就是 `Date`。
 */
function receiverToken(code: string, dotIndex: number): string {
	let end = dotIndex;
	while (end > 0 && /\s/.test(code[end - 1])) end -= 1;

	if (code[end - 1] === ")") {
		let depth = 0;
		let i = end - 1;
		for (; i >= 0; i -= 1) {
			if (code[i] === ")") depth += 1;
			else if (code[i] === "(") {
				depth -= 1;
				if (depth === 0) break;
			}
		}
		end = i;
		while (end > 0 && /\s/.test(code[end - 1])) end -= 1;
	}

	const prefix = code.slice(Math.max(0, end - 64), end);
	const match = prefix.match(/([A-Za-z_$][\w$]*)$/);
	return match ? match[1] : "";
}

/** receiver 名稱看起來是不是一個 Date／時間戳。 */
function looksLikeDateReceiver(token: string): boolean {
	if (!token) return false;
	if (token === "Date") return true;
	if (
		/^(d|dt|date|datetime|time|ts|instant|created|updated|deadline)$/i.test(
			token,
		)
	) {
		return true;
	}
	// camelCase 後綴：createdAt / decidedAt / detectedAt / shipDeadlineAt / lastMessageTime
	return /(At|Date|Time|Deadline)$/.test(token);
}

/**
 * ── 第二種長相：local-time getter／setter（#391 第二輪）──────────────────
 *
 * `getFullYear` / `getHours` / `setHours(0,0,0,0)` / `new Date(y, m, d)` 全部以
 * **runtime 本地時區**解讀，和 `toLocale*` 是同一個 root cause，只是它們不呼叫
 * `Intl`，所以躲得過上面那條閘門。第一輪修完 `toLocale*` 之後，這一族還剩 23 個
 * 渲染點——護欄留著這個洞，下一個人只會以為日期問題已經解決了。
 *
 * ⚠️ 這一族**沒辦法**像 `toLocale*` 那樣用語法形狀分辨對錯：
 *   - `${d.getFullYear()}/...`  → 渲染，錯的
 *   - `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`（聊天分組鍵）→ 也是樣板
 *     字串，但它是**分組鍵不是畫面**，改時區會連帶改「今天/昨天」的邊界
 *   - `new Date(y, m, d)`（日期選擇器）→ 操作的是民曆日期不是 instant，本地時區
 *     在那裡反而是**正確**語意
 * 三者的語法長相一模一樣。硬套啟發式規則只會製造假紅，然後被 `// biome-ignore`
 * 式的敷衍繞過。
 *
 * 所以改用**帳本**：每個還在用這一族的檔案，登記「幾個 + 為什麼」。
 * 斷言是**精確相等**，兩個方向都會紅：
 *   - 多了 → 有人新增了本地時區用法，先確認是不是渲染路徑（是就走 lib/date/taipei）
 *   - 少了 → 有人修掉了一個，把數字調降（順手把 reason 更新成剩下的那些）
 * 帳本本身就是文件：讀的人一眼看到「這些是刻意的、理由是什麼」，而不是一片沉默。
 */
const LOCAL_TIME_MEMBERS = [
	"getFullYear",
	"getMonth",
	"getDate",
	"getHours",
	"getMinutes",
	"getSeconds",
	"getDay",
	"setHours",
	"setMinutes",
	"setSeconds",
	"setDate",
	"setMonth",
	"setFullYear",
];

interface LocalTimeBudget {
	/** 本檔目前仍以本地時區運作的呼叫點數量（含 `new Date(y, m, d)` 建構式）。 */
	count: number;
	/** 為什麼這裡用本地時區是對的／為什麼還沒動。 */
	reason: string;
}

const LOCAL_TIME_ALLOWLIST: Record<string, LocalTimeBudget> = {
	"components/ui/CalendarMonth.tsx": {
		count: 42,
		reason:
			"日曆元件操作的是**民曆日期**（使用者在格子上點的那一天），不是 instant。" +
			"強制台北會讓國外使用者點到的日期位移一天，本地時區在這裡才是正確語意。" +
			"33 → 42：補上 APG grid 的方向鍵導覽（addDays / addMonthsKeepingDay 兩個" +
			"日期位移 helper），走的是同一組民曆語意。",
	},
	"components/ui/DatePicker.tsx": {
		count: 11,
		reason:
			"同 CalendarMonth：輸入元件產出的是 `YYYY-MM-DD` 民曆字串（證照有效期、生日），" +
			"不是時間點。",
	},
	"features/account/screens/messages/ThreadClient.tsx": {
		count: 13,
		reason:
			"聊天日期分隔線的分組鍵（dayKey）與「今天/昨天」判定。「今天」是個人時區語意" +
			"還是商家時區語意需要業務裁決，且 dayKey 與標籤必須同進退，不能只改一半。待裁決。",
	},
	"features/account/screens/overview/AccountOverviewClient.tsx": {
		count: 2,
		reason:
			"「本年度」期間篩選的起點。改成台北年初會位移 ≤8h，改變跨年前後訂單落在哪一年。待裁決。",
	},
	"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx": {
		count: 6,
		reason:
			"賣家分析的今天/昨天/7d/30d 區間邊界（dayStart/dayEnd/resolveRange）。" +
			"改邊界會改變統計筆數。待裁決。",
	},
	"lib/search/repo.ts": {
		count: 2,
		reason:
			"熱門標籤查詢區間。注意它本身已是混用（本地日相減後取 `toISOString()` 的 UTC 日），" +
			"要修得連同送給後端的語意一起想清楚。待裁決。",
	},
	"lib/seller/shipment.ts": {
		count: 3,
		reason:
			"7-11 取號的 `shipDate`＝**本地的明天**，是送給後端／PayUni 的 wire 值不是畫面。" +
			"台灣賣家無差異，人在國外的賣家會送出錯的出貨日。屬第三方整合，待裁決。",
	},
};

interface Violation {
	file: string;
	line: number;
	api: string;
	snippet: string;
}

function listSourceFiles(dir: string, acc: string[]): string[] {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const full = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			// 測試自己要能示範舊寫法（反向驗證），不納入掃描。
			if (entry.name === "__tests__" || entry.name === "node_modules") continue;
			listSourceFiles(full, acc);
			continue;
		}
		if (
			/\.(ts|tsx)$/.test(entry.name) &&
			!/\.unit\.test\.tsx?$/.test(entry.name)
		) {
			acc.push(full);
		}
	}
	return acc;
}

/**
 * 數一個檔案裡「以本地時區運作」的呼叫點：
 *   1. `LOCAL_TIME_MEMBERS` 列出的 getter／setter
 *   2. `new Date(y, m, d)` 這種多引數建構式（也是本地時區；`new Date(ms)` /
 *      `new Date(iso)` / `new Date()` 都是安全的，不計）
 * `getUTC*` / `setUTC*` / `getTime` / `getTimezoneOffset` 不在名單內，天生不受影響。
 */
function countLocalTimeUsages(code: string): number {
	let total = 0;

	for (const member of LOCAL_TIME_MEMBERS) {
		const needle = `.${member}(`;
		let from = 0;
		for (;;) {
			const hit = code.indexOf(needle, from);
			if (hit === -1) break;
			from = hit + needle.length;
			total += 1;
		}
	}

	// `new Date(...)` 帶頂層逗號 = 逐欄位建構 = 本地時區。
	let from = 0;
	for (;;) {
		const hit = code.indexOf("new Date(", from);
		if (hit === -1) break;
		const openParen = hit + "new Date(".length - 1;
		from = openParen + 1;
		const args = argumentText(code, openParen);
		let depth = 0;
		let topLevelComma = false;
		for (let k = 0; k < args.length; k += 1) {
			const ch = args[k];
			if (ch === "(" || ch === "[" || ch === "{") depth += 1;
			else if (ch === ")" || ch === "]" || ch === "}") depth -= 1;
			// formatter 會在多行呼叫的最後一個引數後面加逗號；那不是「多引數」。
			else if (ch === "," && depth === 0 && args.slice(k + 1).trim() !== "") {
				topLevelComma = true;
			}
		}
		if (topLevelComma) total += 1;
	}

	return total;
}

function scan(): Violation[] {
	const violations: Violation[] = [];

	for (const root of SCAN_ROOTS) {
		for (const file of listSourceFiles(path.join(repoRoot, root), [])) {
			const rel = path.relative(repoRoot, file);
			const code = stripCommentsAndStringBodies(readFileSync(file, "utf8"));

			for (const api of [
				"toLocaleDateString",
				"toLocaleTimeString",
				"toLocaleString",
			]) {
				const needle = `.${api}(`;
				let from = 0;
				for (;;) {
					const hit = code.indexOf(needle, from);
					if (hit === -1) break;
					from = hit + needle.length;

					const openParen = hit + needle.length - 1;
					const args = argumentText(code, openParen);
					const receiver = receiverToken(code, hit);

					const isDateUsage =
						api !== "toLocaleString" ||
						looksLikeDateReceiver(receiver) ||
						DATE_OPTION_KEYS.some((key) =>
							new RegExp(`(^|[{,\\s])${key}\\s*:`).test(args),
						);
					if (!isDateUsage) continue;

					const line = code.slice(0, hit).split("\n").length;
					const marker = `${rel}:${receiver}`;
					if (INTENTIONAL_DEVICE_TIMEZONE.includes(marker)) continue;

					violations.push({
						file: rel,
						line,
						api,
						snippet: `${receiver}.${api}(${args.trim().slice(0, 60)})`,
					});
				}
			}
		}
	}

	return violations;
}

test("日期／時間格式化不得裸用 toLocale*，一律走 lib/date/taipei", () => {
	const violations = scan();
	assert.deepEqual(
		violations.map((v) => `${v.file}:${v.line} — ${v.snippet}`),
		[],
		"這些呼叫點的輸出會跟著 runtime 時區／ICU 版本飄（#374 的 root cause）。\n" +
			"改用 @/lib/date/taipei 的 formatTaipeiDate / formatTaipeiDateTime /\n" +
			"formatTaipeiMonthDayTime / formatTaipeiMonthDay；需要新的呈現格式就在該檔\n" +
			"加一個具名輸出，不要在呼叫點自己組 Intl。真的需要使用者裝置本地時區，\n" +
			"把 `檔案:receiver` 加進本檔的 INTENTIONAL_DEVICE_TIMEZONE 並在程式碼寫明理由。",
	);
});

test("local-time getter 只准出現在帳本登記的位置，數量必須完全相符", () => {
	const actual: Record<string, number> = {};

	for (const root of SCAN_ROOTS) {
		for (const file of listSourceFiles(path.join(repoRoot, root), [])) {
			const rel = path.relative(repoRoot, file);
			// helper 自己由下一支測試單獨把關（它只准用 getUTC*）。
			if (rel === "lib/date/taipei.ts") continue;
			const code = stripCommentsAndStringBodies(readFileSync(file, "utf8"));
			const count = countLocalTimeUsages(code);
			if (count > 0) actual[rel] = count;
		}
	}

	const expected = Object.fromEntries(
		Object.entries(LOCAL_TIME_ALLOWLIST).map(([file, { count }]) => [
			file,
			count,
		]),
	);

	assert.deepEqual(
		actual,
		expected,
		"本地時區呼叫點的帳本對不上（tests/date-formatting-taipei.test.ts 的 LOCAL_TIME_ALLOWLIST）。\n" +
			"多出來的檔案／數字 = 有人新增了以瀏覽器時區運作的日期程式碼：\n" +
			"  · 如果它會**渲染成畫面文字** → 改用 @/lib/date/taipei 的 formatter\n" +
			"    （排版湊不出來就用 taipeiParts()，不要回頭碰 getFullYear 家族）\n" +
			"  · 如果它是**篩選／比較／民曆輸入**，本地時區才是對的 → 在帳本加一筆並寫明理由\n" +
			"少掉的數字 = 有人修好了一處：把 count 調降，順手更新 reason。",
	);
});

test("共用 helper 自己不得依賴 runtime 時區（只准用 getUTC*）", () => {
	const src = readFileSync(path.join(repoRoot, "lib/date/taipei.ts"), "utf8");
	const code = stripCommentsAndStringBodies(src);

	// helper 是全站唯一讀時間欄位的地方，一旦它自己用了 local-time getter，
	// 上面那條閘門再綠也沒有意義（全站都會跟著 runtime 時區飄）。
	for (const getter of [
		"getFullYear",
		"getMonth",
		"getDate",
		"getHours",
		"getMinutes",
		"getDay",
		"setHours",
	]) {
		assert.ok(
			!code.includes(`.${getter}(`),
			`lib/date/taipei.ts 不得使用 local-time getter \`${getter}\`（改用 getUTC* + 固定 +08:00 位移）`,
		);
	}

	assert.ok(
		!code.includes("toLocale") && !code.includes("Intl."),
		"lib/date/taipei.ts 必須自己組字串：Intl / toLocale* 的輸出受 runtime ICU 版本影響，是第二條 mismatch 途徑",
	);
});

test("helper 提供的具名輸出涵蓋全站四種日期呈現", () => {
	const src = readFileSync(path.join(repoRoot, "lib/date/taipei.ts"), "utf8");
	for (const fn of [
		"formatTaipeiDate",
		"formatTaipeiDateTime",
		"formatTaipeiMonthDayTime",
		"formatTaipeiMonthDay",
	]) {
		assert.match(
			src,
			new RegExp(`export function ${fn}\\b`),
			`lib/date/taipei.ts 應輸出 ${fn}（呼叫點改用它取代裸 toLocale*）`,
		);
	}
});

// ---------------------------------------------------------------------------
// (c) 無時區標記的牆上時間字串不得被當成 instant 解析
//
// 前兩條閘門守的是**輸出端**（不准用 toLocale* / local-time getter 讀時間），
// 這條守的是**輸入端**：`Date.parse("2026/07/19")` 依規格以 runtime 本地時區
// 解讀，所以瀏覽器時區在進到 partsFromMs 之前就已經污染了 instant。
//
// 實測（修復前，同一頁只換瀏覽器時區）：消費明細的 `orderDate="2026/07/19"`
// 在 Asia/Taipei 顯示 2026/07/19、在 Asia/Tokyo 顯示 2026/07/18。
// 這裡用 process.env.TZ 逐一切換 runtime 時區跑同一份輸入，任何一種輸入只要
// 在兩個時區之間輸出不同就是回歸。
// ---------------------------------------------------------------------------

/** 橫跨 UTC+8 兩側的代表性時區（含最東的 +14 與美西 -7）。 */
const TZ_MATRIX = [
	"Asia/Taipei",
	"Asia/Tokyo", // +09 — 使用者常駐地，修復前就是這裡少一天
	"Pacific/Kiritimati", // +14 — 最極端的東移
	"UTC",
	"America/Los_Angeles", // -07 — 反向偏移
	"Pacific/Niue", // -11 — 最極端的西移
];

/** 每一種 wire 上會出現的時間表示法，都必須是時區不變量。 */
const TZ_INVARIANT_INPUTS: readonly DateLike[] = [
	"2026/07/19", // buyer-service 消費明細 orderDate（台北牆上時間）
	"2026/01/01", // 年界：東移時區在修復前會退回前一年
	"2026-07-19", // 純日期（生日 / 交易日）
	"2026-07-19 08:30", // 空白分隔的牆上時間
	"2026-07-19T08:30:00", // 無位移的 ISO 牆上時間
	"2026-07-19T08:30:00Z", // 真正的 instant（帶 Z）
	"2026-07-19T08:30:00+08:00", // 真正的 instant（帶位移）
	1785000000000, // epoch 毫秒
	1785000000, // epoch 秒
	"1785000000000", // 數字字串
];

test("所有 formatter 的輸出與 runtime 時區脫鉤（含無時區標記的牆上時間字串）", async () => {
	const originalTz = process.env.TZ;
	const mod = await import("../lib/date/taipei");
	const formatters = [
		"formatTaipeiDate",
		"formatTaipeiDateTime",
		"formatTaipeiMonthDayTime",
		"formatTaipeiMonthDay",
		"formatTaipeiShortDate",
		"formatTaipeiTime",
	] as const;

	try {
		for (const input of TZ_INVARIANT_INPUTS) {
			for (const name of formatters) {
				const outputs = TZ_MATRIX.map((tz) => {
					process.env.TZ = tz;
					return `${tz}=${(mod[name] as (v: DateLike) => string)(input)}`;
				});
				const distinct = new Set(outputs.map((o) => o.split("=")[1]));
				assert.equal(
					distinct.size,
					1,
					`${name}(${JSON.stringify(input)}) 的輸出跟著 runtime 時區跑：\n  ` +
						`${outputs.join("\n  ")}\n` +
						"無時區標記的字串一律是台北牆上時間，必須走 toEpochMs 的 WALL_CLOCK_RE\n" +
						"分支（Date.UTC - 08:00），不能落到 Date.parse（它以本地時區解讀）。",
				);
			}
		}
	} finally {
		if (originalTz === undefined) delete process.env.TZ;
		else process.env.TZ = originalTz;
	}
});

test("牆上時間字串以台北時區還原，且不存在的日期不會靜默進位", async () => {
	const originalTz = process.env.TZ;
	const { toEpochMs, formatTaipeiDateTime } = await import(
		"../lib/date/taipei"
	);
	try {
		// 在最會出事的時區跑，確保斷言驗的是解析行為而不是恰好同時區。
		process.env.TZ = "Asia/Tokyo";

		// 台北 2026/07/19 00:00 = 2026-07-18T16:00:00Z。
		assert.equal(toEpochMs("2026/07/19"), Date.UTC(2026, 6, 18, 16, 0, 0));
		// 純日期同樣是台北當日 00:00（不是 UTC 00:00）。
		assert.equal(toEpochMs("2026-07-19"), Date.UTC(2026, 6, 18, 16, 0, 0));
		assert.equal(
			formatTaipeiDateTime("2026-07-19T08:30:00"),
			"2026/07/19 08:30",
		);

		// 語法對但不存在的日期回 null（不得進位成 2026/03/02 之類）。
		for (const bogus of [
			"2026-02-30",
			"2026-13-01",
			"2026/00/10",
			"2026-07-19T08:99",
			"2026-07-19T24:00:00",
			"2026-07-19T08:30:99",
		]) {
			assert.equal(toEpochMs(bogus), null, `${bogus} 應無法解讀`);
		}
	} finally {
		if (originalTz === undefined) delete process.env.TZ;
		else process.env.TZ = originalTz;
	}
});
