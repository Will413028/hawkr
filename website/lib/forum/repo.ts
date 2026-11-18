/**
 * 店家討論區的資料層：SocialPublicService.ListForumEntries 與 SocialAppService 的討論區方法（Connect）。
 * 列表是 page_token 分頁；討論區沒有讚（伺服器沒有討論區的讚，A7 計畫 2026-11-17）。
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	ActorKind,
	CommentState,
	type ForumEntry,
	ForumEntrySchema,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import {
	socialAppClient,
	socialPublicClient,
	socialViewerClient,
} from "../connect/social";
import { imageUri } from "../media/image";

export interface SellerForumComment {
	id: string;
	/** 討論區所屬的店家 ID。 */
	storeId: string;
	parentId?: string | null;
	replyToId?: string | null;
	/** 伺服器不回被回覆者的名字：只在同一批列表裡找得到作者時才有。 */
	replyToName?: string;
	isMine: boolean;
	userName: string;
	avatarKey?: string;
	/** 作者是買家或店家；佔位項目沒有作者。 */
	authorKind?: "buyer" | "store";
	content: string;
	replyCount?: number;
	/** 建立時間（unix 秒）。 */
	createdAt: number;
	/** 不透明的版本字串；修改與刪除時帶回。 */
	etag?: string;
	/** VISIBLE 以外（刪除、隱藏）是佔位項目：沒有作者與內文，不能回覆、修改或刪除。 */
	state: "visible" | "deleted" | "hidden";
	viewerCanEdit: boolean;
	viewerCanDelete: boolean;
	viewerCanReply: boolean;
	/* --- 以下為前端狀態，wire 不帶 --- */
	rootId?: string | null;
	replies?: SellerForumComment[];
	/** 回覆的下一頁憑證；空字串或缺席是沒有下一頁。 */
	repliesNextPageToken?: string;
}

export interface SellerForumListResult {
	records: SellerForumComment[];
	/** 空字串是沒有下一頁。 */
	nextPageToken: string;
	pageSize: number;
}

function forumStateOf(state: CommentState): SellerForumComment["state"] {
	switch (state) {
		case CommentState.VISIBLE:
			return "visible";
		case CommentState.HIDDEN:
			return "hidden";
		default:
			return "deleted";
	}
}

function authorKindOf(entry: ForumEntry): SellerForumComment["authorKind"] {
	switch (entry.author?.kind) {
		case ActorKind.BUYER:
			return "buyer";
		case ActorKind.STORE:
			return "store";
		default:
			return undefined;
	}
}

export function forumEntryFromWire(entry: ForumEntry): SellerForumComment {
	return {
		id: entry.id,
		storeId: entry.storeId,
		parentId: entry.parentId ?? null,
		replyToId: entry.replyToId ?? null,
		isMine: entry.isMine,
		userName: entry.author?.name?.trim() || "匿名",
		avatarKey: imageUri(entry.author?.avatar, "avatar"),
		authorKind: authorKindOf(entry),
		content: entry.body,
		replyCount: Number(entry.replyCount),
		createdAt: entry.createTime
			? timestampDate(entry.createTime).getTime() / 1000
			: 0,
		etag: entry.etag || undefined,
		state: forumStateOf(entry.state),
		viewerCanEdit: entry.viewerCanEdit,
		viewerCanDelete: entry.viewerCanDelete,
		viewerCanReply: entry.viewerCanReply,
	};
}

function withReplyToNames(
	records: SellerForumComment[],
	parentName?: string,
): SellerForumComment[] {
	const nameById = new Map(records.map((c) => [c.id, c.userName]));
	return records.map((c) => {
		if (!c.replyToId) return c;
		const name = nameById.get(c.replyToId) ?? parentName;
		return name ? { ...c, replyToName: name } : c;
	});
}

export interface FetchForumOptions {
	pageToken?: string;
	pageSize?: number;
	signal?: AbortSignal;
	/** 帶 session cookie 讀（瀏覽器）：伺服器才算得出 `is_mine` 與 `viewer_can_*`；SSR 是匿名讀取。 */
	viewer?: boolean;
}

export async function fetchSellerForum(
	storeId: string,
	{
		pageToken = "",
		pageSize = 20,
		signal,
		viewer = false,
	}: FetchForumOptions = {},
): Promise<SellerForumListResult> {
	const client = viewer ? socialViewerClient() : socialPublicClient();
	const res = await client.listForumEntries(
		{ storeId, pageToken, pageSize },
		{ signal },
	);
	// 佔位項目（刪除、隱藏而底下有回覆）沒有內文與作者：沒有回覆要掛的直接略過。
	const records = res.entries
		.filter((e) => e.state === CommentState.VISIBLE || Number(e.replyCount) > 0)
		.map(forumEntryFromWire);
	return { records, nextPageToken: res.nextPageToken, pageSize };
}

export async function fetchSellerForumReplies(
	storeId: string,
	parentId: string,
	{
		pageToken = "",
		pageSize = 20,
		parentName,
		signal,
	}: FetchForumOptions & { parentName?: string } = {},
): Promise<SellerForumListResult> {
	const res = await socialViewerClient().listForumEntries(
		{ storeId, parentId, pageToken, pageSize },
		{ signal },
	);
	const records = withReplyToNames(
		res.entries
			.filter((e) => e.state === CommentState.VISIBLE)
			.map(forumEntryFromWire),
		parentName,
	);
	return { records, nextPageToken: res.nextPageToken, pageSize };
}

export async function createSellerForumComment(
	storeId: string,
	content: string,
	requestId: string = crypto.randomUUID(),
): Promise<SellerForumComment> {
	const res = await socialAppClient().createForumEntry({
		storeId,
		body: content,
		requestId,
	});
	if (!res.entry) throw new Error("forum entry missing");
	return forumEntryFromWire(res.entry);
}

/** 回覆：`parentId` 是頂層留言，`replyToId` 是針對的那一則（回覆某則回覆時兩者不同）。 */
export async function replySellerForumComment(
	storeId: string,
	parentId: string,
	replyToId: string,
	content: string,
	requestId: string = crypto.randomUUID(),
): Promise<SellerForumComment> {
	const res = await socialAppClient().createForumEntry({
		storeId,
		body: content,
		parentId,
		replyToId,
		requestId,
	});
	if (!res.entry) throw new Error("forum entry missing");
	return forumEntryFromWire(res.entry);
}

export async function updateSellerForumComment(
	entryId: string,
	content: string,
	etag: string,
): Promise<SellerForumComment> {
	const res = await socialAppClient().updateForumEntry({
		entry: { id: entryId, body: content, etag },
		updateMask: fieldMask(ForumEntrySchema, ["body"]),
	});
	if (!res.entry) throw new Error("forum entry missing");
	return forumEntryFromWire(res.entry);
}

export async function deleteSellerForumComment(
	entryId: string,
	etag?: string,
): Promise<void> {
	await socialAppClient().deleteForumEntry({ entryId, etag: etag ?? "" });
}
