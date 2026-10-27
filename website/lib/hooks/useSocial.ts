"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	type Comment,
	type CommentListResult,
	createComment,
	deleteComment,
	fetchCommentReplies,
	fetchPostComments,
	toggleCommentLike,
	togglePostLike,
} from "@/lib/social/repo";
import type { SseConnectionStatus } from "@/lib/sse/client";
import { publishCommentEvent } from "@/lib/sse/commentEventBus";
import {
	type CommentCreatedPayload,
	type CommentDeletedPayload,
	type CommentLikeChangedPayload,
	type CommentSnapshot,
	type CommentUpdatedPayload,
	useCommentStream,
} from "@/lib/sse/useCommentStream";
import { errorMessage } from "./queryError";
import { useQueryIdentity } from "./useQueryIdentity";

/** Options for posting a top-level comment. The server decides the author
 *  identity (the shop on its own post, the member everywhere else). The
 *  seller-center surface overrides the optimistic author with the store
 *  name/avatar so the row doesn't flash the member's name first; other
 *  surfaces omit it (author from `identity`). */
export interface PostCommentOptions {
	authorName?: string;
	authorAvatar?: string;
}

interface UsePostCommentsResult {
	comments: Comment[];
	loading: boolean;
	loadingMore: boolean;
	hasMore: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
	post: (content: string, opts?: PostCommentOptions) => Promise<void>;
	remove: (id: number | string) => Promise<void>;
	like: (id: number | string) => Promise<void>;
	/** Realtime stream status. Consumers may render an inline "請重新整理頁面"
	 *  banner when it goes "unavailable" (design book §10 — no polling
	 *  fallback, the user gets a hint instead). */
	streamStatus: SseConnectionStatus;
}

interface UsePostCommentsOptions {
	/** Whether to open the realtime SSE stream for this post id. PostDetailViewer
	 *  mounts every page in its snap pager so this gates the connection to the
	 *  currently-active page; non-active pages stay on their last REST snapshot
	 *  until they swipe into view (or the user re-opens them). */
	active?: boolean;
}

/** Map an SSE CommentSnapshot to the repo's `Comment` shape so the
 *  rest of the UI keeps consuming a single contract. ISO 8601 → unix seconds
 *  matches the existing `Comment.createdAt: number` type. Exported so
 *  `CommentItem` subscribers of `commentEventBus` can hydrate replies pushed
 *  via SSE into the same `Comment` shape the REST-fetched replies use. */
export function snapshotToComment(snapshot: CommentSnapshot): Comment {
	const createdMs = snapshot.createdAt ? Date.parse(snapshot.createdAt) : NaN;
	const createdSec = Number.isFinite(createdMs)
		? Math.floor(createdMs / 1000)
		: Math.floor(Date.now() / 1000);
	// snapshot 的 id 已是字串（TSID 保字串）；"0" / "" 視為無父/回覆。
	const optId = (v: string) => (v && v !== "0" ? v : null);
	return {
		id: snapshot.commentId,
		postId: snapshot.postId,
		parentId: optId(snapshot.parentId),
		replyToId: optId(snapshot.replyToId),
		replyToName: snapshot.replyToName || undefined,
		isMine: snapshot.isMine,
		userName: snapshot.userName || "匿名",
		avatarKey: snapshot.avatarKey || undefined,
		content: snapshot.content,
		likeCount: snapshot.likeCount,
		replyCount: snapshot.replyCount,
		isLiked: false,
		createdAt: createdSec,
	};
}

/** Comments arrive newest-first from the REST list; new SSE-pushed top-level
 *  comments should land at the top — mirroring the optimistic `post()` UX. */
function insertAtTop(prev: Comment[], next: Comment): Comment[] {
	return [next, ...prev];
}

function emptyCommentList(pageSize = 20): CommentListResult {
	return {
		records: [],
		page: 1,
		pageSize,
		total: 0,
	};
}

const COMMENT_PAGE_SIZE = 20;

export function usePostComments(
	postId: number | string | undefined,
	options: UsePostCommentsOptions = {},
): UsePostCommentsResult {
	const active = options.active ?? true;
	const { identity } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const commentsParams = React.useMemo(
		() => ({ pageSize: COMMENT_PAGE_SIZE }),
		[],
	);
	const commentsKey = React.useMemo(
		() => queryKeys.feed.comments(queryIdentity, postId ?? "", commentsParams),
		[queryIdentity, postId, commentsParams],
	);
	const commentsQuery = useQuery({
		queryKey: commentsKey,
		enabled: Boolean(postId && active),
		// placeholderData（非 initialData）：staleTime 30s 下 initialData 會被當成
		// 「剛抓到的新鮮資料」而不在掛載時請求，導致空清單卡 30 秒（留言數顯示 8 但
		// 列表恆為 0）。placeholderData 只在 pending 期間頂著空清單佔位，仍會實際發
		// 請求載入真正的留言。
		placeholderData: emptyCommentList(COMMENT_PAGE_SIZE),
		staleTime: 30_000,
		queryFn: () =>
			fetchPostComments(postId ?? "", {
				page: 1,
				pageSize: COMMENT_PAGE_SIZE,
			}),
	});
	const [loadingMore, setLoadingMore] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	// placeholderData 讓 data 型別為 CommentListResult | undefined，補上空清單兜底
	// 與檔內其他 setQueryData 的 `?? emptyCommentList(...)` 一致。
	const data = commentsQuery.data ?? emptyCommentList(COMMENT_PAGE_SIZE);
	const comments = data.records;
	const page = data.page;
	const total = data.total;
	const refetchComments = commentsQuery.refetch;

	const setCommentData = React.useCallback(
		(updater: (current: CommentListResult) => CommentListResult) => {
			queryClient.setQueryData<CommentListResult>(commentsKey, (current) =>
				updater(current ?? emptyCommentList(COMMENT_PAGE_SIZE)),
			);
		},
		[commentsKey, queryClient],
	);

	// Dedup map for optimistic-vs-SSE-echo reconciliation (design book §13.3).
	// Lives in a ref so SSE handlers can read the current map without causing
	// re-renders or stale closures.
	const byClientRequestIdRef = React.useRef<Map<string, string>>(new Map());

	const refresh = React.useCallback(async () => {
		if (!postId) return;
		setError(null);
		await refetchComments();
	}, [postId, refetchComments]);

	const loadMore = React.useCallback(async () => {
		if (!postId || loadingMore || comments.length >= total) return;
		setLoadingMore(true);
		try {
			const r = await fetchPostComments(postId, {
				page: page + 1,
				pageSize: COMMENT_PAGE_SIZE,
			});
			setCommentData((current) => ({
				...r,
				records: [...current.records, ...r.records],
			}));
		} catch (e) {
			// e 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類，見
			// lib/api/client.ts），errorMessage 的 `!error → null` 分支不會在此觸發；
			// 若未來這個 try 區塊新增可能拋出 falsy 值的呼叫，需重新檢視此假設。
			setError(errorMessage(e, "載入更多失敗"));
		} finally {
			setLoadingMore(false);
		}
	}, [postId, loadingMore, comments.length, total, page, setCommentData]);

	const post = React.useCallback(
		async (content: string, opts?: PostCommentOptions) => {
			if (!postId || !content.trim()) return;
			const clientRequestId =
				typeof crypto !== "undefined" && "randomUUID" in crypto
					? crypto.randomUUID()
					: `cri-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
			const optimistic: Comment = {
				// 暫時 id：字串前綴避免與真實 TSID 字串撞號（取代舊的負數 -Date.now()）。
				id: `tmp-${Date.now()}`,
				postId: String(postId),
				// The viewer's own just-posted comment is mine from the first paint.
				isMine: true,
				// 賣家面以店家身分留言：optimistic 先顯示傳入的店家名／頭像，避免閃一下
				// 買家暱稱再被後端回的店家身分蓋掉。買家面不帶 opts → 沿用個人 identity。
				userName: opts?.authorName ?? identity?.nickname ?? "我",
				avatarKey: opts?.authorAvatar ?? identity?.avatarKey,
				content: content.trim(),
				likeCount: 0,
				replyCount: 0,
				isLiked: false,
				createdAt: Math.floor(Date.now() / 1000),
			};
			byClientRequestIdRef.current.set(clientRequestId, optimistic.id);
			setCommentData((current) => ({
				...current,
				records: [optimistic, ...current.records],
				total: current.total + 1,
			}));
			try {
				const real = await createComment(postId, {
					content: content.trim(),
					clientRequestId,
				});
				if (real?.id) {
					setCommentData((current) => ({
						...current,
						records: current.records.map((c) =>
							c.id === optimistic.id ? { ...optimistic, ...real } : c,
						),
					}));
					// Replace the optimistic id in the dedup map with the real id so
					// the SSE echo (if it arrives after the REST resolve) recognises
					// the reconciled row by either id and stays idempotent.
					byClientRequestIdRef.current.set(clientRequestId, real.id);
				}
			} catch (e) {
				byClientRequestIdRef.current.delete(clientRequestId);
				setCommentData((current) => ({
					...current,
					records: current.records.filter((c) => c.id !== optimistic.id),
					total: Math.max(0, current.total - 1),
				}));
				// e 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(e, "送出留言失敗"));
				throw e;
			}
		},
		[postId, identity?.nickname, identity?.avatarKey, setCommentData],
	);

	const remove = React.useCallback(
		async (id: number | string) => {
			try {
				await deleteComment(id);
				setCommentData((current) => ({
					...current,
					records: current.records.filter((c) => String(c.id) !== String(id)),
					total: Math.max(0, current.total - 1),
				}));
			} catch (e) {
				// e 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(e, "刪除失敗"));
			}
		},
		[setCommentData],
	);

	const like = React.useCallback(
		async (id: number | string) => {
			const target = comments.find((c) => String(c.id) === String(id));
			const wasLiked = target?.isLiked === true;
			setCommentData((current) => ({
				...current,
				records: current.records.map((c) =>
					String(c.id) === String(id)
						? {
								...c,
								isLiked: !wasLiked,
								likeCount: Math.max(
									0,
									(c.likeCount ?? 0) + (wasLiked ? -1 : 1),
								),
							}
						: c,
				),
			}));
			try {
				const r = await toggleCommentLike(id, wasLiked);
				setCommentData((current) => ({
					...current,
					records: current.records.map((c) =>
						String(c.id) === String(id) ? { ...c, isLiked: r.liked } : c,
					),
				}));
			} catch {
				setCommentData((current) => ({
					...current,
					records: current.records.map((c) =>
						String(c.id) === String(id)
							? {
									...c,
									isLiked: wasLiked,
									likeCount: Math.max(
										0,
										(c.likeCount ?? 0) + (wasLiked ? 1 : -1),
									),
								}
							: c,
					),
				}));
			}
		},
		[comments, setCommentData],
	);

	/* ===== Realtime SSE integration ===== */

	const handleCreated = React.useCallback(
		(payload: CommentCreatedPayload) => {
			const snap = payload.comment;
			// Self-echo of an optimistic write — replace the temp row in place.
			if (payload.clientRequestId) {
				const optimisticId = byClientRequestIdRef.current.get(
					payload.clientRequestId,
				);
				if (optimisticId != null) {
					const replacement = snapshotToComment(snap);
					setCommentData((current) => ({
						...current,
						records: current.records.map((c) =>
							c.id === optimisticId || String(c.id) === snap.commentId
								? { ...c, ...replacement }
								: c,
						),
					}));
					byClientRequestIdRef.current.delete(payload.clientRequestId);
					return;
				}
			}
			// Fallback dedup by server id (covers cross-tab post or late echoes
			// after the optimistic mapping has been cleared by Phase A above).
			setCommentData((current) => {
				if (current.records.some((c) => String(c.id) === snap.commentId)) {
					return current;
				}
				return {
					...current,
					records: insertAtTop(current.records, snapshotToComment(snap)),
					total: current.total + 1,
				};
			});
		},
		[setCommentData],
	);

	const handleReplied = React.useCallback(
		(payload: CommentCreatedPayload) => {
			const snap = payload.comment;
			const parentId = snap.parentId;
			// Same defensiveness as `handleDeleted` — numeric check tolerates "0" /
			// "" / "0000" / null-string variants from the wire instead of trusting
			// a single literal.
			const parentIdNum = Number(parentId);
			if (!Number.isFinite(parentIdNum) || parentIdNum <= 0) return;
			// Bump the parent's replyCount so the "查看 N 則回覆" affordance
			// updates instantly, even for collapsed threads.
			setCommentData((current) => ({
				...current,
				records: current.records.map((c) =>
					String(c.id) === parentId
						? { ...c, replyCount: (c.replyCount ?? 0) + 1 }
						: c,
				),
			}));
			// And forward the reply snapshot through the comment bus so the
			// matching `<CommentItem>` (parent) can splice it into its thread
			// without waiting for a manual collapse + re-expand. Bus subscribers
			// dedupe by reply id, so the optimistic-onSent path from
			// `ReplyComposer` co-exists without double-inserting.
			publishCommentEvent(snap.postId, {
				kind: "reply_arrived",
				postId: snap.postId,
				parentCommentId: parentId,
				reply: snap,
				clientRequestId: payload.clientRequestId,
			});
		},
		[setCommentData],
	);

	const handleUpdated = React.useCallback(
		(payload: CommentUpdatedPayload) => {
			setCommentData((current) => ({
				...current,
				records: current.records.map((c) =>
					String(c.id) === payload.commentId
						? { ...c, content: payload.content }
						: c,
				),
			}));
		},
		[setCommentData],
	);

	const handleDeleted = React.useCallback(
		(payload: CommentDeletedPayload) => {
			// Backend may emit any of "0" / "" / "0000" / null-as-"" for top-level
			// deletes (per CodeMeta string rendering of int64 zero). Centralise on
			// a numeric check so format drift doesn't flip the branch — `> 0` is
			// the canonical "this id refers to a real parent".
			const parentIdNum = Number(payload.parentCommentId);
			const isReply = Number.isFinite(parentIdNum) && parentIdNum > 0;
			if (isReply) {
				const parentId = payload.parentCommentId;
				setCommentData((current) => ({
					...current,
					records: current.records.map((c) =>
						String(c.id) === parentId
							? { ...c, replyCount: Math.max(0, (c.replyCount ?? 0) - 1) }
							: c,
					),
				}));
				return;
			}
			setCommentData((current) => {
				const next = current.records.filter(
					(c) => String(c.id) !== payload.commentId,
				);
				return {
					...current,
					records: next,
					total:
						next.length === current.records.length
							? current.total
							: Math.max(0, current.total - 1),
				};
			});
		},
		[setCommentData],
	);

	const handleLikeChanged = React.useCallback(
		(payload: CommentLikeChangedPayload) => {
			const isSelf = payload.likedByViewer;
			setCommentData((current) => ({
				...current,
				records: current.records.map((c) => {
					if (String(c.id) !== payload.commentId) return c;
					return {
						...c,
						likeCount: payload.likeCount,
						// Only mirror isLiked when this event was caused by the
						// current viewer — other users' actions must not flip the
						// viewer's own liked state.
						isLiked: isSelf ? payload.action === "LIKE" : c.isLiked,
					};
				}),
			}));
		},
		[setCommentData],
	);

	const handleBufferMiss = React.useCallback(() => {
		void refresh();
	}, [refresh]);

	const streamHandlers = React.useMemo(
		() => ({
			onCreated: handleCreated,
			onReplied: handleReplied,
			onUpdated: handleUpdated,
			onDeleted: handleDeleted,
			onLikeChanged: handleLikeChanged,
			onBufferMiss: handleBufferMiss,
		}),
		[
			handleCreated,
			handleReplied,
			handleUpdated,
			handleDeleted,
			handleLikeChanged,
			handleBufferMiss,
		],
	);

	const { status: streamStatus } = useCommentStream({
		postId,
		enabled: active,
		handlers: streamHandlers,
	});

	return {
		comments,
		loading: commentsQuery.isLoading,
		loadingMore,
		hasMore: comments.length < total,
		error:
			error ??
			(commentsQuery.error instanceof Error
				? commentsQuery.error.message
				: null),
		refresh,
		loadMore,
		post,
		remove,
		like,
		streamStatus,
	};
}

/* ===== Post like toggle hook ===== */

interface UsePostInteractionsResult {
	liked: boolean;
	likeCount: number;
	likeBusy: boolean;
	toggleLike: () => Promise<boolean | undefined>;
}

interface PostInteractionState {
	liked: boolean;
	likeCount: number;
}

export function usePostInteractions(
	postId: number | string | undefined,
	initial: {
		liked?: boolean;
		likeCount?: number;
	},
): UsePostInteractionsResult {
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const initialState = React.useMemo<PostInteractionState>(
		() => ({
			liked: initial.liked ?? false,
			likeCount: initial.likeCount ?? 0,
		}),
		[initial.liked, initial.likeCount],
	);
	const interactionsKey = React.useMemo(
		() => queryKeys.feed.postInteractions(queryIdentity, postId ?? ""),
		[queryIdentity, postId],
	);
	const interactionsQuery = useQuery({
		queryKey: interactionsKey,
		enabled: Boolean(postId),
		initialData: initialState,
		staleTime: Number.POSITIVE_INFINITY,
		queryFn: () => Promise.resolve(initialState),
	});
	const interaction = interactionsQuery.data;
	const togglePendingRef = React.useRef(false);
	const setInteraction = React.useCallback(
		(updater: (current: PostInteractionState) => PostInteractionState) => {
			queryClient.setQueryData<PostInteractionState>(
				interactionsKey,
				(current) => updater(current ?? initialState),
			);
		},
		[initialState, interactionsKey, queryClient],
	);

	React.useEffect(() => {
		if (!postId) return;
		queryClient.setQueryData<PostInteractionState>(
			interactionsKey,
			(current) => current ?? initialState,
		);
	}, [initialState, interactionsKey, postId, queryClient]);

	const likeMutation = useMutation({
		mutationFn: () => {
			if (!postId) return Promise.resolve({ liked: interaction.liked });
			return togglePostLike(postId);
		},
	});

	const toggleLike = React.useCallback(async () => {
		if (!postId || togglePendingRef.current) return undefined;
		togglePendingRef.current = true;
		const before = interaction;
		const nextLiked = !before.liked;
		setInteraction((current) => ({
			...current,
			liked: nextLiked,
			likeCount: Math.max(0, current.likeCount + (nextLiked ? 1 : -1)),
		}));
		try {
			const r = await likeMutation.mutateAsync();
			setInteraction((current) => ({
				...current,
				liked: r.liked,
			}));
			return r.liked;
		} catch {
			queryClient.setQueryData(interactionsKey, before);
			return before.liked;
		} finally {
			togglePendingRef.current = false;
		}
	}, [
		postId,
		interaction,
		likeMutation,
		setInteraction,
		queryClient,
		interactionsKey,
	]);

	return {
		liked: interaction.liked,
		likeCount: interaction.likeCount,
		likeBusy: likeMutation.isPending,
		toggleLike,
	};
}

/* ===== Replies (per-comment thread) ===== */

export function useCommentReplies(commentId: number | string | undefined) {
	const queryIdentity = useQueryIdentity();
	const [open, setOpen] = React.useState(false);
	const replyParams = React.useMemo(() => ({ pageSize: 20 }), []);
	const repliesKey = React.useMemo(
		() =>
			queryKeys.feed.commentReplies(
				queryIdentity,
				commentId ?? "",
				replyParams,
			),
		[queryIdentity, commentId, replyParams],
	);
	const repliesQuery = useQuery({
		queryKey: repliesKey,
		enabled: Boolean(commentId && open),
		// placeholderData（非 initialData）：理由同上方 commentsQuery —— staleTime 30s
		// 下 initialData 的空陣列會被當成新鮮資料，展開回覆時不發請求。
		placeholderData: [] as Comment[],
		staleTime: 30_000,
		queryFn: async () => {
			if (!commentId) return [];
			const r = await fetchCommentReplies(commentId, { pageSize: 20 });
			return r.records;
		},
	});
	const items = repliesQuery.data ?? [];
	const refetchReplies = repliesQuery.refetch;

	const load = React.useCallback(async () => {
		if (!commentId) return;
		await refetchReplies();
	}, [commentId, refetchReplies]);

	const expand = React.useCallback(async () => {
		setOpen(true);
		if (items.length === 0) await load();
	}, [items.length, load]);

	const collapse = React.useCallback(() => setOpen(false), []);

	return {
		items,
		open,
		loading: repliesQuery.isLoading,
		expand,
		collapse,
		refresh: load,
	};
}
