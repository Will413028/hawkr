import assert from "node:assert/strict";
import test from "node:test";
import { uploadImageToMedia } from "../src/services/media-upload";

type Call = { url: string; init: RequestInit };

async function withFetch<T>(
	respond: (url: string) => unknown,
	run: (calls: Call[]) => Promise<T>,
): Promise<T> {
	const original = globalThis.fetch;
	const calls: Call[] = [];
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		calls.push({ url, init: init ?? {} });
		return new Response(JSON.stringify(respond(url)), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		});
	}) as typeof fetch;
	try {
		return await run(calls);
	} finally {
		globalThis.fetch = original;
	}
}

const file = new File([new Uint8Array([1, 2, 3])], "a.png", {
	type: "image/png",
});

test("公開上傳：presigned PUT 不帶 cookie、不送 referrer、有逾時；http 改成 https", async () => {
	await withFetch(
		(url) => {
			if (url.endsWith("/upload_sessions")) {
				return {
					code: 0,
					data: { sessionId: "s1", uploadUrl: "http://oss.example/put" },
				};
			}
			if (url.endsWith("/complete")) {
				return { code: 0, data: { stagedToken: "tok" } };
			}
			return {};
		},
		async (calls) => {
			assert.equal(await uploadImageToMedia(file), "tok");
			const put = calls.find((c) => c.url.startsWith("https://oss.example"));
			assert.ok(put, "presigned PUT 應改寫成 https");
			assert.equal(put.init.method, "PUT");
			assert.equal(put.init.credentials, "omit");
			assert.equal(put.init.referrerPolicy, "no-referrer");
			assert.ok(put.init.signal instanceof AbortSignal);
		},
	);
});

test("機敏檔走 secure_upload：不經 presigned PUT，也不帶這三個選項", async () => {
	await withFetch(
		(url) =>
			url.endsWith("/upload_sessions")
				? { code: 0, data: { sessionId: "s2" } }
				: { code: 0, data: { stagedToken: "secure-tok" } },
		async (calls) => {
			assert.equal(
				await uploadImageToMedia(file, { isSecure: true }),
				"secure-tok",
			);
			const secure = calls.find((c) => c.url.includes("/secure_upload/s2"));
			assert.ok(secure);
			assert.equal(secure.init.credentials, undefined);
			assert.equal(secure.init.referrerPolicy, undefined);
			assert.equal(
				calls.some((c) => c.url.startsWith("https://oss")),
				false,
			);
		},
	);
});
