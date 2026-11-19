import { describe, expect, it, vi } from "vitest";
import type { OwnerPostClient } from "@/lib/posts/owner";
import { PostPublishStatus } from "../codemeta/enums";
import { createSellerPostRepo } from "./repo";

function setup() {
	const rpc = {
		getMyPost: vi.fn(async () => ({ post: { id: "p", etag: "fresh" } })),
		updatePost: vi.fn(async () => ({})),
		publishPost: vi.fn(async () => ({})),
		unpublishPost: vi.fn(async () => ({})),
	};
	return {
		rpc,
		repo: createSellerPostRepo(() => rpc as unknown as OwnerPostClient),
	};
}

describe("seller post repo carries the etag the UI loaded", () => {
	it("update sends body.etag and never re-reads", async () => {
		const { rpc, repo } = setup();
		await repo.updateSellerPost("p", { content: "x", etag: "loaded" });
		expect(rpc.getMyPost).not.toHaveBeenCalled();
		const [req] = rpc.updatePost.mock.calls[0] as unknown as [
			{ post: { etag: string } },
		];
		expect(req.post.etag).toBe("loaded");
	});

	it("publish and unpublish send the list row's etag", async () => {
		const { rpc, repo } = setup();
		await repo.setSellerPostPublishStatus(
			"p",
			PostPublishStatus.PUBLISHED as never,
			undefined,
			"row-1",
		);
		await repo.setSellerPostPublishStatus(
			"p",
			PostPublishStatus.UNPUBLISHED as never,
			undefined,
			"row-2",
		);
		expect(rpc.getMyPost).not.toHaveBeenCalled();
		expect(rpc.publishPost).toHaveBeenCalledWith({
			postId: "p",
			etag: "row-1",
		});
		expect(rpc.unpublishPost).toHaveBeenCalledWith({
			postId: "p",
			etag: "row-2",
		});
	});
});
