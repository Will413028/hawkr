import { PostStatus } from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { describe, expect, it, vi } from "vitest";
import { createOwnerPostApi, type OwnerPostClient } from "@/lib/posts/owner";
import { fetchPreviewPosts, PREVIEW_POSTS_PAGE_SIZE } from "./preview-posts";

describe("fetchPreviewPosts", () => {
	it("lists only published posts of the owner, page by page", async () => {
		const listMyPosts = vi.fn(async () => ({
			posts: [],
			nextPageToken: "next",
		}));
		const api = createOwnerPostApi({
			listMyPosts,
		} as unknown as OwnerPostClient);

		const page = await fetchPreviewPosts("tok", api);

		expect(listMyPosts).toHaveBeenCalledWith({
			pageSize: PREVIEW_POSTS_PAGE_SIZE,
			pageToken: "tok",
			status: PostStatus.PUBLISHED,
		});
		expect(page.nextPageToken).toBe("next");
	});
});
