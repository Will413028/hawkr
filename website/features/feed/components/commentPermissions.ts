/**
 * 貼文留言的權限與留言身分，抽出 `PostComments.tsx` 讓規則不用渲染元件就能測。
 *
 * 誰能編輯、刪除、回覆每一則留言，與觀看者能不能在這則貼文留言、用什麼身分留言，都由伺服器依觀看者算好
 * （`Comment.viewer_can_*`、`Post.viewer_can_comment`、`Post.viewer_actor`）。client 不看「現在在哪個畫面」，
 * 也不比對店家或買家的 ID，所以店主在公開頁和賣家中心看到同樣的操作。
 */

import type { PostViewerState } from "@/lib/api/openapi-types";
import type { Comment } from "@/lib/comments/repo";

export function canEditComment(c: Pick<Comment, "viewerCanEdit">): boolean {
	return c.viewerCanEdit;
}

export function canDeleteComment(c: Pick<Comment, "viewerCanDelete">): boolean {
	return c.viewerCanDelete;
}

export function canReplyToComment(c: Pick<Comment, "viewerCanReply">): boolean {
	return c.viewerCanReply;
}

/**
 * 店家能不能封鎖這則留言的作者：只有「別人寫的、而我能刪」的留言。伺服器只讓貼文所屬的店刪別人的留言
 * （`viewer_can_delete` 對別人的留言為真，代表這則留言在觀看者此刻的店的內容裡），而封鎖的對象必須是
 * 這家店內容裡的買家（`BlockBuyer`），兩個條件是同一件事。自己的留言（含店家自己的回覆）不是封鎖對象。
 */
export function canBlockCommentAuthor(
	c: Pick<Comment, "isMine" | "viewerCanDelete">,
): boolean {
	return !c.isMine && c.viewerCanDelete;
}

/** 留言框要不要出現：登入、而且伺服器說這則貼文觀看者能留言（被這家店封鎖的買家不能）。 */
export function canComposeComment(
	signedIn: boolean,
	viewer: PostViewerState | undefined,
): boolean {
	return signedIn && viewer?.canComment === true;
}

/** 以店家身分留言時的店名；不是店家身分，或讀不到店名（只帶 kind 與 id）時是 undefined。 */
export function storeActorName(
	viewer: PostViewerState | undefined,
): string | undefined {
	return viewer?.actor?.kind === "store" ? viewer.actor.name : undefined;
}
