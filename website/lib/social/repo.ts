/**
 * Social interactions — post comments / likes / saves.
 *
 * Gateway paths (verified against buyer Flutter `social_repository.dart`,
 * NOT the openapi.json which still lists the old un-namespaced routes):
 *
 *   GET    /public/posts/{id}/comments         ?page&page_size  (public)
 *   POST   /app/posts/{id}/comments            { content, clientRequestId? }
 *   DELETE /app/posts/comments/{id}            (own comment or post owner)
 *   POST   /app/posts/comments/{id}/like       toggle on
 *   DELETE /app/posts/comments/{id}/like       toggle off
 *   POST   /app/posts/{id}/like                toggle (server tracks state)
 *   POST   /app/posts/{id}/save                toggle
 *   POST   /app/posts/{id}/share               share counter bump
 *
 * The list endpoint may return either a flat array (legacy) or a paged
 * envelope ({records, total, page}); normalizeListEnv covers both.
 */

import { apiClient, authedFetch } from "../api/client";

export interface Comment {
	// 主鍵為 64-bit TSID（> JS 2^53），一律保字串以免精度丟失，並對齊 SSE
	// CommentSnapshot（已是字串）與後端翻字串後的 REST。詳見 TSID-as-number 稽核。
	id: string;
	postId: string;
	parentId?: string | null;
	replyToId?: string | null;
	replyToName?: string;
	/** 留言者是不是呼叫者此刻的買家或店家身分（伺服器算，wire `isMine`）。 */
	isMine: boolean;
	userName: string;
	/** 頭像 objectKey（wire `avatarKey`，49 §6.1 SSOT）。 */
	avatarKey?: string;
	content: string;
	likeCount?: number;
	replyCount?: number;
	isLiked?: boolean;
	createdAt: number;
}

export interface CommentListResult {
	records: Comment[];
	page: number;
	pageSize: number;
	total: number;
}

interface ServerCommentEnv {
	comments?: unknown[];
	records?: unknown[];
	list?: unknown[];
	page?: number | string;
	pageSize?: number | string;
	total?: number | string;
}

/**
 * Wire → domain for `gateway/internal/handler/post.go` `commentData`. Every key
 * is camelCase (backend#2235) and each is read under exactly one name: a
 * silently-tolerated alias is how a renamed field turns into a blank row
 * instead of a red test.
 */
export function mapServerComment(raw: unknown): Comment {
	const c = (raw ?? {}) as Record<string, unknown>;
	const num = (v: unknown, fallback = 0) => {
		const n = Number(v);
		return Number.isFinite(n) ? n : fallback;
	};
	const str = (v: unknown) => (typeof v === "string" ? v : "");
	// TSID id：number 或 string 皆接受，一律保字串（後端翻字串前 String(number) 雖已
	// 在 JSON.parse 丟精度，但與 SSE 同步丟同一個值故 dedup 仍對得上；翻字串後則精確）。
	const idStr = (v: unknown) => (v == null ? "" : String(v));
	// 可選父/回覆 id：null / "0" / 0 一律視為「無」，與 SSE snapshotToComment 一致。
	const optId = (v: unknown): string | null => {
		if (v == null) return null;
		const s = String(v);
		return s && s !== "0" ? s : null;
	};
	return {
		id: idStr(c.id),
		postId: idStr(c.postId),
		parentId: optId(c.parentId),
		replyToId: optId(c.replyToId),
		replyToName: str(c.replyToName),
		isMine: c.isMine === true,
		userName: str(c.userName) || "匿名",
		// §231: flat pair `avatarKey` (objectKey SSOT) + `avatarUrl`; read the key.
		// The comment create/reply/update responses share this shape too.
		avatarKey: str(c.avatarKey),
		content: str(c.content),
		likeCount: num(c.likeCount),
		replyCount: num(c.replyCount),
		isLiked: Boolean(c.isLiked),
		createdAt: num(c.createdAt),
	};
}

function normalizeListEnv(d: unknown, fallback: number): CommentListResult {
	if (Array.isArray(d)) {
		return {
			records: d.map(mapServerComment),
			page: 1,
			pageSize: fallback,
			total: d.length,
		};
	}
	const env = (d ?? {}) as ServerCommentEnv;
	const rawRecords = env.comments ?? env.records ?? env.list ?? [];
	const records = rawRecords.map(mapServerComment);
	return {
		records,
		page: Number(env.page ?? 1),
		pageSize: Number(env.pageSize ?? fallback),
		total: Number(env.total ?? records.length),
	};
}

/* ===== Public read paths use plain fetch (no auth required for browsing) ===== */

// Anonymous list read. softFail → 404 (post unindexed) / 204 / 5xx / timeout all
// resolve to null → empty list. BigInt-safe parse (TSID ids) is built into dispatch.
async function publicGet(path: string): Promise<unknown> {
	const { data } = await apiClient.public.get<unknown>(path, {
		credentials: "omit",
		softFail: true,
	});
	return data ?? null;
}

export async function fetchPostComments(
	postId: number | string,
	{ page = 1, pageSize = 20 }: { page?: number; pageSize?: number } = {},
): Promise<CommentListResult> {
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
	});
	const raw = await publicGet(
		`/public/posts/${encodeURIComponent(String(postId))}/comments?${qs}`,
	);
	return normalizeListEnv(raw, pageSize);
}

export async function fetchCommentReplies(
	commentId: number | string,
	{ page = 1, pageSize = 20 }: { page?: number; pageSize?: number } = {},
): Promise<CommentListResult> {
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
	});
	// Pattern guess (gateway hasn't documented this); 404 silently → empty.
	const raw = await publicGet(
		`/public/posts/comments/${encodeURIComponent(String(commentId))}/replies?${qs}`,
	);
	return normalizeListEnv(raw, pageSize);
}

/* ===== Authed mutations ===== */

export async function createComment(
	postId: number | string,
	body: {
		content: string;
		/**
		 * Optional caller-generated UUID used by the SSE pipeline to dedupe
		 * the realtime `comment.created` echo against this optimistic write.
		 * Retries of the same logical submit MUST reuse the same id so the
		 * server's idempotency window collapses them — callers paint the
		 * submit button disabled on the first click as the primary guard,
		 * with `clientRequestId` as the safety net.
		 */
		clientRequestId?: string;
	},
): Promise<Comment> {
	const env = await authedFetch<unknown>(
		`/app/posts/${encodeURIComponent(String(postId))}/comments`,
		{
			method: "POST",
			json: body,
		},
	);
	return mapServerComment(env.data ?? env);
}

/**
 * Create a reply to a comment. Replies are a SEPARATE sub-resource from
 * top-level comments: `POST /app/posts/comments/{parentId}/replies` carries the
 * PARENT comment id in the path. The top-level `POST /app/posts/{id}/comments`
 * endpoint has no parentId/replyToId fields and silently drops them, so a
 * reply sent there is stored as a stray top-level comment (parentId=0) — that
 * was the "回覆變成獨立留言、沒掛在被回覆的留言下" threading bug.
 *
 * postId / replyToId are 64-bit TSIDs → send as strings, never Number()
 * (backend「數字欄位一律 string」契約，見 backend-payuni#1163).
 */
export async function createReply(
	parentCommentId: number | string,
	body: {
		content: string;
		postId: number | string;
		replyToId?: number | string;
		clientRequestId?: string;
	},
): Promise<Comment> {
	const { postId, replyToId, ...rest } = body;
	const env = await authedFetch<unknown>(
		`/app/posts/comments/${encodeURIComponent(String(parentCommentId))}/replies`,
		{
			method: "POST",
			json: {
				...rest,
				postId: String(postId),
				...(replyToId != null ? { replyToId: String(replyToId) } : {}),
			},
		},
	);
	return mapServerComment(env.data ?? env);
}

export async function deleteComment(commentId: number | string): Promise<void> {
	await authedFetch(
		`/app/posts/comments/${encodeURIComponent(String(commentId))}`,
		{ method: "DELETE" },
	);
}

/**
 * Edit own comment / reply. Backend enforces ownership; the UI only surfaces
 * the action on the viewer's own rows. Returns the updated Comment so the
 * caller can swap it into the list (preserves likeCount / replyCount etc.).
 */
export async function updateComment(
	commentId: number | string,
	content: string,
): Promise<Comment> {
	const env = await authedFetch<unknown>(
		`/app/posts/comments/${encodeURIComponent(String(commentId))}`,
		{
			method: "PUT",
			json: { content },
		},
	);
	return mapServerComment((env as { data?: unknown }).data ?? env);
}

export async function togglePostLike(
	postId: number | string,
): Promise<{ liked: boolean }> {
	type Shape = { liked?: boolean };
	const env = (await authedFetch<Shape>(
		`/app/posts/${encodeURIComponent(String(postId))}/like`,
		{ method: "POST" },
	)) as { liked?: boolean; data?: Shape };
	const liked = env.liked ?? env.data?.liked ?? false;
	return { liked };
}

export async function sharePost(
	postId: number | string,
	platform = "web",
): Promise<{ shareCount?: number }> {
	const env = await authedFetch<{ shareCount?: number }>(
		`/app/posts/${encodeURIComponent(String(postId))}/share`,
		{
			method: "POST",
			json: { platform },
			throwOnHttpError: false,
		},
	);
	return { shareCount: env.data?.shareCount };
}

/**
 * Comment like toggle uses POST for like / DELETE for unlike (per buyer
 * Flutter app). Caller passes the *current* liked state; we send the
 * opposite verb. Returns the new state.
 */
export async function toggleCommentLike(
	commentId: number | string,
	currentlyLiked: boolean,
): Promise<{ liked: boolean }> {
	const path = `/app/posts/comments/${encodeURIComponent(String(commentId))}/like`;
	if (currentlyLiked) {
		await authedFetch(path, { method: "DELETE", throwOnHttpError: false });
		return { liked: false };
	}
	await authedFetch(path, { method: "POST", throwOnHttpError: false });
	return { liked: true };
}
