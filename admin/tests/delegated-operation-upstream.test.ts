import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import {
	adminUpstreamFetch,
	safeUpstreamError,
} from "../src/features/delegated-operations/server/upstream";

const realFetch = globalThis.fetch;

afterEach(() => {
	globalThis.fetch = realFetch;
});

test("upstream errors expose only allowlisted machine metadata", () => {
	assert.deepEqual(
		safeUpstreamError({
			code: "DELEGATED_OPERATION_DENIED",
			message: "internal secret: operation-credential-123",
			traceId: "trace-safe_123",
			operationCredential: "must-not-cross",
		}),
		{
			code: "DELEGATED_OPERATION_DENIED",
			message: "代行營運服務暫時無法使用",
			traceId: "trace-safe_123",
		},
	);
	assert.deepEqual(
		safeUpstreamError({
			code: "secret value with spaces",
			message: "credential",
			traceId: "unsafe trace with spaces",
		}),
		{
			code: "UPSTREAM_FAILURE",
			message: "代行營運服務暫時無法使用",
		},
	);
	assert.deepEqual(
		safeUpstreamError({
			code: 0,
			message: "success-shaped payload must not become an error success",
		}),
		{
			code: "UPSTREAM_FAILURE",
			message: "代行營運服務暫時無法使用",
		},
	);
});

test("server transport constructs its own narrow headers and ignores browser header-shaped input", async () => {
	let captured: RequestInit | undefined;
	globalThis.fetch = async (_input, init) => {
		captured = init;
		return new Response(JSON.stringify({ code: 0 }), { status: 200 });
	};

	await adminUpstreamFetch("/api/v1/admin/delegated-operations", {
		method: "POST",
		adminToken: "access-admin-jwt",
		idempotencyKey: "018f4e24-a111-7777-8888-123456789abc", // gitleaks:allow（UUID 形式的冪等鍵範例）
		body: { storeId: "store_123" },
		// Deliberately not part of AdminUpstreamRequest. Runtime callers cannot
		// smuggle these into the constructed Headers object either.
		browserHeaders: {
			Authorization: "Bearer attacker",
			"X-Admin-Actor-Id": "attacker",
			"X-HK-Operation-Session": "attacker",
		},
	} as Parameters<typeof adminUpstreamFetch>[1] & {
		browserHeaders: Record<string, string>;
	});

	const headers = new Headers(captured?.headers);
	// 管理員的身分由 server 端轉送；瀏覽器形狀的 Authorization 進不來，cookie 一個都不帶。
	assert.equal(headers.get("authorization"), "Bearer access-admin-jwt");
	assert.equal(headers.get("cookie"), null);
	assert.equal(headers.get("x-hk-operation-session"), null);
	assert.equal(
		headers.get("idempotency-key"),
		"018f4e24-a111-7777-8888-123456789abc",
	);
	assert.equal(headers.get("x-admin-actor-id"), null);
	assert.equal(headers.get("x-requested-with"), "XMLHttpRequest");
});
