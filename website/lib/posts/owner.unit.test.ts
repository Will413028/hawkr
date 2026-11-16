import { describe, expect, it, vi } from "vitest";
import { createOwnerPostApi, type OwnerPostClient } from "./owner";

function fakeClient() {
	const rpc = {
		createPost: vi.fn(async () => ({ post: { id: "p-new" } })),
		getMyPost: vi.fn(async () => ({ post: { id: "p-1", etag: "cur" } })),
		updatePost: vi.fn(async () => ({})),
		publishPost: vi.fn(async () => ({})),
		unpublishPost: vi.fn(async () => ({})),
		deletePost: vi.fn(async () => ({})),
		listMyPosts: vi.fn(async () => ({ posts: [] })),
		getStoreSocialSummary: vi.fn(async () => ({
			postCounts: [
				{ status: 1, count: BigInt(2) },
				{ status: 2, count: BigInt(5) },
				{ status: 3, count: BigInt(1) },
			],
		})),
	};
	return { rpc, api: createOwnerPostApi(rpc as unknown as OwnerPostClient) };
}

describe("owner post calls", () => {
	it("create publishes in one action with a UUID request id", async () => {
		const { rpc, api } = fakeClient();
		const out = await api.create({
			content: "今天採收",
			stagedToken: "stg",
			productId: "88",
			tags: ["#a"],
		});
		expect(out).toEqual({ id: "p-new" });
		const [req] = rpc.createPost.mock.calls[0] as unknown as [
			Record<string, unknown>,
		];
		expect(req).toMatchObject({
			body: "今天採收",
			imageStagedToken: "stg",
			productId: "88",
			hashtagInputs: ["#a"],
			publish: true,
		});
		expect(req.requestId).toMatch(
			/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
		);
	});

	it("update reads the current etag and masks only what changed", async () => {
		const { rpc, api } = fakeClient();
		await api.update("p-1", { content: "新內文" });
		expect(rpc.getMyPost).toHaveBeenCalledOnce();
		const [req] = rpc.updatePost.mock.calls[0] as unknown as [
			{ post: { etag: string; body: string }; updateMask: { paths: string[] } },
		];
		expect(req.post).toMatchObject({ etag: "cur", body: "新內文" });
		expect(req.updateMask.paths).toEqual(["body"]);
	});

	it("update with a new image and tags adds exactly those paths", async () => {
		const { rpc, api } = fakeClient();
		await api.update(
			"p-1",
			{
				content: "x",
				newStagedToken: "stg2",
				tags: ["t"],
				productId: "9",
			},
			"known",
		);
		expect(rpc.getMyPost).not.toHaveBeenCalled();
		const [req] = rpc.updatePost.mock.calls[0] as unknown as [
			{ updateMask: { paths: string[] } },
		];
		expect(req.updateMask.paths).toEqual([
			"body",
			"image_staged_token",
			"product_id",
			"hashtag_inputs",
		]);
	});

	it("publish and unpublish are separate calls carrying the etag", async () => {
		const { rpc, api } = fakeClient();
		await api.setPublished("p-1", true);
		await api.setPublished("p-1", false, "given");
		expect(rpc.publishPost).toHaveBeenCalledExactlyOnceWith({
			postId: "p-1",
			etag: "cur",
		});
		expect(rpc.unpublishPost).toHaveBeenCalledExactlyOnceWith({
			postId: "p-1",
			etag: "given",
		});
	});

	it("the count is the server's per-status sum, not a list length", async () => {
		const { rpc, api } = fakeClient();
		expect(await api.count()).toBe(8);
		expect(rpc.getStoreSocialSummary).toHaveBeenCalledOnce();
		expect(rpc.listMyPosts).not.toHaveBeenCalled();
	});
});
