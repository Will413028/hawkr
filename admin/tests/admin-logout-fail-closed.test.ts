import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { AuthService } from "../src/services/auth";

const SIGN_OUT_URL =
	"/api/bff/hawkr.admin.identity.v1alpha1.IdentityAdminService/SignOut";
const LOGOUT_ROUTE = "/api/admin/logout";

type Window = { location: { href: string } };
const win = () => (globalThis as unknown as { window: Window }).window;

let alerts: string[];
let requests: { url: string; method?: string }[];
let signOutAnswer: { status: number; code?: string; reject?: boolean };

beforeEach(() => {
	alerts = [];
	requests = [];
	signOutAnswer = { status: 200 };
	(globalThis as Record<string, unknown>).window = {
		location: { href: "/admin" },
	};
	(globalThis as Record<string, unknown>).alert = (msg: string) => {
		alerts.push(msg);
	};
	globalThis.fetch = async (input, init) => {
		requests.push({ url: String(input), method: init?.method });
		if (signOutAnswer.reject) throw new TypeError("fetch failed");
		const body =
			signOutAnswer.status === 200
				? {}
				: { code: signOutAnswer.code, message: "developer text" };
		return new Response(JSON.stringify(body), {
			status: signOutAnswer.status,
			headers: { "content-type": "application/json" },
		});
	};
});

const realFetch = globalThis.fetch;

afterEach(() => {
	globalThis.fetch = realFetch;
	delete (globalThis as Record<string, unknown>).window;
	delete (globalThis as Record<string, unknown>).alert;
});

test("SignOut 成功之後才導向 server logout route", async () => {
	await AuthService.logout();
	assert.deepEqual(requests, [{ url: SIGN_OUT_URL, method: "POST" }]);
	assert.equal(win().location.href, LOGOUT_ROUTE);
	assert.deepEqual(alerts, []);
});

for (const failure of [
	{ name: "後端不可用", answer: { status: 503, code: "unavailable" } },
	{ name: "連不到後端", answer: { status: 0, reject: true } },
	{ name: "後端內部錯誤", answer: { status: 500, code: "internal" } },
]) {
	test(`SignOut 失敗（${failure.name}）時留在原頁`, async () => {
		signOutAnswer = failure.answer;
		await AuthService.logout();
		assert.equal(win().location.href, "/admin");
		assert.deepEqual(alerts, ["登出失敗，請再試一次"]);
	});
}

for (const ended of [
	{ name: "token 無效", answer: { status: 401, code: "unauthenticated" } },
	{ name: "權限不足", answer: { status: 403, code: "permission_denied" } },
]) {
	test(`SignOut ${ended.name} 時導向 server logout route`, async () => {
		signOutAnswer = ended.answer;
		await AuthService.logout();
		assert.equal(win().location.href, LOGOUT_ROUTE);
		assert.deepEqual(alerts, []);
	});
}

test("session 失效時同時失敗的多個呼叫只嘗試 SignOut 一次", async () => {
	AuthService.sessionExpired();
	AuthService.sessionExpired();
	AuthService.sessionExpired();
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(requests, [{ url: SIGN_OUT_URL, method: "POST" }]);
	assert.equal(win().location.href, LOGOUT_ROUTE);
});

test("sessionExpired 遇到 SignOut 401 仍導向 server logout route", async () => {
	signOutAnswer = { status: 401, code: "unauthenticated" };
	AuthService.sessionExpired();
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(requests, [{ url: SIGN_OUT_URL, method: "POST" }]);
	assert.equal(win().location.href, LOGOUT_ROUTE);
	assert.deepEqual(alerts, []);
});

test("sessionExpired 遇到 SignOut 503 保留原頁與登出失敗訊息", async () => {
	signOutAnswer = { status: 503, code: "unavailable" };
	AuthService.sessionExpired();
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.equal(win().location.href, "/admin");
	assert.deepEqual(alerts, ["登出失敗，請再試一次"]);
});

test("REST 的 401 觸發 SignOut", async () => {
	globalThis.fetch = async (input, init) => {
		const url = String(input);
		requests.push({ url, method: init?.method });
		return url === SIGN_OUT_URL
			? new Response("{}", {
					status: 200,
					headers: { "content-type": "application/json" },
				})
			: new Response('{"code":401}', { status: 401 });
	};
	const { request } = await import("../src/services/api");
	await assert.rejects(request("/api/v1/admin/buyers/stats"));
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(
		requests.map((r) => r.url),
		["/api/bff/api/v1/admin/buyers/stats", SIGN_OUT_URL],
	);
	assert.equal(win().location.href, LOGOUT_ROUTE);
});

test("REST 的 401 與 SignOut 的 401 一起發生仍導向 server logout route", async () => {
	globalThis.fetch = async (input, init) => {
		const url = String(input);
		requests.push({ url, method: init?.method });
		return url === SIGN_OUT_URL
			? new Response('{"code":"unauthenticated"}', {
					status: 401,
					headers: { "content-type": "application/json" },
				})
			: new Response('{"code":401}', { status: 401 });
	};
	const { request } = await import("../src/services/api");
	await assert.rejects(request("/api/v1/admin/buyers/stats"));
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(
		requests.map((r) => r.url),
		["/api/bff/api/v1/admin/buyers/stats", SIGN_OUT_URL],
	);
	assert.equal(win().location.href, LOGOUT_ROUTE);
});

test("REST body code 401 不觸發 sessionExpired", async () => {
	globalThis.fetch = async (input, init) => {
		requests.push({ url: String(input), method: init?.method });
		return new Response('{"code":401}', {
			status: 200,
			headers: { "content-type": "application/json" },
		});
	};
	const { request } = await import("../src/services/api");
	await assert.rejects(request("/api/v1/admin/buyers/stats"), {
		message: "需要重新登入或權限不足",
	});
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(requests, [
		{ url: "/api/bff/api/v1/admin/buyers/stats", method: undefined },
	]);
	assert.equal(win().location.href, "/admin");
	assert.deepEqual(alerts, []);
});
