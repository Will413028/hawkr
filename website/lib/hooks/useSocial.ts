"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import {
	COMMENT_PAGE_SIZE,
	type Comment,
	type CommentListResult,
	createComment,
	deleteComment,
	fetchCommentReplies,
	fetchPostComments,
	setCommentLiked,
} from "@/lib/comments/repo";
import { setPostLiked } from "@/lib/posts/repo";
import { queryKeys } from "@/lib/query/keys";
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
	/** 重新讀第一頁：開啟時與下拉更新時呼叫（沒有即時推送）。 */
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
	post: (content: string, opts?: PostCommentOptions) => Promise<void>;
	remove: (id: string) => Promise<void>;
	/** 單則留言修改成功後，把新內文與新 etag 寫回快取。 */
	edited: (updated: Pick<Comment, "id" | "content" | "etag">) => void;
	like: (id: string) => Promise<void>;
}

interface UsePostCommentsOptions {
	/** 是否載入這則貼文的留言。PostDetailViewer 把每一頁都掛進 snap pager，這個旗標讓只有目前那一頁讀。 */
	active?: boolean;
}

function emptyCommentList(pageSize = COMMENT_PAGE_SIZE): CommentListResult {
	return { records: [], nextPageToken: "", pageSize };
}

function newRequestId(): string {
	return crypto.randomUUID();
}

export function usePostComments(
	postId: string | undefined,
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
		// 「剛抓到的新鮮資料」而不在掛載時請求，導致空清單卡 30 秒。
		placeholderData: emptyCommentList(),
		staleTime: 30_000,
		// 沒有即時推送：每次開啟（掛載）都重新讀，不沿用上次的列表。
		refetchOnMount: "always",
		queryFn: ({ signal }) =>
			fetchPostComments(postId ?? "", { pageSize: COMMENT_PAGE_SIZE, signal }),
	});
	const [loadingMore, setLoadingMore] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const data = commentsQuery.data ?? emptyCommentList();
	const comments = data.records;
	const nextPageToken = data.nextPageToken;
	const refetchComments = commentsQuery.refetch;

	const setCommentData = React.useCallback(
		(updater: (current: CommentListResult) => CommentListResult) => {
			queryClient.setQueryData<CommentListResult>(commentsKey, (current) =>
				updater(current ?? emptyCommentList()),
			);
		},
		[commentsKey, queryClient],
	);

	const refresh = React.useCallback(async () => {
		if (!postId) return;
		setError(null);
		await refetchComments();
	}, [postId, refetchComments]);

	const loadMore = React.useCallback(async () => {
		if (!postId || loadingMore || nextPageToken === "") return;
		setLoadingMore(true);
		try {
			const r = await fetchPostComments(postId, {
				pageToken: nextPageToken,
				pageSize: COMMENT_PAGE_SIZE,
			});
			setCommentData((current) => ({
				...r,
				records: [...current.records, ...r.records],
			}));
		} catch (e) {
			setError(errorMessage(e, "載入更多失敗"));
		} finally {
			setLoadingMore(false);
		}
	}, [postId, loadingMore, nextPageToken, setCommentData]);

	const post = React.useCallback(
		async (content: string, opts?: PostCommentOptions) => {
			if (!postId || !content.trim()) return;
			// 一次送出的重試沿用同一個 request_id（UUIDv4），伺服器回同一則留言而不重複建立。
			const requestId = newRequestId();
			const optimistic: Comment = {
				// 暫時 id：字串前綴避免與真實 ID 撞號。
				id: `tmp-${requestId}`,
				postId,
				isMine: true,
				// 賣家面以店家身分留言：optimistic 先顯示傳入的店家名／頭像，避免閃一下買家暱稱。
				userName: opts?.authorName ?? identity?.nickname ?? "我",
				avatarKey: opts?.authorAvatar ?? identity?.avatarKey,
				content: content.trim(),
				likeCount: 0,
				replyCount: 0,
				isLiked: false,
				createdAt: Math.floor(Date.now() / 1000),
				state: "visible",
				viewerCanEdit: false,
				viewerCanDelete: false,
				viewerCanReply: false,
			};
			// 留言依建立時間舊到新排：新留言接在已載入的最後。
			setCommentData((current) => ({
				...current,
				records: [...current.records, optimistic],
			}));
			try {
				const real = await createComment(postId, {
					content: content.trim(),
					requestId,
				});
				setCommentData((current) => ({
					...current,
					records: current.records.map((c) =>
						c.id === optimistic.id ? real : c,
					),
				}));
			} catch (e) {
				setCommentData((current) => ({
					...current,
					records: current.records.filter((c) => c.id !== optimistic.id),
				}));
				setError(errorMessage(e, "送出留言失敗"));
				throw e;
			}
		},
		[postId, identity?.nickname, identity?.avatarKey, setCommentData],
	);

	const remove = React.useCallback(
		async (id: string) => {
			try {
				const target = comments.find((c) => c.id === id);
				await deleteComment(id, target?.etag);
				setCommentData((current) => ({
					...current,
					records: current.records.filter((c) => c.id !== id),
				}));
			} catch (e) {
				setError(errorMessage(e, "刪除失敗"));
			}
		},
		[comments, setCommentData],
	);

	/** 修改成功後伺服器換了新版本：快取裡的 etag 與內文同步，之後的刪除才帶得對。 */
	const edited = React.useCallback(
		(updated: Pick<Comment, "id" | "content" | "etag">) =>
			setCommentData((current) => ({
				...current,
				records: current.records.map((c) =>
					c.id === updated.id
						? {
								...c,
								content: updated.content || c.content,
								etag: updated.etag,
							}
						: c,
				),
			})),
		[setCommentData],
	);

	const like = React.useCallback(
		async (id: string) => {
			const target = comments.find((c) => c.id === id);
			const wasLiked = target?.isLiked === true;
			const apply = (liked: boolean, delta: number) =>
				setCommentData((current) => ({
					...current,
					records: current.records.map((c) =>
						c.id === id
							? {
									...c,
									isLiked: liked,
									likeCount: Math.max(0, (c.likeCount ?? 0) + delta),
								}
							: c,
					),
				}));
			apply(!wasLiked, wasLiked ? -1 : 1);
			try {
				await setCommentLiked(id, !wasLiked);
			} catch {
				apply(wasLiked, wasLiked ? 1 : -1);
			}
		},
		[comments, setCommentData],
	);

	return {
		comments,
		loading: commentsQuery.isLoading,
		loadingMore,
		hasMore: nextPageToken !== "",
		error:
			error ??
			(commentsQuery.error instanceof Error
				? commentsQuery.error.message
				: null),
		refresh,
		loadMore,
		post,
		remove,
		edited,
		like,
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
	postId: string | undefined,
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
		mutationFn: (nextLiked: boolean) => {
			if (!postId) return Promise.resolve({ liked: interaction.liked });
			return setPostLiked(postId, nextLiked);
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
			const r = await likeMutation.mutateAsync(nextLiked);
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

export function useCommentReplies(
	postId: string | undefined,
	commentId: string | undefined,
	parentName?: string,
) {
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
		enabled: Boolean(postId && commentId && open),
		// placeholderData（非 initialData）：理由同 usePostComments。
		placeholderData: [] as Comment[],
		staleTime: 30_000,
		queryFn: async ({ signal }) => {
			if (!postId || !commentId) return [];
			const r = await fetchCommentReplies(postId, commentId, {
				pageSize: 20,
				parentName,
				signal,
			});
			return r.records;
		},
	});
	const items = React.useMemo(
		() => repliesQuery.data ?? [],
		[repliesQuery.data],
	);
	const refetchReplies = repliesQuery.refetch;

	const load = React.useCallback(async () => {
		if (!postId || !commentId) return;
		await refetchReplies();
	}, [postId, commentId, refetchReplies]);

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
