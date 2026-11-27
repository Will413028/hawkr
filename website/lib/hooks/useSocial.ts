"use client";

import {
	useInfiniteQuery,
	useMutation,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import {
	COMMENT_PAGE_SIZE,
	type Comment,
	createComment,
	deleteComment,
	fetchCommentReplies,
	fetchPostComments,
	setCommentLiked,
} from "@/lib/comments/repo";
import { fetchPostViewerState, setPostLiked } from "@/lib/posts/repo";
import {
	addReplyCount,
	appendToLastPage,
	cancelListRefetch,
	dropReplyCount,
	FIRST_PAGE_PARAM,
	findItem,
	flattenPages,
	invalidateList,
	type ListData,
	lastPageLoaded,
	mapItem,
	nextPageParam,
	removeItem,
} from "@/lib/query/infinite-list";
import { queryKeys } from "@/lib/query/keys";
import { useSubmittedItems } from "@/lib/query/submissions";
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

/** 自己剛送出、伺服器那份還不在已載入頁的項目；`key` 從送出中到成功都不變。 */
export interface SubmittedComment {
	key: string;
	comment: Comment;
}

interface UsePostCommentsResult {
	comments: Comment[];
	submitted: SubmittedComment[];
	loading: boolean;
	loadingMore: boolean;
	hasMore: boolean;
	error: unknown;
	loadMoreError: unknown;
	/** 重新讀已載入的頁：開啟時與下拉更新時呼叫（沒有即時推送）。 */
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

type CommentData = ListData<Comment>;

const COMMENTS_PARAMS = { pageSize: COMMENT_PAGE_SIZE } as const;
const REPLY_PAGE_SIZE = 20;
const REPLIES_PARAMS = { pageSize: REPLY_PAGE_SIZE } as const;
const EMPTY_COMMENTS: Comment[] = [];

interface Submission {
	content: string;
	/** UUIDv4：同一次送出沿用同一個，伺服器回同一則留言而不重複建立。 */
	requestId: string;
	authorName: string;
	authorAvatar?: string;
}

interface ReplySubmission extends Submission {
	replyToId: string;
	replyToName?: string;
}

interface SubmitContext {
	optimisticId?: string;
}

function newRequestId(): string {
	return crypto.randomUUID();
}

function optimisticComment(
	postId: string,
	v: Submission,
	createdAt: number,
	extra: Partial<Comment> = {},
): Comment {
	return {
		id: `tmp-${v.requestId}`,
		postId,
		isMine: true,
		userName: v.authorName,
		avatarKey: v.authorAvatar,
		content: v.content,
		likeCount: 0,
		replyCount: 0,
		isLiked: false,
		createdAt,
		state: "visible",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: false,
		optimistic: true,
		...extra,
	};
}

export function usePostComments(
	postId: string | undefined,
	options: UsePostCommentsOptions = {},
): UsePostCommentsResult {
	const active = options.active ?? true;
	const { identity } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const commentsKey = React.useMemo(
		() => queryKeys.feed.comments(queryIdentity, postId ?? "", COMMENTS_PARAMS),
		[queryIdentity, postId],
	);
	const createKey = React.useMemo(
		() => queryKeys.feed.commentCreate(queryIdentity, postId ?? ""),
		[queryIdentity, postId],
	);
	const query = useInfiniteQuery({
		queryKey: commentsKey,
		enabled: Boolean(postId && active),
		initialPageParam: FIRST_PAGE_PARAM,
		getNextPageParam: nextPageParam,
		staleTime: 30_000,
		refetchOnMount: "always",
		queryFn: ({ pageParam, signal }) =>
			fetchPostComments(postId ?? "", {
				pageToken: pageParam,
				pageSize: COMMENT_PAGE_SIZE,
				signal,
			}),
	});
	const comments = React.useMemo(
		() => (query.data ? flattenPages(query.data) : EMPTY_COMMENTS),
		[query.data],
	);
	const { refetch, fetchNextPage, hasNextPage, isFetchingNextPage } = query;

	const setCommentData = React.useCallback(
		(updater: (current: CommentData) => CommentData) => {
			queryClient.setQueryData<CommentData>(commentsKey, (current) =>
				current ? updater(current) : current,
			);
		},
		[commentsKey, queryClient],
	);

	const refresh = React.useCallback(async () => {
		if (!postId) return;
		await refetch();
	}, [postId, refetch]);

	const loadMore = React.useCallback(async () => {
		if (!postId || !hasNextPage || isFetchingNextPage) return;
		await fetchNextPage({ cancelRefetch: false });
	}, [postId, hasNextPage, isFetchingNextPage, fetchNextPage]);

	const createMutation = useMutation<Comment, Error, Submission, SubmitContext>(
		{
			mutationKey: createKey,
			mutationFn: (v) => {
				if (!postId) throw new Error("postId missing");
				return createComment(postId, {
					content: v.content,
					requestId: v.requestId,
				});
			},
			onMutate: async (v) => {
				await cancelListRefetch(queryClient, commentsKey);
				const current = queryClient.getQueryData<CommentData>(commentsKey);
				if (!current || !postId || !lastPageLoaded(current)) return {};
				const optimistic = optimisticComment(
					postId,
					v,
					Math.floor(Date.now() / 1000),
				);
				queryClient.setQueryData<CommentData>(
					commentsKey,
					appendToLastPage(current, optimistic),
				);
				return { optimisticId: optimistic.id };
			},
			onSuccess: (real, _v, context) => {
				const id = context?.optimisticId;
				if (id) setCommentData((current) => mapItem(current, id, () => real));
			},
			onError: (_error, _v, context) => {
				const id = context?.optimisticId;
				if (id) setCommentData((current) => removeItem(current, id));
			},
			onSettled: () => {
				void invalidateList(queryClient, commentsKey);
			},
		},
	);

	const buildSubmitted = React.useCallback(
		(v: Submission, createdAt: number) =>
			optimisticComment(postId ?? "", v, createdAt),
		[postId],
	);
	const submittedItems = useSubmittedItems<Submission, Comment>({
		mutationKey: createKey,
		loaded: comments,
		build: buildSubmitted,
	});
	const submitted = React.useMemo(
		() => submittedItems.map((s) => ({ key: s.key, comment: s.item })),
		[submittedItems],
	);

	const { mutateAsync: createAsync } = createMutation;
	const post = React.useCallback(
		async (content: string, opts?: PostCommentOptions) => {
			const body = content.trim();
			if (!postId || !body) return;
			await createAsync({
				content: body,
				requestId: newRequestId(),
				authorName: opts?.authorName ?? identity?.nickname ?? "我",
				authorAvatar: opts?.authorAvatar ?? identity?.avatarKey,
			});
		},
		[postId, identity?.nickname, identity?.avatarKey, createAsync],
	);

	const deleteMutation = useMutation({
		mutationFn: (v: { id: string; etag?: string }) =>
			deleteComment(v.id, v.etag),
		onSuccess: (_data, v) => {
			setCommentData((current) => removeItem(current, v.id));
		},
		onSettled: () => {
			void invalidateList(queryClient, commentsKey);
		},
	});
	const { mutateAsync: deleteAsync } = deleteMutation;
	const remove = React.useCallback(
		async (id: string) => {
			const target = comments.find((c) => c.id === id);
			await deleteAsync({ id, etag: target?.etag });
		},
		[comments, deleteAsync],
	);

	/** 修改成功後伺服器換了新版本：快取裡的 etag 與內文同步，之後的刪除才帶得對。 */
	const edited = React.useCallback(
		(updated: Pick<Comment, "id" | "content" | "etag">) =>
			setCommentData((current) =>
				mapItem(current, updated.id, (c) => ({
					...c,
					content: updated.content || c.content,
					etag: updated.etag,
				})),
			),
		[setCommentData],
	);

	const like = React.useCallback(
		async (id: string) => {
			const target = comments.find((c) => c.id === id);
			const wasLiked = target?.isLiked === true;
			const apply = (liked: boolean, delta: number) =>
				setCommentData((current) =>
					mapItem(current, id, (c) => ({
						...c,
						isLiked: liked,
						likeCount: Math.max(0, (c.likeCount ?? 0) + delta),
					})),
				);
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
		submitted,
		loading: query.isLoading,
		loadingMore: isFetchingNextPage,
		hasMore: hasNextPage,
		error: query.isFetchNextPageError ? null : query.error,
		loadMoreError: query.isFetchNextPageError ? query.error : null,
		refresh,
		loadMore,
		post,
		remove,
		edited,
		like,
	};
}


/**
 * 觀看者在這則貼文的留言權限與身分，由伺服器依觀看者算。貼文已經帶著（依觀看者讀的）答案時呼叫端傳
 * `enabled: false`，不重讀。
 */
export function usePostViewerState(
	postId: string | undefined,
	enabled: boolean,
) {
	const queryIdentity = useQueryIdentity();
	return useQuery({
		queryKey: queryKeys.feed.postViewer(queryIdentity, postId ?? ""),
		enabled: Boolean(postId) && enabled,
		staleTime: 30_000,
		queryFn: ({ signal }) => fetchPostViewerState(postId ?? "", signal),
	});
}


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
		onMutate: async (nextLiked) => {
			await queryClient.cancelQueries({ queryKey: interactionsKey });
			setInteraction((current) => ({
				...current,
				liked: nextLiked,
				likeCount: Math.max(0, current.likeCount + (nextLiked ? 1 : -1)),
			}));
		},
		onSuccess: (r) => {
			setInteraction((current) => ({ ...current, liked: r.liked }));
		},
		onError: (_error, nextLiked) => {
			setInteraction((current) => ({
				...current,
				liked: !nextLiked,
				likeCount: Math.max(0, current.likeCount + (nextLiked ? -1 : 1)),
			}));
		},
	});
	const { mutateAsync: likeAsync } = likeMutation;

	const toggleLike = React.useCallback(async () => {
		if (!postId || togglePendingRef.current) return undefined;
		togglePendingRef.current = true;
		const nextLiked = !interaction.liked;
		try {
			const r = await likeAsync(nextLiked);
			return r.liked;
		} catch {
			return !nextLiked;
		} finally {
			togglePendingRef.current = false;
		}
	}, [postId, interaction.liked, likeAsync]);

	return {
		liked: interaction.liked,
		likeCount: interaction.likeCount,
		likeBusy: likeMutation.isPending,
		toggleLike,
	};
}


interface ReplyContext extends SubmitContext {
	countBumped?: boolean;
}

export function useCommentReplies(
	postId: string | undefined,
	commentId: string | undefined,
	parentName?: string,
) {
	const { identity } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const [open, setOpen] = React.useState(false);
	const repliesKey = React.useMemo(
		() =>
			queryKeys.feed.commentReplies(
				queryIdentity,
				commentId ?? "",
				REPLIES_PARAMS,
			),
		[queryIdentity, commentId],
	);
	const commentsKey = React.useMemo(
		() => queryKeys.feed.comments(queryIdentity, postId ?? "", COMMENTS_PARAMS),
		[queryIdentity, postId],
	);
	const createKey = React.useMemo(
		() => queryKeys.feed.replyCreate(queryIdentity, commentId ?? ""),
		[queryIdentity, commentId],
	);
	const query = useInfiniteQuery({
		queryKey: repliesKey,
		enabled: Boolean(postId && commentId && open),
		initialPageParam: FIRST_PAGE_PARAM,
		getNextPageParam: nextPageParam,
		staleTime: 30_000,
		queryFn: ({ pageParam, signal }) =>
			fetchCommentReplies(postId ?? "", commentId ?? "", {
				pageSize: REPLY_PAGE_SIZE,
				pageToken: pageParam,
				parentName,
				signal,
			}),
	});
	const items = React.useMemo(
		() => (query.data ? flattenPages(query.data) : EMPTY_COMMENTS),
		[query.data],
	);
	const { refetch, fetchNextPage, hasNextPage, isFetchingNextPage } = query;

	const setReplyData = React.useCallback(
		(updater: (current: CommentData) => CommentData) => {
			queryClient.setQueryData<CommentData>(repliesKey, (current) =>
				current ? updater(current) : current,
			);
		},
		[repliesKey, queryClient],
	);
	const setCommentData = React.useCallback(
		(updater: (current: CommentData) => CommentData) => {
			queryClient.setQueryData<CommentData>(commentsKey, (current) =>
				current ? updater(current) : current,
			);
		},
		[commentsKey, queryClient],
	);

	const loadMore = React.useCallback(async () => {
		if (!hasNextPage || isFetchingNextPage) return;
		await fetchNextPage({ cancelRefetch: false });
	}, [hasNextPage, isFetchingNextPage, fetchNextPage]);

	const load = React.useCallback(async () => {
		if (!postId || !commentId) return;
		await refetch();
	}, [postId, commentId, refetch]);

	const expand = React.useCallback(() => setOpen(true), []);
	const collapse = React.useCallback(() => setOpen(false), []);

	const replyMutation = useMutation<
		Comment,
		Error,
		ReplySubmission,
		ReplyContext
	>({
		mutationKey: createKey,
		mutationFn: (v) => {
			if (!postId || !commentId) throw new Error("commentId missing");
			return createComment(postId, {
				content: v.content,
				parentId: commentId,
				replyToId: v.replyToId,
				requestId: v.requestId,
			});
		},
		onMutate: async (v) => {
			await Promise.all([
				cancelListRefetch(queryClient, repliesKey),
				cancelListRefetch(queryClient, commentsKey),
			]);
			const context: ReplyContext = {};
			const parent = queryClient.getQueryData<CommentData>(commentsKey);
			if (parent && findItem(parent, commentId ?? "")) {
				setCommentData((current) => addReplyCount(current, commentId ?? "", 1));
				context.countBumped = true;
			}
			const current = queryClient.getQueryData<CommentData>(repliesKey);
			if (current && postId && lastPageLoaded(current)) {
				const optimistic = optimisticComment(
					postId,
					v,
					Math.floor(Date.now() / 1000),
					{
						parentId: commentId,
						replyToId: v.replyToId,
						replyToName: v.replyToName ?? parentName,
					},
				);
				queryClient.setQueryData<CommentData>(
					repliesKey,
					appendToLastPage(current, optimistic),
				);
				context.optimisticId = optimistic.id;
			}
			return context;
		},
		onSuccess: (real, v, context) => {
			const id = context?.optimisticId;
			if (id) {
				setReplyData((current) =>
					mapItem(current, id, () => ({
						...real,
						replyToName: real.replyToName ?? v.replyToName ?? parentName,
					})),
				);
			}
		},
		onError: (_error, _v, context) => {
			const id = context?.optimisticId;
			if (id) setReplyData((current) => removeItem(current, id));
			if (context?.countBumped) {
				setCommentData((current) =>
					addReplyCount(current, commentId ?? "", -1),
				);
			}
		},
		onSettled: () => {
			void invalidateList(queryClient, repliesKey);
		},
	});
	const { mutateAsync: replyAsync } = replyMutation;

	const reply = React.useCallback(
		async (
			content: string,
			opts: {
				replyToId: string;
				replyToName?: string;
				authorName?: string;
				authorAvatar?: string;
			},
		) => {
			const body = content.trim();
			if (!postId || !commentId || !body) return;
			await replyAsync({
				content: body,
				requestId: newRequestId(),
				replyToId: opts.replyToId,
				replyToName: opts.replyToName,
				authorName: opts.authorName ?? identity?.nickname ?? "我",
				authorAvatar: opts.authorAvatar ?? identity?.avatarKey,
			});
		},
		[postId, commentId, identity?.nickname, identity?.avatarKey, replyAsync],
	);

	const buildSubmitted = React.useCallback(
		(v: ReplySubmission, createdAt: number) =>
			optimisticComment(postId ?? "", v, createdAt, {
				parentId: commentId,
				replyToId: v.replyToId,
				replyToName: v.replyToName ?? parentName,
			}),
		[postId, commentId, parentName],
	);
	const landedReply = React.useCallback(
		(data: Comment, v: ReplySubmission): Comment => ({
			...data,
			replyToName: data.replyToName ?? v.replyToName ?? parentName,
		}),
		[parentName],
	);
	const submittedItems = useSubmittedItems<ReplySubmission, Comment>({
		mutationKey: createKey,
		loaded: items,
		build: buildSubmitted,
		landed: landedReply,
	});
	const submitted = React.useMemo(
		() => submittedItems.map((s) => ({ key: s.key, comment: s.item })),
		[submittedItems],
	);

	/** 刪除一則回覆成功後：從快取移除並減少所屬留言的回覆數。 */
	const removed = React.useCallback(
		(replyId: string) => {
			setReplyData((current) => removeItem(current, replyId));
			setCommentData((current) => dropReplyCount(current, commentId ?? ""));
			void invalidateList(queryClient, repliesKey);
		},
		[setReplyData, setCommentData, commentId, queryClient, repliesKey],
	);

	return {
		items,
		submitted,
		open,
		loading: query.isLoading,
		hasMore: hasNextPage,
		loadingMore: isFetchingNextPage,
		loadMoreError: query.isFetchNextPageError ? query.error : null,
		loadMore,
		expand,
		collapse,
		refresh: load,
		reply,
		removed,
	};
}
