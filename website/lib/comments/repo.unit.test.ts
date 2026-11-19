import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	CommentSchema,
	CommentState,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
	likeComment: vi.fn(),
	unlikeComment: vi.fn(),
	createComment: vi.fn(),
	updateComment: vi.fn(),
	deleteComment: vi.fn(),
	listComments: vi.fn(),
}));

vi.mock("../connect/social", () => ({
	socialAppClient: () => rpc,
	socialViewerClient: () => rpc,
}));

import {
	commentFromWire,
	createComment,
	deleteComment,
	fetchCommentReplies,
	fetchPostComments,
	setCommentLiked,
	updateComment,
} from "./repo";

beforeEach(() => {
	for (const fn of Object.values(rpc)) fn.mockReset();
});

describe("comment likes (CD1)", () => {
	it("unliking calls UnlikeComment, never LikeComment", async () => {
		rpc.unlikeComment.mockResolvedValue({});
		const result = await setCommentLiked("c-1", false);
		expect(rpc.unlikeComment).toHaveBeenCalledExactlyOnceWith({
			commentId: "c-1",
		});
		expect(rpc.likeComment).not.toHaveBeenCalled();
		expect(result).toEqual({ liked: false });
	});

	it("liking calls LikeComment, never UnlikeComment", async () => {
		rpc.likeComment.mockResolvedValue({});
		const result = await setCommentLiked("c-1", true);
		expect(rpc.likeComment).toHaveBeenCalledExactlyOnceWith({
			commentId: "c-1",
		});
		expect(rpc.unlikeComment).not.toHaveBeenCalled();
		expect(result).toEqual({ liked: true });
	});

	it("a failed unlike rejects instead of being swallowed", async () => {
		rpc.unlikeComment.mockRejectedValue(new Error("offline"));
		await expect(setCommentLiked("c-1", false)).rejects.toThrow("offline");
	});
});

function wire(id: string, init: Record<string, unknown> = {}) {
	return create(CommentSchema, {
		id,
		postId: "p-1",
		state: CommentState.VISIBLE,
		body: `body ${id}`,
		author: { name: `author ${id}` },
		createTime: timestampFromDate(new Date("2026-11-01T00:00:00Z")),
		etag: `e-${id}`,
		...init,
	} as never);
}

describe("comment mapping", () => {
	it("carries the server's viewer flags, etag and counts", () => {
		const c = commentFromWire(
			wire("c-1", {
				likeCount: BigInt(3),
				replyCount: BigInt(2),
				viewerLiked: true,
				isMine: true,
				viewerCanEdit: true,
				viewerCanDelete: true,
				viewerCanReply: true,
			}),
		);
		expect(c).toMatchObject({
			id: "c-1",
			postId: "p-1",
			content: "body c-1",
			userName: "author c-1",
			likeCount: 3,
			replyCount: 2,
			isLiked: true,
			isMine: true,
			etag: "e-c-1",
			state: "visible",
			viewerCanEdit: true,
			viewerCanDelete: true,
			viewerCanReply: true,
			createdAt: Date.parse("2026-11-01T00:00:00Z") / 1000,
		});
	});

	it("keeps a placeholder only when it still has replies to hang", async () => {
		rpc.listComments.mockResolvedValue({
			comments: [
				wire("gone", { state: CommentState.DELETED, replyCount: BigInt(0) }),
				wire("kept", { state: CommentState.DELETED, replyCount: BigInt(2) }),
				wire("ok"),
			],
			nextPageToken: "t2",
		});
		const page = await fetchPostComments("p-1");
		expect(page.records.map((c) => c.id)).toEqual(["kept", "ok"]);
		expect(page.nextPageToken).toBe("t2");
	});

	it("reads replies of a parent and names the replied-to author from the same batch", async () => {
		rpc.listComments.mockResolvedValue({
			comments: [
				wire("r-1", { parentId: "c-1", author: { name: "店家" } }),
				wire("r-2", { parentId: "c-1", replyToId: "r-1" }),
				wire("r-3", { parentId: "c-1", replyToId: "c-1" }),
			],
			nextPageToken: "",
		});
		const page = await fetchCommentReplies("p-1", "c-1", {
			parentName: "頂層作者",
		});
		expect(rpc.listComments).toHaveBeenCalledWith(
			{ postId: "p-1", parentId: "c-1", pageSize: 20, pageToken: "" },
			expect.anything(),
		);
		expect(page.records.map((c) => c.replyToName)).toEqual([
			undefined,
			"店家",
			"頂層作者",
		]);
	});
});

describe("comment writes", () => {
	it("creating sends the UUID request_id and parent for a reply", async () => {
		rpc.createComment.mockResolvedValue({ comment: wire("new") });
		await createComment("p-1", {
			content: "hi",
			requestId: "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
			parentId: "c-1",
			replyToId: "r-1",
		});
		expect(rpc.createComment).toHaveBeenCalledWith({
			postId: "p-1",
			body: "hi",
			requestId: "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
			parentId: "c-1",
			replyToId: "r-1",
		});
	});

	it("editing sends the etag with a body-only update mask", async () => {
		rpc.updateComment.mockResolvedValue({ comment: wire("c-1") });
		await updateComment("c-1", "new", "e-1");
		expect(rpc.updateComment).toHaveBeenCalledWith({
			comment: { id: "c-1", body: "new", etag: "e-1" },
			updateMask: { paths: ["body"] },
		});
	});

	it("deleting forwards the etag when known", async () => {
		rpc.deleteComment.mockResolvedValue({});
		await deleteComment("c-1", "e-1");
		expect(rpc.deleteComment).toHaveBeenCalledWith({
			commentId: "c-1",
			etag: "e-1",
		});
	});
});
