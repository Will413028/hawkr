import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	ConversationRole,
	GetConversationMessagesResponseSchema,
	ListConversationsResponseSchema,
	RequestStatus,
	SlotKey,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	deleteAgentConversation,
	fetchAgentConversations,
	fetchAgentTranscript,
	renameAgentConversation,
} from "./conversations";

const rpc = vi.hoisted(() => ({
	handlers: {} as Record<string, (request: never) => unknown>,
}));

vi.mock("@/lib/connect/procurement", async () => {
	const { fakeProcurementClient } = await import("./testing/fake-procurement");
	return { buyerProcurementClient: () => fakeProcurementClient(rpc.handlers) };
});

afterEach(() => {
	rpc.handlers = {};
});

describe("對話列表", () => {
	it("以 page_token 續抓，next_page_token 原樣當游標；空字串表示最後一頁", async () => {
		const list = vi.fn(() =>
			create(ListConversationsResponseSchema, {
				conversations: [
					{
						id: "conv-1",
						title: "蘋果",
						lastMessagePreview: "好的",
						updateTime: timestampFromDate(new Date("2026-09-13T02:00:00Z")),
					},
				],
				nextPageToken: "tok-2",
			}),
		);
		rpc.handlers.listConversations = list;

		const page = await fetchAgentConversations({ cursorId: "tok-1" });

		expect(list).toHaveBeenCalledWith(
			expect.objectContaining({ pageToken: "tok-1", pageSize: 50 }),
		);
		expect(page).toMatchObject({
			hasMore: true,
			nextCursorId: "tok-2",
			available: true,
			records: [
				{
					conversationId: "conv-1",
					title: "蘋果",
					updatedAt: "2026-09-13T02:00:00.000Z",
				},
			],
		});

		rpc.handlers.listConversations = () =>
			create(ListConversationsResponseSchema, {});
		const last = await fetchAgentConversations();
		expect(last).toMatchObject({ hasMore: false, nextCursorId: null });
	});
});

describe("對話紀錄", () => {
	it("一次讀整份、角色轉成 buyer／assistant、還原當輪已記下的條件", async () => {
		const get = vi.fn(() =>
			create(GetConversationMessagesResponseSchema, {
				conversationId: "conv-1",
				requestId: "req-1",
				status: RequestStatus.CLARIFYING,
				title: "蘋果",
				messages: [
					{ id: "m1", role: ConversationRole.USER, text: "我要買蘋果" },
					{
						id: "m2",
						role: ConversationRole.ASSISTANT,
						text: "好的",
						recordedThisTurn: [
							{ key: SlotKey.ITEM, label: "品項", value: "蘋果" },
						],
					},
				],
			}),
		);
		rpc.handlers.getConversationMessages = get;

		const transcript = await fetchAgentTranscript("conv-1");

		expect(get).toHaveBeenCalledTimes(1);
		expect(transcript.requestId).toBe("req-1");
		expect(transcript.records.map((r) => [r.id, r.role])).toEqual([
			["m1", "buyer"],
			["m2", "assistant"],
		]);
		expect(transcript.records[1].recordedThisTurn).toEqual([
			{ key: "item", label: "品項", value: "蘋果" },
		]);
		expect(transcript.records[0].recordedThisTurn).toBeUndefined();
	});
});

describe("改名與刪除", () => {
	it("兩者都帶全新的 client_request_id", async () => {
		const rename = vi.fn((_request: unknown) => ({}));
		const remove = vi.fn((_request: unknown) => ({}));
		rpc.handlers.renameConversation = rename as never;
		rpc.handlers.deleteConversation = remove as never;

		await renameAgentConversation("conv-1", "新名稱");
		await deleteAgentConversation("conv-1");

		expect(rename.mock.calls[0][0]).toMatchObject({
			conversationId: "conv-1",
			title: "新名稱",
		});
		const renameId = (rename.mock.calls[0][0] as { clientRequestId: string })
			.clientRequestId;
		const deleteId = (remove.mock.calls[0][0] as { clientRequestId: string })
			.clientRequestId;
		expect(renameId).not.toBe("");
		expect(deleteId).not.toBe("");
		expect(renameId).not.toBe(deleteId);
	});
});
