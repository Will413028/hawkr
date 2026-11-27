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

const DEFAULT_HEARTBEAT_TIMEOUT_MS = 35_000;
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
			"Cache-Control": "no-cache",
		});
		if (this.lastEventId) headers.set("Last-Event-ID", this.lastEventId);
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
			if ((err as Error)?.name === "AbortError" && this.closed) return;
			this.scheduleReconnect();
			return;
		}

		if (response.status === 401) {
			this.sessionEnded();
			return;
		}
		if (response.status === 429 || response.status === 503) {
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
			if (!line || line.startsWith(":")) continue;
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
			}
		}
		if (id) this.lastEventId = id;

		if (eventName === "heartbeat") {
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
		if (dataLines.length === 0) return;

		this.handlers.onEvent(eventName, parsed, id);
	}

	private armHeartbeatTimer() {
		this.clearHeartbeatTimer();
		this.heartbeatTimer = setTimeout(() => {
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
			}
			this.abortController = null;
		}
	}

	private setStatus(next: SseConnectionStatus) {
		if (this.status === next) return;
		this.status = next;
		this.handlers.onStatusChange?.(next);
	}

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
