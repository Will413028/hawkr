/**
 * 貼文留言的資料層：SocialPublicService.ListComments 與 SocialAppService 的留言方法（Connect）。
 * 列表是 page_token 分頁（舊到新）；沒有即時推送：開啟時與下拉更新時重新讀（A7 計畫 Q3、SD20）。
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	CommentSchema,
	CommentState,
	type Comment as WireComment,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { socialAppClient, socialViewerClient } from "../connect/social";
import { imageUri } from "../media/image";

export interface Comment {
	/** 留言 ID（UUID 字串）；樂觀項目用 `tmp-` 加 request_id。 */
	id: string;
	postId: string;
	parentId?: string | null;
	replyToId?: string | null;
	/** 伺服器不回被回覆者的名字：只在同一批列表裡找得到作者時才有。 */
	replyToName?: string;
	/** 留言者是不是呼叫者此刻的買家或店家身分（伺服器算）。 */
	isMine: boolean;
	userName: string;
	/** 頭像圖片網址（伺服器給的 uri_template 代入 preset）；沒有頭像是 undefined。 */
	avatarKey?: string;
	content: string;
	likeCount?: number;
	replyCount?: number;
	isLiked?: boolean;
	/** 建立時間（unix 秒）。 */
	createdAt: number;
	/** 不透明的版本字串；修改與刪除時帶回。 */
	etag?: string;
	/** VISIBLE 以外（刪除、隱藏）是佔位項目：沒有作者與內文，只為了保留底下的回覆。 */
	state: "visible" | "deleted" | "hidden";
	viewerCanEdit: boolean;
	viewerCanDelete: boolean;
	viewerCanReply: boolean;
	/** 送出中或伺服器那份還不在已載入頁的項目：只顯示，沒有任何操作。 */
	optimistic?: boolean;
}

export interface CommentListResult {
	records: Comment[];
	/** 空字串是沒有下一頁。 */
	nextPageToken: string;
	pageSize: number;
}

function stateOf(state: CommentState): Comment["state"] {
	switch (state) {
		case CommentState.VISIBLE:
			return "visible";
		case CommentState.HIDDEN:
			return "hidden";
		default:
			return "deleted";
	}
}

/** 單則留言的映射；`replyToName` 由呼叫端依同一批的作者補。 */
export function commentFromWire(wire: WireComment): Comment {
	return {
		id: wire.id,
		postId: wire.postId,
		parentId: wire.parentId ?? null,
		replyToId: wire.replyToId ?? null,
		isMine: wire.isMine,
		userName: wire.author?.name?.trim() || "匿名",
		avatarKey: imageUri(wire.author?.avatar, "avatar"),
		content: wire.body,
		likeCount: Number(wire.likeCount),
		replyCount: Number(wire.replyCount),
		isLiked: wire.viewerLiked,
		createdAt: wire.createTime
			? timestampDate(wire.createTime).getTime() / 1000
			: 0,
		etag: wire.etag || undefined,
		state: stateOf(wire.state),
		viewerCanEdit: wire.viewerCanEdit,
		viewerCanDelete: wire.viewerCanDelete,
		viewerCanReply: wire.viewerCanReply,
	};
}

/** 同一批列表裡，有 reply_to_id 的留言補上被回覆者的名字。 */
function withReplyToNames(records: Comment[], parentName?: string): Comment[] {
	const nameById = new Map(
		records.filter((c) => c.state === "visible").map((c) => [c.id, c.userName]),
	);
	return records.map((c) => {
		if (!c.replyToId) return c;
		const name = nameById.get(c.replyToId) ?? parentName;
		return name ? { ...c, replyToName: name } : c;
	});
}

/** 沒有內文可顯示、也沒有回覆要掛的佔位項目沒有存在的理由。 */
function keepable(c: Comment): boolean {
	return c.state === "visible" || (c.replyCount ?? 0) > 0;
}

export const COMMENT_PAGE_SIZE = 20;

export async function fetchPostComments(
	postId: string,
	{
		pageToken = "",
		pageSize = COMMENT_PAGE_SIZE,
		signal,
	}: { pageToken?: string; pageSize?: number; signal?: AbortSignal } = {},
): Promise<CommentListResult> {
	const res = await socialViewerClient().listComments(
		{ postId, pageSize, pageToken },
		{ signal },
	);
	return {
		records: res.comments.map(commentFromWire).filter(keepable),
		nextPageToken: res.nextPageToken,
		pageSize,
	};
}

export async function fetchCommentReplies(
	postId: string,
	parentId: string,
	{
		pageToken = "",
		pageSize = COMMENT_PAGE_SIZE,
		parentName,
		signal,
	}: {
		pageToken?: string;
		pageSize?: number;
		/** 回覆沒有指定對象時，被回覆的是這則頂層留言的作者。 */
		parentName?: string;
		signal?: AbortSignal;
	} = {},
): Promise<CommentListResult> {
	const res = await socialViewerClient().listComments(
		{ postId, parentId, pageSize, pageToken },
		{ signal },
	);
	const records = withReplyToNames(
		res.comments.map(commentFromWire).filter((c) => c.state === "visible"),
		parentName,
	);
	return { records, nextPageToken: res.nextPageToken, pageSize };
}

export async function createComment(
	postId: string,
	body: {
		content: string;
		/** UUIDv4：同一次送出的重試沿用同一個，伺服器回目前的留言而不重複建立。 */
		requestId: string;
		parentId?: string;
		replyToId?: string;
	},
): Promise<Comment> {
	const res = await socialAppClient().createComment({
		postId,
		body: body.content,
		requestId: body.requestId,
		...(body.parentId ? { parentId: body.parentId } : {}),
		...(body.replyToId ? { replyToId: body.replyToId } : {}),
	});
	if (!res.comment) throw new Error("comment missing");
	return commentFromWire(res.comment);
}

export async function deleteComment(
	commentId: string,
	etag?: string,
): Promise<void> {
	await socialAppClient().deleteComment({ commentId, etag: etag ?? "" });
}

/** 修改自己的留言內文；etag 必填，沒有就先讀（版本衝突回 aborted）。 */
export async function updateComment(
	commentId: string,
	content: string,
	etag: string,
): Promise<Comment> {
	const res = await socialAppClient().updateComment({
		comment: { id: commentId, body: content, etag },
		updateMask: fieldMask(CommentSchema, ["body"]),
	});
	if (!res.comment) throw new Error("comment missing");
	return commentFromWire(res.comment);
}

/** 對留言按讚或取消：兩個明確的方法（取消讚是 UnlikeComment，不是對讚的端點發 DELETE）。 */
export async function setCommentLiked(
	commentId: string,
	liked: boolean,
): Promise<{ liked: boolean }> {
	const client = socialAppClient();
	if (liked) await client.likeComment({ commentId });
	else await client.unlikeComment({ commentId });
	return { liked };
}
