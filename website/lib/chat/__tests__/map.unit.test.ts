import { create } from "@bufbuild/protobuf";
import {
	ChatParty,
	ConversationSchema,
	MessageKind,
	MessageSchema,
} from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { describe, expect, it } from "vitest";
import {
	conversationFromProto,
	messageFromProto,
	peerName,
	previewText,
} from "../map";

const ut = (m: { key: string }) => `[${m.key}]`;

describe("previewText", () => {
	it("uses the catalog for image, hidden and erased; the server text for text", () => {
		const base = { sender: "buyer" as const, hidden: false, erased: false };
		expect(previewText(ut, { ...base, kind: "image" })).toBe(
			"[chat.Preview.image]",
		);
		expect(previewText(ut, { ...base, kind: "text", hidden: true })).toBe(
			"[chat.Preview.hidden]",
		);
		expect(previewText(ut, { ...base, kind: "text", erased: true })).toBe(
			"[chat.Preview.erased]",
		);
		expect(previewText(ut, { ...base, kind: "text", text: "hello" })).toBe(
			"hello",
		);
	});
	it("uses the catalog for an unsupported kind and has no text without a preview", () => {
		expect(
			previewText(ut, {
				kind: "unsupported",
				sender: "store",
				hidden: false,
				erased: false,
			}),
		).toBe("[chat.Message.unsupported_kind]");
		expect(previewText(ut, undefined)).toBe("");
	});
});

describe("messageFromProto", () => {
	it("keeps ids opaque, maps sender and kind, and builds the chat image url", () => {
		const m = messageFromProto(
			create(MessageSchema, {
				id: "m1",
				conversationId: "c1",
				seq: BigInt(7),
				sender: ChatParty.STORE,
				kind: MessageKind.IMAGE,
				image: { uriTemplate: "https://media.example/i/{preset}/x.png" },
			}),
		);
		expect(m).toMatchObject({
			id: "m1",
			conversationId: "c1",
			seq: 7,
			sender: "store",
			kind: "image",
		});
		expect(m.imageUrl).toBe("https://media.example/i/chat/x.png");
	});
	it("maps an unspecified kind to unsupported", () => {
		expect(
			messageFromProto(create(MessageSchema, { id: "m", seq: BigInt(1) })).kind,
		).toBe("unsupported");
	});
});

describe("conversationFromProto", () => {
	it("picks the peer by side and carries unread and read cursors", () => {
		const c = create(ConversationSchema, {
			id: "c1",
			store: { id: "9", name: "Shop", storeCode: "shop" },
			buyer: { id: "5", name: "Bob" },
			unreadCount: BigInt(3),
			peerReadSeq: BigInt(4),
		});
		expect(conversationFromProto(c, "buyer")).toMatchObject({
			targetUserId: "9",
			targetNickname: "Shop",
			storeCode: "shop",
			unreadCount: 3,
			peerReadSeq: 4,
		});
		expect(conversationFromProto(c, "store")).toMatchObject({
			targetUserId: "5",
			targetNickname: "Bob",
		});
	});
	it("gives a deleted buyer no name", () => {
		const c = create(ConversationSchema, {
			id: "c",
			buyer: { id: "5", name: "Old", deleted: true },
		});
		const conv = conversationFromProto(c, "store");
		expect(conv.targetNickname).toBe("");
		expect(conv.targetDeleted).toBe(true);
		expect(peerName(ut, conv)).toBe("[buyer.Profile.deleted_name]");
		expect(peerName(ut, { targetNickname: "Bob" })).toBe("Bob");
		expect(peerName(ut, { targetNickname: "" })).toBe("");
	});
});
