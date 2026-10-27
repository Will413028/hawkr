import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { middleware } from "../src/middleware";

const OWN = "https://admin.example.test";

function run(
	path: string,
	init: { method?: string; headers?: Record<string, string> } = {},
) {
	return middleware(new NextRequest(`${OWN}${path}`, init));
}

test("跨來源的 /api 寫入回 403", () => {
	const blocked = run("/api/bff/api/v1/admin/banners", {
		method: "POST",
		headers: { "sec-fetch-site": "same-site" },
	});
	assert.equal(blocked.status, 403);
});

test("同源的 /api 寫入放行", () => {
	const res = run(
		"/api/bff/hawkr.admin.identity.v1alpha1.IdentityAdminService/SignOut",
		{ method: "POST", headers: { "sec-fetch-site": "same-origin" } },
	);
	assert.notEqual(res.status, 403);
	assert.equal(res.headers.get("location"), null);
});

test("頁面與靜態資源不在 middleware 判斷身分", () => {
	for (const path of ["/admin/members", "/brand/logo-mark.svg"]) {
		const res = run(path);
		assert.equal(res.headers.get("location"), null, path);
		assert.ok(res.status < 300, `${path}: ${res.status}`);
	}
});
