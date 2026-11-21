import type { PostListItem } from "@/lib/api/openapi-types";
import { socialAppClient } from "@/lib/connect/social";
import { createOwnerPostApi } from "@/lib/posts/owner";

export const PREVIEW_POSTS_PAGE_SIZE = 12;

/** 店面預覽的貼文分頁：只讀店主端的已發布貼文，不經公開讀取與共用快取。 */
export function fetchPreviewPosts(
	pageToken = "",
	api = createOwnerPostApi(socialAppClient()),
): Promise<{ items: PostListItem[]; nextPageToken: string }> {
	return api.listPublished(PREVIEW_POSTS_PAGE_SIZE, pageToken);
}
