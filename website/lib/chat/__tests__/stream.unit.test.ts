import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	type ChatEvent,
	ChatEventSchema,
	MessageKind,
} from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { describe, expect, it } from "vitest";
import {
	ChatEventStream,
	type ChatStreamEvent,
	MAX_ATTEMPTS,
	READY_TIMEOUT_MS,
} from "../stream";

const ready = (hbSeconds = 10, unread = 3): ChatEvent =>
	create(ChatEventSchema, {
		event: {
			case: "ready",
			value: {
				unreadCount: BigInt(unread),
				heartbeatInterval: { seconds: BigInt(hbSeconds) },
			},
		},
	});
const message = (
	conversationId: string,
	seq: number,
	updated = false,
): ChatEvent =>
	create(ChatEventSchema, {
		event: {
			case: "message",
			value: {
				updated,
				message: {
					id: `m${seq}`,
					conversationId,
					seq: BigInt(seq),
					kind: MessageKind.TEXT,
					text: "x",
				},
			},
		},
	});
const heartbeat = (): ChatEvent =>
	create(ChatEventSchema, { event: { case: "heartbeat", value: {} } });
const resync = (conversationId?: string): ChatEvent =>
	create(ChatEventSchema, {
		event: {
			case: "resync",
			value: {
				scope: conversationId
					? { case: "conversationId", value: conversationId }
					: { case: "all", value: true },
			},
		},
	});

type Script = (
	signal: AbortSignal,
	cursors: Record<string, bigint>,
) => AsyncGenerator<ChatEvent>;

/** 等到 abort 才結束（模擬一直沒有 frame 的半開連線）。 */
function hang(signal: AbortSignal): Promise<never> {
	return new Promise((_, reject) => {
		signal.addEventListener("abort", () =>
			reject(new ConnectError("aborted", Code.Canceled)),
		);
	});
}

function harness(scripts: Script[]) {
	const timers: { fn: () => void; ms: number; live: boolean }[] = [];
	const opens: Record<string, bigint>[] = [];
	let n = 0;
	const stream = new ChatEventStream({
		open: (cursors, signal) => {
			opens.push(cursors);
			const script = scripts[n++];
			if (!script)
				// biome-ignore lint/correctness/useYield: ends or fails before the first frame
				return (async function* () {
					await hang(signal);
				})();
			return script(signal, cursors);
		},
		random: () => 0.5,
		setTimer: (fn, ms) => {
			const t = {
				fn: () => {
					t.live = false;
					fn();
				},
				ms,
				live: true,
			};
			timers.push(t);
			return t;
		},
		clearTimer: (t) => {
			(t as { live: boolean }).live = false;
		},
	});
	const events: ChatStreamEvent[] = [];
	const states: string[] = [];
	stream.onEvent((e) => events.push(e));
	stream.onState((s) => states.push(s));
	const flush = async () => {
		for (let i = 0; i < 20; i++) await new Promise((r) => setImmediate(r));
	};
	const live = () => timers.filter((t) => t.live);
	return { stream, timers, opens, events, states, flush, live };
}

describe("ChatEventStream", () => {
	it("goes connected on Ready and delivers message, unread, peerRead and resync frames", async () => {
		const h = harness([
			async function* () {
				yield ready(10, 3);
				yield message("c1", 1);
				yield create(ChatEventSchema, {
					event: { case: "unread", value: { unreadCount: BigInt(5) } },
				});
				yield create(ChatEventSchema, {
					event: {
						case: "peerRead",
						value: { conversationId: "c1", peerReadSeq: BigInt(1) },
					},
				});
				yield resync("c1");
				yield heartbeat();
				await new Promise(() => {});
			},
		]);
		h.stream.start();
		await h.flush();
		expect(h.states).toEqual(["connecting", "connected"]);
		expect(h.events.map((e) => e.type)).toEqual([
			"ready",
			"message",
			"unread",
			"peerRead",
			"resync",
		]);
		expect(h.events[0]).toEqual({ type: "ready", unreadCount: 3 });
		expect(h.events[4]).toEqual({ type: "resync", conversationId: "c1" });
		h.stream.stop();
	});

	it("reconnects at once after the normal max_lifetime end, resuming from the last seq per conversation", async () => {
		const h = harness([
			async function* () {
				yield ready();
				yield message("c1", 4);
				yield message("c2", 9);
			},
		]);
		h.stream.start();
		await h.flush();
		expect(h.opens).toHaveLength(2);
		expect(h.opens[0]).toEqual({});
		expect(h.opens[1]).toEqual({ c1: BigInt(4), c2: BigInt(9) });
		expect(h.stream.lastEnd).toBe("normal");
		// 正常結束不排 backoff：沒有手動觸發任何 timer 就已經重連。
		h.stream.stop();
	});

	it("a stream that ends before Ready is a failure with backoff, not an instant reconnect", async () => {
		const h = harness([
			async function* () {},
			async function* () {
				yield ready();
				await new Promise(() => {});
			},
		]);
		h.stream.start();
		await h.flush();
		expect(h.opens).toHaveLength(1);
		expect(h.stream.lastEnd).toBe("error");
		h.live()
			.find((t) => t.ms === 500)
			?.fn();
		await h.flush();
		expect(h.opens).toHaveLength(2);
		h.stream.stop();
	});

	it("noteSeq only moves a cursor forward", async () => {
		const h = harness([
			async function* () {
				yield ready();
			},
		]);
		h.stream.noteSeq("c1", 8);
		h.stream.noteSeq("c1", 3);
		h.stream.start();
		await h.flush();
		expect(h.opens[1]).toEqual({ c1: BigInt(8) });
		h.stream.stop();
	});

	it("unavailable reconnects after a full-jitter backoff and shows reconnecting", async () => {
		const h = harness([
			// biome-ignore lint/correctness/useYield: ends or fails before the first frame
			async function* () {
				throw new ConnectError("down", Code.Unavailable);
			},
			async function* () {
				yield ready();
				await new Promise(() => {});
			},
		]);
		h.stream.start();
		await h.flush();
		expect(h.opens).toHaveLength(1);
		const backoff = h.live().find((t) => t.ms === 500); // 0.5 * 1000ms
		expect(backoff).toBeDefined();
		backoff?.fn();
		await h.flush();
		expect(h.opens).toHaveLength(2);
		expect(h.states).toEqual(["connecting", "reconnecting", "connected"]);
		h.stream.stop();
	});

	it.each([
		["unauthenticated", Code.Unauthenticated],
		["resource_exhausted", Code.ResourceExhausted],
		["failed_precondition (STORE_CLOSED)", Code.FailedPrecondition],
		["not_found", Code.NotFound],
		["invalid_argument", Code.InvalidArgument],
	])("%s ends the stream without reconnecting", async (_name, code) => {
		const h = harness([
			// biome-ignore lint/correctness/useYield: ends or fails before the first frame
			async function* () {
				throw new ConnectError("no", code);
			},
		]);
		h.stream.start();
		await h.flush();
		expect(h.opens).toHaveLength(1);
		expect(h.stream.lastEnd).toBe("terminal");
		expect(h.stream.state).toBe("disconnected");
		expect(h.live()).toHaveLength(0);
	});

	it("treats a silent connection as dead after twice the heartbeat interval and reconnects", async () => {
		const h = harness([
			async function* (signal) {
				yield ready(10);
				await hang(signal);
			},
			async function* () {
				yield ready(10);
				await new Promise(() => {});
			},
		]);
		h.stream.start();
		await h.flush();
		const watchdog = h.live().find((t) => t.ms === 20_000);
		expect(watchdog).toBeDefined();
		watchdog?.fn(); // 兩倍心跳間隔沒有任何 frame
		await h.flush();
		expect(h.stream.lastEnd).toBe("timeout");
		h.live()
			.find((t) => t.ms === 500)
			?.fn();
		await h.flush();
		expect(h.opens).toHaveLength(2);
		h.stream.stop();
	});

	it("a heartbeat re-arms the watchdog (the connection is not dropped while heartbeats arrive)", async () => {
		let release: () => void = () => {};
		const gate = new Promise<void>((r) => (release = r));
		const h = harness([
			async function* () {
				yield ready(10);
				await gate;
				yield heartbeat();
				await new Promise(() => {});
			},
		]);
		h.stream.start();
		await h.flush();
		const before = h.timers.length;
		release();
		await h.flush();
		const armed = h.timers
			.slice(before)
			.filter((t) => t.live && t.ms === 20_000);
		expect(armed).toHaveLength(1);
		expect(h.opens).toHaveLength(1);
		h.stream.stop();
	});

	it("gives up Ready after READY_TIMEOUT_MS", async () => {
		const h = harness([]);
		h.stream.start();
		await h.flush();
		const t = h.live().find((x) => x.ms === READY_TIMEOUT_MS);
		expect(t).toBeDefined();
		t?.fn();
		await h.flush();
		expect(h.stream.lastEnd).toBe("timeout");
		h.stream.stop();
	});

	it(`stops retrying after ${MAX_ATTEMPTS} failures and waits for resume()`, async () => {
		// biome-ignore lint/correctness/useYield: ends or fails before the first frame
		const failing: Script = async function* () {
			throw new ConnectError("down", Code.Unavailable);
		};
		const h = harness(Array.from({ length: MAX_ATTEMPTS + 2 }, () => failing));
		h.stream.start();
		for (let i = 0; i < MAX_ATTEMPTS - 1; i++) {
			await h.flush();
			h.live()[0]?.fn();
		}
		await h.flush();
		expect(h.opens).toHaveLength(MAX_ATTEMPTS);
		expect(h.stream.state).toBe("disconnected");
		expect(h.live()).toHaveLength(0);
		h.stream.resume();
		await h.flush();
		expect(h.opens).toHaveLength(MAX_ATTEMPTS + 1);
		h.stream.stop();
	});

	it("stop() aborts the open call and does not reconnect", async () => {
		let aborted = false;
		const h = harness([
			async function* (signal) {
				yield ready();
				signal.addEventListener("abort", () => {
					aborted = true;
				});
				await hang(signal);
			},
		]);
		h.stream.start();
		await h.flush();
		h.stream.stop();
		await h.flush();
		expect(aborted).toBe(true);
		expect(h.opens).toHaveLength(1);
		expect(h.stream.state).toBe("disconnected");
	});
});
