import assert from "node:assert/strict";
import test from "node:test";
import {
	encodeGenericBffPath,
	isAllowedGenericBffRequestHeader,
	isDelegatedOperationNamespace,
	isReservedDelegatedOperationHeader,
} from "../src/features/delegated-operations/server/bff-boundary";

test("catch-all BFF rejects all delegated operation namespace variants", () => {
	for (const path of [
		["api", "v1", "admin", "delegated-operations"],
		["api", "v1", "admin", "sellers", "s1", "delegated-operation"],
		["api", "v1", "admin", "%64elegated-operations"],
		["api", "v1", "admin", "%2564elegated-operations"],
		["api", "v1", "admin", "%252564elegated-operations"],
		["api", "v1", "admin", "%25252564elegated-operations"],
		["api", "v1", "admin", "delegated-operation%2Fpreflight"],
		["api", "v1", "admin", "delegated-operations%3F"],
		["api", "v1", "admin", "delegated-operations%23fragment"],
		["api", "v1", "admin", "delegated-operations%09"],
		["api", "v1", "admin", "delegated-operations%0A"],
		["api", "v1", "admin", "delegated-operations%0D"],
		["api", "v1", "admin", "delegated-operations%20"],
		["api", "v1", "admin", "delegated-operations "],
		["api", "v1", "admin", ".."],
		["api", "v1", "admin", "%"],
	]) {
		assert.equal(isDelegatedOperationNamespace(path), true, path.join("/"));
	}
	assert.equal(
		isDelegatedOperationNamespace(["api", "v1", "admin", "sellers"]),
		false,
	);
	let overEncoded = "%64elegated-operations";
	for (let pass = 0; pass < 32; pass += 1) {
		overEncoded = overEncoded.replaceAll("%", "%25");
	}
	assert.equal(
		isDelegatedOperationNamespace(["api", "v1", "admin", overEncoded]),
		true,
		"decode work over the fixed budget must fail closed",
	);
});

test("catch-all BFF re-encodes path segments before constructing the target URL", () => {
	assert.equal(
		encodeGenericBffPath(["api", "v1", "delegated-operations?"]),
		"api/v1/delegated-operations%3F",
	);
});

test("catch-all BFF forwards representation metadata but not browser identity", () => {
	for (const header of [
		"Accept",
		"Content-Type",
		"Idempotency-Key",
		"If-Match",
		"Range",
		"Sentry-Trace",
	]) {
		assert.equal(isAllowedGenericBffRequestHeader(header), true, header);
	}
	for (const header of [
		"Authorization",
		"Cookie",
		// 跨來源的防護在後台自己的 middleware（src/lib/cross-origin.ts）；後端收到的是伺服器的請求，
		// 轉送瀏覽器的 Origin 只會讓後端多信任一個 origin。
		"Origin",
		"X-Requested-With",
		// 後台沒有視角（ADR 2026-10-07-connect-without-perspective）：BFF 不轉送 X-Active-Role，
		// 單體對後台身分只看 Cloudflare Access 的 JWT，不看 client 的工作區提示。
		"X-Active-Role",
		"x-active-role",
		"X-Admin-Actor-Id",
		"X-HK-Operation-Session",
	]) {
		assert.equal(isAllowedGenericBffRequestHeader(header), false, header);
	}
});

test("catch-all BFF rejects operation and identity-context headers case-insensitively", () => {
	for (const header of [
		"X-HK-Operation-Session",
		"x-hk-operation-revision",
		"X-Delegated-Operation-Id",
		"X-Active-Store",
		"X-Target-Seller-Id",
		"X-Admin-Actor-Id",
		"X-Operation-Session",
		"X-Impersonated-User-Id",
		"X-HK-Origin-Auth",
		"x-hk-origin-auth",
	]) {
		assert.equal(isReservedDelegatedOperationHeader(header), true, header);
	}
	assert.equal(isReservedDelegatedOperationHeader("X-Requested-With"), false);
	assert.equal(isReservedDelegatedOperationHeader("Content-Type"), false);
});
