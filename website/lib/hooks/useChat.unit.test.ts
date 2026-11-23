// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ChatStreamEvent } from "@/lib/chat/stream";
import type { ChatMessage, Conversation } from "@/lib/chat/types";

const repo = vi.hoisted(() => ({
	listConversations: vi.fn(),
	listLatestMessages: vi.fn(),
	listMessagesAfter: vi.fn(),
	markConversationRead: vi.fn(),
	fetchPresence: vi.fn(),
	sendMessage: vi.fn(),
}));
const stream = vi.hoisted(() => ({
	handler: undefined as undefined | ((e: ChatStreamEvent) => void),
	noteSeq: vi.fn(),
}));

vi.mock("@/lib/chat/repo", () => repo);
vi.mock("@/lib/chat/useChatStream", () => ({
	useChatStream: (
		_side: string,
		_enabled: boolean,
		onEvent?: (e: ChatStreamEvent) => void,
	) => {
		stream.handler = onEvent;
		return { state: "connected", stream: { noteSeq: stream.noteSeq } };
	},
}));
vi.mock("@/lib/auth/use-account-status", () => ({
	useAccountStatus: () => "signedIn",
}));
vi.mock("../hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "a1", activeRole: "buyer" }),
}));
vi.mock("@/lib/intl/navigation", () => ({
	usePathname: () => "/account/messages/9",
}));
vi.mock("@/lib/chat/unread-context", () => ({
	useChatUnread: () => ({ refresh: async () => {} }),
}));
vi.mock("@/lib/i18n/ui-text", () => ({
	useDefaultErrorText: () => () => "default-error",
}));
vi.mock("@/lib/media/upload", () => ({ uploadImageFile: vi.fn() }));

import { useChatThread, useConversations } from "./useChat";

const conv: Conversation = {
	id: "c1",
	targetUserId: "9",
	targetNickname: "Shop",
	lastMessageTimeMs: 0,
	unreadCount: 0,
	peerReadSeq: 0,
};
const msg = (seq: number, p: Partial<ChatMessage> = {}): ChatMessage => ({
	id: `m${seq}`,
	conversationId: "c1",
	seq,
	sender: "store",
	kind: "text",
	text: `t${seq}`,
	hidden: false,
	erased: false,
	timeMs: seq,
	...p,
});

function wrapper() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, gcTime: Infinity } },
	});
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	vi.clearAllMocks();
	stream.handler = undefined;
	repo.markConversationRead.mockResolvedValue(undefined);
	repo.fetchPresence.mockResolvedValue(new Map());
});

describe("read failure is distinct from empty", () => {
	it("a failed conversation list read sets error and is not an empty inbox", async () => {
		repo.listConversations.mockRejectedValue(
			new ConnectError("x", Code.Unavailable),
		);
		const { result } = renderHook(() => useConversations(), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.error).toBe("default-error"));
		expect(result.current.loading).toBe(false);
	});

	it("a successful empty list has no error", async () => {
		repo.listConversations.mockResolvedValue([]);
		const { result } = renderHook(() => useConversations(), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.loading).toBe(false));
		expect(result.current.error).toBeNull();
		expect(result.current.items).toEqual([]);
	});

	it("a failed thread read surfaces the error instead of an empty thread", async () => {
		repo.listConversations.mockResolvedValue([conv]);
		repo.listLatestMessages.mockRejectedValue(
			new ConnectError("x", Code.Internal),
		);
		const { result } = renderHook(() => useChatThread("9"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.error).toBe("default-error"));
	});

	it("no conversation yet is a real empty thread (no error, nothing fetched)", async () => {
		repo.listConversations.mockResolvedValue([]);
		const { result } = renderHook(() => useChatThread("9"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.loading).toBe(false));
		expect(result.current.error).toBeNull();
		expect(result.current.messages).toEqual([]);
		expect(repo.listLatestMessages).not.toHaveBeenCalled();
	});
});

describe("useChatThread stream handling", () => {
	async function openThread() {
		repo.listConversations.mockResolvedValue([conv]);
		repo.listLatestMessages.mockResolvedValue({
			messages: [msg(1), msg(2)],
			hasMore: false,
			peerReadSeq: 0,
		});
		const hook = renderHook(() => useChatThread("9"), { wrapper: wrapper() });
		await waitFor(() => expect(hook.result.current.messages).toHaveLength(2));
		return hook;
	}

	it("appends the next consecutive message without a gap fill", async () => {
		const { result } = await openThread();
		act(() =>
			stream.handler?.({ type: "message", message: msg(3), updated: false }),
		);
		await waitFor(() =>
			expect(result.current.messages.map((m) => m.seq)).toEqual([1, 2, 3]),
		);
		expect(repo.listMessagesAfter).not.toHaveBeenCalled();
	});

	it("fills a seq gap with ListMessages after the last seq it holds", async () => {
		repo.listMessagesAfter.mockResolvedValue({
			messages: [msg(3), msg(4)],
			hasMore: false,
			peerReadSeq: 0,
		});
		const { result } = await openThread();
		act(() =>
			stream.handler?.({ type: "message", message: msg(5), updated: false }),
		);
		await waitFor(() =>
			expect(result.current.messages.map((m) => m.seq)).toEqual([
				1, 2, 3, 4, 5,
			]),
		);
		expect(repo.listMessagesAfter).toHaveBeenCalledWith("c1", 2);
	});

	it("keeps filling while the server says hasMore", async () => {
		repo.listMessagesAfter
			.mockResolvedValueOnce({
				messages: [msg(3), msg(4)],
				hasMore: true,
				peerReadSeq: 0,
			})
			.mockResolvedValueOnce({
				messages: [msg(5), msg(6)],
				hasMore: false,
				peerReadSeq: 0,
			});
		const { result } = await openThread();
		act(() =>
			stream.handler?.({ type: "message", message: msg(7), updated: false }),
		);
		await waitFor(() =>
			expect(result.current.messages.map((m) => m.seq)).toEqual([
				1, 2, 3, 4, 5, 6, 7,
			]),
		);
		expect(repo.listMessagesAfter.mock.calls.map((c) => c[1])).toEqual([2, 4]);
	});

	it("rereads the thread when a gap fill fails instead of swallowing it", async () => {
		repo.listMessagesAfter.mockRejectedValue(
			new ConnectError("x", Code.Unavailable),
		);
		await openThread();
		repo.listLatestMessages.mockClear();
		act(() =>
			stream.handler?.({ type: "message", message: msg(5), updated: false }),
		);
		await waitFor(() => expect(repo.listLatestMessages).toHaveBeenCalled());
	});

	it("replaces the message in place when the server marks a visibility change", async () => {
		const { result } = await openThread();
		act(() =>
			stream.handler?.({
				type: "message",
				message: msg(2, { text: undefined, hidden: true }),
				updated: true,
			}),
		);
		await waitFor(() => expect(result.current.messages[1]?.hidden).toBe(true));
		expect(result.current.messages).toHaveLength(2);
	});

	it("peerRead advances the read cursor for this conversation only", async () => {
		const { result } = await openThread();
		act(() =>
			stream.handler?.({
				type: "peerRead",
				conversationId: "other",
				peerReadSeq: 9,
			}),
		);
		expect(result.current.peerReadSeq).toBe(0);
		act(() =>
			stream.handler?.({
				type: "peerRead",
				conversationId: "c1",
				peerReadSeq: 2,
			}),
		);
		await waitFor(() => expect(result.current.peerReadSeq).toBe(2));
	});

	it("resync for this conversation rereads the thread", async () => {
		await openThread();
		repo.listLatestMessages.mockClear();
		act(() => stream.handler?.({ type: "resync", conversationId: "c1" }));
		await waitFor(() => expect(repo.listLatestMessages).toHaveBeenCalled());
	});

	it("resync for another conversation is ignored", async () => {
		await openThread();
		repo.listLatestMessages.mockClear();
		act(() => stream.handler?.({ type: "resync", conversationId: "other" }));
		await new Promise((r) => setTimeout(r, 20));
		expect(repo.listLatestMessages).not.toHaveBeenCalled();
	});
});

describe("conversation list reconnect", () => {
	it("invalidates the list on every ready after the first, not on the first", async () => {
		repo.listConversations.mockResolvedValue([conv]);
		const { result } = renderHook(() => useConversations(), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.loading).toBe(false));
		repo.listConversations.mockClear();
		act(() => stream.handler?.({ type: "ready", unreadCount: 0 }));
		await new Promise((r) => setTimeout(r, 20));
		expect(repo.listConversations).not.toHaveBeenCalled();
		act(() => stream.handler?.({ type: "ready", unreadCount: 0 }));
		await waitFor(() =>
			expect(repo.listConversations).toHaveBeenCalledTimes(1),
		);
	});
});

describe("useChatThread send", () => {
	it("does not cancel the first read when sending before history arrived, and rereads after", async () => {
		repo.listConversations.mockResolvedValue([conv]);
		let resolveHistory: (v: unknown) => void = () => {};
		repo.listLatestMessages.mockReturnValueOnce(
			new Promise((r) => (resolveHistory = r)),
		);
		repo.sendMessage.mockResolvedValue(msg(3, { id: "srv", sender: "buyer" }));
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false, gcTime: Infinity } },
		});
		const cancel = vi.spyOn(client, "cancelQueries");
		const { result } = renderHook(() => useChatThread("9"), {
			wrapper: ({ children }: { children: React.ReactNode }) =>
				React.createElement(QueryClientProvider, { client }, children),
		});
		await waitFor(() => expect(repo.listLatestMessages).toHaveBeenCalled());
		repo.listLatestMessages.mockResolvedValue({
			messages: [msg(1), msg(2), msg(3, { id: "srv", sender: "buyer" })],
			hasMore: false,
			peerReadSeq: 0,
		});
		await act(async () => {
			await result.current.send("hi");
		});
		expect(cancel).not.toHaveBeenCalled();
		await act(async () => {
			resolveHistory({
				messages: [msg(1), msg(2)],
				hasMore: false,
				peerReadSeq: 0,
			});
		});
		await waitFor(() =>
			expect(result.current.messages.map((m) => m.seq)).toEqual([1, 2, 3]),
		);
	});

	it("cancels an in-flight read when history already exists", async () => {
		repo.listConversations.mockResolvedValue([conv]);
		repo.listLatestMessages.mockResolvedValue({
			messages: [msg(1)],
			hasMore: false,
			peerReadSeq: 0,
		});
		repo.sendMessage.mockResolvedValue(msg(2, { id: "srv", sender: "buyer" }));
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false, gcTime: Infinity } },
		});
		const cancel = vi.spyOn(client, "cancelQueries");
		const { result } = renderHook(() => useChatThread("9"), {
			wrapper: ({ children }: { children: React.ReactNode }) =>
				React.createElement(QueryClientProvider, { client }, children),
		});
		await waitFor(() => expect(result.current.messages).toHaveLength(1));
		await act(async () => {
			await result.current.send("hi");
		});
		expect(cancel).toHaveBeenCalledTimes(1);
	});

	it("sends with a UUID request_id that is also the optimistic id, then swaps in the server message", async () => {
		repo.listConversations.mockResolvedValue([conv]);
		repo.listLatestMessages.mockResolvedValue({
			messages: [msg(1)],
			hasMore: false,
			peerReadSeq: 0,
		});
		let resolve: (m: ChatMessage) => void = () => {};
		repo.sendMessage.mockReturnValue(
			new Promise<ChatMessage>((r) => (resolve = r)),
		);
		const { result } = renderHook(() => useChatThread("9"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.messages).toHaveLength(1));

		let sent: Promise<void> = Promise.resolve();
		act(() => {
			sent = result.current.send("  hello ");
		});
		await waitFor(() => expect(result.current.messages).toHaveLength(2));
		const requestId = repo.sendMessage.mock.calls[0]?.[2] as string;
		expect(requestId).toMatch(/^[0-9a-f-]{36}$/);
		expect(repo.sendMessage.mock.calls[0]?.[0]).toEqual({
			side: "buyer",
			conversationId: "c1",
			peerId: "9",
		});
		expect(repo.sendMessage.mock.calls[0]?.[1]).toEqual({
			kind: "text",
			text: "hello",
		});
		expect(result.current.messages[1]?.id).toBe(requestId);
		expect(result.current.messages[1]?.seq).toBeNull();

		await act(async () => {
			resolve(msg(2, { id: "srv", sender: "buyer", text: "hello" }));
			await sent;
		});
		await waitFor(() =>
			expect(result.current.messages.map((m) => m.id)).toEqual(["m1", "srv"]),
		);
	});

	it("rolls the optimistic message back and surfaces the error when the send fails", async () => {
		repo.listConversations.mockResolvedValue([conv]);
		repo.listLatestMessages.mockResolvedValue({
			messages: [],
			hasMore: false,
			peerReadSeq: 0,
		});
		repo.sendMessage.mockRejectedValue(
			new ConnectError("no", Code.FailedPrecondition),
		);
		const { result } = renderHook(() => useChatThread("9"), {
			wrapper: wrapper(),
		});
		await waitFor(() => expect(result.current.loading).toBe(false));
		await act(async () => {
			await result.current.send("hi").catch(() => {});
		});
		expect(result.current.messages).toEqual([]);
		expect(result.current.error).toBe("default-error");
	});
});
