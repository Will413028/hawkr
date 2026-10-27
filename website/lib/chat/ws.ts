/**
 * Chat WebSocket client.
 *
 * Connects to `wss://<gatewayHost>/api/v1/app/chat/ws?ticket=<ticket>`.
 * Browser WebSocket cannot set custom headers, and the httpOnly session cookie
 * does not reach a cross-site WS handshake. Instead, an authed
 * `POST /app/chat/ws-ticket` (cookie-authenticated via `apiClient.authed`)
 * mints a short-lived single-use ticket; the WS URL carries ONLY that ticket —
 * no cookie, no long-lived credential. The gateway consumes the ticket
 * (single-use) and derives identity/role server-side. Each (re)connect mints a
 * fresh ticket because single-use requires it.
 *
 * Lifecycle:
 *   - connect():    open, set state=connecting → connected
 *   - heartbeat:    every 15s send `{type:'ping', from, timestamp}`; server
 *                   replies `{type:'pong'}` (im-service #1198)
 *   - read-deadline: arm a liveness timer on open; any inbound frame re-arms
 *                   it. No frame within 30s (2× ping) → the socket is half-open
 *                   (dead) → close + reconnect. Catches silent NAT/proxy drops
 *                   that never fire `onclose`.
 *   - reconnect:    full-jitter backoff (random up to 1,2,4,8,16 s; 5 attempts),
 *                   then held
 *                   "disconnected" until a focus / online / visibility event
 *                   revives it (counter reset) — never permanently dead. Missed
 *                   messages are recovered by a full REST refetch (useChat), not
 *                   an incremental WS sync.
 *   - presence:     subscribePresence(peer, role) → server emits live status
 *   - typing:       sendTypingIndicator() / stop_typing
 *
 * Read receipts are REST-only (markConversationRead); the WS read_receipt frame
 * is consumed by mobile peers and merely swallowed here.
 *
 * Dedup: by message id in the timeline (timeline.upsertMessage).
 */

import { apiClient } from "../api/client";
import { getEnv } from "../config/env";
import {
	type ChatMessage,
	type ConnectionState,
	parseWSMessage,
	type WSPayload,
} from "./types";

/** Parse `unreadCount` from an `unread_changed` payload: coerce string→number,
 * treat missing/NaN/negative as 0. Keeps 0 to support mark-read zeroing. */
export function parseUnreadCount(payload: {
	unreadCount?: number | string;
}): number {
	const raw = payload.unreadCount;
	if (raw === undefined || raw === null) return 0;
	const n = Number(raw);
	if (!Number.isFinite(n)) return 0;
	return Math.max(0, Math.floor(n));
}

type Listener<T> = (event: T) => void;

interface PresenceEvent {
	peerId: string;
	peerRole: number;
	online: boolean;
	lastSeenAtMs: number;
}

/** App-level ping cadence. 15s is a carrier-NAT-conservative provisional value
 *  (idle ~30s → ping at 1/3–1/2), matched across web + buyer + seller; the IM
 *  design (D6 / §11.2) gates the final value on measuring CF/edge idle timeout.
 *  ponytail: provisional 15s, retune once CF/edge idle is measured. */
const PING_INTERVAL_MS = 15_000;
/** Read-side deadline: no inbound frame within this window ⇒ half-open socket.
 *  2× the ping interval — one missed pong is tolerated, two is dead. */
const LIVENESS_TIMEOUT_MS = 30_000;
/** Per the IM Service design (§2.4): stop hammering after 5 consecutive
 *  failures. Recovery then rides the focus / online / visibility listeners
 *  below — they reset this counter and retry immediately when the user or
 *  network comes back, so a sustained outage never leaves chat permanently
 *  dead (the old behaviour that left "即時更新暫停中" stuck until a refresh). */
const MAX_RECONNECT = 5;
/** Full-Jitter backoff (§6.2): delay = random(0, min(cap, base * 2^(attempt-1))).
 *  Jitter decorrelates a fleet of tabs so a single-replica restart doesn't make
 *  them all reconnect in lockstep and re-mint tickets in one thundering herd. */
const RECONNECT_BASE_MS = 1_000;
const RECONNECT_CAP_MS = 30_000;

/** Derive the WS origin from the gateway origin (https→wss, http→ws). */
function gatewayWsBase(): string {
	return getEnv().gatewayHost.replace(/^http/, "ws");
}

/**
 * Mint a short-lived single-use WS ticket via an authed POST (sends the cookie
 * + X-Active-Role) and compose the wss URL. The WS connects to the absolute
 * gateway origin — NOT the same-origin dev proxy, which does not forward WS
 * upgrades. Returns null when the user is not authenticated or minting fails,
 * so the socket never opens and chat degrades gracefully.
 */
export async function buildWsUrl(): Promise<{
	url: string;
	userId: string;
	userType: number;
} | null> {
	try {
		const { data } = await apiClient.authed.post<{
			ticket: string;
			userId: string;
			userType: number;
		}>("/chat/ws-ticket");
		if (!data?.ticket) return null;
		const url = `${gatewayWsBase()}/api/v1/app/chat/ws?ticket=${encodeURIComponent(data.ticket)}`;
		return { url, userId: data.userId, userType: data.userType };
	} catch {
		return null;
	}
}

export class ChatSocket {
	private socket: WebSocket | null = null;
	private state: ConnectionState = "disconnected";
	private reconnectAttempts = 0;
	private pingTimer: ReturnType<typeof setInterval> | null = null;
	private livenessTimer: ReturnType<typeof setTimeout> | null = null;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

	private msgListeners = new Set<Listener<ChatMessage>>();
	private stateListeners = new Set<Listener<ConnectionState>>();
	private typingListeners = new Set<
		Listener<{ from: string; typing: boolean }>
	>();
	private presenceListeners = new Set<Listener<PresenceEvent>>();
	private unreadChangedListeners = new Set<
		Listener<{ role: string; unreadCount: number }>
	>();

	private currentUserId = "";
	/** Set by disconnect() (e.g. leaving chat) to suppress auto-reconnect and
	 *  wake-driven reconnect until an explicit connect() re-enables it. */
	private manualClose = false;
	private listenersInstalled = false;

	connect(): void {
		this.manualClose = false;
		this.installGlobalListeners();
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		if (this.state === "connected" || this.state === "connecting") return;
		this.setState("connecting");
		void this.openWithTicket();
	}

	private async openWithTicket(): Promise<void> {
		const built = await buildWsUrl();
		// disconnect() may have run while we awaited the ticket.
		if (this.state !== "connecting") return;
		if (!built) {
			// Transient ticket-mint failure (offline / brief 401). Treat as a
			// reconnect attempt rather than dying outright, so a blip doesn't
			// permanently kill live updates.
			this.scheduleReconnect();
			return;
		}
		this.currentUserId = built.userId;

		let socket: WebSocket;
		try {
			socket = new WebSocket(built.url);
		} catch {
			this.scheduleReconnect();
			return;
		}
		this.socket = socket;

		socket.onopen = () => {
			this.reconnectAttempts = 0;
			this.setState("connected");
			this.startPing();
			this.armLiveness();
		};
		socket.onmessage = (ev) => {
			// Any inbound frame (chat, pong, presence, …) proves the link is alive.
			this.armLiveness();
			try {
				const data = JSON.parse(String(ev.data)) as WSPayload;
				this.dispatch(data);
			} catch {
				/* ignore malformed frame */
			}
		};
		socket.onerror = () => {
			// onclose will follow; let it handle the lifecycle.
		};
		socket.onclose = () => {
			this.stopPing();
			this.stopLiveness();
			if (this.state !== "disconnected") {
				this.scheduleReconnect();
			}
		};
	}

	disconnect(): void {
		this.manualClose = true;
		this.stopPing();
		this.stopLiveness();
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.reconnectAttempts = 0;
		this.setState("disconnected");
		try {
			this.socket?.close();
		} catch {
			/* ignore */
		}
		this.socket = null;
	}

	get connectionState(): ConnectionState {
		return this.state;
	}

	/* ===== Send helpers ===== */

	sendTypingIndicator(receiverId: string | number, typing = true): void {
		this.send({
			// from/to must be JSON strings: the backend domain.Message tags them
			// `int64,string`, so a bare number (large TSID) fails unmarshal and the
			// whole frame is dropped. Mirror subscribePresence, which sends strings.
			type: typing ? "typing" : "stop_typing",
			from: String(this.currentUserId),
			to: String(receiverId),
			timestamp: Date.now(),
		});
	}

	subscribePresence(peerId: number | string, peerRole: number): void {
		this.send({ type: "presence_sub", to: peerId, peerRole });
	}

	unsubscribePresence(peerId: number | string, peerRole: number): void {
		this.send({ type: "presence_unsub", to: peerId, peerRole });
	}

	/* ===== Subscriptions ===== */

	onMessage(fn: Listener<ChatMessage>): () => void {
		this.msgListeners.add(fn);
		return () => this.msgListeners.delete(fn);
	}

	onState(fn: Listener<ConnectionState>): () => void {
		this.stateListeners.add(fn);
		// Emit the current state immediately: connect() is a no-op when the
		// singleton is already connected (NotificationToastBridge connects it
		// app-wide), so a future-only subscription would otherwise leave a
		// late-mounting consumer (ThreadClient) stuck at its initial
		// "disconnected" — a false "網路連線異常" banner over a live socket.
		fn(this.state);
		return () => this.stateListeners.delete(fn);
	}

	onTyping(fn: Listener<{ from: string; typing: boolean }>): () => void {
		this.typingListeners.add(fn);
		return () => this.typingListeners.delete(fn);
	}

	onPresence(fn: Listener<PresenceEvent>): () => void {
		this.presenceListeners.add(fn);
		return () => this.presenceListeners.delete(fn);
	}

	onUnreadChanged(
		fn: Listener<{ role: string; unreadCount: number }>,
	): () => void {
		this.unreadChangedListeners.add(fn);
		return () => this.unreadChangedListeners.delete(fn);
	}

	/* ===== Internals ===== */

	private setState(state: ConnectionState) {
		if (this.state === state) return;
		this.state = state;
		for (const fn of this.stateListeners) fn(state);
	}

	private send(payload: Record<string, unknown>): void {
		if (!this.socket || this.state !== "connected") return;
		try {
			this.socket.send(JSON.stringify(payload));
		} catch {
			/* ignore — onclose will pick up the disconnect */
		}
	}

	private startPing(): void {
		this.stopPing();
		this.pingTimer = setInterval(() => {
			this.send({
				type: "ping",
				from: this.currentUserId,
				timestamp: Date.now(),
			});
		}, PING_INTERVAL_MS);
	}

	private stopPing(): void {
		if (this.pingTimer) {
			clearInterval(this.pingTimer);
			this.pingTimer = null;
		}
	}

	/** (Re)arm the read-side deadline. Called on open and on every inbound
	 *  frame; if it fires, no frame arrived for LIVENESS_TIMEOUT_MS so the socket
	 *  is half-open — close it and let onclose drive the reconnect. */
	private armLiveness(): void {
		this.stopLiveness();
		this.livenessTimer = setTimeout(() => {
			try {
				this.socket?.close();
			} catch {
				/* ignore — onclose handles the lifecycle */
			}
		}, LIVENESS_TIMEOUT_MS);
	}

	private stopLiveness(): void {
		if (this.livenessTimer) {
			clearTimeout(this.livenessTimer);
			this.livenessTimer = null;
		}
	}

	private scheduleReconnect(): void {
		if (this.manualClose) return;
		if (this.reconnectAttempts >= MAX_RECONNECT) {
			// Gave up for now — the focus / online / visibility listeners revive it
			// (resetting the counter) the moment the user or network returns.
			this.setState("disconnected");
			return;
		}
		this.setState("reconnecting");
		this.reconnectAttempts += 1;
		const ceiling = Math.min(
			RECONNECT_BASE_MS * 2 ** (this.reconnectAttempts - 1),
			RECONNECT_CAP_MS,
		);
		const delay = Math.random() * ceiling;
		if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
		this.reconnectTimer = setTimeout(() => {
			this.connect();
		}, delay);
	}

	/**
	 * Revive a dropped socket the instant the user returns or the network comes
	 * back — the recovery path after the backoff has given up. Resets the
	 * attempt counter so we get a fresh set of tries. Installed once per tab
	 * (the socket is a tab singleton); SSR-safe.
	 */
	private installGlobalListeners(): void {
		if (this.listenersInstalled || typeof window === "undefined") return;
		this.listenersInstalled = true;
		const wake = () => this.wake();
		window.addEventListener("online", wake);
		window.addEventListener("focus", wake);
		document.addEventListener("visibilitychange", () => {
			if (document.visibilityState === "visible") this.wake();
		});
	}

	private wake(): void {
		if (this.manualClose) return;
		if (this.state === "connected" || this.state === "connecting") return;
		this.reconnectAttempts = 0;
		this.connect();
	}

	private dispatch(payload: WSPayload): void {
		const t = payload.type;
		switch (t) {
			// Liveness acks — the deadline was already re-armed in onmessage; nothing
			// to do but avoid treating them as chat messages.
			case "pong":
			case "heartbeat":
				return;
			case "typing":
			case "start_typing":
			case "stop_typing": {
				const from = String(payload.from ?? payload.fromUserId ?? "");
				const typing = t !== "stop_typing";
				for (const fn of this.typingListeners) fn({ from, typing });
				return;
			}
			// Web tracks read state via REST (markConversationRead) and confirms its
			// own sends via the REST response, so neither read_receipt (a mobile peer
			// still emits it) nor chat_ack is consumed here. Swallow both rather than
			// let them fall through to `default` and render as empty message bubbles.
			case "read_receipt":
			case "chat_ack":
				return;
			case "presence": {
				const peerId = String(payload.to ?? "");
				const peerRole = Number(payload.peerRole ?? 0);
				if (!peerId || !peerRole) return;
				const ev: PresenceEvent = {
					peerId,
					peerRole,
					online: payload.online === true,
					lastSeenAtMs: Number(payload.lastSeenAt ?? 0),
				};
				for (const fn of this.presenceListeners) fn(ev);
				return;
			}
			case "unread_changed": {
				const role = String(payload.role ?? "");
				const unreadCount = parseUnreadCount(payload);
				for (const fn of this.unreadChangedListeners) fn({ role, unreadCount });
				return;
			}
			default: {
				// Dedup is by message id in the timeline (upsertMessage), so no
				// second {conversationId}_{seqId} layer is needed here.
				const msg = parseWSMessage(payload);
				for (const fn of this.msgListeners) fn(msg);
			}
		}
	}
}

/** Singleton — one socket per tab keeps things simple. */
let singleton: ChatSocket | null = null;
export function getChatSocket(): ChatSocket {
	if (!singleton) singleton = new ChatSocket();
	return singleton;
}
