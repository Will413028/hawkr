import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { sessionRevokedByText } from "../src/features/delegated-operations/format";
import { parseDelegatedSessionReport } from "../src/features/delegated-operations/governance";

/**
 * 工作階段摘要的撤銷四件套（`revokedAt` / `revokedByAdminId` /
 * `revokedByAdminDisplayName` / `revokeReason`，backend#2483 A9）。
 *
 * 修前這四欄後端一路填到 gateway，ADM 這一側**型別上根本不存在** —— parser 不解析、
 * 畫面不渲染。撤銷理由在 `SessionRevokeDialog` 是必填的，於是變成「強制收集一個永遠
 * 不會被讀回來的欄位」：操作者被訓練成亂填，而它正是爭議時唯一能舉證「為什麼中止」的
 * 字串。這種缺口不會 crash、不會報錯、不會讓任何測試轉紅，只是那段文字從不出現。
 *
 * 三件事各要能單獨被打紅：
 *   ① 四欄解析得出（型別上拿得到）
 *   ② 已撤銷時 markup 真的含撤銷理由與撤銷者
 *   ③ 未撤銷時整塊不渲染（不得多出一塊空白區）
 */

// 後端契約（gateway `DelegatedSessionSummary`）。數字一律字串。
const SESSION_ID = "7494008076136087552";

const SESSION_SUMMARY = {
	sessionId: SESSION_ID,
	displayCode: "OPS-1KXP05J5SRQPS",
	target: {
		storeId: "2685914682",
		sellerAccountId: "2685914682",
		storeName: "梨山盛花園",
	},
	actor: { adminId: "2661027599", displayName: "alexischen" },
	issuedAt: "2026-08-14T12:32:37Z",
	endedAt: null,
	durationSeconds: "327",
	status: "active",
	operationCount: "2",
	riskOperationCount: "0",
	pendingAuditCount: "0",
	notifiedSeller: true,
	fulfillmentActionCount: "0",
	financeActionCount: "0",
	auditRetentionDays: "730",
	revokedAt: null,
	revokedByAdminId: null,
	revokedByAdminDisplayName: null,
	revokeReason: null,
};

const REVOKED_AT = "2026-08-14T12:39:29Z";
const REVOKE_REASON = "賣家來電表示已自行處理，提前中止本次代行。";

const REVOKED_SUMMARY = {
	...SESSION_SUMMARY,
	status: "revoked",
	endedAt: REVOKED_AT,
	revokedAt: REVOKED_AT,
	revokedByAdminId: "2661027599",
	// 後端填的是 `admin_accounts.email`（該表沒有顯示名欄位）。
	revokedByAdminDisplayName: "ops-admin@example.com",
	revokeReason: REVOKE_REASON,
};

// ---------------------------------------------------------------------------
// ① parser
// ---------------------------------------------------------------------------

test("① 摘要 parser 收下撤銷四件套", () => {
	const parsed = parseDelegatedSessionReport(REVOKED_SUMMARY);
	assert.ok(parsed);
	assert.equal(parsed.revokedAt, REVOKED_AT);
	assert.equal(parsed.revokedByAdminId, "2661027599");
	assert.equal(parsed.revokedByAdminDisplayName, "ops-admin@example.com");
	assert.equal(parsed.revokeReason, REVOKE_REASON);
});

test("未撤銷時四欄是 null，不是 undefined 或空字串", () => {
	const parsed = parseDelegatedSessionReport(SESSION_SUMMARY);
	assert.ok(parsed);
	assert.equal(parsed.revokedAt, null);
	assert.equal(parsed.revokedByAdminId, null);
	assert.equal(parsed.revokedByAdminDisplayName, null);
	assert.equal(parsed.revokeReason, null);
});

/**
 * ADM 與 gateway 各自 promote。要求這四欄存在，會讓兩者之間那段窗口整份摘要 fail
 * closed —— 為了三個補充欄位把整張稽核頁擋掉，代價不對等。**型別錯仍舊整份失敗**。
 */
test("缺欄視同沒有；型別錯仍整份 fail closed", () => {
	const withoutRevocation = { ...SESSION_SUMMARY } as Record<string, unknown>;
	for (const key of [
		"revokedAt",
		"revokedByAdminId",
		"revokedByAdminDisplayName",
		"revokeReason",
	]) {
		delete withoutRevocation[key];
	}
	const parsed = parseDelegatedSessionReport(withoutRevocation);
	assert.ok(parsed, "缺欄不該讓整份摘要作廢");
	assert.equal(parsed.revokedAt, null);
	assert.equal(parsed.revokeReason, null);

	for (const broken of [
		// 帶偏移的時間：同一筆紀錄在兩個畫面上會差 8 小時，而讀的人正在比對時序。
		{ ...REVOKED_SUMMARY, revokedAt: "2026-08-14T20:39:29+08:00" },
		{ ...REVOKED_SUMMARY, revokedAt: 1_755_175_169 },
		{ ...REVOKED_SUMMARY, revokedByAdminId: 2_661_027_599 },
		{ ...REVOKED_SUMMARY, revokedByAdminDisplayName: { email: "x@y.z" } },
		{ ...REVOKED_SUMMARY, revokeReason: "超".repeat(201) },
		// 控制字元（此處 BEL）：`hasAsciiControlCharacters` 擋的就是這一類。
		{ ...REVOKED_SUMMARY, revokeReason: "含控制字元\u0007" },
	]) {
		assert.equal(
			parseDelegatedSessionReport(broken),
			null,
			`should fail closed: ${JSON.stringify(broken.revokedAt)}／${JSON.stringify(
				broken.revokedByAdminId,
			)}`,
		);
	}
});

test("摘要 parser 是冪等的：BFF 與瀏覽器會各解析一次同一份資料", () => {
	const first = parseDelegatedSessionReport(REVOKED_SUMMARY);
	assert.ok(first);
	// 第二次看到的是第一次的產物（計數已是 number）。不冪等的症狀是治理頁永遠顯示
	// 「摘要格式不完整」，而兩邊各自單看都正確。
	const second = parseDelegatedSessionReport(first);
	assert.deepEqual(second, first);
});

// ---------------------------------------------------------------------------
// 撤銷者那一行的拼接
// ---------------------------------------------------------------------------

test("撤銷者那一行：全形括號結尾不再補半形空格", () => {
	const line = sessionRevokedByText(
		REVOKED_AT,
		"ops-admin@example.com",
		"2661027599",
	);
	assert.match(line, /由 ops-admin@example\.com（2661027599）撤銷$/u);
	// 修前是 JSX 的 `{expr}{" "}` 換行補的空白 ＋ 全形括號自帶的視覺留白疊在一起，
	// 看起來像「（2661027599） 撤銷」中間少了一個字。
	assert.doesNotMatch(line, /）\s撤銷/u);
});

test("撤銷者缺席時說得出缺了什麼，不是省略整句", () => {
	// join 落空但仍知道是誰。
	assert.match(
		sessionRevokedByText(REVOKED_AT, null, "2661027599"),
		/由 管理員（2661027599）撤銷$/u,
	);
	// 顯示名是 email（西文結尾）→ 這裡才需要半形空格。
	assert.match(
		sessionRevokedByText(REVOKED_AT, "ops-admin@example.com", null),
		/由 ops-admin@example\.com 撤銷$/u,
	);
	// 兩者皆缺仍要留下「誰撤的」這個問題被問過的痕跡。
	assert.match(
		sessionRevokedByText(REVOKED_AT, null, null),
		/由 未記錄操作者撤銷$/u,
	);
});

// ---------------------------------------------------------------------------
// ②③ 真的渲染得出來
// ---------------------------------------------------------------------------

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

interface ReactLike {
	createElement: (
		type: unknown,
		props?: unknown,
		...children: unknown[]
	) => unknown;
}

const { createElement } = requireFromTest("react") as ReactLike;
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const { QueryClient, QueryClientProvider } = requireFromTest(
	"@tanstack/react-query",
) as {
	QueryClient: new (
		options?: unknown,
	) => {
		prefetchQuery: (options: unknown) => Promise<void>;
	};
	QueryClientProvider: unknown;
};
const { AppRouterContext } = requireFromTest(
	"next/dist/shared/lib/app-router-context.shared-runtime",
) as { AppRouterContext: { Provider: unknown } };

const SessionAuditConsole = requireFromTest(
	"../src/features/delegated-operations/components/SessionAuditConsole",
).default as unknown;
const { getDelegatedSessionReport } = requireFromTest(
	"../src/services/delegated-operations",
) as {
	getDelegatedSessionReport: (sessionId: string) => Promise<unknown>;
};

const ROUTER_STUB = {
	push: () => {},
	replace: () => {},
	refresh: () => {},
	back: () => {},
	forward: () => {},
	prefetch: () => {},
};

/**
 * 把摘要放進 cache，元件 render 時直接讀到它。
 *
 * 走的是**元件自己那條鏈**（`getDelegatedSessionReport` → parser），只把 `fetch`
 * 換掉：這樣一來 service 或 parser 少收一欄時這支也會紅，而不是只守住 JSX。
 * 另外兩支查詢（稽核列表、失敗筆數）不 seed —— `renderToStaticMarkup` 不跑 effect，
 * 它們維持 pending，不影響摘要區。
 */
async function seedSummary(summary: unknown): Promise<unknown> {
	const original = globalThis.fetch;
	globalThis.fetch = (async () =>
		new Response(JSON.stringify({ code: 0, data: summary }), {
			status: 200,
			headers: { "content-type": "application/json" },
		})) as typeof fetch;
	try {
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false, retryOnMount: false } },
		});
		await client.prefetchQuery({
			queryKey: ["delegated-governance", SESSION_ID, "summary"],
			queryFn: () => getDelegatedSessionReport(SESSION_ID),
		});
		return client;
	} finally {
		globalThis.fetch = original;
	}
}

function renderConsole(client: unknown): string {
	return renderToStaticMarkup(
		createElement(
			AppRouterContext.Provider,
			{ value: ROUTER_STUB },
			createElement(
				QueryClientProvider,
				{ client },
				createElement(SessionAuditConsole, { sessionId: SESSION_ID }),
			),
		),
	);
}

test("② 已撤銷的工作階段：畫面上讀得到誰撤的、什麼時候、為什麼", async () => {
	const html = renderConsole(await seedSummary(REVOKED_SUMMARY));

	// 這一句是整張票的標的：修前它在任何畫面都渲染不出來。
	assert.match(
		html,
		new RegExp(REVOKE_REASON, "u"),
		`撤銷理由沒有渲染出來：${html.slice(0, 1200)}`,
	);
	// 撤銷者。後端填的是 email，所以斷言的是 email 而不是姓名。
	assert.match(html, /ops-admin@example\.com/u);
	// email 會換，數字 admin id 才是維運 SQL 認的那一個，兩者都要在。
	assert.match(html, /2661027599/u);
	// 撤銷時刻用的是 `revokedAt`，不是同一頁上任何別的時間欄位。
	//
	// 🔴 預期值**寫死台北**，不用 `toLocaleString` 現算（backend#3053 的稽查）：
	// 現算等於「用實作驗實作」，而且它會跟著跑測試那台機器的時區走 ——
	// 同一支測試在 JST 的筆電上綠、在 UTC 的 CI 上紅。
	// REVOKED_AT = 2026-08-14T12:39:29Z ⇒ 台北 2026/08/14 20:39:29。
	assert.match(html, /2026\/08\/14 20:39:29/u);
	assert.match(html, /撤銷紀錄/u);
	assert.match(html, /adm-audit-revocation/u);
});

test("③ 未撤銷的工作階段不得多出一塊空白區", async () => {
	const html = renderConsole(await seedSummary(SESSION_SUMMARY));

	// 摘要區本身有渲染（否則下面三條是因為整頁沒渲染而通過的，那是假綠）。
	assert.match(html, /工作階段摘要/u);
	assert.match(html, /OPS-1KXP05J5SRQPS/u);

	assert.doesNotMatch(html, /adm-audit-revocation/u);
	assert.doesNotMatch(html, /撤銷紀錄/u);
	// 佔位字串也不得出現：一塊寫著「未記錄撤銷理由」的卡片同樣是多出來的空白區。
	assert.doesNotMatch(html, /未記錄撤銷理由|未記錄操作者/u);
});

/**
 * gateway 還沒帶這四欄的那段窗口（ADM 與 gateway 各自 promote）。
 *
 * 判準若寫成 `status === "revoked"`，這一態會印出「未記錄撤銷理由」—— 那是假的，
 * 理由一直都在 DB 裡，只是這一版的 gateway 還沒下發。宣稱「沒有紀錄」比不說更糟：
 * 它會讓看的人停止追查。
 */
test("gateway 尚未下發四欄時整塊不渲染，不得宣稱「未記錄」", async () => {
	const legacy = { ...REVOKED_SUMMARY } as Record<string, unknown>;
	for (const key of [
		"revokedAt",
		"revokedByAdminId",
		"revokedByAdminDisplayName",
		"revokeReason",
	]) {
		delete legacy[key];
	}
	const html = renderConsole(await seedSummary(legacy));

	assert.match(html, /工作階段摘要/u);
	// 狀態徽章照常說「已撤銷」，但不會多出一句沒有根據的「未記錄撤銷理由」。
	assert.match(html, /已撤銷/u);
	assert.doesNotMatch(html, /adm-audit-revocation/u);
	assert.doesNotMatch(html, /未記錄撤銷理由/u);
});
