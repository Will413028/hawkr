import { describe, expect, it } from "vitest";
import {
	compareMessages,
	gapAfter,
	lastSeq,
	reconcileOptimistic,
	sortTimeline,
	upsertServerMessage,
} from "../timeline";
import type { ChatMessage } from "../types";

function msg(p: Partial<ChatMessage> & { id: string }): ChatMessage {
	return {
		conversationId: "c1",
		seq: null,
		sender: "buyer",
		kind: "text",
		text: "",
		hidden: false,
		erased: false,
		timeMs: 0,
		...p,
	};
}

describe("compareMessages", () => {
	it("orders persisted messages by seq", () => {
		expect(
			compareMessages(msg({ id: "a", seq: 2 }), msg({ id: "b", seq: 10 })),
		).toBeLessThan(0);
	});

	it("puts optimistic messages after persisted ones, in send order", () => {
		const sorted = sortTimeline([
			msg({ id: "o2", timeMs: 20 }),
			msg({ id: "p", seq: 5 }),
			msg({ id: "o1", timeMs: 10 }),
		]);
		expect(sorted.map((m) => m.id)).toEqual(["p", "o1", "o2"]);
	});
});

describe("upsertServerMessage", () => {
	it("inserts by seq, not by arrival order", () => {
		const list = upsertServerMessage(
			[msg({ id: "a", seq: 1 }), msg({ id: "c", seq: 3 })],
			msg({ id: "b", seq: 2 }),
		);
		expect(list.map((m) => m.id)).toEqual(["a", "b", "c"]);
	});

	it("replaces the message with the same seq (visibility change becomes a tombstone)", () => {
		const list = upsertServerMessage(
			[msg({ id: "a", seq: 1, text: "hi" })],
			msg({ id: "a", seq: 1, text: undefined, hidden: true }),
		);
		expect(list).toHaveLength(1);
		expect(list[0]?.hidden).toBe(true);
		expect(list[0]?.text).toBeUndefined();
	});

	it("does not duplicate a replayed message", () => {
		const m = msg({ id: "a", seq: 1 });
		expect(upsertServerMessage([m], { ...m })).toHaveLength(1);
	});
});

describe("reconcileOptimistic", () => {
	it("swaps the optimistic message for the server message", () => {
		const list = reconcileOptimistic(
			[msg({ id: "p", seq: 1 }), msg({ id: "req-1", timeMs: 5 })],
			"req-1",
			msg({ id: "srv-2", seq: 2 }),
		);
		expect(list.map((m) => m.id)).toEqual(["p", "srv-2"]);
	});

	it("when the stream echo already arrived, only drops the optimistic copy", () => {
		const list = reconcileOptimistic(
			[msg({ id: "srv-2", seq: 2 }), msg({ id: "req-1", timeMs: 5 })],
			"req-1",
			msg({ id: "srv-2", seq: 2 }),
		);
		expect(list.map((m) => m.id)).toEqual(["srv-2"]);
	});
});

describe("gapAfter / lastSeq", () => {
	const have = [msg({ id: "a", seq: 1 }), msg({ id: "b", seq: 2 })];
	it("returns the seq to fill after when a message skips ahead", () => {
		expect(gapAfter(have, 5)).toBe(2);
	});
	it("returns null for the next consecutive seq, a duplicate, or an empty timeline", () => {
		expect(gapAfter(have, 3)).toBeNull();
		expect(gapAfter(have, 2)).toBeNull();
		expect(gapAfter([], 9)).toBeNull();
	});
	it("ignores optimistic messages when finding the last seq", () => {
		expect(lastSeq([...have, msg({ id: "o" })])).toBe(2);
	});
});
