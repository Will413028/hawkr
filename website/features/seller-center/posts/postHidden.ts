import type { PostListItem } from "@/lib/api/openapi-types";

/** 賣家自己頁面：貼文是否被平台下架（有下架措施 ID，或 hiddenAt unix > 0）。 */
export function isPostHidden(
	p: Pick<PostListItem, "hiddenAt" | "takedownMeasureId">,
): boolean {
	return (
		Boolean(p.takedownMeasureId) ||
		(typeof p.hiddenAt === "number" && p.hiddenAt > 0)
	);
}
