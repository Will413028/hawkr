import type { PostListItem } from "@/lib/api/openapi-types";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import { isPostHidden } from "./postHidden";

/**
 * 店主看自己動態的留言區時，買家此刻看不到這些留言的原因。店主不論動態公不公開都讀得到、刪得了自己動態的留言
 * （伺服器對店主放行），所以這裡只決定「要不要加一句說明」，不擋留言的讀取。
 *
 * 多個原因同時成立時依序是：平台下架、店家不能公開、店主自己下架（平台下架與發布狀態無關，且店主自己的動作解不開）。
 * 草稿沒有發布過、不會有留言，不加說明。
 *
 * `storePublic` 是店家的 `PUBLIC_LISTING` 能力是否啟用；沒有這個資訊（代行的工作區、尚未載入）傳 `undefined`，
 * 只依動態自己的狀態判斷。`publishStatus` 缺值是「這條路徑的上游沒給」，不下結論。
 */
export type PostCommentsNotice =
	| "store-not-public"
	| "unpublished"
	| "platform-takedown";

export function postCommentsNotice(
	post: Pick<PostListItem, "hiddenAt" | "publishStatus" | "takedownMeasureId">,
	storePublic: boolean | undefined,
): PostCommentsNotice | null {
	if (isPostHidden(post)) return "platform-takedown";
	if (storePublic === false) return "store-not-public";
	if (post.publishStatus === PostPublishStatus.UNPUBLISHED)
		return "unpublished";
	return null;
}

/** 說明的訊息 key（`seller.posts.commentsNotice.*`）。 */
export const POST_COMMENTS_NOTICE_KEY = {
	"store-not-public": "storeNotPublic",
	unpublished: "unpublished",
	"platform-takedown": "platformTakedown",
} as const satisfies Record<PostCommentsNotice, string>;
