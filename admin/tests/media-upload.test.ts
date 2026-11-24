import assert from "node:assert/strict";
import test from "node:test";
import {
	MediaRejectedError,
	uploadFailureText,
	uploadImageToMedia,
} from "../src/services/media-upload";

type Call = { url: string; init: RequestInit };

const RPC = "/api/bff/hawkr.media.v1alpha1.MediaAppService/";

function json(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { "Content-Type": "application/json" },
	});
}

async function withFetch<T>(
	respond: (url: string, init: RequestInit) => Response,
	run: (calls: Call[]) => Promise<T>,
): Promise<T> {
	const original = globalThis.fetch;
	const calls: Call[] = [];
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		calls.push({ url, init: init ?? {} });
		return respond(url, init ?? {});
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

function publicFlow(getUpload: unknown) {
	return (url: string): Response => {
		if (url.endsWith(`${RPC}BeginUpload`)) {
			return json({
				stagedToken: "tok",
				uploadUrl: "https://oss.example/put",
				uploadHeaders: { "Content-Type": "image/png" },
			});
		}
		if (url === "https://oss.example/put") return new Response(null);
		if (url.endsWith(`${RPC}CompleteUpload`)) return json({});
		if (url.endsWith(`${RPC}GetUpload`)) return json(getUpload);
		return json({}, 404);
	};
}

test("公開上傳：BeginUpload、預簽 PUT、CompleteUpload、GetUpload 到 READY 回 staged token", async () => {
	await withFetch(
		publicFlow({ state: "UPLOAD_STATE_READY" }),
		async (calls) => {
			assert.equal(await uploadImageToMedia(file), "tok");
			const urls = calls.map((c) => c.url);
			const order = [
				`${RPC}BeginUpload`,
				"https://oss.example/put",
				`${RPC}CompleteUpload`,
				`${RPC}GetUpload`,
			].map((suffix) => urls.findIndex((u) => u.endsWith(suffix)));
			assert.deepEqual(
				order,
				[...order].sort((a, b) => a - b),
			);
			assert.ok(order.every((i) => i >= 0));
			const put = calls.find((c) => c.url === "https://oss.example/put");
			assert.equal(put?.init.method, "PUT");
		},
	);
});

test("被拒收：丟 MediaRejectedError，訊息是目錄的原因句", async () => {
	await withFetch(
		publicFlow({
			state: "UPLOAD_STATE_REJECTED",
			rejectReason: "REJECT_REASON_UNRECOGNIZED_FORMAT",
		}),
		async () => {
			await assert.rejects(uploadImageToMedia(file), (err: unknown) => {
				assert.ok(err instanceof MediaRejectedError);
				assert.equal(
					err.message,
					"無法辨識檔案格式，請改用 JPEG、PNG、WebP 或 GIF 圖片。",
				);
				return true;
			});
		},
	);
});

test("機敏檔：BeginSecureUpload 後把分段 PUT 到 BFF 的段端點，不經預簽網址", async () => {
	await withFetch(
		(url) => {
			if (url.endsWith(`${RPC}BeginSecureUpload`)) {
				return json({ stagedToken: "sec", chunkSize: 4, chunkCount: 1 });
			}
			if (url.includes("/chunks/")) return new Response(null);
			return json({}, 404);
		},
		async (calls) => {
			assert.equal(await uploadImageToMedia(file, { isSecure: true }), "sec");
			const chunk = calls.find((c) => c.url.includes("/chunks/"));
			assert.ok(chunk);
			assert.equal(chunk.url, "/api/bff/api/files/secure_uploads/sec/chunks/0");
			assert.equal(chunk.init.method, "PUT");
			assert.equal(
				calls.some((c) => c.url.includes("oss.example")),
				false,
			);
		},
	);
});

test("uploadFailureText：拒收顯示原因句，其餘用固定句", () => {
	const rejected = new MediaRejectedError(2);
	assert.equal(uploadFailureText(rejected, "固定句"), rejected.message);
	assert.equal(
		uploadFailureText(new Error("Failed to fetch"), "固定句"),
		"固定句",
	);
});
