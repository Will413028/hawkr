import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	auditConfirmationText,
	delegationScopeDelta,
	sessionDurationText,
	sessionEndText,
} from "../src/features/delegated-operations/format";
import { delegatedAuditResourceDisplay } from "../src/features/delegated-operations/labels";

/**
 * 稽核列的「目標資源」必須指得回一個對象。
 *
 * `resourceName` 不保證有值（建立類在 intent 之前沒有 pre-image；訂單面的
 * `resource_id` 存不下 `O` 開頭的訂單編號；遭拒的讀取列根本沒讀到名稱），而
 * 只畫一個「—」是在宣稱「這一列沒有目標」——即使 API 明明回了 `resourceId`。
 */
test("有名稱時顯示名稱", () => {
	assert.equal(
		delegatedAuditResourceDisplay({
			resourceName: "嘉義布袋鮮蚵",
			resourceId: "10015",
		}),
		"嘉義布袋鮮蚵",
	);
});

test("名稱為空但有 ID 時退到 ID，而不是「—」", () => {
	assert.equal(
		delegatedAuditResourceDisplay({ resourceName: "", resourceId: "10015" }),
		"#10015",
	);
});

test("兩者都沒有才是「—」", () => {
	assert.equal(
		delegatedAuditResourceDisplay({ resourceName: "", resourceId: "" }),
		"—",
	);
});

/**
 * 列表列與稽核內容對話框顯示同一列稽核紀錄的同一格。兩處各寫一份必然漂移成兩種說法，
 * 而讀的人會以為那是兩筆不同的紀錄。
 */
test("列表與稽核內容對話框共用同一支顯示函式，不各自寫 fallback", async () => {
	for (const file of [
		"src/features/delegated-operations/components/SessionAuditConsole.tsx",
		"src/features/delegated-operations/components/AuditDetailDialog.tsx",
	]) {
		const source = await readFile(file, "utf8");
		assert.match(source, /delegatedAuditResourceDisplay\(entry\)/u, file);
		assert.doesNotMatch(source, /entry\.resourceName\s*\|\|/u, file);
	}
});

/**
 * 工作階段摘要不得自我矛盾。以下兩組守的是**畫面上並陳的兩句話不能互相打架**，
 * 那是稽核頁最貴的失效模式：讀的人拿它當舉證，而它在騙人。
 */

test("零筆紀錄不得宣稱「均已確認」——那是對空集合的全稱宣稱", () => {
	assert.equal(
		auditConfirmationText({ operationCount: 0, pendingAuditCount: 0 }, 0),
		"本次沒有任何操作紀錄",
	);
});

test("失敗筆數還沒回來且成功/未確認皆為 0 時說「正在確認」，不得當成零筆", () => {
	assert.equal(
		auditConfirmationText({ operationCount: 0, pendingAuditCount: 0 }, null),
		"正在確認稽核紀錄",
	);
});

test("有紀錄且都已確認才說「所有結果均已確認」（成功或失敗任一非零皆算有紀錄）", () => {
	assert.equal(
		auditConfirmationText({ operationCount: 3, pendingAuditCount: 0 }, 0),
		"所有結果均已確認",
	);
	// 全部失敗也是「有紀錄」：不必等 operationCount 非零才承認做過事。
	assert.equal(
		auditConfirmationText({ operationCount: 0, pendingAuditCount: 0 }, 2),
		"所有結果均已確認",
	);
	// 成功數非零時不必等失敗查詢回來就答得出來。
	assert.equal(
		auditConfirmationText({ operationCount: 3, pendingAuditCount: 0 }, null),
		"所有結果均已確認",
	);
});

test("有未確認一律優先報幾筆，不被「有紀錄」蓋掉", () => {
	assert.equal(
		auditConfirmationText({ operationCount: 5, pendingAuditCount: 2 }, 1),
		"2 筆結果未確認",
	);
});

/**
 * 這一列的措辭與表格「變更摘要」欄的「未寫入」主語不同（前者是稽核列的結果確認、
 * 後者是領域寫入）。兩個「寫入」並陳過一次，讀的人無從分辨哪個是哪個。
 */
test("稽核狀態的任何文案都不含「寫入」，避免與變更摘要欄的「未寫入」撞語意", () => {
	const cases: Array<
		[{ operationCount: number; pendingAuditCount: number }, number | null]
	> = [
		[{ operationCount: 0, pendingAuditCount: 0 }, 0],
		[{ operationCount: 0, pendingAuditCount: 0 }, null],
		[{ operationCount: 3, pendingAuditCount: 0 }, 0],
		[{ operationCount: 0, pendingAuditCount: 0 }, 2],
		[{ operationCount: 5, pendingAuditCount: 2 }, 1],
	];
	for (const [counts, failedCount] of cases) {
		const text = auditConfirmationText(counts, failedCount);
		assert.doesNotMatch(
			text,
			/寫入/u,
			`${JSON.stringify(counts)} / failed=${failedCount} → 「${text}」含「寫入」`,
		);
	}
});

test("表格「變更摘要」欄的 FAILED 文案仍是「未寫入」（衝突只由稽核狀態那一側讓開）", async () => {
	const source = await readFile(
		"src/features/delegated-operations/components/SessionAuditConsole.tsx",
		"utf8",
	);
	assert.match(source, /case "FAILED":\s*\n\s*return "未寫入";/u);
});

test("已撤銷／已到期但沒有 endedAt 時，不得宣稱「尚未結束」", () => {
	assert.equal(sessionEndText(null, "revoked"), "已撤銷（未記錄時刻）");
	assert.equal(sessionEndText(null, "expired"), "已結束（未記錄時刻）");
});

test("真的還在進行時才說「尚未結束」；有 endedAt 一律顯示時刻", () => {
	assert.equal(sessionEndText(null, "active"), "尚未結束");
	assert.notEqual(
		sessionEndText("2026-08-16T12:14:53Z", "revoked"),
		"尚未結束",
	);
	assert.notEqual(
		sessionEndText("2026-08-16T12:14:53Z", "revoked"),
		"已撤銷（未記錄時刻）",
	);
});

/**
 * 「工作時長」與「結束時間」是同一個前提的兩半：後端的 duration 只有在右端接不到
 * 任何一個位於過去的候選時才會退回 `now`，而唯一會發生這件事的狀態組合是「已撤銷
 * 且沒有結束時刻」（撤銷不讓 `expires_at` 提前）。推導寫在 `sessionDurationText`。
 */

test("已撤銷但沒有 endedAt 時，工作時長不得印出一個以「現在」為右端的秒數", () => {
	assert.equal(
		sessionDurationText(48, null, "revoked"),
		"未知（撤銷未記錄結束時刻）",
	);
});

test("右端釘得住的三種情形照印秒數，不因為守門而漏掉真的量得到的時長", () => {
	// 撤銷有寫 ended_at（現行 RevokeSession 的行為）。
	assert.equal(
		sessionDurationText(48, "2026-08-16T12:22:45Z", "revoked"),
		"48 秒",
	);
	// 已到期＝兩支到期述詞至少一支成立，右端必在過去，即使 ended_at 還沒被寫上。
	assert.equal(sessionDurationText(900, null, "expired"), "15 分 0 秒");
	// 進行中本來就該以 now 為右端，這一列問的正是「目前已經進行多久」。
	assert.equal(sessionDurationText(48, null, "active"), "48 秒");
});

test("同一組輸入下，兩列不得一列說時刻沒記、另一列照報一個量過的秒數", () => {
	const end = sessionEndText(null, "revoked");
	const duration = sessionDurationText(48, null, "revoked");
	assert.match(end, /未記錄時刻/u);
	assert.doesNotMatch(
		duration,
		/\d+\s*秒/u,
		`結束時間說「${end}」，工作時長卻說「${duration}」—— 同一張卡自己打自己`,
	);
});

/**
 * 上面四支只驗純函式。JSX 那一半要單獨守：把 `<strong>` 改回直接印
 * `durationSeconds`，純函式測試照樣全綠，而畫面回到修前。
 */
test("摘要卡的工作時長走 sessionDurationText，且三個參數都傳", async () => {
	const source = await readFile(
		"src/features/delegated-operations/components/SessionAuditConsole.tsx",
		"utf8",
	);
	// 先剝註解：本次修改在那一列上方留了一段解釋，函式名寫在裡面。
	const code = source
		.replace(/\{\s*\/\*[\s\S]*?\*\/\s*\}/gu, "")
		.replace(/\/\*[\s\S]*?\*\//gu, "")
		.replace(/^[ \t]*\/\/.*$/gmu, "");
	assert.match(
		code,
		/sessionDurationText\(\s*report\.durationSeconds,\s*report\.endedAt,\s*report\.status,?\s*\)/u,
		"工作時長沒有同時吃進 endedAt 與 status ⇒ 它無從得知右端是不是 now",
	);
	// 本地重新實作一份格式化＝繞過守門的最短路徑。
	assert.doesNotMatch(code, /function formatDuration\b/u);
});

/**
 * 授權變更歷程的「範圍變更」欄。
 *
 * 修前印的是兩份完整清單（`before.join("、") → after.join("、")`）：九項全開的列光
 * after 就約 50 字，連同 before 超過 110 字折成四五行，而同列其餘欄位只有一兩行
 * （`vertical-align: top` ⇒ 列高取最高者），整張表高度參差；更要命的是集合減法丟給
 * 讀的人自己做，而這張表存在的理由正是「誰在什麼時候把範圍改成什麼樣」。
 *
 * 鑑別力優先：五種輸入必須給出五種不同結果，全部壓成同一句與什麼都不算沒有差別。
 */
test("範圍差異：新增／移除／同時增減／未變／首次開通各自算得出來", () => {
	assert.deepEqual(
		delegationScopeDelta(
			["products.read"],
			["products.read", "products.write"],
		),
		{ added: ["products.write"], removed: [] },
	);
	assert.deepEqual(
		delegationScopeDelta(
			["products.read", "products.write"],
			["products.read"],
		),
		{ added: [], removed: ["products.write"] },
	);
	assert.deepEqual(
		delegationScopeDelta(
			["products.read", "posts.read"],
			["products.read", "orders.read"],
		),
		{ added: ["orders.read"], removed: ["posts.read"] },
	);
	assert.deepEqual(delegationScopeDelta(["products.read"], ["products.read"]), {
		added: [],
		removed: [],
	});
	// 首次開通：before 為空 ⇒ 全部算新增（呼叫端據 before.length === 0 再標「首次開通」，
	// 否則九項全列成「新增」會看不出這是開通不是擴權）。
	assert.deepEqual(
		delegationScopeDelta([], ["products.read", "products.write"]),
		{ added: ["products.read", "products.write"], removed: [] },
	);
	// 撤回：after 為空。與「未變更」必須分得開。
	assert.deepEqual(delegationScopeDelta(["products.read"], []), {
		added: [],
		removed: ["products.read"],
	});
});

test("順序沿用來源陣列，不另行排序", () => {
	assert.deepEqual(
		delegationScopeDelta([], ["posts.publish", "products.read", "orders.read"])
			.added,
		["posts.publish", "products.read", "orders.read"],
	);
});

/**
 * 純函式綠不代表畫面用了它：把 `<td>` 改回兩份完整清單，上面幾支照樣全過。
 */
test("歷程表的範圍欄走差異呈現，且完整清單有地方可看", async () => {
	const source = await readFile(
		"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
		"utf8",
	);
	const code = source
		.replace(/\{\s*\/\*[\s\S]*?\*\/\s*\}/gu, "")
		.replace(/\/\*[\s\S]*?\*\//gu, "")
		.replace(/^[ \t]*\/\/.*$/gmu, "");
	assert.match(code, /delegationScopeDelta\(before, after\)/u);
	assert.match(code, /<ScopeChangeCell/u);
	// 修前那個形狀：兩份完整清單並排。回退到它就轉紅。
	assert.doesNotMatch(
		code,
		/capabilitiesBefore\s*\n?\s*\.map\(delegationCapabilityLabel\)/u,
		"範圍欄又把 before 完整清單直接印進儲存格",
	);
	// 差異隱藏了 after 全集，展開面板是它的歸宿；沒有它就等於把資訊刪掉。
	assert.match(code, /查看完整範圍/u);
	assert.match(code, /aria-expanded=\{expanded\}/u);
	// 「未變更」不得畫成「—」：那與「首次開通」（before 為空）在畫面上長得一樣。
	assert.match(code, /範圍未變更/u);
});

/**
 * 兩張稽核表的「理由」欄。兩邊的病相反，同一個 class 一起治：
 *   稽核台（第 4 欄）沒有任何寬度保護 ⇒ 被 `.adm-audit-col-summary` 的 width:100%
 *   擠成一字一行；授權歷程表（td:last-child）反而吃到 nowrap ⇒ 200 字的理由被拉成
 *   一整行、把整張表推進橫捲。
 *
 * ⚠️ 選擇器必須帶 `td`：`.adm-audit-table td:last-child` 是 (0,2,1)，只寫
 * `.adm-audit-col-reason`（0,1,0）會輸給它，`white-space` 那半靜默失效。
 */
test("理由欄有自己的下限與換行規則，且兩張表都掛上了", async () => {
	const [css, console_, panel] = await Promise.all([
		readFile("src/styles/pages/DelegatedOperations.css", "utf8"),
		readFile(
			"src/features/delegated-operations/components/SessionAuditConsole.tsx",
			"utf8",
		),
		readFile(
			"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
			"utf8",
		),
	]);
	const rule = /\.adm-audit-table td\.adm-audit-col-reason \{[^}]*\}/u.exec(
		css,
	);
	assert.ok(
		rule,
		"沒有 `.adm-audit-table td.adm-audit-col-reason` ⇒ 特異性贏不了 td:last-child 的 nowrap",
	);
	assert.match(rule[0], /min-width: var\(--adm-audit-reason-min\)/u);
	assert.match(rule[0], /white-space: normal/u);
	// 變數只宣告在 `.adm-audit-table-wrap`（兩張表都在它之下）。多一份就多一個會漂移
	// 的數字；少了它 var() 解析不到，整條規則靜默失效。
	const declarations = [
		...css.matchAll(/--adm-audit-reason-min:\s*(\d+)px;/gu),
	];
	assert.equal(
		declarations.length,
		1,
		`--adm-audit-reason-min 宣告了 ${declarations.length} 次，應為 1（.adm-audit-table-wrap）`,
	);
	assert.match(
		/\.adm-audit-table-wrap \{[^}]*\}/u.exec(css)?.[0] ?? "",
		/--adm-audit-reason-min:/u,
		"變數沒宣告在 .adm-audit-table-wrap ⇒ 獨立使用該 wrap 的授權歷程表拿不到它",
	);
	for (const [name, source] of [
		["SessionAuditConsole", console_],
		["MerchantDelegationAuthorizationPanel", panel],
	] as const) {
		assert.equal(
			(source.match(/adm-audit-col-reason/gu) ?? []).length,
			2,
			`${name} 的 .adm-audit-col-reason 應恰出現 2 次（一個 <th> ＋ 一個 <td>）`,
		);
		assert.match(
			source,
			/<th[^>]*className="adm-audit-col-reason"/u,
			`${name} 的 <th> 沒掛 .adm-audit-col-reason ⇒ 表頭與內容欄寬各自為政`,
		);
	}
});
