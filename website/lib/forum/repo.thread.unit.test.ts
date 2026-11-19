import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	CommentState,
	ForumEntrySchema,
	GetForumThreadResponseSchema,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { describe, expect, it, vi } from "vitest";

const asked = vi.hoisted(() => ({ req: undefined as unknown }));
vi.mock("../connect/social", () => ({
	socialPublicClient: () => ({
		getForumThread: async (req: unknown) => {
			asked.req = req;
			return create(GetForumThreadResponseSchema, {
				entry: create(ForumEntrySchema, {
					id: "t1",
					storeId: "7",
					state: CommentState.DELETED,
					replyCount: BigInt(2),
					createTime: timestampFromDate(new Date(1000)),
				}),
				replies: [
					create(ForumEntrySchema, {
						id: "r1",
						storeId: "7",
						parentId: "t1",
						state: CommentState.VISIBLE,
						body: "hi",
					}),
					create(ForumEntrySchema, {
						id: "r2",
						storeId: "7",
						parentId: "t1",
						state: CommentState.HIDDEN,
					}),
				],
				nextPageToken: "tok",
			});
		},
	}),
	socialViewerClient: () => {
		throw new Error("unused");
	},
	socialAppClient: () => {
		throw new Error("unused");
	},
}));

import { fetchSellerForumThread } from "./repo";

describe("fetchSellerForumThread", () => {
	it("maps the placeholder top as a one-record page and keeps only visible replies with the token", async () => {
		const res = await fetchSellerForumThread("7", "t1", { pageSize: 5 });
		expect(asked.req).toMatchObject({
			storeId: "7",
			threadId: "t1",
			pageSize: 5,
		});
		expect(res.top.records.map((r) => [r.id, r.state])).toEqual([
			["t1", "deleted"],
		]);
		expect(res.top.nextPageToken).toBe("");
		expect(res.replies.records.map((r) => r.id)).toEqual(["r1"]);
		expect(res.replies.nextPageToken).toBe("tok");
	});
});
