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
	revokedByAdminDisplayName: "ops-admin@example.com",
	revokeReason: REVOKE_REASON,
};

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
		{ ...REVOKED_SUMMARY, revokedAt: "2026-08-14T20:39:29+08:00" },
		{ ...REVOKED_SUMMARY, revokedAt: 1_755_175_169 },
		{ ...REVOKED_SUMMARY, revokedByAdminId: 2_661_027_599 },
		{ ...REVOKED_SUMMARY, revokedByAdminDisplayName: { email: "x@y.z" } },
		{ ...REVOKED_SUMMARY, revokeReason: "超".repeat(201) },
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
	const second = parseDelegatedSessionReport(first);
	assert.deepEqual(second, first);
});

test("撤銷者那一行：全形括號結尾不再補半形空格", () => {
	const line = sessionRevokedByText(
		REVOKED_AT,
		"ops-admin@example.com",
		"2661027599",
	);
	assert.match(line, /由 ops-admin@example\.com（2661027599）撤銷$/u);
	assert.doesNotMatch(line, /）\s撤銷/u);
});

test("撤銷者缺席時說得出缺了什麼，不是省略整句", () => {
	assert.match(
		sessionRevokedByText(REVOKED_AT, null, "2661027599"),
		/由 管理員（2661027599）撤銷$/u,
	);
	assert.match(
		sessionRevokedByText(REVOKED_AT, "ops-admin@example.com", null),
		/由 ops-admin@example\.com 撤銷$/u,
	);
	assert.match(
		sessionRevokedByText(REVOKED_AT, null, null),
		/由 未記錄操作者撤銷$/u,
	);
});

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

	assert.match(
		html,
		new RegExp(REVOKE_REASON, "u"),
		`撤銷理由沒有渲染出來：${html.slice(0, 1200)}`,
	);
	assert.match(html, /ops-admin@example\.com/u);
	assert.match(html, /2661027599/u);
	assert.match(html, /2026\/08\/14 20:39:29/u);
	assert.match(html, /撤銷紀錄/u);
	assert.match(html, /adm-audit-revocation/u);
});

test("③ 未撤銷的工作階段不得多出一塊空白區", async () => {
	const html = renderConsole(await seedSummary(SESSION_SUMMARY));

	assert.match(html, /工作階段摘要/u);
	assert.match(html, /OPS-1KXP05J5SRQPS/u);

	assert.doesNotMatch(html, /adm-audit-revocation/u);
	assert.doesNotMatch(html, /撤銷紀錄/u);
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
	assert.match(html, /已撤銷/u);
	assert.doesNotMatch(html, /adm-audit-revocation/u);
	assert.doesNotMatch(html, /未記錄撤銷理由/u);
});
