import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";
import {
	buildDelegatedSessionSearch,
	delegatedSessionListQueryKey,
	isDelegatedDisplayCode,
	parseDelegatedOperationSessionPage,
	resolveDelegatedLookup,
} from "../src/features/delegated-operations/governance";

/**
 * 代行紀錄入口頁的「工作階段列表」（56 §11.3 P1，gateway
 * `GET /api/v1/admin/delegated-operations`）。
 *
 * 這一頁在後端交付列表端點之前只是一個查詢框，檔頭寫著「因為本期沒有工作階段列表
 * 端點」。本檔守的是那句話不會以另一種形式回來 —— 三種形式各要能單獨被打紅：
 *
 *   ① 列表**真的由資料渲染**（不是渲染得出來但永遠是空的）
 *   ② 查詢框收 `OPS-` 代號，不是只收數字 ID
 *   ③ BFF 的身分是 ADM session，不吃 operation credential
 */

const SESSION_RECORD = {
	sessionId: "7494008076136087552",
	displayCode: "OPS-1KXP05J5SRQPS",
	target: {
		storeId: "2685914682",
		sellerAccountId: "2685914682",
		storeName: "梨山盛花園",
	},
	actor: { adminId: "2661027599", displayName: "alexischen" },
	reason: {
		code: "seller_request",
		label: "賣家請求",
		caseReferenceRequired: false,
	},
	caseReference: "HK-2026-0814",
	capabilities: ["storefront.read", "storefront.write"],
	issuedAt: "2026-08-14T12:32:37Z",
	expiresAt: "2026-08-14T12:42:37Z",
	endedAt: null,
	status: "active",
	revokedAt: null,
	revokedByAdminId: null,
	revokedByAdminDisplayName: null,
	revokeReason: null,
};

const SESSION_PAGE = {
	records: [SESSION_RECORD],
	total: "1",
	page: "1",
	pageSize: "20",
};


test("列表 parser 收下整列治理欄位，並保留後端推導的 status", () => {
	const parsed = parseDelegatedOperationSessionPage(SESSION_PAGE);
	assert.ok(parsed);
	assert.equal(parsed.total, 1);
	const [record] = parsed.records;
	assert.equal(record.displayCode, "OPS-1KXP05J5SRQPS");
	assert.equal(record.reason.label, "賣家請求");
	assert.equal(record.caseReference, "HK-2026-0814");
	assert.deepEqual(record.capabilities, [
		"storefront.read",
		"storefront.write",
	]);
	assert.equal(record.status, "active");
	assert.equal(record.endedAt, null);
});

test("列表 parser 是冪等的：同一份資料會被 BFF 與瀏覽器各解析一次", () => {
	const first = parseDelegatedOperationSessionPage(SESSION_PAGE);
	assert.ok(first);
	const second = parseDelegatedOperationSessionPage(first);
	assert.deepEqual(second, first);
});

test("已撤銷的列：撤銷者與理由一併解析得出", () => {
	const parsed = parseDelegatedOperationSessionPage({
		...SESSION_PAGE,
		records: [
			{
				...SESSION_RECORD,
				status: "revoked",
				endedAt: "2026-08-14T12:39:29Z",
				revokedAt: "2026-08-14T12:39:29Z",
				revokedByAdminId: "2661027599",
				revokedByAdminDisplayName: "alexischen",
				revokeReason: "誤選店家",
			},
		],
	});
	assert.ok(parsed);
	assert.equal(parsed.records[0].revokeReason, "誤選店家");
	assert.equal(parsed.records[0].revokedByAdminDisplayName, "alexischen");
});

test("單列不合契約時整頁失敗，不是靜默跳過那一列", () => {
	for (const broken of [
		{ ...SESSION_RECORD, status: "ended" },
		{ ...SESSION_RECORD, sessionId: "not-a-tsid" },
		{ ...SESSION_RECORD, displayCode: "OPS-1KXP05J5SRQ" },
		{ ...SESSION_RECORD, displayCode: "" },
		{ ...SESSION_RECORD, issuedAt: "2026-08-14T12:32:37+08:00" },
		{ ...SESSION_RECORD, expiresAt: null },
		{ ...SESSION_RECORD, reason: { code: "", label: "賣家請求" } },
		{ ...SESSION_RECORD, capabilities: "storefront.read" },
		{ ...SESSION_RECORD, target: { storeId: "2685914682" } },
	]) {
		assert.equal(
			parseDelegatedOperationSessionPage({
				...SESSION_PAGE,
				records: [SESSION_RECORD, broken],
				total: "2",
			}),
			null,
			`should fail closed: ${JSON.stringify(broken.displayCode ?? broken.status)}`,
		);
	}
});

test("分頁數字缺席或超上限一律失敗，空字串不得被當成 0 筆", () => {
	for (const patch of [
		{ total: "" },
		{ total: undefined },
		{ page: "0" },
		{ pageSize: "51" },
	]) {
		assert.equal(
			parseDelegatedOperationSessionPage({
				records: [],
				total: "0",
				page: "1",
				pageSize: "20",
				...patch,
			}),
			null,
		);
	}
	assert.ok(
		parseDelegatedOperationSessionPage({
			records: [],
			total: "0",
			page: "1",
			pageSize: "20",
		}),
	);
});

test("列表列與摘要是兩種形狀，不得互相冒充", () => {
	assert.equal(
		parseDelegatedOperationSessionPage({
			records: [
				{
					sessionId: SESSION_RECORD.sessionId,
					displayCode: SESSION_RECORD.displayCode,
					target: SESSION_RECORD.target,
					actor: SESSION_RECORD.actor,
					issuedAt: SESSION_RECORD.issuedAt,
					endedAt: null,
					durationSeconds: "327",
					status: "active",
					operationCount: "2",
				},
			],
			total: "1",
			page: "1",
			pageSize: "20",
		}),
		null,
	);
});


/**
 * 這一支是「查詢框的接受條件」的唯一判定處。把 `OPS-` 那一支拿掉就會紅 —— 而這正是
 * 修前的狀態：`isDelegatedSessionId` 只收 `^\d{1,19}$`，錯誤訊息還寫著「是一組數字」，
 * 於是治理台摘要卡上那個 `OPS-…`（唯一掛著複製鍵、最常被貼出去的值）貼回查詢框必被拒。
 */
test("② 查詢框兩種識別碼都收：數字跳詳情頁、OPS 代號套成列表篩選", () => {
	assert.deepEqual(resolveDelegatedLookup("7494008076136087552"), {
		kind: "session",
		sessionId: "7494008076136087552",
	});

	assert.deepEqual(resolveDelegatedLookup("OPS-1KXP05J5SRQPS"), {
		kind: "displayCode",
		displayCode: "OPS-1KXP05J5SRQPS",
	});

	assert.deepEqual(resolveDelegatedLookup("  ops-1kxp05j5srqps  "), {
		kind: "displayCode",
		displayCode: "OPS-1KXP05J5SRQPS",
	});

	for (const bad of [
		"",
		"   ",
		"OPS-1KXP05J5SRQ",
		"OPS-1KXP05J5SRQPSX",
		"OPS1KXP05J5SRQPS",
		"O005001417",
		"12345678901234567890",
		"7494008076136087552; DROP TABLE",
	]) {
		assert.deepEqual(
			resolveDelegatedLookup(bad),
			{ kind: "invalid" },
			`「${bad}」不該被當成合法的查詢輸入`,
		);
	}
});

test("代號值域與 gateway／DB CHECK 同一份形狀", () => {
	assert.equal(isDelegatedDisplayCode("OPS-1KXP05J5SRQPS"), true);
	for (const bad of [
		"ops-1kxp05j5srqps",
		"OPS-1KXP05J5SRQP",
		"OPS-1KXP05J5SRQP!",
		"",
		null,
	]) {
		assert.equal(isDelegatedDisplayCode(bad), false);
	}
});


test("篩選為空或格式不合時不送出該參數，且 page_size 夾在上限內", () => {
	const empty = buildDelegatedSessionSearch({ page: 1, pageSize: 20 });
	assert.equal(empty.get("page"), "1");
	assert.equal(empty.get("page_size"), "20");
	for (const key of [
		"actor_admin_id",
		"store_id",
		"display_code",
		"date_from",
		"date_to",
	]) {
		assert.equal(empty.has(key), false);
	}

	const filtered = buildDelegatedSessionSearch({
		page: 2,
		pageSize: 999,
		actorAdminId: "2661027599",
		storeId: "2685914682",
		displayCode: "OPS-1KXP05J5SRQPS",
		dateFrom: "2026-08-14T00:00:00Z",
	});
	assert.equal(filtered.get("actor_admin_id"), "2661027599");
	assert.equal(filtered.get("store_id"), "2685914682");
	assert.equal(filtered.get("display_code"), "OPS-1KXP05J5SRQPS");
	assert.equal(filtered.get("date_from"), "2026-08-14T00:00:00Z");
	assert.equal(filtered.get("page_size"), "50");

	const partial = buildDelegatedSessionSearch({
		page: 1,
		pageSize: 20,
		storeId: "26859146a",
		displayCode: "OPS-",
	});
	assert.equal(partial.has("store_id"), false);
	assert.equal(partial.has("display_code"), false);
});

test("query key 就是送出去的 query string，兩者不可能漂移", () => {
	const a = delegatedSessionListQueryKey({ page: 1, pageSize: 20 });
	const b = delegatedSessionListQueryKey({
		page: 1,
		pageSize: 20,
		displayCode: "OPS-1KXP05J5SRQPS",
	});
	assert.notDeepEqual(
		a,
		b,
		"換了篩選卻是同一把 key ⇒ 畫面會拿上一組條件的快取當新結果，而它看起來完全正常",
	);
	assert.equal(
		a[2],
		buildDelegatedSessionSearch({ page: 1, pageSize: 20 }).toString(),
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

const SessionListConsole = requireFromTest(
	"../src/features/delegated-operations/components/SessionListConsole",
).default as unknown;
const { listDelegatedOperationSessions } = requireFromTest(
	"../src/services/delegated-operations",
) as {
	listDelegatedOperationSessions: (query: unknown) => Promise<unknown>;
};

/** `useRouter()` 沒有 provider 時會直接丟例外，元件第一行就炸。 */
const ROUTER_STUB = {
	push: () => {},
	replace: () => {},
	refresh: () => {},
	back: () => {},
	forward: () => {},
	prefetch: () => {},
};

/** 元件首次渲染時送出的那一組條件。與 `SessionListConsole` 的初始 state 一致。 */
const FIRST_QUERY = {
	page: 1,
	pageSize: 20,
	actorAdminId: "",
	storeId: "",
	displayCode: "",
	dateFrom: "",
	dateTo: "",
};

/**
 * 把列表結果放進 cache，元件 render 時直接讀到它。
 *
 * 走的是**元件自己那條鏈**（`listDelegatedOperationSessions` → parser），只把
 * `fetch` 換掉：這樣一來 service 或 parser 壞掉時這支也會紅，而不是只守住 JSX。
 */
async function seedSessions(
	payload: unknown,
	status = 200,
): Promise<{ client: unknown; requested: string[] }> {
	const requested: string[] = [];
	const original = globalThis.fetch;
	globalThis.fetch = (async (input: unknown) => {
		requested.push(String(input));
		return new Response(JSON.stringify(payload), {
			status,
			headers: { "content-type": "application/json" },
		});
	}) as typeof fetch;
	try {
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false, retryOnMount: false } },
		});
		await client.prefetchQuery({
			queryKey: delegatedSessionListQueryKey(FIRST_QUERY),
			queryFn: () => listDelegatedOperationSessions(FIRST_QUERY),
		});
		return { client, requested };
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
				createElement(SessionListConsole, {}),
			),
		),
	);
}

test("① 列表由資料渲染：每一列帶代號、店家、操作者與進詳情頁的連結", async () => {
	const { client, requested } = await seedSessions({
		code: 0,
		data: {
			...SESSION_PAGE,
			records: [
				SESSION_RECORD,
				{
					...SESSION_RECORD,
					sessionId: "7494008076136087553",
					displayCode: "OPS-1KXP05J5SRQPT",
					target: {
						storeId: "2632049831",
						sellerAccountId: "2632049831",
						storeName: "陳記",
					},
					status: "revoked",
					endedAt: "2026-08-14T12:39:29Z",
					revokedAt: "2026-08-14T12:39:29Z",
				},
			],
			total: "2",
		},
	});

	assert.equal(requested.length, 1);
	assert.match(requested[0], /^\/api\/admin\/delegated-operations\?/u);

	const html = renderConsole(client);
	const rows = html.match(/<tr>[\s\S]*?<\/tr>/gu) ?? [];
	assert.equal(
		rows.length,
		3,
		`列表沒有渲染出兩列資料（實際 ${rows.length - 1} 列）：${html.slice(0, 800)}`,
	);

	assert.match(html, /OPS-1KXP05J5SRQPS/u);
	assert.match(html, /OPS-1KXP05J5SRQPT/u);
	assert.match(html, /梨山盛花園/u);
	assert.match(html, /陳記/u);
	assert.match(html, /alexischen/u);
	assert.match(html, /7494008076136087552/u);
	assert.match(
		html,
		/href="\/admin\/delegated-operations\/7494008076136087552"/u,
		`列沒有連到詳情頁：${html.slice(0, 800)}`,
	);
	assert.match(
		html,
		/href="\/admin\/delegated-operations\/7494008076136087553"/u,
	);

	assert.match(html, /查看店面內容/u);
	assert.match(html, /編輯店面內容/u);

	assert.match(html, /進行中/u);
	assert.match(html, /已撤銷/u);
});

test("① 真的沒有資料時說的是「還沒有任何代行工作階段」，不是空白表格", async () => {
	const { client } = await seedSessions({
		code: 0,
		data: { records: [], total: "0", page: "1", pageSize: "20" },
	});
	const html = renderConsole(client);
	assert.match(html, /平台上還沒有任何代行工作階段/u);
	assert.doesNotMatch(html, /顯示第/u);
});

test("① 上游壞掉時說得出是壞掉，不是顯示成一份空列表", async () => {
	const { client } = await seedSessions(
		{ code: 9280, message: "代行營運服務暫時無法使用" },
		502,
	);
	const html = renderConsole(client);
	assert.doesNotMatch(html, /平台上還沒有任何代行工作階段/u);
	assert.match(html, /代行營運服務暫時無法使用/u);
});


test("入口頁掛的是列表本體，不是一個自己持有查詢狀態的查詢框", async () => {
	const page = await readFile(
		"src/app/admin/delegated-operations/page.tsx",
		"utf8",
	);
	assert.match(page, /<SessionListConsole/u);
	assert.doesNotMatch(
		page,
		/useState/u,
		"入口頁又自己持有查詢狀態 ⇒ 退回只有查詢框的那一版",
	);
	assert.doesNotMatch(page, /<form/u);

	const console_ = await readFile(
		"src/features/delegated-operations/components/SessionListConsole.tsx",
		"utf8",
	);
	assert.match(console_, /resolveDelegatedLookup\(/u);
	assert.doesNotMatch(
		console_.replace(/\/\*[\s\S]*?\*\//gu, ""),
		/OPS-\[0-9A-Z\]/u,
		"元件內自建了第二份代號正則 ⇒ 兩份值域會漂移",
	);
});

test("③ 列表 BFF 的身分是 ADM session，不吃 operation credential", async () => {
	const [route, upstream] = await Promise.all([
		readFile("src/app/api/admin/delegated-operations/route.ts", "utf8"),
		readFile("src/features/delegated-operations/server/upstream.ts", "utf8"),
	]);
	assert.match(route, /export async function GET\(/u);
	assert.match(route, /adminSession\.token\(request\)/u);
	const getBody = route.slice(
		route.indexOf("export async function GET("),
		route.indexOf("export async function POST("),
	);
	assert.doesNotMatch(getBody, /operationSession/u);
	assert.match(getBody, /DELEGATED_OPERATION_UPSTREAM\.sessions/u);
	assert.match(upstream, /sessions: SESSIONS_PATH/u);
	assert.match(
		upstream,
		/const SESSIONS_PATH = "\/api\/v1\/admin\/delegated-operations"/u,
	);
});
