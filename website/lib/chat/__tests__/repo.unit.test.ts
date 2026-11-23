import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	ChatParty,
	ConversationSchema,
	MessageKind,
	MessageSchema,
} from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { describe, expect, it, vi } from "vitest";
import {
	fetchPresence,
	listConversations,
	listLatestMessages,
	listMessagesAfter,
	markConversationRead,
	sendMessage,
} from "../repo";

// biome-ignore lint/suspicious/noExplicitAny: partial fake of the generated client
type Fake = any;
const client = (fake: Fake) => () => fake;
const srvMsg = (seq: number) =>
	create(MessageSchema, {
		id: `m${seq}`,
		conversationId: "c1",
		seq: BigInt(seq),
		sender: ChatParty.BUYER,
		kind: MessageKind.TEXT,
		text: "t",
	});

describe("listConversations", () => {
	it("uses the buyer RPC for the buyer side and the store RPC for the store side", async () => {
		const buyer = vi
			.fn()
			.mockResolvedValue({ conversations: [], nextPageToken: "" });
		const store = vi
			.fn()
			.mockResolvedValue({ conversations: [], nextPageToken: "" });
		const fake = { listConversations: buyer, listStoreConversations: store };
		await listConversations("buyer", client(fake));
		await listConversations("store", client(fake));
		expect(buyer).toHaveBeenCalledOnce();
		expect(store).toHaveBeenCalledOnce();
	});

	it("follows next_page_token until it is empty", async () => {
		const list = vi
			.fn()
			.mockResolvedValueOnce({
				conversations: [create(ConversationSchema, { id: "a" })],
				nextPageToken: "t1",
			})
			.mockResolvedValueOnce({
				conversations: [create(ConversationSchema, { id: "b" })],
				nextPageToken: "",
			});
		const rows = await listConversations(
			"buyer",
			client({ listConversations: list }),
		);
		expect(rows.map((r) => r.id)).toEqual(["a", "b"]);
		expect(list.mock.calls[1]?.[0]).toMatchObject({ pageToken: "t1" });
	});

	it("a read failure rejects; it is not turned into an empty list", async () => {
		const list = vi
			.fn()
			.mockRejectedValue(new ConnectError("boom", Code.Internal));
		await expect(
			listConversations("buyer", client({ listConversations: list })),
		).rejects.toBeInstanceOf(ConnectError);
	});
});

describe("messages", () => {
	it("returns the latest page oldest-first (the server returns newest-first)", async () => {
		const list = vi.fn().mockResolvedValue({
			messages: [srvMsg(3), srvMsg(2), srvMsg(1)],
			hasMore: true,
			peerReadSeq: BigInt(2),
		});
		const page = await listLatestMessages("c1", client({ listMessages: list }));
		expect(page.messages.map((m) => m.seq)).toEqual([1, 2, 3]);
		expect(page).toMatchObject({ hasMore: true, peerReadSeq: 2 });
	});

	it("fills a gap with after_seq and keeps the server's ascending order", async () => {
		const list = vi.fn().mockResolvedValue({
			messages: [srvMsg(3), srvMsg(4)],
			hasMore: false,
			peerReadSeq: BigInt(0),
		});
		const page = await listMessagesAfter(
			"c1",
			2,
			client({ listMessages: list }),
		);
		expect(list.mock.calls[0]?.[0]).toMatchObject({
			conversationId: "c1",
			afterSeq: BigInt(2),
		});
		expect(page.messages.map((m) => m.seq)).toEqual([3, 4]);
	});

	it("a thread read failure rejects instead of returning an empty thread", async () => {
		const list = vi
			.fn()
			.mockRejectedValue(new ConnectError("x", Code.Unavailable));
		await expect(
			listLatestMessages("c1", client({ listMessages: list })),
		).rejects.toBeInstanceOf(ConnectError);
	});

	it("marks read up to a seq and queries presence by side", async () => {
		const mark = vi.fn().mockResolvedValue({});
		await markConversationRead("c1", 7, client({ markConversationRead: mark }));
		expect(mark.mock.calls[0]?.[0]).toEqual({
			conversationId: "c1",
			uptoSeq: BigInt(7),
		});

		const pres = vi.fn().mockResolvedValue({
			presences: [{ conversationId: "c1", online: true }],
		});
		const map = await fetchPresence(
			"store",
			["c1"],
			client({ getStorePresence: pres }),
		);
		expect(map.get("c1")).toBe(true);
	});
});

describe("sendMessage", () => {
	const ok = (m = srvMsg(1)) => vi.fn().mockResolvedValue({ message: m });

	it("sends request_id with text to an existing conversation", async () => {
		const send = ok();
		await sendMessage(
			{ side: "buyer", conversationId: "c1", peerId: "9" },
			{ kind: "text", text: "hi" },
			"rid-1",
			client({ sendMessage: send }),
		);
		expect(send.mock.calls[0]?.[0]).toEqual({
			conversationId: "c1",
			requestId: "rid-1",
			content: { content: { case: "text", value: "hi" } },
		});
	});

	it("opens a conversation by peer id: buyer -> store, store -> buyer", async () => {
		const toStore = ok();
		const toBuyer = ok();
		const fake = { sendMessageToStore: toStore, sendMessageToBuyer: toBuyer };
		await sendMessage(
			{ side: "buyer", peerId: "9" },
			{ kind: "text", text: "a" },
			"r1",
			client(fake),
		);
		await sendMessage(
			{ side: "store", peerId: "5" },
			{ kind: "text", text: "b" },
			"r2",
			client(fake),
		);
		expect(toStore.mock.calls[0]?.[0]).toMatchObject({
			storeId: "9",
			requestId: "r1",
		});
		expect(toBuyer.mock.calls[0]?.[0]).toMatchObject({
			buyerId: "5",
			requestId: "r2",
		});
	});

	it("sends an image as a staged token, never the preview url", async () => {
		const send = ok();
		await sendMessage(
			{ side: "buyer", conversationId: "c1", peerId: "9" },
			{ kind: "image", stagedToken: "tok" },
			"r",
			client({ sendMessage: send }),
		);
		expect(send.mock.calls[0]?.[0].content).toEqual({
			content: { case: "imageStagedToken", value: "tok" },
		});
	});

	it("a retry with the same request_id sends the same request_id (server dedupes)", async () => {
		const send = ok();
		const target = {
			side: "buyer" as const,
			conversationId: "c1",
			peerId: "9",
		};
		await sendMessage(
			target,
			{ kind: "text", text: "hi" },
			"same",
			client({ sendMessage: send }),
		);
		await sendMessage(
			target,
			{ kind: "text", text: "hi" },
			"same",
			client({ sendMessage: send }),
		);
		expect(send.mock.calls.map((c) => c[0].requestId)).toEqual([
			"same",
			"same",
		]);
	});
});
