import type { Client } from "@connectrpc/connect";
import { createMD5 } from "hash-wasm";
import {
	type MediaAppService,
	type MediaDelegatedService,
	RejectReason,
	UploadState,
} from "../gen/hawkr/media/v1alpha1/media_pb";

export { RejectReason };

const SECURE_CHUNK_PATH = "/api/files/secure_uploads";
const DELEGATED_SECURE_CHUNK_PATH = "/api/files/delegated/secure_uploads";
const NEXT_INDEX_HEADER = "Hawkr-Next-Index";

const DEFAULT_POLL_INTERVAL_MS = 1000;
const DEFAULT_POLL_TIMEOUT_MS = 60_000;
const MAX_RESUMES = 10;
const MAX_CHUNK_RETRIES = 2;

export type UploadErrorKind =
	| "put_failed"
	| "chunk_failed"
	| "poll_timeout"
	| "unexpected_state"
	| "aborted";

export class UploadError extends Error {
	readonly kind: UploadErrorKind;
	readonly status?: number;
	constructor(kind: UploadErrorKind, message: string, status?: number) {
		super(message);
		this.name = "UploadError";
		this.kind = kind;
		this.status = status;
	}
}

export type UploadOutcome =
	| { state: "ready"; stagedToken: string }
	| { state: "rejected"; stagedToken: string; reason: RejectReason };

export interface SecureUploadOutcome {
	stagedToken: string;
}

export interface PutRequest {
	url: string;
	body: Blob;
	headers: Record<string, string>;
	signal?: AbortSignal;
	onProgress?: (ratio: number) => void;
}

export interface PutResponse {
	status: number;
	headers: { get(name: string): string | null };
}

/** 傳送一個 PUT；預設用 fetch（沒有進度），呼叫端要上傳進度時以 XHR 實作取代。 */
export type PutTransport = (req: PutRequest) => Promise<PutResponse>;

export interface UploadOptions {
	signal?: AbortSignal;
	/** 已送出的位元組比例，0 到 1。 */
	onProgress?: (ratio: number) => void;
	put?: PutTransport;
	sleep?: (ms: number) => Promise<void>;
	/** 預設逐片計算檔案內容 MD5 的 base64（BeginUploadRequest.md5，16 位元組）；測試或原生實作可取代。 */
	md5Base64?: (file: Blob) => Promise<string>;
	pollIntervalMs?: number;
	pollTimeoutMs?: number;
}

export interface SecureUploadOptions extends UploadOptions {
	/** 段端點的路徑前綴（含代行時的 /api/files/delegated/secure_uploads）；預設是顧客端點。 */
	chunkPath?: string;
	/** 每個段請求都帶的標頭，例如代行的 X-HK-Operation-Session。 */
	headers?: Record<string, string>;
}

export type PublicUploadClient = Pick<
	Client<typeof MediaAppService>,
	"beginUpload" | "completeUpload" | "getUpload"
>;
export type SecureUploadClient = Pick<
	Client<typeof MediaAppService>,
	"beginSecureUpload"
>;
export type DelegatedSecureUploadClient = Pick<
	Client<typeof MediaDelegatedService>,
	"beginSecureUpload"
>;

export type DelegatedPublicUploadClient = Pick<
	Client<typeof MediaDelegatedService>,
	"beginUpload" | "completeUpload" | "getUpload"
>;

interface PublicUploadOps {
	begin(
		req: { size: bigint; contentType: string; md5: string },
		signal?: AbortSignal,
	): Promise<{
		stagedToken: string;
		uploadUrl: string;
		uploadHeaders: Record<string, string>;
	}>;
	complete(stagedToken: string, signal?: AbortSignal): Promise<unknown>;
	get(
		stagedToken: string,
		signal?: AbortSignal,
	): Promise<{ state: UploadState; rejectReason?: RejectReason }>;
}

const defaultSleep = (ms: number) =>
	new Promise<void>((resolve) => setTimeout(resolve, ms));

/** 預簽 PUT 的預設逾時：超過就中止，不讓上傳永遠卡住。 */
export const DEFAULT_PUT_TIMEOUT_MS = 120_000;

/** 預簽網址是跨來源的物件儲存：不帶 cookie、不送 referrer，預設有逾時。 */
export const fetchPut: PutTransport = async (req) => {
	const timeout = AbortSignal.timeout(DEFAULT_PUT_TIMEOUT_MS);
	const res = await fetch(req.url, {
		method: "PUT",
		body: req.body,
		headers: req.headers,
		signal: req.signal ? AbortSignal.any([req.signal, timeout]) : timeout,
		credentials: "omit",
		referrerPolicy: "no-referrer",
	});
	return res;
};

function throwIfAborted(signal?: AbortSignal) {
	if (signal?.aborted) throw new UploadError("aborted", "上傳已取消");
}

/**
 * 公開圖片上傳：BeginUpload、把檔案 PUT 到預簽網址（帶回應指定的標頭）、CompleteUpload，再輪詢 GetUpload 到 READY 或
 * REJECTED。被拒絕不是例外，回 `state: "rejected"` 與依 proto enum 的原因；檔案格式（HEIC 轉 JPEG）由呼叫端先處理。
 * 預簽網址是跨來源的物件儲存，不帶 cookie。
 */
export function uploadPublicFile(
	client: PublicUploadClient,
	file: Blob,
	opts: UploadOptions = {},
): Promise<UploadOutcome> {
	return runPublicUpload(
		{
			begin: (req, signal) => client.beginUpload(req, { signal }),
			complete: (stagedToken, signal) =>
				client.completeUpload({ stagedToken }, { signal }),
			get: (stagedToken, signal) =>
				client.getUpload({ stagedToken }, { signal }),
		},
		file,
		opts,
	);
}

/**
 * 公開圖片上傳（代行營運）：流程同 `uploadPublicFile`，三個呼叫走 MediaDelegatedService；
 * 憑證只有呼叫端注入的 client 帶的代行 session 標頭（X-HK-Operation-Session），上傳者是被代行的店。
 */
export function uploadDelegatedPublicFile(
	client: DelegatedPublicUploadClient,
	file: Blob,
	opts: UploadOptions = {},
): Promise<UploadOutcome> {
	return runPublicUpload(
		{
			begin: (req, signal) => client.beginUpload(req, { signal }),
			complete: (stagedToken, signal) =>
				client.completeUpload({ stagedToken }, { signal }),
			get: (stagedToken, signal) =>
				client.getUpload({ stagedToken }, { signal }),
		},
		file,
		opts,
	);
}

async function runPublicUpload(
	ops: PublicUploadOps,
	file: Blob,
	opts: UploadOptions,
): Promise<UploadOutcome> {
	const sleep = opts.sleep ?? defaultSleep;
	const put = opts.put ?? fetchPut;
	const md5 = opts.md5Base64 ?? ((f: Blob) => md5Base64(f, opts.signal));
	throwIfAborted(opts.signal);
	const begin = await ops.begin(
		{
			size: BigInt(file.size),
			contentType: file.type,
			md5: await md5(file),
		},
		opts.signal,
	);
	const res = await put({
		url: begin.uploadUrl,
		body: file,
		headers: { ...begin.uploadHeaders },
		signal: opts.signal,
		onProgress: opts.onProgress,
	}).catch((err: unknown) => {
		if (opts.signal?.aborted) throw new UploadError("aborted", "上傳已取消");
		throw new UploadError("put_failed", errorText(err));
	});
	if (res.status < 200 || res.status >= 300) {
		throw new UploadError(
			"put_failed",
			`物件儲存回 HTTP ${res.status}`,
			res.status,
		);
	}
	opts.onProgress?.(1);
	await ops.complete(begin.stagedToken, opts.signal);

	const interval = opts.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS;
	const timeout = opts.pollTimeoutMs ?? DEFAULT_POLL_TIMEOUT_MS;
	let waited = 0;
	for (;;) {
		throwIfAborted(opts.signal);
		const got = await ops.get(begin.stagedToken, opts.signal);
		switch (got.state) {
			case UploadState.READY:
				return { state: "ready", stagedToken: begin.stagedToken };
			case UploadState.REJECTED:
				return {
					state: "rejected",
					stagedToken: begin.stagedToken,
					reason: got.rejectReason ?? RejectReason.UNSPECIFIED,
				};
			case UploadState.PENDING:
			case UploadState.UPLOADED:
				break;
			default:
				throw new UploadError(
					"unexpected_state",
					`暫存處於非預期的狀態 ${UploadState[got.state] ?? got.state}`,
				);
		}
		if (waited >= timeout) {
			throw new UploadError("poll_timeout", "等待上傳處理逾時");
		}
		await sleep(interval);
		waited += interval;
	}
}

/** 機敏檔分段上傳（顧客）：BeginSecureUpload 後依段大小依序 PUT 到單體的段端點。 */
export function uploadSecureFile(
	client: SecureUploadClient,
	file: Blob,
	opts: SecureUploadOptions = {},
): Promise<SecureUploadOutcome> {
	return uploadChunks(
		(size, signal) => client.beginSecureUpload({ size }, { signal }),
		file,
		{ chunkPath: SECURE_CHUNK_PATH, ...opts },
	);
}

/**
 * 機敏檔分段上傳（代行營運）：開會話走 MediaDelegatedService，段走 /api/files/delegated/secure_uploads，
 * 憑證只有呼叫端放進 `headers` 的代行 session 標頭。
 */
export function uploadDelegatedSecureFile(
	client: DelegatedSecureUploadClient,
	file: Blob,
	opts: SecureUploadOptions = {},
): Promise<SecureUploadOutcome> {
	return uploadChunks(
		(size, signal) => client.beginSecureUpload({ size }, { signal }),
		file,
		{ chunkPath: DELEGATED_SECURE_CHUNK_PATH, ...opts },
	);
}

interface SecureSession {
	stagedToken: string;
	chunkSize: number;
	chunkCount: number;
}

async function uploadChunks(
	begin: (size: bigint, signal?: AbortSignal) => Promise<SecureSession>,
	file: Blob,
	opts: SecureUploadOptions & { chunkPath: string },
): Promise<SecureUploadOutcome> {
	const sleep = opts.sleep ?? defaultSleep;
	const put = opts.put ?? fetchPut;
	throwIfAborted(opts.signal);
	const session = await begin(BigInt(file.size), opts.signal);
	const base = `${opts.chunkPath.replace(/\/+$/, "")}/${encodeURIComponent(session.stagedToken)}/chunks`;
	const headers = {
		"Content-Type": "application/octet-stream",
		...opts.headers,
	};

	let index = 0;
	let resumes = 0;
	while (index < session.chunkCount) {
		throwIfAborted(opts.signal);
		const start = index * session.chunkSize;
		const body = file.slice(start, start + session.chunkSize);
		const res = await putChunk(
			put,
			sleep,
			{
				url: `${base}/${index}`,
				body,
				headers,
				signal: opts.signal,
			},
			opts.signal,
		);
		if (res.status >= 200 && res.status < 300) {
			index++;
			opts.onProgress?.(Math.min(1, (start + body.size) / (file.size || 1)));
			continue;
		}
		if (res.status === 409) {
			const next = Number(res.headers.get(NEXT_INDEX_HEADER));
			if (
				!Number.isInteger(next) ||
				next < 0 ||
				next > session.chunkCount ||
				next === index ||
				++resumes > MAX_RESUMES
			) {
				throw new UploadError("chunk_failed", "段序號無法續傳", 409);
			}
			index = next;
			continue;
		}
		throw new UploadError(
			"chunk_failed",
			`段 ${index} 上傳回 HTTP ${res.status}`,
			res.status,
		);
	}
	return { stagedToken: session.stagedToken };
}

async function putChunk(
	put: PutTransport,
	sleep: (ms: number) => Promise<void>,
	req: PutRequest,
	signal?: AbortSignal,
): Promise<PutResponse> {
	for (let attempt = 0; ; attempt++) {
		let res: PutResponse | undefined;
		let failure: unknown;
		try {
			res = await put(req);
		} catch (err) {
			if (signal?.aborted) throw new UploadError("aborted", "上傳已取消");
			failure = err;
		}
		const transient =
			res === undefined || res.status === 503 || res.status === 408;
		if (!transient) return res as PutResponse;
		if (attempt >= MAX_CHUNK_RETRIES) {
			if (res) return res;
			throw new UploadError("chunk_failed", errorText(failure));
		}
		const retryAfter = Number(res?.headers.get("Retry-After"));
		await sleep(
			Number.isFinite(retryAfter) && retryAfter > 0
				? retryAfter * 1000
				: 500 * 2 ** attempt,
		);
	}
}

function errorText(err: unknown): string {
	return err instanceof Error ? err.message : String(err);
}

/** 算 MD5 時一次讀進記憶體的切片大小。 */
const MD5_SLICE_BYTES = 4 * 1024 * 1024;

/** 檔案內容 MD5 的標準 base64：逐片讀入 hash-wasm 的 MD5，不把整個檔案載入記憶體；WebCrypto 沒有 MD5，BeginUpload 要的是內容的 MD5。 */
export async function md5Base64(
	file: Blob,
	signal?: AbortSignal,
): Promise<string> {
	const hasher = await createMD5();
	hasher.init();
	for (let offset = 0; offset < file.size; offset += MD5_SLICE_BYTES) {
		throwIfAborted(signal);
		const slice = file.slice(offset, offset + MD5_SLICE_BYTES);
		hasher.update(new Uint8Array(await slice.arrayBuffer()));
	}
	let bin = "";
	for (const byte of hasher.digest("binary")) bin += String.fromCharCode(byte);
	return btoa(bin);
}
