import { createHash } from "node:crypto";
import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
} from "@connectrpc/connect";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	DEFAULT_PUT_TIMEOUT_MS,
	fetchPut,
	md5Base64,
	type PutRequest,
	type PutResponse,
	RejectReason,
	UploadError,
	uploadDelegatedPublicFile,
	uploadDelegatedSecureFile,
	uploadPublicFile,
	uploadSecureFile,
} from "../src/connect/upload";
import {
	BeginSecureUploadResponseSchema,
	BeginUploadResponseSchema,
	CompleteUploadResponseSchema,
	GetUploadResponseSchema,
	MediaAppService,
	MediaDelegatedService,
	MediaDelegatedServiceBeginSecureUploadResponseSchema,
	MediaDelegatedServiceBeginUploadResponseSchema,
	MediaDelegatedServiceCompleteUploadResponseSchema,
	MediaDelegatedServiceGetUploadResponseSchema,
	UploadState,
} from "../src/gen/hawkr/media/v1alpha1/media_pb";

const noSleep = async () => {};
const ok = (
	status = 200,
	headers: Record<string, string> = {},
): PutResponse => ({
	status,
	headers: { get: (n) => headers[n] ?? null },
});

function publicClient(
	states: Array<{ state: UploadState; reason?: RejectReason }>,
) {
	const calls: string[] = [];
	const seen: { begin?: { size: bigint; contentType: string; md5: string } } =
		{};
	let polls = 0;
	const transport = createRouterTransport((router) =>
		router.service(MediaAppService, {
			beginUpload(req) {
				calls.push("begin");
				seen.begin = {
					size: req.size,
					contentType: req.contentType,
					md5: req.md5,
				};
				return create(BeginUploadResponseSchema, {
					stagedToken: "tok",
					uploadUrl: "https://oss.example/put",
					uploadHeaders: { "Content-MD5": req.md5, "x-oss-meta": "a" },
				});
			},
			completeUpload() {
				calls.push("complete");
				return create(CompleteUploadResponseSchema, {});
			},
			getUpload() {
				calls.push("get");
				const s = states[
					Math.min(polls++, states.length - 1)
				] as (typeof states)[number];
				return create(GetUploadResponseSchema, {
					state: s.state,
					rejectReason: s.reason,
				});
			},
		}),
	);
	return { client: createClient(MediaAppService, transport), calls, seen };
}

describe("md5Base64", () => {
	it.each([0, 1, 55, 56, 63, 64, 65, 1000])(
		"matches node crypto for %i bytes",
		(n) => {
			const data = new Uint8Array(n).map((_, i) => (i * 31 + 7) & 255);
			expect(md5Base64(data)).toBe(
				createHash("md5").update(data).digest("base64"),
			);
		},
	);
});

describe("uploadPublicFile", () => {
	const file = new Blob([new Uint8Array([1, 2, 3, 4])], { type: "image/png" });

	it("begins, PUTs with the response headers, completes and polls to READY", async () => {
		const { client, calls, seen } = publicClient([
			{ state: UploadState.PENDING },
			{ state: UploadState.UPLOADED },
			{ state: UploadState.READY },
		]);
		const puts: PutRequest[] = [];
		const out = await uploadPublicFile(client, file, {
			put: async (r) => {
				puts.push(r);
				return ok();
			},
			sleep: noSleep,
		});
		expect(out).toEqual({ state: "ready", stagedToken: "tok" });
		expect(calls).toEqual(["begin", "complete", "get", "get", "get"]);
		expect(seen.begin).toEqual({
			size: BigInt(4),
			contentType: "image/png",
			md5: createHash("md5")
				.update(new Uint8Array([1, 2, 3, 4]))
				.digest("base64"),
		});
		expect(puts).toHaveLength(1);
		expect(puts[0]?.url).toBe("https://oss.example/put");
		expect(puts[0]?.headers).toEqual({
			"Content-MD5": seen.begin?.md5,
			"x-oss-meta": "a",
		});
	});

	it("returns the typed reject reason instead of throwing", async () => {
		const { client } = publicClient([
			{ state: UploadState.REJECTED, reason: RejectReason.CHECKSUM_MISMATCH },
		]);
		const out = await uploadPublicFile(client, file, {
			put: async () => ok(),
			sleep: noSleep,
		});
		expect(out).toEqual({
			state: "rejected",
			stagedToken: "tok",
			reason: RejectReason.CHECKSUM_MISMATCH,
		});
	});

	it("does not call CompleteUpload when the PUT fails", async () => {
		const { client, calls } = publicClient([{ state: UploadState.READY }]);
		await expect(
			uploadPublicFile(client, file, {
				put: async () => ok(403),
				sleep: noSleep,
			}),
		).rejects.toMatchObject({ kind: "put_failed", status: 403 });
		expect(calls).toEqual(["begin"]);
	});

	it("times out while the state stays PENDING", async () => {
		const { client } = publicClient([{ state: UploadState.PENDING }]);
		await expect(
			uploadPublicFile(client, file, {
				put: async () => ok(),
				sleep: noSleep,
				pollIntervalMs: 10,
				pollTimeoutMs: 30,
			}),
		).rejects.toMatchObject({ kind: "poll_timeout" });
	});

	it("rejects a terminal non-upload state", async () => {
		const { client } = publicClient([{ state: UploadState.DELETED }]);
		await expect(
			uploadPublicFile(client, file, { put: async () => ok(), sleep: noSleep }),
		).rejects.toMatchObject({ kind: "unexpected_state" });
	});

	it("propagates Connect errors from BeginUpload", async () => {
		const transport = createRouterTransport((router) =>
			router.service(MediaAppService, {
				beginUpload() {
					throw new ConnectError("too big", Code.InvalidArgument);
				},
			}),
		);
		await expect(
			uploadPublicFile(createClient(MediaAppService, transport), file, {
				put: async () => ok(),
			}),
		).rejects.toBeInstanceOf(ConnectError);
	});

	it("stops when aborted", async () => {
		const { client } = publicClient([{ state: UploadState.PENDING }]);
		const ac = new AbortController();
		ac.abort();
		await expect(
			uploadPublicFile(client, file, {
				signal: ac.signal,
				put: async () => ok(),
			}),
		).rejects.toMatchObject({ kind: "aborted" });
	});
});

function secureClient(chunkSize: number, chunkCount: number) {
	const sizes: bigint[] = [];
	const transport = createRouterTransport((router) => {
		router.service(MediaAppService, {
			beginSecureUpload(req) {
				sizes.push(req.size);
				return create(BeginSecureUploadResponseSchema, {
					stagedToken: "s/t",
					chunkSize,
					chunkCount,
				});
			},
		});
		router.service(MediaDelegatedService, {
			beginSecureUpload(req) {
				sizes.push(req.size);
				return create(MediaDelegatedServiceBeginSecureUploadResponseSchema, {
					stagedToken: "d1",
					chunkSize,
					chunkCount,
				});
			},
		});
	});
	return {
		app: createClient(MediaAppService, transport),
		del: createClient(MediaDelegatedService, transport),
		sizes,
	};
}

describe("uploadDelegatedPublicFile", () => {
	const file = new Blob([new Uint8Array([1, 2, 3, 4])], { type: "image/png" });

	function delegatedClient(
		states: Array<{ state: UploadState; reason?: RejectReason }>,
	) {
		const calls: string[] = [];
		const tokens: string[] = [];
		let polls = 0;
		const transport = createRouterTransport((router) =>
			router.service(MediaDelegatedService, {
				beginUpload(req) {
					calls.push("begin");
					return create(MediaDelegatedServiceBeginUploadResponseSchema, {
						stagedToken: "dtok",
						uploadUrl: "https://oss.example/dput",
						uploadHeaders: { "Content-MD5": req.md5 },
					});
				},
				completeUpload(req) {
					calls.push("complete");
					tokens.push(req.stagedToken);
					return create(MediaDelegatedServiceCompleteUploadResponseSchema, {});
				},
				getUpload(req) {
					calls.push("get");
					tokens.push(req.stagedToken);
					const s = states[
						Math.min(polls++, states.length - 1)
					] as (typeof states)[number];
					return create(MediaDelegatedServiceGetUploadResponseSchema, {
						state: s.state,
						rejectReason: s.reason,
					});
				},
			}),
		);
		return {
			client: createClient(MediaDelegatedService, transport),
			calls,
			tokens,
		};
	}

	it("runs begin, PUT, complete and polls through the delegated service", async () => {
		const { client, calls, tokens } = delegatedClient([
			{ state: UploadState.UPLOADED },
			{ state: UploadState.READY },
		]);
		const puts: PutRequest[] = [];
		const out = await uploadDelegatedPublicFile(client, file, {
			put: async (r) => {
				puts.push(r);
				return ok();
			},
			sleep: noSleep,
		});
		expect(out).toEqual({ state: "ready", stagedToken: "dtok" });
		expect(calls).toEqual(["begin", "complete", "get", "get"]);
		expect(tokens).toEqual(["dtok", "dtok", "dtok"]);
		expect(puts[0]?.url).toBe("https://oss.example/dput");
	});

	it("returns the typed reject reason and skips complete when the PUT fails", async () => {
		const rejected = delegatedClient([
			{ state: UploadState.REJECTED, reason: RejectReason.SIZE_MISMATCH },
		]);
		expect(
			await uploadDelegatedPublicFile(rejected.client, file, {
				put: async () => ok(),
				sleep: noSleep,
			}),
		).toEqual({
			state: "rejected",
			stagedToken: "dtok",
			reason: RejectReason.SIZE_MISMATCH,
		});
		const failed = delegatedClient([{ state: UploadState.READY }]);
		await expect(
			uploadDelegatedPublicFile(failed.client, file, {
				put: async () => ok(403),
				sleep: noSleep,
			}),
		).rejects.toMatchObject({ kind: "put_failed", status: 403 });
		expect(failed.calls).toEqual(["begin"]);
	});
});

describe("uploadSecureFile", () => {
	const file = new Blob([new Uint8Array([0, 1, 2, 3, 4, 5, 6, 7, 8, 9])]);

	it("PUTs each chunk in order to the secure endpoint", async () => {
		const { app, sizes } = secureClient(4, 3);
		const puts: Array<{ url: string; n: number; ct?: string }> = [];
		const progress: number[] = [];
		const out = await uploadSecureFile(app, file, {
			put: async (r) => {
				puts.push({
					url: r.url,
					n: r.body.size,
					ct: r.headers["Content-Type"],
				});
				return ok(204);
			},
			onProgress: (p) => progress.push(p),
		});
		expect(out).toEqual({ stagedToken: "s/t" });
		expect(sizes).toEqual([BigInt(10)]);
		expect(puts).toEqual([
			{
				url: "/api/files/secure_uploads/s%2Ft/chunks/0",
				n: 4,
				ct: "application/octet-stream",
			},
			{
				url: "/api/files/secure_uploads/s%2Ft/chunks/1",
				n: 4,
				ct: "application/octet-stream",
			},
			{
				url: "/api/files/secure_uploads/s%2Ft/chunks/2",
				n: 2,
				ct: "application/octet-stream",
			},
		]);
		expect(progress).toEqual([0.4, 0.8, 1]);
	});

	it("resumes at Hawkr-Next-Index after 409", async () => {
		const { app } = secureClient(4, 3);
		const urls: string[] = [];
		let first = true;
		await uploadSecureFile(app, file, {
			put: async (r) => {
				urls.push(r.url.split("/").pop() as string);
				if (first && r.url.endsWith("/1")) {
					first = false;
					return ok(409, { "Hawkr-Next-Index": "2" });
				}
				return ok(204);
			},
		});
		expect(urls).toEqual(["0", "1", "2"]);
	});

	it("fails on a 409 that does not advance", async () => {
		const { app } = secureClient(4, 3);
		let puts = 0;
		await expect(
			uploadSecureFile(app, file, {
				put: async () => {
					puts++;
					return ok(409, { "Hawkr-Next-Index": "0" });
				},
			}),
		).rejects.toMatchObject({ kind: "chunk_failed", status: 409 });
		// 立刻放棄：不靠續傳次數上限才停下來。
		expect(puts).toBe(1);
	});

	it("fails on a 409 without a usable header", async () => {
		const { app } = secureClient(4, 3);
		await expect(
			uploadSecureFile(app, file, { put: async () => ok(409) }),
		).rejects.toBeInstanceOf(UploadError);
	});

	it("retries a 503 then succeeds, and gives up on persistent 503", async () => {
		const { app } = secureClient(10, 1);
		let n = 0;
		await uploadSecureFile(app, file, {
			sleep: noSleep,
			put: async () => ok(++n < 2 ? 503 : 204),
		});
		expect(n).toBe(2);
		let m = 0;
		await expect(
			uploadSecureFile(app, file, {
				sleep: noSleep,
				put: async () => {
					m++;
					return ok(503);
				},
			}),
		).rejects.toMatchObject({ kind: "chunk_failed", status: 503 });
		expect(m).toBe(3);
	});

	it("does not retry a 412", async () => {
		const { app } = secureClient(10, 1);
		let n = 0;
		await expect(
			uploadSecureFile(app, file, {
				put: async () => {
					n++;
					return ok(412);
				},
			}),
		).rejects.toMatchObject({
			status: 412,
		});
		expect(n).toBe(1);
	});

	it("uses the delegated endpoint and forwards credential headers", async () => {
		const { del } = secureClient(10, 1);
		const seen: PutRequest[] = [];
		const out = await uploadDelegatedSecureFile(del, file, {
			headers: { "X-HK-Operation-Session": "cred" },
			put: async (r) => {
				seen.push(r);
				return ok(204);
			},
		});
		expect(out).toEqual({ stagedToken: "d1" });
		expect(seen[0]?.url).toBe(
			"/api/files/delegated/secure_uploads/d1/chunks/0",
		);
		expect(seen[0]?.headers["X-HK-Operation-Session"]).toBe("cred");
	});
});

describe("fetchPut", () => {
	afterEach(() => vi.unstubAllGlobals());

	it("sends the presigned PUT without credentials or referrer and with a timeout signal", async () => {
		const fetchMock = vi.fn(async () => new Response(null, { status: 200 }));
		vi.stubGlobal("fetch", fetchMock);
		await fetchPut({
			url: "https://oss.example/put",
			headers: {},
			body: new Blob(["x"]),
		} as PutRequest);
		const init = (
			fetchMock.mock.calls[0] as unknown as [string, RequestInit]
		)[1];
		expect(init.credentials).toBe("omit");
		expect(init.referrerPolicy).toBe("no-referrer");
		expect(init.signal).toBeInstanceOf(AbortSignal);
		expect(DEFAULT_PUT_TIMEOUT_MS).toBe(120_000);
	});

	it("still aborts when the caller's signal aborts", async () => {
		const fetchMock = vi.fn(async () => new Response(null, { status: 200 }));
		vi.stubGlobal("fetch", fetchMock);
		const ctl = new AbortController();
		await fetchPut({
			url: "https://oss.example/put",
			headers: {},
			body: new Blob(["x"]),
			signal: ctl.signal,
		} as PutRequest);
		const init = (
			fetchMock.mock.calls[0] as unknown as [string, RequestInit]
		)[1];
		const signal = init.signal as AbortSignal;
		expect(signal.aborted).toBe(false);
		ctl.abort();
		expect(signal.aborted).toBe(true);
	});
});
