import { PostPublishStatus } from "@/lib/codemeta/enums";
import type { PostPublishTarget } from "@/lib/seller/repo";

export type { PostPublishTarget };

/**
 * 由目前的發布狀態推出「按一下」要送的目標狀態。對齊商品那側的
 * `shelfToggleTarget`（`features/seller-center/components/ShelfStatusControl.tsx`）
 * —— 兩者的狀態機同構：起始態與撤下態都往「上線」走，上線態往「撤下」走，而兩邊
 * 都沒有一條路徑回得到起始態。
 *
 * 缺值回 `null`，呼叫端因此什麼都不畫。理由同 `isPostDraft`：缺值代表**這條資料
 * 路徑的上游沒給**，不是「這則貼文已發布」——推定成已發布會畫出一顆「下架」，
 * 而那顆按鈕對一則其實未發布的貼文按下去是 4403，賣家從此再也回不到未發布。
 */
export function postPublishToggleTarget(
	publishStatus?: string,
): PostPublishTarget | null {
	switch (publishStatus) {
		case PostPublishStatus.DRAFT:
		case PostPublishStatus.UNPUBLISHED:
			return PostPublishStatus.PUBLISHED;
		case PostPublishStatus.PUBLISHED:
			return PostPublishStatus.UNPUBLISHED;
		default:
			return null;
	}
}

/**
 * 這個目標狀態是不是「擴張型」操作（把內容推向買家）。
 *
 * 46 §3.3 的分類原則按這條分帳，而賣家能力層也照它掛：發布＝`CAP_MARKETING`
 * （停權賣家不可），下架＝`CAP_CONTENT_CONTRACT`（收縮型，停權賣家仍可自清）。
 * 見 `gateway/internal/router/routes_post.go:76-77`。
 */
export function isExpandingPublishTarget(target: PostPublishTarget): boolean {
	return target === PostPublishStatus.PUBLISHED;
}

/** 按鈕上的動詞。 */
export const POST_PUBLISH_ACTION_LABEL: Record<PostPublishTarget, string> = {
	[PostPublishStatus.PUBLISHED]: "發布",
	[PostPublishStatus.UNPUBLISHED]: "下架",
};

/**
 * 動作用語**不取 CodeMeta Label**：`POST_PUBLISH_STATUS_LABEL` 是狀態名
 * （未發布／已發布／已下架），按鈕上要的是動詞。商品那側同樣這樣分
 * （`ProductsPage.tsx:767-770`：「動作用語不在其內」），兩邊一致。
 */
export const POST_PUBLISH_PENDING_LABEL: Record<PostPublishTarget, string> = {
	[PostPublishStatus.PUBLISHED]: "發布中…",
	[PostPublishStatus.UNPUBLISHED]: "下架中…",
};

/** 有排程的貼文按「發布」時，確認文案多加的一句：發布會取消排程。 */
export const POST_PUBLISH_SCHEDULED_CONFIRM_TEXT =
	"這則動態已排程，現在發布會取消排程。";

/** 送出前的確認文字：發布一則有排程的貼文時，在確認文案後面多一句取消排程。 */
export function postPublishConfirmText(
	target: PostPublishTarget,
	scheduled: boolean,
): string {
	const base = POST_PUBLISH_CONFIRM_TEXT[target];
	return scheduled && isExpandingPublishTarget(target)
		? `${base}\n${POST_PUBLISH_SCHEDULED_CONFIRM_TEXT}`
		: base;
}

/** 送出前的二次確認文案。發布會讓內容進入買家動態，下架只是撤下，語氣不同。 */
export const POST_PUBLISH_CONFIRM_TEXT: Record<PostPublishTarget, string> = {
	[PostPublishStatus.PUBLISHED]: "發布這則動態？發布後買家會在動態牆看到它。",
	[PostPublishStatus.UNPUBLISHED]:
		"下架這則動態？內容會保留，但買家不再看得到。下架後可以再次發布，但無法改回「未發布」。",
};
