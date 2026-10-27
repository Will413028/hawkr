import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { afterEach, beforeEach, mock } from "node:test";
import { NextRequest } from "next/server";
import { adminSession } from "../src/lib/session";

/** Access JWT（測試以 mock 取代來源）；null 是沒有憑證。 */
let adminToken: string | null = "qa-admin-session-token";
beforeEach(() => {
	adminToken = "qa-admin-session-token";
	mock.method(adminSession, "token", async () => adminToken);
});
afterEach(() => {
	mock.restoreAll();
});

/**
 * 建立代行工作階段時的 404 有兩種來源，而它們的下一步相反：
 *
 *   ① 路由不存在 → 這個環境沒有代行營運服務，操作者該找維運
 *   ② 9286「這家店從來沒有授權列」→ 該去商家詳情頁按開通
 *
 * 後端把 preflight 的拒絕碼原樣透傳（account-service `session.go`），而 9286 在
 * gateway 的狀態表上就是 404（`bizhttp.go`）。BFF 若以 status 分派，②會被講成①，
 * 管理員去找維運等一個沒有壞掉的服務。
 *
 * 本檔守的就是「以碼分派」：三個情境的 `delegation_error` 必須兩兩不同，而不只是
 * 各自「有值」——後者在把全部 404 壓成同一個碼時照樣成立。
 */

const WEB_ORIGIN = "https://web-dev.example.com";
process.env.WEB_ORIGIN = WEB_ORIGIN;

async function loadRoute() {
	return (await import("../src/app/api/admin/delegated-operations/route")).POST;
}

const realFetch = globalThis.fetch;

const admOrigin = "http://localhost:3001";
const admHost = "localhost:3001";
const storeId = "store-123";
const reasonCode = "support-request";
const idempotencyKey = "denial-code-test-key-123456";

const target = {
	storeId,
	sellerAccountId: "seller-456",
	storeName: "測試商店",
};

const preflight = {
	allowed: true,
	target,
	actor: { adminId: "admin-789", displayName: "ADM 測試員" },
	reasonOptions: [
		{ code: reasonCode, label: "客服案件處理", caseReferenceRequired: false },
	],
	capabilities: ["storefront.read", "products.read", "orders.read"],
	ttlSeconds: 600,
	idleSeconds: 900,
	caseReferenceRequired: false,
};

function jsonResponse(data: unknown, status = 200): Response {
	return Response.json(data, { status });
}

function request(): NextRequest {
	return new NextRequest("http://localhost/api/admin/delegated-operations", {
		method: "POST",
		headers: {
			"Content-Type": "application/x-www-form-urlencoded",
			Origin: admOrigin,
			Host: admHost,
		},
		body: new URLSearchParams({
			storeId,
			reasonCode,
			ttlSeconds: "600",
			idempotencyKey,
		}).toString(),
	});
}

async function startFailureCodeFor(create: Response): Promise<string | null> {
	const POST = await loadRoute();
	let calls = 0;
	globalThis.fetch = async () => {
		calls += 1;
		if (calls === 1) return jsonResponse({ code: 0, data: preflight });
		if (calls === 2) return create;
		throw new Error(`unexpected upstream call ${calls}`);
	};
	try {
		const response = await POST(request());
		assert.equal(response.status, 303);
		const location = new URL(response.headers.get("location") ?? "", admOrigin);
		assert.equal(location.pathname, `/admin/merchants/${storeId}`);
		return location.searchParams.get("delegation_error");
	} finally {
		globalThis.fetch = realFetch;
	}
}

test("建立端點的 404 以業務碼分派，不以 HTTP status 分派", async () => {
	const neverGranted = await startFailureCodeFor(
		jsonResponse(
			{ code: 9286, message: "找不到此店家的代行營運授權設定" },
			404,
		),
	);
	const gone = await startFailureCodeFor(
		jsonResponse(
			{ code: 9264, message: "此店家的代行營運授權已撤回或已到期" },
			404,
		),
	);
	const routeMissing = await startFailureCodeFor(
		new Response("<!doctype html>not found", {
			status: 404,
			headers: { "Content-Type": "text/html" },
		}),
	);

	assert.equal(neverGranted, "9286");
	assert.equal(gone, "9264");
	assert.equal(routeMissing, "DELEGATED_OPERATION_UNAVAILABLE");

	// 三者兩兩不同才是這支測試的重點：把全部 404 壓成同一個碼時，上面三條
	// 「各自有值」的斷言照樣可以寫成成立的樣子。
	assert.equal(
		new Set([neverGranted, gone, routeMissing]).size,
		3,
		"三種 404 必須帶回三個不同的原因碼",
	);
});

/**
 * 碼帶回來之後還要被譯成句子，而那份對照是 `authorization.ts` 的
 * `DELEGATION_AUTHORIZATION_ERROR_MESSAGES`（治理面板與代行入口共用一份）。
 * 這一支把元件真的渲染出來，確認入口那邊沒有繞過它自己寫一份。
 */
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
	QueryClient: new (options?: unknown) => unknown;
	QueryClientProvider: unknown;
};
const { SearchParamsContext } = requireFromTest(
	"next/dist/shared/lib/hooks-client-context.shared-runtime",
) as { SearchParamsContext: { Provider: unknown } };
const MerchantOperationEntry = requireFromTest(
	"../src/features/delegated-operations/components/MerchantOperationEntry",
).default as unknown;

function renderStartError(code: string): string {
	const search = new URLSearchParams({
		delegation_store: storeId,
		delegation_error: code,
		delegation_key: idempotencyKey,
	});
	return renderToStaticMarkup(
		createElement(
			SearchParamsContext.Provider,
			{ value: search },
			createElement(
				QueryClientProvider,
				{
					client: new QueryClient({
						defaultOptions: { queries: { retry: false } },
					}),
				},
				createElement(MerchantOperationEntry, { storeId, enabled: true }),
			),
		),
	);
}

test("代行入口把 9286 與 9264 譯成兩句不同的話", () => {
	const neverGranted = renderStartError("9286");
	const gone = renderStartError("9264");

	assert.match(neverGranted, /尚未開通/u);
	assert.doesNotMatch(
		neverGranted,
		/此環境尚未啟用代行營運服務/u,
		"「這家店沒開通」不得被講成「整個環境沒有代行服務」",
	);
	assert.match(gone, /已撤回或已到期/u);
	assert.match(gone, /重新開通/u);

	// 兩句話必須不同。相同 ⇒ 管理員無從得知該按「開通」還是「重新開通」，
	// 而上面兩組 match 在共用同一句籠統文案時仍可能各自成立。
	assert.notEqual(
		neverGranted,
		gone,
		"9286 與 9264 的畫面文字相同＝一碼兩義又回來了",
	);
});
