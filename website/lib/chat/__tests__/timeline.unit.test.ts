import { describe, expect, it } from "vitest";
import {
	compareMessages,
	reconcileOptimistic,
	sortTimeline,
	upsertMessage,
} from "../timeline";
import type { ChatMessage } from "../types";

function msg(p: Partial<ChatMessage> & { id: string }): ChatMessage {
	return {
		senderId: "1",
		receiverId: "2",
		content: "",
		type: "text",
		timestamp: 0,
		read: false,
		...p,
	};
}

describe("compareMessages", () => {
	it("orders by seqId ascending (BigInt — beyond 2^53)", () => {
		const a = msg({ id: "a", seqId: "9007199254740993" }); // 2^53 + 1
		const b = msg({ id: "b", seqId: "9007199254740995" }); // 2^53 + 3
		expect(compareMessages(a, b)).toBeLessThan(0);
		expect(compareMessages(b, a)).toBeGreaterThan(0);
	});

	it("Number() would tie these huge seqIds; BigInt must not", () => {
		// Both collapse to 9007199254740992 under Number() → would mis-tie.
		const a = msg({ id: "a", seqId: "9007199254740993" });
		const b = msg({ id: "b", seqId: "9007199254740994" });
		expect(compareMessages(a, b)).toBeLessThan(0);
	});

	it("a persisted (has seqId) message sorts before an optimistic (no seqId) one regardless of timestamp", () => {
		const persisted = msg({ id: "p", seqId: "100", timestamp: 1 });
		const optimistic = msg({ id: "o", timestamp: 999 });
		expect(compareMessages(persisted, optimistic)).toBeLessThan(0);
		expect(compareMessages(optimistic, persisted)).toBeGreaterThan(0);
	});

	it("two optimistic messages order by timestamp (send order)", () => {
		const a = msg({ id: "a", timestamp: 10 });
		const b = msg({ id: "b", timestamp: 20 });
		expect(compareMessages(a, b)).toBeLessThan(0);
	});

	it("falls back to timestamp when a seqId is malformed", () => {
		const a = msg({ id: "a", seqId: "not-a-number", timestamp: 10 });
		const b = msg({ id: "b", seqId: "abc", timestamp: 20 });
		expect(compareMessages(a, b)).toBeLessThan(0);
	});
});

describe("sortTimeline", () => {
	it("is a consistent total order across mixed seqId/no-seqId (no cycle)", () => {
		const list = [
			msg({ id: "opt", timestamp: 5 }),
			msg({ id: "s101", seqId: "101", timestamp: 2 }),
			msg({ id: "s100", seqId: "100", timestamp: 3 }),
		];
		expect(sortTimeline(list).map((m) => m.id)).toEqual([
			"s100",
			"s101",
			"opt",
		]);
	});

	it("does not mutate its input", () => {
		const list = [msg({ id: "b", seqId: "2" }), msg({ id: "a", seqId: "1" })];
		const snapshot = [...list];
		sortTimeline(list);
		expect(list).toEqual(snapshot);
	});
});

describe("upsertMessage", () => {
	it("appends a new message in sorted position", () => {
		const list = [msg({ id: "a", seqId: "1" }), msg({ id: "c", seqId: "3" })];
		const out = upsertMessage(list, msg({ id: "b", seqId: "2" }));
		expect(out.map((m) => m.id)).toEqual(["a", "b", "c"]);
	});

	it("dedups by id — does not add a second bubble", () => {
		const list = [msg({ id: "a", seqId: "1" })];
		const out = upsertMessage(list, msg({ id: "a", seqId: "1" }));
		expect(out).toHaveLength(1);
	});

	it("merges a seqId into an existing seqId-less message (own-echo settles ordering)", () => {
		// Reconciled-but-seqId-less sent message, then the WS echo arrives with the
		// authoritative seqId. The echo must upgrade the existing bubble (not drop
		// the seqId), so it sorts into place instead of being pinned to the bottom.
		const list = [
			msg({ id: "real", timestamp: 50 }), // sent, no seqId yet
			msg({ id: "peer", seqId: "200", timestamp: 40 }),
		];
		const out = upsertMessage(list, msg({ id: "real", seqId: "201" }));
		const real = out.find((m) => m.id === "real");
		expect(real?.seqId).toBe("201");
		expect(out.map((m) => m.id)).toEqual(["peer", "real"]);
	});

	it("merges a conversationId the REST send response omitted", () => {
		// Reconciled own-send may carry neither seqId nor conversationId; the echo
		// supplies both. They're needed for correct dedup and timeline ordering.
		const list = [msg({ id: "real", timestamp: 50 })];
		const out = upsertMessage(
			list,
			msg({ id: "real", seqId: "201", conversationId: "b1_s2" }),
		);
		const real = out.find((m) => m.id === "real");
		expect(real?.seqId).toBe("201");
		expect(real?.conversationId).toBe("b1_s2");
	});

	it("returns the same array reference when nothing new is merged (no re-render)", () => {
		const list = [msg({ id: "a", seqId: "1", conversationId: "c" })];
		const out = upsertMessage(list, msg({ id: "a", seqId: "1" }));
		expect(out).toBe(list);
	});

	it("handles an undefined current list", () => {
		const out = upsertMessage(undefined, msg({ id: "a", seqId: "1" }));
		expect(out.map((m) => m.id)).toEqual(["a"]);
	});
});

describe("reconcileOptimistic", () => {
	it("renames the optimistic message to the real id when the echo has not arrived", () => {
		const list = [msg({ id: "tmp-uuid", timestamp: 10 })];
		const out = reconcileOptimistic(list, "tmp-uuid", {
			id: "real",
			timestamp: 11,
		});
		expect(out.map((m) => m.id)).toEqual(["real"]);
		expect(out[0].timestamp).toBe(11);
	});

	it("drops the optimistic message when the WS echo already inserted the real id (echo-first race)", () => {
		const list = [
			msg({ id: "real", seqId: "300", timestamp: 11 }), // echo arrived first
			msg({ id: "tmp-uuid", timestamp: 10 }), // our optimistic
		];
		const out = reconcileOptimistic(list, "tmp-uuid", {
			id: "real",
			timestamp: 11,
		});
		expect(out).toHaveLength(1);
		expect(out[0].id).toBe("real");
		expect(out[0].seqId).toBe("300"); // echo's authoritative copy is kept
	});

	it("applies the seqId the REST send response carries so the sent message sorts into place (backend has no WS self-echo for the sender)", () => {
		// The sender gets no WS echo of its own message, so the REST response is the
		// only source of seqId for an own-send. Without applying it the message stays
		// seqId-less and pins under every received (seqId-bearing) message — the
		// "all my messages stuck at the bottom" bug.
		const list = [
			msg({ id: "p1", seqId: "200", timestamp: 10 }), // peer, earlier
			msg({ id: "tmp-uuid", timestamp: 20 }), // our optimistic send
			msg({ id: "p2", seqId: "202", timestamp: 30 }), // peer, later
		];
		const out = reconcileOptimistic(list, "tmp-uuid", {
			id: "real",
			timestamp: 21,
			seqId: "201",
		});
		const sent = out.find((m) => m.id === "real");
		expect(sent?.seqId).toBe("201");
		expect(out.map((m) => m.id)).toEqual(["p1", "real", "p2"]);
	});

	it("applies a content patch (image: temp preview → permanent CDN url)", () => {
		const list = [msg({ id: "tmp-uuid", content: "blob:preview" })];
		const out = reconcileOptimistic(list, "tmp-uuid", {
			id: "real",
			timestamp: 1,
			content: "https://cdn/x.jpg",
		});
		expect(out[0].content).toBe("https://cdn/x.jpg");
	});

	it("echo-first image: drops the optimistic AND patches the survivor with the CDN url", () => {
		// The echo arrived first carrying a staged token; the REST CDN url must win.
		const list = [
			msg({ id: "real", type: "image", content: "staged://tok", seqId: "9" }),
			msg({ id: "tmp-uuid", type: "image", content: "blob:preview" }),
		];
		const out = reconcileOptimistic(list, "tmp-uuid", {
			id: "real",
			timestamp: 1,
			content: "https://cdn/x.jpg",
		});
		expect(out).toHaveLength(1);
		expect(out[0].content).toBe("https://cdn/x.jpg");
	});
});
