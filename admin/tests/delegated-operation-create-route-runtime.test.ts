import assert from "node:assert/strict";
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

// 兌換去向在 module 載入時解析，因此必須早於 route 的 import（動態載入）。
const WEB_ORIGIN = "https://web-dev.example.com";
process.env.WEB_ORIGIN = WEB_ORIGIN;

async function loadRoute() {
	return (await import("../src/app/api/admin/delegated-operations/route")).POST;
}

const realFetch = globalThis.fetch;
const realDateNow = Date.now;

const admOrigin = "http://localhost:3001";
const admHost = "localhost:3001";
const storeId = "store-123";
const reasonCode = "support-request";
const caseReference = "CASE-123";
const idempotencyKey = "runtime-test-key-123456";
const handoffCode = "dhof_9tJ0mQx7Kd2ZfVb3Rn8sYuC1Ea6Ph4Lw5Ti0Og2NkJ8";
const sessionId = "123456789";
const issuedAt = "2026-08-06T01:00:00.000Z";
const expiresAt = "2026-08-06T01:10:00.000Z";
const initialNow = Date.parse(issuedAt) + 1_000;

const target = {
	storeId,
	sellerAccountId: "seller-456",
	storeName: "測試商店",
};
const actor = { adminId: "admin-789", displayName: "ADM 測試員" };
const reason = {
	code: reasonCode,
	label: "客服案件處理",
	caseReferenceRequired: true,
};
const capabilities = [
	"storefront.read",
	"products.read",
	"orders.read",
] as const;

const preflight = {
	allowed: true,
	target,
	actor,
	reasonOptions: [reason],
	capabilities,
	ttlSeconds: 600,
	idleSeconds: 900,
	caseReferenceRequired: true,
};

const session = {
	sessionId,
	target,
	actor,
	reason,
	caseReference,
	capabilities,
	issuedAt,
	expiresAt,
	status: "active",
	revision: 1,
};

type RuntimeScenario = {
	name: string;
	createResponse?: () => Response;
	summaryResponse: () => Response;
	/** 成功＝導到 website 兌換；失敗＝導回商家頁並帶原因碼。 */
	expectedError: string | null;
};

function jsonResponse(data: unknown, status = 200): Response {
	return Response.json(data, { status });
}

function request(overrides: Record<string, string> = {}): NextRequest {
	const form = new URLSearchParams({
		storeId,
		reasonCode,
		caseReference,
		ttlSeconds: "900",
		idempotencyKey,
		...overrides,
	});
	return new NextRequest("http://localhost/api/admin/delegated-operations", {
		method: "POST",
		headers: {
			"Content-Type": "application/x-www-form-urlencoded",
			Origin: admOrigin,
			Host: admHost,
		},
		body: form.toString(),
	});
}

function createOk(): Response {
	return jsonResponse({
		code: 0,
		data: { handoffCode, session },
	});
}

test("create route reconciles upstream state before releasing the handoff code", async () => {
	const POST = await loadRoute();
	let fakeNow = initialNow;
	const scenarios: RuntimeScenario[] = [
		{
			name: "matching admin summary",
			summaryResponse: () => jsonResponse({ code: 0, data: { session } }),
			expectedError: null,
		},
		{
			name: "redeemed replay returns the session without a handoff code",
			createResponse: () => jsonResponse({ code: 0, data: { session } }),
			summaryResponse: () => jsonResponse({ code: 0, data: { session } }),
			expectedError: "DELEGATED_OPERATION_ALREADY_REDEEMED",
		},
		{
			name: "redeemed replay with an empty handoff code",
			createResponse: () =>
				jsonResponse({ code: 0, data: { handoffCode: "", session } }),
			summaryResponse: () => jsonResponse({ code: 0, data: { session } }),
			expectedError: "DELEGATED_OPERATION_ALREADY_REDEEMED",
		},
		{
			name: "create response includes credential",
			createResponse: () =>
				jsonResponse({
					code: 0,
					data: { handoffCode, session, operationCredential: "forbidden" },
				}),
			summaryResponse: () => jsonResponse({ code: 0, data: { session } }),
			expectedError: "INVALID_DELEGATED_OPERATION_CONTRACT",
		},
		{
			name: "admin summary session mismatch",
			summaryResponse: () =>
				jsonResponse({
					code: 0,
					data: { session: { ...session, revision: 2 } },
				}),
			expectedError: "DELEGATED_OPERATION_RECONCILIATION_MISMATCH",
		},
		{
			name: "admin summary session ID mismatch",
			summaryResponse: () =>
				jsonResponse({
					code: 0,
					data: { session: { ...session, sessionId: "987654321" } },
				}),
			expectedError: "DELEGATED_OPERATION_RECONCILIATION_MISMATCH",
		},
		{
			name: "admin summary service 5xx",
			summaryResponse: () => jsonResponse({ code: "UPSTREAM_FAILURE" }, 503),
			expectedError: "DELEGATED_OPERATION_RECONCILIATION_FAILED",
		},
		{
			name: "admin summary endpoint terminal response",
			summaryResponse: () =>
				jsonResponse({ code: "DELEGATED_OPERATION_REVOKED" }, 410),
			expectedError: "DELEGATED_OPERATION_RECONCILIATION_FAILED",
		},
		{
			name: "admin summary session is expired",
			summaryResponse: () =>
				jsonResponse({
					code: 0,
					data: { session: { ...session, status: "expired" } },
				}),
			expectedError: "DELEGATED_OPERATION_RECONCILIATION_MISMATCH",
		},
		{
			name: "clock passes expiresAt after summary starts",
			summaryResponse: () => {
				fakeNow = Date.parse(expiresAt) + 1;
				return jsonResponse({ code: 0, data: { session } });
			},
			expectedError: "DELEGATED_OPERATION_RECONCILIATION_MISMATCH",
		},
	];

	try {
		for (const scenario of scenarios) {
			fakeNow = initialNow;
			Date.now = () => fakeNow;
			const calls: Array<{ url: string; init?: RequestInit }> = [];
			globalThis.fetch = async (input, init) => {
				calls.push({ url: String(input), init });
				switch (calls.length) {
					case 1:
						return jsonResponse({ code: 0, data: preflight });
					case 2:
						return (scenario.createResponse ?? createOk)();
					case 3:
						return scenario.summaryResponse();
					default:
						throw new Error(`unexpected upstream call ${calls.length}`);
				}
			};

			const response = await POST(request());
			assert.equal(response.status, 303, scenario.name);
			assert.equal(
				response.headers.get("set-cookie"),
				null,
				`${scenario.name}: the credential never becomes a cookie on this host`,
			);
			const location = response.headers.get("location") ?? "";
			if (scenario.expectedError === null) {
				assert.equal(
					location,
					`${WEB_ORIGIN}/adm/enter?code=${handoffCode}`,
					scenario.name,
				);
			} else {
				const target = new URL(location, admOrigin);
				assert.equal(
					target.pathname,
					`/admin/merchants/${storeId}`,
					`${scenario.name}: failures land back on the store`,
				);
				assert.equal(
					target.searchParams.get("delegation_error"),
					scenario.expectedError,
					scenario.name,
				);
				assert.equal(
					target.searchParams.get("delegation_key"),
					scenario.expectedError === "DELEGATED_OPERATION_ALREADY_REDEEMED"
						? null
						: idempotencyKey,
					`${scenario.name}: only an unredeemed request keeps its retry key`,
				);
				assert.doesNotMatch(
					location,
					/dhof_|operation-credential/u,
					`${scenario.name}: no credential material in a failure Location`,
				);
			}

			const expectedCalls = scenario.createResponse ? 2 : 3;
			assert.equal(
				calls.length,
				expectedCalls,
				`${scenario.name}: exact call count`,
			);
			assert.deepEqual(
				calls.map(({ url }) => new URL(url).pathname),
				[
					`/api/v1/admin/sellers/${storeId}/delegated-operation/preflight`,
					"/api/v1/admin/delegated-operations",
					`/api/v1/admin/delegated-operations/${sessionId}/summary`,
				].slice(0, expectedCalls),
				`${scenario.name}: fixed preflight -> create -> admin summary order`,
			);
			assert.deepEqual(
				calls.map(({ init }) => init?.method ?? "GET"),
				["GET", "POST", "GET"].slice(0, expectedCalls),
				`${scenario.name}: upstream methods`,
			);

			const createHeaders = new Headers(calls[1]?.init?.headers);
			assert.equal(createHeaders.get("idempotency-key"), idempotencyKey);
			const createBody = JSON.parse(String(calls[1]?.init?.body));
			assert.deepEqual(createBody.capabilities, capabilities);
			// The browser asked for 900s; the preflight ceiling wins.
			assert.equal(createBody.ttlSeconds, preflight.ttlSeconds);
			if (expectedCalls === 3) {
				const summaryHeaders = new Headers(calls[2]?.init?.headers);
				assert.equal(
					summaryHeaders.get("authorization"),
					"Bearer qa-admin-session-token",
				);
				assert.equal(summaryHeaders.get("x-hk-operation-session"), null);
			}
		}
	} finally {
		globalThis.fetch = realFetch;
		Date.now = realDateNow;
	}
});

/**
 * 頂層表單提交送不出自訂標頭，原本靠 `Idempotency-Key` 標頭順帶擋下的跨站
 * 提交改由 Origin 檢查承擔。少了它，任何站點都能用一張表單替已登入的管理員
 * 建立工作階段（`SameSite=Lax` 對頂層 POST 不送 cookie，但那是第二層）。
 */
test("跨站送出在打任何上游之前就被拒絕", async () => {
	const POST = await loadRoute();
	const calls: string[] = [];
	globalThis.fetch = async (input) => {
		calls.push(String(input));
		return jsonResponse({ code: 0, data: preflight });
	};
	try {
		const cases: Record<string, string>[] = [
			{ Origin: "https://evil.example", Host: admHost },
			{ Host: admHost },
			{ Origin: admOrigin },
		];
		for (const headers of cases) {
			const response = await POST(
				new NextRequest("http://localhost/api/admin/delegated-operations", {
					method: "POST",
					headers: {
						"Content-Type": "application/x-www-form-urlencoded",
						...headers,
					},
					body: "storeId=store-123",
				}),
			);
			assert.equal(response.status, 403);
		}
		assert.deepEqual(calls, [], "no upstream call on a rejected origin");
	} finally {
		globalThis.fetch = realFetch;
	}
});

test("沒有 ADM token 時回 401，且不打任何上游", async () => {
	adminToken = null;
	const POST = await loadRoute();
	const calls: string[] = [];
	globalThis.fetch = async (input) => {
		calls.push(String(input));
		return jsonResponse({ code: 0, data: preflight });
	};
	try {
		const response = await POST(
			new NextRequest("http://localhost/api/admin/delegated-operations", {
				method: "POST",
				headers: {
					"Content-Type": "application/x-www-form-urlencoded",
					Origin: admOrigin,
					Host: admHost,
				},
				body: "storeId=store-123",
			}),
		);
		assert.equal(response.status, 401);
		assert.equal(response.headers.get("location"), null);
		assert.deepEqual(calls, []);
	} finally {
		globalThis.fetch = realFetch;
	}
});
