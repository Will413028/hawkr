/**
 * 聊天的即時事件流（`StreamEvents`／`StreamStoreEvents`，server streaming）的 client。
 *
 * 串流的生命週期（見 chat.proto）：
 * - 第一個 frame 是 `Ready`（總未讀、心跳間隔、最長壽命）；之後是訊息、未讀、對方已讀、`Resync`，閒置時 `Heartbeat`。
 * - 超過心跳間隔的兩倍沒收到任何 frame，就當連線已斷：中止並重連。
 * - 活到 `max_lifetime` 後正常結束（沒有錯誤）：立刻以每個對話最後的 seq 重連，不算失敗。
 * - `unavailable`（與網路錯誤）：full-jitter backoff 重連，連續失敗 `MAX_ATTEMPTS` 次後停下，等 focus／online／
 *   visibility 的 `resume()`。
 * - `unauthenticated`、`not_found`、`resource_exhausted`、`failed_precondition`（含 STORE_CLOSED）、
 *   `permission_denied`：不重連（登入失效由 transport 發 AUTH_EXPIRED_EVENT）。
 * - `invalid_argument` 照一般失敗 backoff 重連，不當終止：cursors 超過伺服器上限會回這個 code，
 *   終止會讓聊天永遠斷線；cursors 本身也限在最近 `MAX_CURSORS` 個對話。
 */

import type { Duration } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import type { ChatEvent } from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { chatStreamClient } from "@/lib/connect/chat";
import { messageFromProto } from "./map";
import type { ChatMessage, ChatSide, ConnectionState } from "./types";

export type ChatStreamEvent =
	| { type: "ready"; unreadCount: number }
	| { type: "message"; message: ChatMessage; updated: boolean }
	| { type: "unread"; unreadCount: number }
	| { type: "peerRead"; conversationId: string; peerReadSeq: number }
	| { type: "resync"; conversationId?: string };

/** 重連前的最多連續失敗次數，超過後等 `resume()`。 */
export const MAX_ATTEMPTS = 5;
/** backoff 的上限（毫秒）：1、2、4、8、16 秒，full jitter。 */
export const BACKOFF_CAP_MS = 16_000;
/** 收到 `Ready` 之前，等第一個 frame 的上限。 */
export const READY_TIMEOUT_MS = 30_000;
/** `cursors` 的對話數上限（chat.proto）：超過的伺服器回 invalid_argument。只送最近有動靜的這麼多個。 */
export const MAX_CURSORS = 50;

export type OpenStream = (
	cursors: Record<string, bigint>,
	signal: AbortSignal,
) => AsyncIterable<ChatEvent>;

export interface StreamDeps {
	open: OpenStream;
	/** [0, 1)；測試注入。 */
	random?: () => number;
	setTimer?: (fn: () => void, ms: number) => unknown;
	clearTimer?: (t: unknown) => void;
}

function durationMs(d: Duration | undefined): number {
	return d ? Number(d.seconds) * 1000 + Math.floor(d.nanos / 1_000_000) : 0;
}

/** 這些 code 結束串流且不重連。 */
function isTerminal(code: Code): boolean {
	return (
		code === Code.Unauthenticated ||
		code === Code.NotFound ||
		code === Code.ResourceExhausted ||
		code === Code.FailedPrecondition ||
		code === Code.PermissionDenied
	);
}

export class ChatEventStream {
	private readonly eventListeners = new Set<(e: ChatStreamEvent) => void>();
	private readonly stateListeners = new Set<(s: ConnectionState) => void>();
	private readonly cursors = new Map<string, number>();
	private current: ConnectionState = "disconnected";
	private running = false;
	private stopped = true;
	private abort: AbortController | null = null;
	private wake: (() => void) | null = null;
	private readonly random: () => number;
	private readonly setTimer: (fn: () => void, ms: number) => unknown;
	private readonly clearTimer: (t: unknown) => void;
	/** 最後一次結束的原因，給測試與除錯。 */
	lastEnd: "normal" | "timeout" | "error" | "terminal" | null = null;

	constructor(private readonly deps: StreamDeps) {
		this.random = deps.random ?? Math.random;
		this.setTimer = deps.setTimer ?? ((fn, ms) => setTimeout(fn, ms));
		this.clearTimer =
			deps.clearTimer ??
			((t) => clearTimeout(t as ReturnType<typeof setTimeout>));
	}

	get state(): ConnectionState {
		return this.current;
	}

	onEvent(fn: (e: ChatStreamEvent) => void): () => void {
		this.eventListeners.add(fn);
		return () => this.eventListeners.delete(fn);
	}

	onState(fn: (s: ConnectionState) => void): () => void {
		this.stateListeners.add(fn);
		return () => this.stateListeners.delete(fn);
	}

	/** 記下一個對話手上最後的 seq（只前進）；重連時當 cursors 送。 */
	noteSeq(conversationId: string, seq: number): void {
		if (seq > (this.cursors.get(conversationId) ?? 0)) {
			// 先刪再放，Map 的插入順序就是「最近有動靜」的順序。
			this.cursors.delete(conversationId);
			this.cursors.set(conversationId, seq);
			while (this.cursors.size > MAX_CURSORS) {
				const oldest = this.cursors.keys().next().value;
				if (oldest === undefined) break;
				this.cursors.delete(oldest);
			}
		}
	}

	start(): void {
		if (this.running) return;
		this.stopped = false;
		this.running = true;
		void this.loop().finally(() => {
			this.running = false;
		});
	}

	stop(): void {
		this.stopped = true;
		this.abort?.abort();
		this.wake?.();
		this.setState("disconnected");
	}

	/** focus／online／visibility：停在「等待」的串流立刻再試。 */
	resume(): void {
		this.wake?.();
	}

	private setState(s: ConnectionState): void {
		if (s === this.current) return;
		this.current = s;
		for (const fn of [...this.stateListeners]) fn(s);
	}

	private emit(e: ChatStreamEvent): void {
		for (const fn of [...this.eventListeners]) fn(e);
	}

	private sleep(ms: number): Promise<void> {
		return new Promise((resolve) => {
			const t = this.setTimer(() => {
				this.wake = null;
				resolve();
			}, ms);
			this.wake = () => {
				this.clearTimer(t);
				this.wake = null;
				resolve();
			};
		});
	}

	private waitForResume(): Promise<void> {
		return new Promise((resolve) => {
			this.wake = () => {
				this.wake = null;
				resolve();
			};
		});
	}

	private async loop(): Promise<void> {
		let failures = 0;
		while (!this.stopped) {
			this.setState(failures === 0 ? "connecting" : "reconnecting");
			const outcome = await this.runOnce();
			if (this.stopped) return;
			if (outcome.kind === "terminal") {
				this.lastEnd = "terminal";
				this.setState("disconnected");
				return;
			}
			// 沒有收到 Ready 就結束不是壽命到期，當成失敗走 backoff（避免對立刻結束的伺服器熱迴圈）。
			if (outcome.kind === "normal" && outcome.gotReady) {
				this.lastEnd = "normal";
				failures = 0;
				continue;
			}
			this.lastEnd = outcome.kind === "normal" ? "error" : outcome.kind;
			if (outcome.gotReady) failures = 0;
			failures++;
			if (failures >= MAX_ATTEMPTS) {
				this.setState("disconnected");
				await this.waitForResume();
				failures = 0;
				continue;
			}
			const ceiling = Math.min(BACKOFF_CAP_MS, 1000 * 2 ** (failures - 1));
			await this.sleep(Math.floor(this.random() * ceiling));
		}
	}

	private async runOnce(): Promise<
		| { kind: "normal"; gotReady: boolean }
		| { kind: "terminal" }
		| { kind: "timeout" | "error"; gotReady: boolean }
	> {
		const ac = new AbortController();
		this.abort = ac;
		let gotReady = false;
		let timedOut = false;
		let timer: unknown = null;
		const arm = (ms: number) => {
			if (timer !== null) this.clearTimer(timer);
			timer = this.setTimer(() => {
				timedOut = true;
				ac.abort();
			}, ms);
		};
		let idleMs = READY_TIMEOUT_MS;
		arm(idleMs);
		const cursors: Record<string, bigint> = {};
		for (const [id, seq] of this.cursors) cursors[id] = BigInt(seq);
		try {
			for await (const ev of this.deps.open(cursors, ac.signal)) {
				arm(idleMs);
				const e = ev.event;
				switch (e.case) {
					case "ready": {
						gotReady = true;
						const hb = durationMs(e.value.heartbeatInterval);
						if (hb > 0) idleMs = hb * 2;
						arm(idleMs);
						this.setState("connected");
						this.emit({
							type: "ready",
							unreadCount: Number(e.value.unreadCount),
						});
						break;
					}
					case "message": {
						if (!e.value.message) break;
						const message = messageFromProto(e.value.message);
						if (message.seq !== null)
							this.noteSeq(message.conversationId, message.seq);
						this.emit({ type: "message", message, updated: e.value.updated });
						break;
					}
					case "unread":
						this.emit({
							type: "unread",
							unreadCount: Number(e.value.unreadCount),
						});
						break;
					case "peerRead":
						this.emit({
							type: "peerRead",
							conversationId: e.value.conversationId,
							peerReadSeq: Number(e.value.peerReadSeq),
						});
						break;
					case "resync":
						this.emit({
							type: "resync",
							conversationId:
								e.value.scope.case === "conversationId"
									? e.value.scope.value
									: undefined,
						});
						break;
					default:
						// heartbeat 與不認得的 frame：只重設看門狗。
						break;
				}
			}
			return { kind: "normal", gotReady };
		} catch (err) {
			if (this.stopped) return { kind: "normal", gotReady };
			if (timedOut) return { kind: "timeout", gotReady };
			const code = ConnectError.from(err).code;
			if (isTerminal(code)) return { kind: "terminal" };
			return { kind: "error", gotReady };
		} finally {
			if (timer !== null) this.clearTimer(timer);
			this.abort = null;
		}
	}
}

/** 正式環境的串流來源：買家方與店家方是兩支 RPC。 */
export function openStreamFor(side: ChatSide): OpenStream {
	return async function* (cursors, signal) {
		const client = chatStreamClient();
		const responses =
			side === "store"
				? client.streamStoreEvents({ cursors }, { signal })
				: client.streamEvents({ cursors }, { signal });
		for await (const res of responses) {
			if (res.event) yield res.event;
		}
	};
}

interface Shared {
	stream: ChatEventStream;
	refs: number;
	cleanup: () => void;
}

const shared = new Map<ChatSide, Shared>();

/**
 * 取得某一方的共用串流（引用計數）：全站只開一條，未讀徽章、對話列表、聊天室與通知共用；
 * 最後一個使用者釋放時關閉。
 */
export function acquireChatStream(side: ChatSide): {
	stream: ChatEventStream;
	release: () => void;
} {
	let entry = shared.get(side);
	if (!entry) {
		const stream = new ChatEventStream({ open: openStreamFor(side) });
		const resume = () => {
			if (typeof document !== "undefined" && document.hidden) return;
			stream.resume();
		};
		const cleanup = () => {
			if (typeof window === "undefined") return;
			window.removeEventListener("focus", resume);
			window.removeEventListener("online", resume);
			document.removeEventListener("visibilitychange", resume);
		};
		if (typeof window !== "undefined") {
			window.addEventListener("focus", resume);
			window.addEventListener("online", resume);
			document.addEventListener("visibilitychange", resume);
		}
		entry = { stream, refs: 0, cleanup };
		shared.set(side, entry);
		stream.start();
	}
	entry.refs++;
	const held = entry;
	let released = false;
	return {
		stream: held.stream,
		release: () => {
			if (released) return;
			released = true;
			held.refs--;
			if (held.refs <= 0) {
				held.stream.stop();
				held.cleanup();
				if (shared.get(side) === held) shared.delete(side);
			}
		},
	};
}
