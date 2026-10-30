/**
 * SSE client wrapper used by the realtime hooks (留言 / 通知).
 *
 * Built on `fetch + ReadableStream` rather than the native EventSource
 * because:
 *   - EventSource cannot set custom headers, but the authed notification
 *     stream needs `X-Active-Role` + the session cookie, and
 *     reconnect needs `Last-Event-ID` set explicitly (the cached id may
 *     come from a previous in-memory state, not the browser's own).
 *   - We want fine-grained control over reconnect timing, heartbeat
 *     watchdog (30s — design book §3 / §13), and 429 connection-cap
 *     handling (per-account 3 concurrent streams; subsequent tabs degrade
 *     to "stale" rather than thrashing the gateway).
 *
 * Design-book mappings:
 *   - §10 Lifecycle: caller decides when to `open()` / `close()`.
 *   - §10 Failure: reconnect with exponential backoff; after the total
 *     downtime exceeds 30s OR we hit 429/503, we surface status
 *     "unavailable" and STOP retrying. The hook caller renders the
 *     "請重新整理頁面" copy in that state — we do not fall back to polling.
 *     A dead stream is revived (with a fresh downtime budget) when the tab
 *     becomes visible again or the network comes back, so backgrounding a
 *     tab past the budget doesn't freeze realtime for the rest of the session.
 *   - §13.1 Last-Event-ID: passed in via `lastEventId` on open() and
 *     refreshed on every event so reconnect resumes from the freshest id.
 *   - §13 Buffer-miss: `ready` events with `bufferMiss=true` are surfaced
 *     to the caller, which then triggers a one-shot REST refresh.
 *
 * Concurrency contract: one in-flight fetch at a time. `close()` aborts
 * it and marks the client closed so any pending reconnect timer is
 * dropped. Calling `open()` after `close()` is supported (new instance
 * is preferred; reuse only valid before close).
 */

import { AUTH_EXPIRED_EVENT, apiClient } from "@/lib/api/client";
import { getStoredActiveRole, roleToHeader } from "@/lib/auth/active-role";

export type SseConnectionStatus =
	| "idle"
	| "connecting"
	| "connected"
	| "offline"
	| "unavailable";

export interface SseReadyInfo {
	topic: string;
	serverTime: string;
	bufferMiss: boolean;
	replayed: number;
}

export interface SseClientHandlers {
	/** Per-event dispatch. `id` is the SSE `id:` line (null if absent). */
	onEvent: (eventName: string, data: unknown, id: string | null) => void;
	/** Server sent its `ready` envelope after (re)connect. */
	onReady: (info: SseReadyInfo) => void;
	/** Status changed (mirrors design-book lifecycle states). */
	onStatusChange?: (status: SseConnectionStatus) => void;
}

export interface SseClientOptions {
	/** Path relative to the gateway base; `mode` decides /api/v1/public vs /api/v1/app. */
	path: string;
	mode: "public" | "authed";
	/** Cached Last-Event-ID to send on the first connect (and refresh on every event). */
	initialLastEventId?: string | null;
	/** Override defaults (mainly for tests). */
	heartbeatTimeoutMs?: number;
	maxDowntimeMs?: number;
	handlers: SseClientHandlers;
}

const DEFAULT_HEARTBEAT_TIMEOUT_MS = 35_000; // 30s heartbeat + 5s grace
const DEFAULT_MAX_DOWNTIME_MS = 30_000;
const BACKOFF_INITIAL_MS = 800;
const BACKOFF_MAX_MS = 8_000;

export class SseClient {
	private sessionEnded(): void {
		this.close();
		if (this.mode === "authed" && typeof window !== "undefined") {
			window.dispatchEvent(
				new CustomEvent(AUTH_EXPIRED_EVENT, {
					detail: { path: this.path, message: "SSE 401" },
				}),
			);
		}
		this.setStatus("unavailable");
	}

	private readonly path: string;
	private readonly mode: "public" | "authed";
	private readonly handlers: SseClientHandlers;
	private readonly heartbeatTimeoutMs: number;
	private readonly maxDowntimeMs: number;

	private lastEventId: string | null;
	private status: SseConnectionStatus = "idle";

	private abortController: AbortController | null = null;
	private heartbeatTimer: ReturnType<typeof setTimeout> | null = null;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private offlineListenersBound = false;

	private firstDisconnectAt: number | null = null;
	private reconnectAttempt = 0;
	/** One Retry-After-honouring retry budget for 429 (reset on connect). */
	private rateLimitRetried = false;

	private closed = false;

	constructor(options: SseClientOptions) {
		this.path = options.path;
		this.mode = options.mode;
		this.handlers = options.handlers;
		this.heartbeatTimeoutMs =
			options.heartbeatTimeoutMs ?? DEFAULT_HEARTBEAT_TIMEOUT_MS;
		this.maxDowntimeMs = options.maxDowntimeMs ?? DEFAULT_MAX_DOWNTIME_MS;
		this.lastEventId = options.initialLastEventId ?? null;
	}

	/** Current Last-Event-ID seen by this client (read for cache writes). */
	getLastEventId(): string | null {
		return this.lastEventId;
	}

	open() {
		if (this.closed) return;
		this.bindOfflineListeners();
		void this.connect();
	}

	close() {
		this.closed = true;
		this.clearReconnectTimer();
		this.clearHeartbeatTimer();
		this.abort();
		this.unbindOfflineListeners();
		this.setStatus("idle");
	}

	private async connect() {
		if (this.closed) return;
		if (typeof navigator !== "undefined" && navigator.onLine === false) {
			this.setStatus("offline");
			return;
		}
		this.abort();
		this.setStatus("connecting");
		const controller = new AbortController();
		this.abortController = controller;

		const headers = new Headers({
			Accept: "text/event-stream",
			// Avoid intermediary caches buffering the stream.
			"Cache-Control": "no-cache",
		});
		if (this.lastEventId) headers.set("Last-Event-ID", this.lastEventId);
		// Authed streams mirror the apiClient.dispatch() header policy: the
		// stored UI workspace is sent so the gateway scopes notifications to
		// the same role the REST list / unread-count endpoints use. Without
		// this header, a dual-role account in seller view would receive a
		// stream that doesn't match what `/app/notifications/unread-count`
		// returns. The role is re-read on every (re)connect, and role
		// switching restarts the stream via the hook's `resetSignal`, so the
		// header always tracks the current selection.
		if (this.mode === "authed") {
			const role = roleToHeader(getStoredActiveRole());
			if (role) headers.set("X-Active-Role", role);
		}

		const url = apiClient.buildUrl(this.mode, this.path);

		let response: Response;
		const request: RequestInit = {
			method: "GET",
			headers,
			credentials: "include",
			signal: controller.signal,
			cache: "no-store",
		};
		try {
			response = await fetch(url, request);
		} catch (err) {
			// AbortError from `close()` is terminal (closed flag set), but
			// AbortError from the heartbeat watchdog should fall through to
			// scheduleReconnect — `scheduleReconnect` itself bails when closed.
			if ((err as Error)?.name === "AbortError" && this.closed) return;
			this.scheduleReconnect();
			return;
		}

		if (response.status === 401) {
			this.sessionEnded();
			return;
		}
		if (response.status === 429 || response.status === 503) {
			// Per-account connection-cap or gateway shed — design book §11.6 /
			// §10 says we surface "請重新整理頁面" and stop hitting the gateway.
			// One Retry-After-honouring retry first, though: the commonest 429
			// here is the 3-streams-per-account cap hit transiently while
			// another page's stream is still tearing down (e.g. fast viewer
			// paging). A second consecutive 429 is treated as real.
			if (response.status === 429 && !this.rateLimitRetried && !this.closed) {
				this.rateLimitRetried = true;
				const after = Number(response.headers.get("retry-after"));
				const delay =
					Number.isFinite(after) && after > 0
						? Math.min(after * 1000, 30_000)
						: 8_000;
				this.clearReconnectTimer();
				this.reconnectTimer = setTimeout(() => {
					this.reconnectTimer = null;
					void this.connect();
				}, delay);
				return;
			}
			this.setStatus("unavailable");
			return;
		}
		if (!response.ok || !response.body) {
			this.scheduleReconnect();
			return;
		}

		this.setStatus("connected");
		this.firstDisconnectAt = null;
		this.reconnectAttempt = 0;
		this.rateLimitRetried = false;
		this.armHeartbeatTimer();

		try {
			await this.readStream(response.body);
		} catch (err) {
			// User close: closed flag is set → don't reconnect.
			// Watchdog abort: closed=false → fall through to scheduleReconnect.
			if ((err as Error)?.name === "AbortError" && this.closed) return;
		}
		this.clearHeartbeatTimer();
		if (!this.closed) this.scheduleReconnect();
	}

	private async readStream(body: ReadableStream<Uint8Array>) {
		const reader = body.getReader();
		const decoder = new TextDecoder();
		let buffer = "";

		while (true) {
			const { value, done } = await reader.read();
			if (done) break;
			buffer += decoder.decode(value, { stream: true });
			while (true) {
				const idx = buffer.indexOf("\n\n");
				if (idx === -1) break;
				const raw = buffer.slice(0, idx);
				buffer = buffer.slice(idx + 2);
				this.armHeartbeatTimer();
				this.dispatchRaw(raw);
			}
		}
	}

	private dispatchRaw(raw: string) {
		let eventName = "message";
		let id: string | null = null;
		const dataLines: string[] = [];
		for (const line of raw.split("\n")) {
			if (!line || line.startsWith(":")) continue; // comments / heartbeats lacking event:
			if (line.startsWith("event:")) {
				eventName = line.slice(6).trim();
			} else if (line.startsWith("data:")) {
				dataLines.push(line.slice(5).replace(/^\s/, ""));
			} else if (line.startsWith("id:")) {
				id = line.slice(3).trim();
			}
		}
		const dataRaw = dataLines.join("\n");
		let parsed: unknown = dataRaw;
		if (dataRaw) {
			try {
				parsed = JSON.parse(dataRaw);
			} catch {
				// Plain string payload — leave as-is.
			}
		}
		if (id) this.lastEventId = id;

		if (eventName === "heartbeat") {
			// Already rearmed the watchdog in readStream(); nothing else to do.
			return;
		}
		if (eventName === "ready") {
			const info = (parsed ?? {}) as Partial<SseReadyInfo>;
			this.handlers.onReady({
				topic: typeof info.topic === "string" ? info.topic : "",
				serverTime: typeof info.serverTime === "string" ? info.serverTime : "",
				bufferMiss: info.bufferMiss === true,
				replayed: Number(info.replayed ?? 0) || 0,
			});
			return;
		}
		// 註解 frame（`: ping` 之類的 keepalive）沒有任何 data: 行，依 SSE 規格
		// 「data buffer 是空的就不 dispatch」，它根本不是一個事件。分派出去會讓
		// 呼叫端收到 data 為空字串的假事件——留言／通知那兩條靠 switch default
		// 吞掉，但 AG-UI 那條會拿 "" 去跑 schema 驗證、判定契約不符而收線，
		// gateway 的 20 秒 keepalive 就足以殺掉整條訂閱。
		// 注意 `data:`（有這行但值是空的）不算註解，dataLines 會有一個空字串，
		// 仍然要照常分派——所以判斷的是「有沒有 data: 行」，不是 dataRaw 是否為空。
		// watchdog 的 rearm 在 readStream 就做完了，keepalive 的 liveness 訊號
		// 不會因為這裡提早 return 而漏掉。
		if (dataLines.length === 0) return;

		this.handlers.onEvent(eventName, parsed, id);
	}

	private armHeartbeatTimer() {
		this.clearHeartbeatTimer();
		this.heartbeatTimer = setTimeout(() => {
			// No data within the heartbeat window — abort + reconnect.
			this.abort();
		}, this.heartbeatTimeoutMs);
	}

	private clearHeartbeatTimer() {
		if (this.heartbeatTimer) {
			clearTimeout(this.heartbeatTimer);
			this.heartbeatTimer = null;
		}
	}

	private scheduleReconnect() {
		if (this.closed) return;
		if (typeof navigator !== "undefined" && navigator.onLine === false) {
			this.setStatus("offline");
			return;
		}
		if (this.firstDisconnectAt === null) this.firstDisconnectAt = Date.now();
		const downtime = Date.now() - this.firstDisconnectAt;
		if (downtime >= this.maxDowntimeMs) {
			this.setStatus("unavailable");
			return;
		}
		this.reconnectAttempt += 1;
		const delay = Math.min(
			BACKOFF_MAX_MS,
			BACKOFF_INITIAL_MS * 2 ** (this.reconnectAttempt - 1),
		);
		this.clearReconnectTimer();
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			void this.connect();
		}, delay);
	}

	private clearReconnectTimer() {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
	}

	private abort() {
		if (this.abortController) {
			try {
				this.abortController.abort();
			} catch {
				/* noop */
			}
			this.abortController = null;
		}
	}

	private setStatus(next: SseConnectionStatus) {
		if (this.status === next) return;
		this.status = next;
		this.handlers.onStatusChange?.(next);
	}

	// A stream that exhausted its 30s downtime budget (or hit 429/503) lands in
	// "unavailable" and stops retrying — correct while the tab is backgrounded or
	// the network is down, but it must not stay dead once the user returns or the
	// connection recovers, or realtime (notifications / 留言) silently freezes for
	// the rest of the session. Network-back and tab-visible are both fresh chances:
	// reset the downtime window so the reconnect gets the full budget again rather
	// than inheriting the exhausted one.
	private reviveFromDeadState() {
		if (this.closed) return;
		if (this.status !== "unavailable" && this.status !== "offline") return;
		if (typeof navigator !== "undefined" && navigator.onLine === false) return;
		this.firstDisconnectAt = null;
		this.reconnectAttempt = 0;
		this.rateLimitRetried = false;
		this.clearReconnectTimer();
		void this.connect();
	}

	private onOnline = () => {
		this.reviveFromDeadState();
	};

	private onVisibilityChange = () => {
		if (typeof document === "undefined") return;
		if (document.visibilityState === "visible") this.reviveFromDeadState();
	};

	private onOffline = () => {
		this.abort();
		this.clearHeartbeatTimer();
		this.clearReconnectTimer();
		this.setStatus("offline");
	};

	private bindOfflineListeners() {
		if (this.offlineListenersBound) return;
		if (typeof window === "undefined") return;
		window.addEventListener("online", this.onOnline);
		window.addEventListener("offline", this.onOffline);
		// Refocusing a backgrounded tab is the commonest way a user meets a stream
		// that died while hidden; revive it on visibility too, not just network.
		if (typeof document !== "undefined") {
			document.addEventListener("visibilitychange", this.onVisibilityChange);
		}
		this.offlineListenersBound = true;
	}

	private unbindOfflineListeners() {
		if (!this.offlineListenersBound) return;
		if (typeof window === "undefined") return;
		window.removeEventListener("online", this.onOnline);
		window.removeEventListener("offline", this.onOffline);
		if (typeof document !== "undefined") {
			document.removeEventListener("visibilitychange", this.onVisibilityChange);
		}
		this.offlineListenersBound = false;
	}
}
