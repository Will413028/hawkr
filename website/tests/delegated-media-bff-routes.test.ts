import assert from "node:assert/strict";
import test from "node:test";
import { getDelegatedCookieName } from "../features/delegated-operations/server/session";

/**
 * 代行公開圖片上傳的三支 BFF（MediaDelegatedService 的 BeginUpload、CompleteUpload、GetUpload）：
 * 憑證只走 X-HK-Operation-Session，送出前的檢查失敗一律不打上游，且不做綁定店比對（路徑沒有 storeId）。
 */

const CREDENTIAL = "dops_test_credential_0123456789abcdef";
const LOCAL_ORIGIN = "http://localhost:3000";
const SERVICE = "hawkr.media.v1alpha1.MediaDelegatedService";
const TOKEN = "eyJhbGciOiJIUzI1NiJ9.payload.sig";

type Handler = (
	request: Request,
	context: { params: Promise<Record<string, string>> },
) => Promise<Response>;

interface Call {
	url: string;
	init: RequestInit;
}

function stubFetch(responder: (call: Call) => Response): {
	calls: Call[];
	restore(): void;
} {
	const calls: Call[] = [];
	const original = globalThis.fetch;
	globalThis.fetch = (async (input: unknown, init: RequestInit = {}) => {
		const call = { url: String(input), init };
		calls.push(call);
		return responder(call);
	}) as typeof fetch;
	return {
		calls,
		restore() {
			globalThis.fetch = original;
		},
	};
}

function request(
	path: string,
	method: "GET" | "POST",
	extra: { cookie?: boolean; body?: unknown } = {},
): Request {
	const headers = new Headers();
	if (extra.cookie !== false) {
		headers.set("cookie", `${getDelegatedCookieName()}=${CREDENTIAL}`);
	}
	if (method === "POST") {
		headers.set("x-requested-with", "XMLHttpRequest");
		headers.set("origin", LOCAL_ORIGIN);
	}
	return new Request(`${LOCAL_ORIGIN}${path}`, {
		method,
		headers,
		body: method === "POST" ? JSON.stringify(extra.body ?? {}) : undefined,
	});
}

const BEGIN_BODY = {
	size: "3",
	contentType: "image/png",
	md5: "AAAAAAAAAAAAAAAAAAAAAA==",
};

async function begin(): Promise<Handler> {
	return (
		await import("../app/(delegated)/adm/api/media/upload_sessions/route")
	).POST as Handler;
}
async function complete(): Promise<Handler> {
	return (
		await import(
			"../app/(delegated)/adm/api/media/upload_sessions/[sessionId]/complete/route"
		)
	).POST as Handler;
}
async function get(): Promise<Handler> {
	return (
		await import(
			"../app/(delegated)/adm/api/media/upload_sessions/[sessionId]/route"
		)
	).GET as Handler;
}

const ctx = (sessionId: string) => ({ params: Promise.resolve({ sessionId }) });

test("BeginUpload：打 MediaDelegatedService，憑證在標頭，回 stagedToken、uploadUrl、uploadHeaders，不讀 session 綁定店", async () => {
	const stub = stubFetch(() =>
		Response.json({
			stagedToken: TOKEN,
			uploadUrl: "https://r2.example/put",
			uploadHeaders: { "Content-MD5": "AAAAAAAAAAAAAAAAAAAAAA==" },
		}),
	);
	try {
		const response = await (await begin())(
			request("/adm/api/media/upload_sessions", "POST", { body: BEGIN_BODY }),
			ctx(""),
		);
		assert.equal(response.status, 200);
		assert.equal(stub.calls.length, 1);
		assert.ok(stub.calls[0].url.includes(`/${SERVICE}/BeginUpload`));
		const sent = new Headers(stub.calls[0].init.headers);
		assert.equal(sent.get("X-HK-Operation-Session"), CREDENTIAL);
		assert.ok(!stub.calls[0].url.includes(CREDENTIAL));
		const payload = (await response.json()) as {
			code: number;
			data: { stagedToken: string; uploadUrl: string };
		};
		assert.equal(payload.code, 0);
		assert.equal(payload.data.stagedToken, TOKEN);
		assert.equal(payload.data.uploadUrl, "https://r2.example/put");
	} finally {
		stub.restore();
	}
});

test("CompleteUpload 與 GetUpload：staged token 取自路徑，GetUpload 回狀態與拒絕原因", async () => {
	const stub = stubFetch((call) =>
		call.url.includes("/GetUpload")
			? Response.json({
					state: "UPLOAD_STATE_REJECTED",
					rejectReason: "REJECT_REASON_UNRECOGNIZED_FORMAT",
				})
			: Response.json({}),
	);
	try {
		const done = await (await complete())(
			request(`/adm/api/media/upload_sessions/${TOKEN}/complete`, "POST"),
			ctx(TOKEN),
		);
		assert.equal(done.status, 200);
		assert.ok(stub.calls[0].url.includes(`/${SERVICE}/CompleteUpload`));
		assert.ok(
			String(stub.calls[0].init.body).includes(TOKEN) ||
				stub.calls[0].init.body instanceof Uint8Array,
		);

		const got = await (await get())(
			request(`/adm/api/media/upload_sessions/${TOKEN}`, "GET"),
			ctx(TOKEN),
		);
		assert.equal(got.status, 200);
		assert.ok(stub.calls[1].url.includes(`/${SERVICE}/GetUpload`));
		const payload = (await got.json()) as {
			data: { state: number; rejectReason: number | null };
		};
		assert.equal(payload.data.state, 4);
		assert.equal(payload.data.rejectReason, 1);
	} finally {
		stub.restore();
	}
});

test("送出前檢查：沒憑證 401、非法 token 路徑段 400、begin 的 body 不合法 400，都不打上游", async () => {
	const stub = stubFetch(() => {
		throw new Error("檢查失敗時打了上游");
	});
	try {
		const noCookie = await (await begin())(
			request("/adm/api/media/upload_sessions", "POST", {
				cookie: false,
				body: BEGIN_BODY,
			}),
			ctx(""),
		);
		assert.equal(noCookie.status, 401);

		for (const body of [
			{},
			{ ...BEGIN_BODY, size: "0" },
			{ ...BEGIN_BODY, size: 3 },
			{ ...BEGIN_BODY, md5: undefined },
		]) {
			const bad = await (await begin())(
				request("/adm/api/media/upload_sessions", "POST", { body }),
				ctx(""),
			);
			assert.equal(bad.status, 400, JSON.stringify(body));
		}

		for (const token of ["../../current", "a/b", "a?x=1", ""]) {
			const c = await (await complete())(
				request("/adm/api/media/upload_sessions/x/complete", "POST"),
				ctx(token),
			);
			assert.equal(c.status, 400, `complete ${token}`);
			const g = await (await get())(
				request("/adm/api/media/upload_sessions/x", "GET"),
				ctx(token),
			);
			assert.equal(g.status, 400, `get ${token}`);
		}
		assert.equal(stub.calls.length, 0);
	} finally {
		stub.restore();
	}
});
