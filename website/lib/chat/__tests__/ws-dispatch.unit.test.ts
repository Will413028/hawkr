import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Mock the network/env deps so ChatSocket can open against a fake WebSocket in
// the node test env (no DOM). Paths resolve to the same modules ws.ts imports.
// `post` goes through vi.hoisted so it exists when the hoisted vi.mock runs.
const { post } = vi.hoisted(() => ({ post: vi.fn() }));
vi.mock("../../api/client", () => ({ apiClient: { authed: { post } } }));
vi.mock("../../config/env", () => ({
	getEnv: () => ({ gatewayHost: "https://gw.test" }),
}));

import type { ChatMessage } from "../types";
import { ChatSocket } from "../ws";

class FakeWS {
	static last: FakeWS | null = null;
	onopen: (() => void) | null = null;
	onmessage: ((ev: { data: string }) => void) | null = null;
	onerror: (() => void) | null = null;
	onclose: (() => void) | null = null;
	sent: string[] = [];
	closed = false;
	constructor(public url: string) {
		FakeWS.last = this;
	}
	send(s: string) {
		this.sent.push(s);
	}
	close() {
		this.closed = true;
		this.onclose?.();
	}
}

beforeEach(() => {
	FakeWS.last = null;
	post.mockResolvedValue({
		data: { ticket: "tkt", userId: "1", userType: 1 },
	});
	(globalThis as unknown as { WebSocket: typeof FakeWS }).WebSocket = FakeWS;
});

afterEach(() => {
	vi.restoreAllMocks();
	vi.useRealTimers();
});

/** Open a ChatSocket against the fake transport and fire its onopen. The
 *  ticket-mint is async (mocked microtask), so flush a few ticks. */
async function open() {
	const sock = new ChatSocket();
	const seen: ChatMessage[] = [];
	sock.onMessage((m) => seen.push(m));
	sock.connect();
	for (let i = 0; i < 5 && !FakeWS.last; i++) await Promise.resolve();
	const ws = FakeWS.last;
	if (!ws) throw new Error("socket did not open");
	ws.onopen?.();
	return { sock, ws, seen };
}

describe("ChatSocket dispatch", () => {
	it("swallows read_receipt — never surfaces it as a chat message bubble", async () => {
		const { ws, seen } = await open();
		// A mobile peer still emits read_receipt over WS; web must not render it.
		ws.onmessage?.({
			data: JSON.stringify({ type: "read_receipt", from: "9", to: "1" }),
		});
		expect(seen).toHaveLength(0);
	});

	it("swallows pong (liveness ack), not a message", async () => {
		const { ws, seen } = await open();
		ws.onmessage?.({ data: JSON.stringify({ type: "pong", timestamp: 1 }) });
		expect(seen).toHaveLength(0);
	});

	it("swallows chat_ack — a send-side ack must not render as a bubble", async () => {
		const { ws, seen } = await open();
		ws.onmessage?.({
			data: JSON.stringify({ type: "chat_ack", messageId: "m1" }),
		});
		expect(seen).toHaveLength(0);
	});

	it("surfaces a real chat message to listeners", async () => {
		const { ws, seen } = await open();
		ws.onmessage?.({
			data: JSON.stringify({
				type: "chat",
				id: "m1",
				fromUserId: "9",
				toUserId: "1",
				content: "hi",
				seqId: "5",
			}),
		});
		expect(seen).toHaveLength(1);
		expect(seen[0].content).toBe("hi");
		expect(seen[0].seqId).toBe("5");
	});

	it("sends an app-level ping typed 'ping' (not legacy 'heartbeat')", async () => {
		vi.useFakeTimers();
		const sock = new ChatSocket();
		sock.connect();
		for (let i = 0; i < 5 && !FakeWS.last; i++) await Promise.resolve();
		const ws = FakeWS.last;
		if (!ws) throw new Error("socket did not open");
		ws.onopen?.();
		vi.advanceTimersByTime(15_000);
		const frame = JSON.parse(ws.sent.at(-1) ?? "{}");
		expect(frame.type).toBe("ping");
	});
});

describe("ChatSocket read-deadline (liveness)", () => {
	it("closes a silent socket after the liveness window", async () => {
		vi.useFakeTimers();
		const sock = new ChatSocket();
		sock.connect();
		for (let i = 0; i < 5 && !FakeWS.last; i++) await Promise.resolve();
		const ws = FakeWS.last;
		if (!ws) throw new Error("socket did not open");
		ws.onopen?.();
		vi.advanceTimersByTime(30_000 + 100);
		expect(ws.closed).toBe(true);
	});

	it("an inbound frame re-arms the deadline (no premature close)", async () => {
		vi.useFakeTimers();
		const sock = new ChatSocket();
		sock.connect();
		for (let i = 0; i < 5 && !FakeWS.last; i++) await Promise.resolve();
		const ws = FakeWS.last;
		if (!ws) throw new Error("socket did not open");
		ws.onopen?.();
		vi.advanceTimersByTime(29_000);
		ws.onmessage?.({ data: JSON.stringify({ type: "pong" }) }); // re-arm
		vi.advanceTimersByTime(29_000);
		expect(ws.closed).toBe(false); // 58s elapsed but reset at 29s
		vi.advanceTimersByTime(2_000);
		expect(ws.closed).toBe(true); // now 31s since the last frame
	});
});
