import { ConnectError } from "@connectrpc/connect";
import type { WatchRunResponse } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import {
	failureCode,
	failureRetryable,
	type ProcurementFailureCode,
} from "./errors";

/**
 * `WatchRun`（server streaming）的接續訂閱。
 *
 * 伺服器每條串流最長 100 秒就正常結束；結束時沒有收到 `finished` 就是被截斷。這裡以最後收到的 cursor 重新訂閱，
 * 事件依序、不漏不重：cursor 是不透明的字串，只定義相等，所以以「已見過的 cursor 集合」丟掉重播的事件。
 *
 * - 正常結束或暫時性錯誤（網路、`unavailable`、`internal`、帶 RetryInfo 的 `resource_exhausted`）：重連。
 *   連續沒有進展的失敗以 full-jitter backoff 重試，`MAX_ATTEMPTS` 次後停下回 `exhausted`。
 * - `REPLAY_CURSOR_EXPIRED`／`REPLAY_CURSOR_AHEAD`：回 `resync`，由呼叫端讀快照重建。
 * - 其餘錯誤（`unauthenticated`、`not_found`、`permission_denied`、`failed_precondition`…）：回 `failed`，不重連。
 * - 收到 `finished` 事件：交給 `onEvent` 之後回 `finished`。
 */

export const MAX_ATTEMPTS = 5;
export const BACKOFF_CAP_MS = 16_000;
/** 連線活了這麼久才結束，視為健康（run 閒置到伺服器的 100 秒上限也是正常結束）。 */
export const HEALTHY_STREAM_MS = 5_000;
/** 去重用的 cursor 集合上限；超過時丟掉最舊的。 */
export const MAX_SEEN_CURSORS = 4096;

export interface WatchRunRequest {
	runId: string;
	cursor: string;
}

export type WatchOpen = (
	request: WatchRunRequest,
	signal: AbortSignal,
) => AsyncIterable<WatchRunResponse>;

export interface WatchDeps {
	/** [0, 1)；測試注入。 */
	random?: () => number;
	setTimer?: (fn: () => void, ms: number) => unknown;
	clearTimer?: (t: unknown) => void;
	now?: () => number;
}

export type WatchOutcome =
	| { kind: "finished" }
	| { kind: "resync"; reason: "REPLAY_CURSOR_EXPIRED" | "REPLAY_CURSOR_AHEAD" }
	| { kind: "failed"; error: unknown; code: ProcurementFailureCode }
	| { kind: "exhausted"; error: unknown }
	| { kind: "aborted" };

export interface WatchRunOptions {
	open: WatchOpen;
	runId: string;
	signal: AbortSignal;
	onEvent: (event: WatchRunResponse) => void;
	deps?: WatchDeps;
}

type StreamEnd =
	| { kind: "ended" }
	| { kind: "finished" }
	| { kind: "error"; error: unknown };

function isTransient(code: ProcurementFailureCode, err: ConnectError): boolean {
	return (
		code === "NETWORK_ERROR" ||
		code === "SERVER_ERROR" ||
		failureRetryable(err)
	);
}

export async function watchRun(opts: WatchRunOptions): Promise<WatchOutcome> {
	const { open, runId, signal, onEvent } = opts;
	const random = opts.deps?.random ?? Math.random;
	const setTimer =
		opts.deps?.setTimer ?? ((fn: () => void, ms: number) => setTimeout(fn, ms));
	const clearTimer =
		opts.deps?.clearTimer ??
		((t: unknown) => clearTimeout(t as ReturnType<typeof setTimeout>));
	const now = opts.deps?.now ?? Date.now;

	let cursor = "";
	const seen = new Set<string>();
	const remember = (value: string) => {
		seen.delete(value);
		seen.add(value);
		while (seen.size > MAX_SEEN_CURSORS) {
			const oldest = seen.values().next().value;
			if (oldest === undefined) break;
			seen.delete(oldest);
		}
	};

	const sleep = (ms: number) =>
		new Promise<void>((resolve) => {
			const onAbort = () => {
				clearTimer(timer);
				resolve();
			};
			const timer = setTimer(() => {
				signal.removeEventListener("abort", onAbort);
				resolve();
			}, ms);
			signal.addEventListener("abort", onAbort, { once: true });
		});

	let failures = 0;
	while (!signal.aborted) {
		const connection = new AbortController();
		const relay = () => connection.abort();
		signal.addEventListener("abort", relay, { once: true });
		const startedAt = now();
		let progressed = false;
		let end: StreamEnd = { kind: "ended" };
		try {
			const iterator = open({ runId, cursor }, connection.signal)[
				Symbol.asyncIterator
			]();
			while (true) {
				let next: IteratorResult<WatchRunResponse>;
				try {
					next = await iterator.next();
				} catch (error) {
					end = { kind: "error", error };
					break;
				}
				if (next.done) break;
				if (signal.aborted) return { kind: "aborted" };
				const event = next.value;
				if (event.cursor !== "") {
					if (seen.has(event.cursor)) continue;
					remember(event.cursor);
					cursor = event.cursor;
				}
				progressed = true;
				onEvent(event);
				if (event.event.case === "finished") {
					end = { kind: "finished" };
					break;
				}
			}
		} catch (error) {
			end = { kind: "error", error };
		} finally {
			signal.removeEventListener("abort", relay);
			connection.abort();
		}

		if (signal.aborted) return { kind: "aborted" };
		if (end.kind === "finished") return { kind: "finished" };

		const healthy = progressed || now() - startedAt >= HEALTHY_STREAM_MS;
		if (end.kind === "error") {
			const err = ConnectError.from(end.error);
			const code = failureCode(err);
			if (code === "REPLAY_CURSOR_EXPIRED" || code === "REPLAY_CURSOR_AHEAD") {
				return { kind: "resync", reason: code };
			}
			if (!isTransient(code, err)) {
				return { kind: "failed", error: end.error, code };
			}
		}

		if (healthy && end.kind === "ended") {
			failures = 0;
			continue;
		}
		failures = healthy ? 1 : failures + 1;
		if (failures >= MAX_ATTEMPTS) {
			return {
				kind: "exhausted",
				error: end.kind === "error" ? end.error : undefined,
			};
		}
		const ceiling = Math.min(BACKOFF_CAP_MS, 1000 * 2 ** (failures - 1));
		await sleep(Math.floor(random() * ceiling));
	}
	return { kind: "aborted" };
}
