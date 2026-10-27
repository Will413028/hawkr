import { ApiClientError, apiClient } from "@/lib/api/client";
import { mapServerComment } from "@/lib/social/repo";

/**
 * 討論區留言。`gateway/internal/handler/seller_forum.go` 的 `ForumRecord` 是
 * wire 形狀（全 camelCase，backend#2235）：
 * `id / sellerId / isMine / userName / avatarKey / avatarUrl / content /
 *  replyToName / parentId / replyToId / createdAt / replyCount`。
 *
 * `rootId` / `likeCount` / `isLiked` / `replies` **不在 wire 上** —— 它們是
 * useSellerForum 維護的前端狀態（replies 由 `/replies` 端點另外載入合併、
 * like 是樂觀更新）。標成 client-only 是為了讓「為什麼 mapper 不去讀它」
 * 這件事寫在型別上，而不是靠下一個人自己發現。
 */
export interface SellerForumComment {
	id: string;
	sellerId?: string | null;
	parentId?: string | null;
	replyToId?: string | null;
	replyToName?: string;
	isMine: boolean;
	userName: string;
	avatarKey?: string;
	content: string;
	replyCount?: number;
	createdAt: number;
	/* --- 以下為前端狀態，wire 不帶 --- */
	rootId?: string | null;
	likeCount?: number;
	isLiked?: boolean;
	replies?: SellerForumComment[];
}

export interface SellerForumListResult {
	records: SellerForumComment[];
	page: number;
	pageSize: number;
	total: number;
}

interface ServerListEnvelope {
	records?: unknown[];
	page?: number | string;
	pageSize?: number | string;
	total?: number | string;
}

function num(value: unknown, fallback = 0): number {
	const n = Number(value);
	return Number.isFinite(n) ? n : fallback;
}

// str / idStr 已下放到 `mapServerComment`（lib/social/repo.ts:65-80）——
// 本檔的 mapper 現在整段委派給它，TSID 保字串的保證也在那裡（同檔 :74）。
function optionalId(value: unknown): string | null {
	const normalized = typeof value === "string" ? value.trim() : value;
	if (normalized == null || normalized === "" || normalized === "null") {
		return null;
	}
	const s = String(normalized);
	return s !== "0" ? s : null;
}

function unwrapData(value: unknown): unknown {
	if (value && typeof value === "object" && "data" in value) {
		return (value as { data?: unknown }).data;
	}
	return value;
}

function rawRecords(value: unknown): unknown[] {
	if (Array.isArray(value)) return value;
	const env = (value ?? {}) as ServerListEnvelope;
	return env.records ?? [];
}

export function mapSellerForumComment(raw: unknown): SellerForumComment {
	const c = (raw ?? {}) as Record<string, unknown>;
	// `ForumRecord` 與貼文留言的 `commentData` 是同一組欄名，共用 mapper 即可；
	// 差異只有 forum 多 `sellerId`、少 likeCount/isLiked（見上方型別註解）。
	const base = mapServerComment(raw);
	return {
		id: base.id,
		sellerId: optionalId(c.sellerId),
		parentId: base.parentId,
		replyToId: base.replyToId,
		replyToName: base.replyToName,
		isMine: base.isMine,
		userName: base.userName,
		// §231: ForumRecord carries the flat pair `avatarKey` (objectKey SSOT) +
		// `avatarUrl` (Gateway-composed) — seller_forum.go:19-20. Read the key;
		// the create/reply/update responses share this same shape.
		avatarKey: base.avatarKey,
		content: base.content,
		replyCount: num(c.replyCount),
		createdAt: base.createdAt,
	};
}

export function normalizeSellerForumList(
	value: unknown,
	fallbackPageSize = 20,
): SellerForumListResult {
	const data = unwrapData(value);
	if (Array.isArray(data)) {
		return {
			records: data.map(mapSellerForumComment),
			page: 1,
			pageSize: fallbackPageSize,
			total: data.length,
		};
	}

	const env = (data ?? {}) as ServerListEnvelope;
	const records = rawRecords(data).map(mapSellerForumComment);
	return {
		records,
		page: num(env.page, 1),
		pageSize: num(env.pageSize, fallbackPageSize),
		total: num(env.total, records.length),
	};
}

async function emptyOnMissing<T>(task: Promise<T>, fallback: T): Promise<T> {
	try {
		return await task;
	} catch (err) {
		if (
			err instanceof ApiClientError &&
			(err.status === 404 || err.status === 204)
		) {
			return fallback;
		}
		throw err;
	}
}

export async function fetchSellerForum(
	sellerId: number | string,
	{ page = 1, pageSize = 20 }: { page?: number; pageSize?: number } = {},
): Promise<SellerForumListResult> {
	const env = await emptyOnMissing(
		apiClient.public.get<unknown>(
			`/sellers/${encodeURIComponent(String(sellerId))}/forum`,
			{
				query: { page, page_size: pageSize },
			},
		),
		{ code: 0, data: [] as unknown },
	);
	return normalizeSellerForumList(env, pageSize);
}

export async function fetchSellerForumReplies(
	commentId: number | string,
	{ page = 1, pageSize = 20 }: { page?: number; pageSize?: number } = {},
): Promise<SellerForumListResult> {
	const env = await emptyOnMissing(
		apiClient.public.get<unknown>(
			`/sellers/forum/${encodeURIComponent(String(commentId))}/replies`,
			{ query: { page, page_size: pageSize } },
		),
		{ code: 0, data: [] as unknown },
	);
	return normalizeSellerForumList(env, pageSize);
}

export async function createSellerForumComment(
	sellerId: number | string,
	content: string,
): Promise<SellerForumComment> {
	const env = await apiClient.authed.post<unknown>(
		`/sellers/${encodeURIComponent(String(sellerId))}/forum`,
		{ json: { content } },
	);
	return mapSellerForumComment(env.data ?? env);
}

export async function replySellerForumComment(
	commentId: number | string,
	content: string,
): Promise<SellerForumComment> {
	const env = await apiClient.authed.post<unknown>(
		`/sellers/forum/${encodeURIComponent(String(commentId))}/reply`,
		{ json: { content } },
	);
	return mapSellerForumComment(env.data ?? env);
}

export async function updateSellerForumComment(
	commentId: number | string,
	content: string,
): Promise<SellerForumComment> {
	const env = await apiClient.authed.put<unknown>(
		`/sellers/forum/${encodeURIComponent(String(commentId))}`,
		{ json: { content } },
	);
	return mapSellerForumComment(env.data ?? env);
}

export async function deleteSellerForumComment(
	commentId: number | string,
): Promise<void> {
	await apiClient.authed.delete(
		`/sellers/forum/${encodeURIComponent(String(commentId))}`,
	);
}

export async function toggleSellerForumCommentLike(
	commentId: number | string,
	currentlyLiked: boolean,
): Promise<{ liked: boolean }> {
	// TODO(seller-forum): APP uses the shared post-comment like endpoint for
	// forum likes. Swap this single path if backend adds a seller-forum route.
	const path = `/posts/comments/${encodeURIComponent(String(commentId))}/like`;
	if (currentlyLiked) {
		await apiClient.authed.delete(path, { throwOnHttpError: false });
		return { liked: false };
	}
	await apiClient.authed.post(path, { throwOnHttpError: false });
	return { liked: true };
}
