import { afterEach, describe, expect, it, vi } from "vitest";
import { getDelegatedCookieName } from "./server/session";

/**
 * 代行工作區的機敏檔上傳 BFF（B1-D16／D17）：段上傳 PUT 與暫存預覽 GET 轉到單體的
 * `/api/files/delegated/secure_uploads/…`。憑證只有 operation credential cookie → `X-HK-Operation-Session`，
 * 檢查順序與 `_forward.ts` 一致：path 值域 → 保留標頭／CSRF → 憑證 → body。
 */

const CREDENTIAL = "dops_test_credential_0123456789abcdef";
const ORIGIN = "http://localhost:3000";
const TOKEN = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCde";
const CHUNK_URL = `${ORIGIN}/adm/api/media/secure_uploads/${TOKEN}/chunks/2`;
const PREVIEW_URL = `${ORIGIN}/adm/api/media/secure_uploads/${TOKEN}`;
const BYTES = new Uint8Array([0, 1, 2, 250, 251, 252, 255, 10, 13]);
const params = { token: TOKEN, index: "2" };

type Handler = (
	request: Request,
	context: { params: Promise<Record<string, string>> },
) => Promise<Response>;

async function chunkHandler(): Promise<Handler> {
	const mod = (await import(
		"@/app/(delegated)/adm/api/media/secure_uploads/[token]/chunks/[index]/route"
	)) as { PUT: Handler };
	return mod.PUT;
}

async function previewHandler(): Promise<Handler> {
	const mod = (await import(
		"@/app/(delegated)/adm/api/media/secure_uploads/[token]/route"
	)) as { GET: Handler };
	return mod.GET;
}

function stubFetch(
	responder: () => Response = () => new Response(null, { status: 204 }),
) {
	const fetchMock = vi.fn(async (_input: unknown, _init?: RequestInit) =>
		responder(),
	);
	vi.stubGlobal("fetch", fetchMock);
	return fetchMock;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

function putRequest(
	overrides: {
		cookie?: boolean;
		xhr?: boolean;
		origin?: string | null;
		headers?: Record<string, string>;
		contentLength?: string | null;
	} = {},
): Request {
	const headers = new Headers(overrides.headers ?? {});
	if (overrides.cookie !== false) {
		headers.set("cookie", `${getDelegatedCookieName()}=${CREDENTIAL}`);
	}
	if (overrides.xhr !== false) {
		headers.set("x-requested-with", "XMLHttpRequest");
	}
	const origin = overrides.origin === undefined ? ORIGIN : overrides.origin;
	if (origin !== null) headers.set("origin", origin);
	if (overrides.contentLength !== null) {
		headers.set(
			"content-length",
			overrides.contentLength ?? String(BYTES.length),
		);
	}
	return new Request(CHUNK_URL, { method: "PUT", headers, body: BYTES });
}

describe("delegated secure upload BFF", () => {
	it("forwards a chunk to the monolith delegated path with the credential header and the body unchanged", async () => {
		const put = await chunkHandler();
		const fetchMock = stubFetch();
		const response = await put(putRequest(), {
			params: Promise.resolve(params),
		});
		expect(response.status).toBe(204);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(
			url.endsWith(`/api/files/delegated/secure_uploads/${TOKEN}/chunks/2`),
		).toBe(true);
		expect(url).not.toContain("/admin/");
		expect(url).not.toContain(CREDENTIAL);
		expect(init.method).toBe("PUT");
		const sent = new Headers(init.headers);
		expect(sent.get("X-HK-Operation-Session")).toBe(CREDENTIAL);
		expect(sent.get("Content-Length")).toBe(String(BYTES.length));
		expect(sent.get("Authorization")).toBeNull();
		expect(sent.get("Cookie")).toBeNull();
		expect([...new Uint8Array(init.body as ArrayBuffer)]).toEqual([...BYTES]);
	});

	it("returns upstream 409 with Hawkr-Next-Index and drops the failure body and cookies", async () => {
		const put = await chunkHandler();
		stubFetch(
			() =>
				new Response("detail that must not leak", {
					status: 409,
					headers: { "Hawkr-Next-Index": "3", "Set-Cookie": "x=1" },
				}),
		);
		const response = await put(putRequest(), {
			params: Promise.resolve(params),
		});
		expect(response.status).toBe(409);
		expect(response.headers.get("Hawkr-Next-Index")).toBe("3");
		expect(response.headers.get("Set-Cookie")).toBeNull();
		expect(await response.text()).toBe("");
	});

	it("rejects missing credential, CSRF failures and reserved headers without reaching upstream", async () => {
		const put = await chunkHandler();
		const cases: Array<[string, Request, number, string]> = [
			["no cookie", putRequest({ cookie: false }), 401, "CREDENTIAL_MISSING"],
			["no xhr", putRequest({ xhr: false }), 403, "CSRF_REJECTED"],
			["no origin", putRequest({ origin: null }), 403, "CSRF_REJECTED"],
			[
				"foreign origin",
				putRequest({ origin: "https://evil.example" }),
				403,
				"CSRF_REJECTED",
			],
			[
				"reserved header",
				putRequest({ headers: { "X-HK-Operation-Session": "forged" } }),
				400,
				"RESERVED_HEADER",
			],
		];
		for (const [name, request, status, code] of cases) {
			const fetchMock = stubFetch();
			const response = await put(request, { params: Promise.resolve(params) });
			expect(response.status, name).toBe(status);
			expect(((await response.json()) as { code: string }).code, name).toBe(
				code,
			);
			expect(fetchMock, name).not.toHaveBeenCalled();
		}
	});

	it("checks path values and body length before reaching upstream", async () => {
		const put = await chunkHandler();
		const fetchMock = stubFetch();
		for (const bad of [
			{ token: "../../current", index: "0" },
			{ token: TOKEN, index: "01" },
			{ token: TOKEN, index: "1/../2" },
			{ token: TOKEN, index: "-1" },
		]) {
			const response = await put(putRequest(), {
				params: Promise.resolve(bad),
			});
			expect(response.status, JSON.stringify(bad)).toBe(400);
		}
		const missing = await put(putRequest({ contentLength: null }), {
			params: Promise.resolve(params),
		});
		expect(missing.status).toBe(411);
		const tooBig = await put(
			putRequest({ contentLength: String((4 << 20) + 1) }),
			{
				params: Promise.resolve(params),
			},
		);
		expect(tooBig.status).toBe(413);
		const mismatch = await put(putRequest({ contentLength: "3" }), {
			params: Promise.resolve(params),
		});
		expect(mismatch.status).toBe(400);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("forwards the preview GET with the credential header and returns bytes unchanged, uncached", async () => {
		const get = await previewHandler();
		const pdf = new Uint8Array([37, 80, 68, 70, 45]);
		const fetchMock = stubFetch(
			() =>
				new Response(pdf, {
					status: 200,
					headers: {
						"Content-Type": "application/pdf",
						"Content-Disposition": "inline",
					},
				}),
		);
		const response = await get(
			new Request(PREVIEW_URL, {
				headers: { cookie: `${getDelegatedCookieName()}=${CREDENTIAL}` },
			}),
			{ params: Promise.resolve({ token: TOKEN }) },
		);
		expect(response.status).toBe(200);
		expect(response.headers.get("Content-Type")).toBe("application/pdf");
		expect(response.headers.get("Cache-Control")).toBe("private, no-store");
		expect([...new Uint8Array(await response.arrayBuffer())]).toEqual([...pdf]);
		const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(url.endsWith(`/api/files/delegated/secure_uploads/${TOKEN}`)).toBe(
			true,
		);
		expect(init.method).toBe("GET");
		expect(new Headers(init.headers).get("X-HK-Operation-Session")).toBe(
			CREDENTIAL,
		);
		const noCookie = await get(new Request(PREVIEW_URL), {
			params: Promise.resolve({ token: TOKEN }),
		});
		expect(noCookie.status).toBe(401);
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});
});
