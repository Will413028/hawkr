"use client";

import {
	useInfiniteQuery,
	useMutation,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import {
	createSellerForumComment,
	deleteSellerForumComment,
	fetchSellerForum,
	fetchSellerForumReplies,
	fetchSellerForumThread,
	replySellerForumComment,
	type SellerForumComment,
	type SellerForumListResult,
	updateSellerForumComment,
} from "@/lib/forum/repo";
import {
	addReplyCount,
	appendToLastPage,
	cancelListRefetch,
	dropReplyCount,
	FIRST_PAGE_PARAM,
	findItem,
	flattenPages,
	type ItemLocation,
	invalidateList,
	type ListData,
	lastPageLoaded,
	locateItem,
	mapItem,
	nextPageParam,
	removeItem,
	restoreItem,
	seedPages,
} from "@/lib/query/infinite-list";
import { queryKeys } from "@/lib/query/keys";
import { useSubmittedItems } from "@/lib/query/submissions";
import { useQueryIdentity } from "./useQueryIdentity";

/** 自己剛送出、伺服器那份還不在已載入頁的項目；`key` 從送出中到成功都不變。 */
export interface SubmittedEntry {
	key: string;
	entry: SellerForumComment;
}

interface UseSellerForumResult {
	comments: SellerForumComment[];
	submitted: SubmittedEntry[];
	loading: boolean;
	refreshing: boolean;
	loadingMore: boolean;
	hasMore: boolean;
	error: unknown;
	loadMoreError: unknown;
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
	post: (content: string) => Promise<void>;
	/** `target` 是被回覆的那一則（頂層留言或某則回覆）。 */
	reply: (target: SellerForumComment, content: string) => Promise<void>;
	update: (entry: SellerForumComment, content: string) => Promise<void>;
	remove: (entry: SellerForumComment) => Promise<void>;
}

type ForumData = ListData<SellerForumComment>;

const REPLY_PAGE_SIZE = 20;
const REPLIES_PARAMS = { pageSize: REPLY_PAGE_SIZE } as const;
const EMPTY_ENTRIES: SellerForumComment[] = [];

interface Submission {
	content: string;
	/** UUIDv4：同一次送出沿用同一個，伺服器回同一則發言而不重複建立。 */
	requestId: string;
	authorName: string;
	authorAvatar?: string;
}

interface ReplySubmission extends Submission {
	threadId: string;
	replyToId: string;
	replyToName?: string;
}

interface SubmitContext {
	optimisticId?: string;
	countBumped?: boolean;
}

interface RemoveContext {
	key?: readonly unknown[];
	row?: SellerForumComment;
	at?: ItemLocation;
	parent?: { row: SellerForumComment; at: ItemLocation };
}

function optimisticEntry(
	storeId: string,
	v: Submission,
	createdAt: number,
	extra: Partial<SellerForumComment> = {},
): SellerForumComment {
	return {
		id: `tmp-${v.requestId}`,
		storeId,
		isMine: true,
		userName: v.authorName,
		avatarKey: v.authorAvatar,
		content: v.content,
		replyCount: 0,
		createdAt,
		state: "visible",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: false,
		optimistic: true,
		...extra,
	};
}

function replyExtra(v: ReplySubmission): Partial<SellerForumComment> {
	return {
		parentId: v.threadId,
		replyToId: v.replyToId,
		replyToName: v.replyToName,
	};
}

/**
 * 刪除的頂層留言底下還有回覆時，伺服器留下佔位項目（state 為 DELETED，沒有作者與內文）；
 * 沒有回覆的才消失（ListForumEntries 的契約）。
 */
function placeholderOf(target: SellerForumComment): SellerForumComment {
	return {
		id: target.id,
		storeId: target.storeId,
		isMine: false,
		userName: "",
		content: "",
		createdAt: target.createdAt,
		replyCount: target.replyCount,
		state: "deleted",
		viewerCanEdit: false,
		viewerCanDelete: false,
		viewerCanReply: false,
	};
}

/**
 * `threadId` 設了是單一討論串：主列表只有該串的頂層項目（`GetForumThread`），其餘的回覆、修改、刪除照同一套邏輯對這份資料做；
 * 不設是店家討論區的頂層列表。
 */
export function useSellerForum(
	sellerId: string | undefined,
	initialData?: SellerForumListResult,
	threadId?: string,
): UseSellerForumResult {
	const { identity } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const storeId = String(sellerId ?? "");
	const pageSize = initialData?.pageSize ?? 20;
	const forumParams = React.useMemo(() => ({ pageSize }), [pageSize]);
	const forumKey = React.useMemo(
		() =>
			threadId
				? queryKeys.seller.forumThread(queryIdentity, storeId, threadId)
				: queryKeys.seller.forum(queryIdentity, storeId, forumParams),
		[queryIdentity, storeId, forumParams, threadId],
	);
	const postKey = React.useMemo(
		() => queryKeys.seller.forumPostCreate(queryIdentity, storeId),
		[queryIdentity, storeId],
	);
	const replyKey = React.useMemo(
		() => queryKeys.seller.forumReplyCreate(queryIdentity, storeId),
		[queryIdentity, storeId],
	);
	const repliesKeyOf = React.useCallback(
		(threadId: string) =>
			queryKeys.seller.forumReplies(
				queryIdentity,
				storeId,
				threadId,
				REPLIES_PARAMS,
			),
		[queryIdentity, storeId],
	);
	const seed = React.useMemo(
		() => (initialData ? seedPages(initialData) : undefined),
		[initialData],
	);
	const query = useInfiniteQuery({
		queryKey: forumKey,
		enabled: Boolean(sellerId),
		initialPageParam: FIRST_PAGE_PARAM,
		getNextPageParam: nextPageParam,
		// SSR 帶入的真資料才走 initialData；缺席時沒有資料，請求完成前是 loading。
		initialData: seed,
		queryFn: ({ pageParam, signal }) =>
			threadId
				? fetchSellerForumThread(storeId, threadId, {
						pageSize,
						signal,
						viewer: true,
					}).then((thread) => thread.top)
				: fetchSellerForum(storeId, {
						pageToken: pageParam,
						pageSize,
						signal,
						viewer: true,
					}),
	});
	const comments = React.useMemo(
		() => (query.data ? flattenPages(query.data) : EMPTY_ENTRIES),
		[query.data],
	);
	const { refetch, fetchNextPage, hasNextPage, isFetchingNextPage } = query;
	const refreshPromiseRef = React.useRef<Promise<void> | null>(null);

	const setForumData = React.useCallback(
		(updater: (current: ForumData) => ForumData) => {
			queryClient.setQueryData<ForumData>(forumKey, (current) =>
				current ? updater(current) : current,
			);
		},
		[forumKey, queryClient],
	);
	const setKeyData = React.useCallback(
		(key: readonly unknown[], updater: (current: ForumData) => ForumData) => {
			queryClient.setQueryData<ForumData>(key, (current) =>
				current ? updater(current) : current,
			);
		},
		[queryClient],
	);

	const refresh = React.useCallback(() => {
		if (!sellerId) return Promise.resolve();
		if (refreshPromiseRef.current) return refreshPromiseRef.current;
		const pending = refetch()
			.then(() => undefined)
			.finally(() => {
				if (refreshPromiseRef.current === pending) {
					refreshPromiseRef.current = null;
				}
			});
		refreshPromiseRef.current = pending;
		return pending;
	}, [sellerId, refetch]);

	const loadMore = React.useCallback(async () => {
		if (!sellerId || !hasNextPage || isFetchingNextPage) return;
		await fetchNextPage({ cancelRefetch: false });
	}, [sellerId, hasNextPage, isFetchingNextPage, fetchNextPage]);

	const postMutation = useMutation<
		SellerForumComment,
		Error,
		Submission,
		SubmitContext
	>({
		mutationKey: postKey,
		mutationFn: (v) =>
			createSellerForumComment(storeId, v.content, v.requestId),
		onMutate: async (v) => {
			await cancelListRefetch(queryClient, forumKey);
			const current = queryClient.getQueryData<ForumData>(forumKey);
			// 伺服器把新發言放在最後：最後一頁還沒載入時它不在已載入的資料裡。
			if (!current || !lastPageLoaded(current)) return {};
			const optimistic = optimisticEntry(
				storeId,
				v,
				Math.floor(Date.now() / 1000),
			);
			queryClient.setQueryData<ForumData>(
				forumKey,
				appendToLastPage(current, optimistic),
			);
			return { optimisticId: optimistic.id };
		},
		onSuccess: (real, _v, context) => {
			const id = context?.optimisticId;
			if (id) setForumData((current) => mapItem(current, id, () => real));
		},
		onError: (_error, _v, context) => {
			const id = context?.optimisticId;
			if (id) setForumData((current) => removeItem(current, id));
		},
		onSettled: () => {
			void invalidateList(queryClient, forumKey);
		},
	});
	const { mutateAsync: postAsync } = postMutation;

	const replyMutation = useMutation<
		SellerForumComment,
		Error,
		ReplySubmission,
		SubmitContext
	>({
		mutationKey: replyKey,
		mutationFn: (v) =>
			replySellerForumComment(
				storeId,
				v.threadId,
				v.replyToId,
				v.content,
				v.requestId,
			),
		onMutate: async (v) => {
			const repliesKey = repliesKeyOf(v.threadId);
			await Promise.all([
				cancelListRefetch(queryClient, repliesKey),
				cancelListRefetch(queryClient, forumKey),
			]);
			const context: SubmitContext = {};
			const forum = queryClient.getQueryData<ForumData>(forumKey);
			if (forum && findItem(forum, v.threadId)) {
				setForumData((current) => addReplyCount(current, v.threadId, 1));
				context.countBumped = true;
			}
			const current = queryClient.getQueryData<ForumData>(repliesKey);
			if (current && lastPageLoaded(current)) {
				const optimistic = optimisticEntry(
					storeId,
					v,
					Math.floor(Date.now() / 1000),
					replyExtra(v),
				);
				queryClient.setQueryData<ForumData>(
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
				setKeyData(repliesKeyOf(v.threadId), (current) =>
					mapItem(current, id, () => ({
						...real,
						replyToName: v.replyToName,
					})),
				);
			}
		},
		onError: (_error, v, context) => {
			const id = context?.optimisticId;
			if (id) {
				setKeyData(repliesKeyOf(v.threadId), (current) =>
					removeItem(current, id),
				);
			}
			if (context?.countBumped) {
				setForumData((current) => addReplyCount(current, v.threadId, -1));
			}
		},
		onSettled: (_data, _error, v) => {
			void invalidateList(queryClient, repliesKeyOf(v.threadId));
		},
	});
	const { mutateAsync: replyAsync } = replyMutation;

	const author = React.useCallback(
		() => ({
			authorName: identity?.nickname ?? "我",
			authorAvatar: identity?.avatarKey,
		}),
		[identity?.nickname, identity?.avatarKey],
	);

	const post = React.useCallback(
		async (content: string) => {
			const body = content.trim();
			if (!sellerId || !body) return;
			await postAsync({
				content: body,
				requestId: crypto.randomUUID(),
				...author(),
			});
		},
		[sellerId, postAsync, author],
	);

	const reply = React.useCallback(
		async (target: SellerForumComment, content: string) => {
			const body = content.trim();
			if (!sellerId || !body) return;
			// 伺服器的討論區只有兩層：parent_id 是頂層留言，reply_to_id 是針對的那一則。
			await replyAsync({
				content: body,
				requestId: crypto.randomUUID(),
				threadId: target.parentId ?? target.id,
				replyToId: target.id,
				replyToName: target.userName,
				...author(),
			});
		},
		[sellerId, replyAsync, author],
	);

	const buildPost = React.useCallback(
		(v: Submission, createdAt: number) =>
			optimisticEntry(storeId, v, createdAt),
		[storeId],
	);
	const submittedItems = useSubmittedItems<Submission, SellerForumComment>({
		mutationKey: postKey,
		loaded: comments,
		build: buildPost,
	});
	const submittedPosts = React.useMemo(
		() => submittedItems.map((s) => ({ key: s.key, entry: s.item })),
		[submittedItems],
	);

	const updateMutation = useMutation<
		SellerForumComment,
		Error,
		{ id: string; parentId?: string; content: string; etag: string },
		{ key: readonly unknown[]; previous?: string }
	>({
		mutationFn: (v) => updateSellerForumComment(v.id, v.content, v.etag),
		onMutate: async (v) => {
			const key = v.parentId ? repliesKeyOf(v.parentId) : forumKey;
			await queryClient.cancelQueries({ queryKey: key });
			const previous = findItem(
				queryClient.getQueryData<ForumData>(key),
				v.id,
			)?.content;
			setKeyData(key, (current) =>
				mapItem(current, v.id, (item) => ({ ...item, content: v.content })),
			);
			return { key, previous };
		},
		onSuccess: (real, v, context) => {
			if (!context) return;
			setKeyData(context.key, (current) =>
				mapItem(current, v.id, (item) => ({
					...item,
					...real,
					replyToName: item.replyToName,
				})),
			);
		},
		onError: (_error, v, context) => {
			if (!context || context.previous === undefined) return;
			const previous = context.previous;
			setKeyData(context.key, (current) =>
				mapItem(current, v.id, (item) =>
					item.content === v.content ? { ...item, content: previous } : item,
				),
			);
		},
		onSettled: (_data, _error, _v, context) => {
			if (context) {
				void invalidateList(queryClient, context.key);
			}
		},
	});
	const { mutateAsync: updateAsync } = updateMutation;

	const update = React.useCallback(
		async (entry: SellerForumComment, content: string) => {
			const body = content.trim();
			if (!body) return;
			const key = entry.parentId ? repliesKeyOf(entry.parentId) : forumKey;
			const etag =
				findItem(queryClient.getQueryData<ForumData>(key), entry.id)?.etag ??
				entry.etag ??
				"";
			await updateAsync({
				id: entry.id,
				parentId: entry.parentId ?? undefined,
				content: body,
				etag,
			});
		},
		[forumKey, repliesKeyOf, queryClient, updateAsync],
	);

	const removeMutation = useMutation<
		void,
		Error,
		{ id: string; parentId?: string; etag?: string },
		RemoveContext
	>({
		mutationFn: (v) => deleteSellerForumComment(v.id, v.etag),
		onMutate: async (v) => {
			const key = v.parentId ? repliesKeyOf(v.parentId) : forumKey;
			await Promise.all([
				cancelListRefetch(queryClient, key),
				cancelListRefetch(queryClient, forumKey),
			]);
			const data = queryClient.getQueryData<ForumData>(key);
			const row = findItem(data, v.id);
			const at = locateItem(data, v.id);
			if (!row || !at) return {};
			const context: RemoveContext = { key, row, at };
			if (!v.parentId) {
				setForumData((current) =>
					(row.replyCount ?? 0) > 0
						? mapItem(current, v.id, placeholderOf)
						: removeItem(current, v.id),
				);
				return context;
			}
			const forum = queryClient.getQueryData<ForumData>(forumKey);
			const parentRow = findItem(forum, v.parentId);
			const parentAt = locateItem(forum, v.parentId);
			if (parentRow && parentAt)
				context.parent = { row: parentRow, at: parentAt };
			setKeyData(key, (current) => removeItem(current, v.id));
			setForumData((current) => dropReplyCount(current, v.parentId ?? ""));
			return context;
		},
		onError: (_error, v, context) => {
			if (!context?.key || !context.row || !context.at) return;
			const { row, at } = context;
			setKeyData(context.key, (current) => restoreItem(current, row, at));
			const parent = context.parent;
			if (v.parentId && parent) {
				const parentId = v.parentId;
				setForumData((current) =>
					findItem(current, parentId)
						? addReplyCount(current, parentId, 1)
						: restoreItem(current, parent.row, parent.at),
				);
			}
		},
		onSettled: (_data, _error, _v, context) => {
			void invalidateList(queryClient, context?.key ?? forumKey);
		},
	});
	const { mutateAsync: removeAsync } = removeMutation;

	const remove = React.useCallback(
		async (entry: SellerForumComment) => {
			const key = entry.parentId ? repliesKeyOf(entry.parentId) : forumKey;
			const etag =
				findItem(queryClient.getQueryData<ForumData>(key), entry.id)?.etag ??
				entry.etag;
			await removeAsync({
				id: entry.id,
				parentId: entry.parentId ?? undefined,
				etag,
			});
		},
		[forumKey, repliesKeyOf, queryClient, removeAsync],
	);

	return {
		comments,
		submitted: submittedPosts,
		loading: query.isLoading,
		refreshing: query.isFetching,
		loadingMore: isFetchingNextPage,
		hasMore: hasNextPage,
		error: query.isFetchNextPageError ? null : query.error,
		loadMoreError: query.isFetchNextPageError ? query.error : null,
		refresh,
		loadMore,
		post,
		reply,
		update,
		remove,
	};
}

/** 一則頂層留言的回覆串：展開時才讀，分頁與送出的做法與頂層相同。 */
export function useForumReplies(
	sellerId: string | undefined,
	threadId: string,
	parentName: string | undefined,
	enabled: boolean,
) {
	const queryIdentity = useQueryIdentity();
	const storeId = String(sellerId ?? "");
	const repliesKey = React.useMemo(
		() =>
			queryKeys.seller.forumReplies(
				queryIdentity,
				storeId,
				threadId,
				REPLIES_PARAMS,
			),
		[queryIdentity, storeId, threadId],
	);
	const replyKey = React.useMemo(
		() => queryKeys.seller.forumReplyCreate(queryIdentity, storeId),
		[queryIdentity, storeId],
	);
	const query = useInfiniteQuery({
		queryKey: repliesKey,
		enabled: Boolean(sellerId) && enabled,
		initialPageParam: FIRST_PAGE_PARAM,
		getNextPageParam: nextPageParam,
		queryFn: ({ pageParam, signal }) =>
			fetchSellerForumReplies(storeId, threadId, {
				pageSize: REPLY_PAGE_SIZE,
				pageToken: pageParam,
				parentName,
				signal,
			}),
	});
	const items = React.useMemo(
		() => (query.data ? flattenPages(query.data) : EMPTY_ENTRIES),
		[query.data],
	);
	const { refetch, fetchNextPage, hasNextPage, isFetchingNextPage } = query;

	const loadMore = React.useCallback(async () => {
		if (!hasNextPage || isFetchingNextPage) return;
		await fetchNextPage({ cancelRefetch: false });
	}, [hasNextPage, isFetchingNextPage, fetchNextPage]);
	const refresh = React.useCallback(async () => {
		await refetch();
	}, [refetch]);

	const accepts = React.useCallback(
		(v: ReplySubmission) => v.threadId === threadId,
		[threadId],
	);
	const build = React.useCallback(
		(v: ReplySubmission, createdAt: number) =>
			optimisticEntry(storeId, v, createdAt, {
				...replyExtra(v),
				replyToName: v.replyToName ?? parentName,
			}),
		[storeId, parentName],
	);
	const landed = React.useCallback(
		(data: SellerForumComment, v: ReplySubmission): SellerForumComment => ({
			...data,
			replyToName: v.replyToName ?? parentName,
		}),
		[parentName],
	);
	const submittedItems = useSubmittedItems<ReplySubmission, SellerForumComment>(
		{
			mutationKey: replyKey,
			loaded: items,
			accepts,
			build,
			landed,
		},
	);
	const submitted = React.useMemo(
		() => submittedItems.map((s) => ({ key: s.key, entry: s.item })),
		[submittedItems],
	);

	return {
		items,
		submitted,
		loading: query.isLoading,
		hasMore: hasNextPage,
		loadingMore: isFetchingNextPage,
		error: query.isFetchNextPageError ? null : query.error,
		loadMoreError: query.isFetchNextPageError ? query.error : null,
		loadMore,
		refresh,
	};
}
