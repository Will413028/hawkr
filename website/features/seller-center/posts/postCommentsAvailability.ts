import type { PostListItem } from "@/lib/api/openapi-types";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import { isPostHidden } from "./postHidden";

/**
 * 這則動態此刻是否公開，也就是買家看得到它：店家能公開、已發布、沒有被平台下架。
 * 不公開時，公開的留言、回覆與即時串流都讀不到它，賣家中心的留言區不呼叫它們，改顯示原因。
 *
 * 多個原因同時成立時依序是：平台下架、店家不能公開、未發布（平台下架與發布狀態無關，
 * 且賣家自己的動作解不開）。
 *
 * `storePublic` 是店家的 `PUBLIC_LISTING` 能力是否啟用；沒有這個資訊（代行的工作區、
 * 尚未載入）傳 `undefined`，只依動態自己的狀態判斷。`publishStatus` 缺值是「這條路徑的上游
 * 沒給」，不下結論。
 */
export type PostCommentsAvailability =
	| "available"
	| "store-not-public"
	| "not-published"
	| "platform-hidden";

export function postCommentsAvailability(
	post: Pick<PostListItem, "hiddenAt" | "publishStatus">,
	storePublic: boolean | undefined,
): PostCommentsAvailability {
	if (isPostHidden(post)) return "platform-hidden";
	if (storePublic === false) return "store-not-public";
	if (
		post.publishStatus !== undefined &&
		post.publishStatus !== PostPublishStatus.PUBLISHED
	) {
		return "not-published";
	}
	return "available";
}

export const POST_COMMENTS_UNAVAILABLE_TEXT: Record<
	Exclude<PostCommentsAvailability, "available">,
	string
> = {
	"store-not-public": "店家公開後，買家的留言會顯示在這裡。",
	"not-published": "動態發布後，買家的留言會顯示在這裡。",
	"platform-hidden": "這則動態已被平台下架，買家看不到留言。",
};
