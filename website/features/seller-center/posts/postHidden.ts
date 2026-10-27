import type { PostListItem } from "@/lib/api/openapi-types";

/** 賣家自己頁面：貼文是否被平台下架（hiddenAt unix > 0）。 */
export function isPostHidden(p: Pick<PostListItem, "hiddenAt">): boolean {
	return typeof p.hiddenAt === "number" && p.hiddenAt > 0;
}
