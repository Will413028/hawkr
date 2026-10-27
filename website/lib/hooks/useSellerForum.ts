"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	createSellerForumComment,
	deleteSellerForumComment,
	fetchSellerForum,
	fetchSellerForumReplies,
	replySellerForumComment,
	type SellerForumComment,
	type SellerForumListResult,
	toggleSellerForumCommentLike,
	updateSellerForumComment,
} from "@/lib/seller/forum-repo";
import { errorMessage } from "./queryError";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseSellerForumResult {
	comments: SellerForumComment[];
	loading: boolean;
	refreshing: boolean;
	loadingMore: boolean;
	hasMore: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
	loadReplies: (commentId: number | string) => Promise<SellerForumComment[]>;
	post: (content: string) => Promise<void>;
	reply: (commentId: number | string, content: string) => Promise<void>;
	update: (commentId: number | string, content: string) => Promise<void>;
	remove: (commentId: number | string) => Promise<void>;
	like: (commentId: number | string) => Promise<void>;
}

function sameId(a: number | string | undefined, b: number | string) {
	return String(a) === String(b);
}

function replaceComment(
	items: SellerForumComment[],
	id: number | string,
	mapper: (item: SellerForumComment) => SellerForumComment,
): SellerForumComment[] {
	return items.map((item) => {
		if (sameId(item.id, id)) return mapper(item);
		if (item.replies?.length) {
			return { ...item, replies: replaceComment(item.replies, id, mapper) };
		}
		return item;
	});
}

function removeComment(
	items: SellerForumComment[],
	id: number | string,
): SellerForumComment[] {
	return items
		.filter((item) => !sameId(item.id, id))
		.map((item) =>
			item.replies?.length
				? { ...item, replies: removeComment(item.replies, id) }
				: item,
		);
}

function findComment(
	items: SellerForumComment[],
	id: number | string,
): SellerForumComment | undefined {
	for (const item of items) {
		if (sameId(item.id, id)) return item;
		const nested = item.replies?.length
			? findComment(item.replies, id)
			: undefined;
		if (nested) return nested;
	}
	return undefined;
}

function insertReply(
	items: SellerForumComment[],
	parentId: number | string,
	reply: SellerForumComment,
): SellerForumComment[] {
	return replaceComment(items, parentId, (item) => ({
		...item,
		// Always +1: the previous Math.max(prev_count, replies.length + 1)
		// left replyCount stale when the user hadn't expanded replies yet
		// (e.g. count=3 stayed 3 after adding one because replies was
		// undefined and Math.max(3, 0+1) === 3). The collapsed "X 則回覆"
		// button needs to reflect the new total immediately.
		replyCount: (item.replyCount ?? 0) + 1,
		replies: [...(item.replies ?? []), reply],
	}));
}

export function useSellerForum(
	sellerId: number | string | undefined,
	initialData?: SellerForumListResult,
): UseSellerForumResult {
	const { identity } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const pageSize = initialData?.pageSize ?? 20;
	const forumParams = React.useMemo(() => ({ pageSize }), [pageSize]);
	const forumKey = React.useMemo(
		() => queryKeys.seller.forum(queryIdentity, sellerId ?? "", forumParams),
		[queryIdentity, sellerId, forumParams],
	);
	const emptyData: SellerForumListResult = React.useMemo(
		() => ({
			records: [],
			page: 1,
			pageSize,
			total: 0,
		}),
		[pageSize],
	);
	const query = useQuery({
		queryKey: forumKey,
		enabled: Boolean(sellerId),
		// SSR 帶入的真資料才走 initialData；缺席時用 placeholderData 佔位，
		// 避免空清單在 staleTime 內被當成新鮮資料而不發請求。
		initialData,
		placeholderData: emptyData,
		queryFn: () => fetchSellerForum(sellerId ?? "", { page: 1, pageSize }),
	});
	const [loadingMore, setLoadingMore] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const data = query.data ?? emptyData;
	const comments = data.records;
	const page = data.page;
	const total = data.total;
	const refetchForum = query.refetch;
	const refreshPromiseRef = React.useRef<Promise<void> | null>(null);

	const setForumData = React.useCallback(
		(updater: (current: SellerForumListResult) => SellerForumListResult) => {
			queryClient.setQueryData<SellerForumListResult>(forumKey, (current) =>
				updater(current ?? emptyData),
			);
		},
		[emptyData, forumKey, queryClient],
	);

	const refresh = React.useCallback(() => {
		if (!sellerId) return Promise.resolve();
		if (refreshPromiseRef.current) return refreshPromiseRef.current;
		setError(null);
		const pending = refetchForum()
			.then(() => undefined)
			.finally(() => {
				if (refreshPromiseRef.current === pending) {
					refreshPromiseRef.current = null;
				}
			});
		refreshPromiseRef.current = pending;
		return pending;
	}, [sellerId, refetchForum]);

	const loadMore = React.useCallback(async () => {
		if (!sellerId || loadingMore || comments.length >= total) return;
		setLoadingMore(true);
		setError(null);
		try {
			const result = await fetchSellerForum(sellerId, {
				page: page + 1,
				pageSize,
			});
			setForumData((current) => ({
				...result,
				records: [...current.records, ...result.records],
			}));
		} catch (err) {
			// err 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類，見
			// lib/api/client.ts），errorMessage 的 `!error → null` 分支不會在此觸發；
			// 若未來這個 try 區塊新增可能拋出 falsy 值的呼叫，需重新檢視此假設。
			setError(errorMessage(err, "載入更多失敗"));
		} finally {
			setLoadingMore(false);
		}
	}, [
		sellerId,
		loadingMore,
		comments.length,
		total,
		page,
		pageSize,
		setForumData,
	]);

	const loadReplies = React.useCallback(
		async (commentId: number | string) => {
			const result = await fetchSellerForumReplies(commentId, { pageSize: 20 });
			setForumData((current) => ({
				...current,
				records: replaceComment(current.records, commentId, (item) => ({
					...item,
					replies: result.records,
					replyCount: result.total,
				})),
			}));
			return result.records;
		},
		[setForumData],
	);

	const post = React.useCallback(
		async (content: string) => {
			if (!sellerId || !content.trim()) return;
			const optimistic: SellerForumComment = {
				id: `temp-${Date.now()}`,
				sellerId: String(sellerId),
				isMine: true,
				userName: identity?.nickname ?? "我",
				avatarKey: identity?.avatarKey,
				content: content.trim(),
				likeCount: 0,
				replyCount: 0,
				isLiked: false,
				createdAt: Math.floor(Date.now() / 1000),
			};
			setForumData((current) => ({
				...current,
				records: [optimistic, ...current.records],
				total: current.total + 1,
			}));
			try {
				const real = await createSellerForumComment(sellerId, content.trim());
				setForumData((current) => ({
					...current,
					records: current.records.map((item) =>
						sameId(item.id, optimistic.id) ? real : item,
					),
				}));
			} catch (err) {
				setForumData((current) => ({
					...current,
					records: current.records.filter(
						(item) => !sameId(item.id, optimistic.id),
					),
					total: Math.max(0, current.total - 1),
				}));
				// err 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(err, "送出留言失敗"));
				throw err;
			}
		},
		[sellerId, identity?.nickname, identity?.avatarKey, setForumData],
	);

	const reply = React.useCallback(
		async (commentId: number | string, content: string) => {
			if (!content.trim()) return;
			const parent = findComment(comments, commentId);
			const optimistic: SellerForumComment = {
				id: `temp-${Date.now()}`,
				parentId: String(commentId),
				rootId: parent?.rootId ?? parent?.id ?? String(commentId),
				replyToName: parent?.userName,
				isMine: true,
				userName: identity?.nickname ?? "我",
				avatarKey: identity?.avatarKey,
				content: content.trim(),
				likeCount: 0,
				replyCount: 0,
				isLiked: false,
				createdAt: Math.floor(Date.now() / 1000),
			};
			setForumData((current) => ({
				...current,
				records: insertReply(current.records, commentId, optimistic),
			}));
			try {
				const real = await replySellerForumComment(commentId, content.trim());
				setForumData((current) => ({
					...current,
					records: replaceComment(current.records, optimistic.id, () => real),
				}));
			} catch (err) {
				setForumData((current) => ({
					...current,
					records: replaceComment(
						removeComment(current.records, optimistic.id),
						commentId,
						(item) => ({
							...item,
							replyCount: Math.max(0, (item.replyCount ?? 1) - 1),
						}),
					),
				}));
				// err 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(err, "回覆失敗"));
				throw err;
			}
		},
		[comments, identity?.nickname, identity?.avatarKey, setForumData],
	);

	const update = React.useCallback(
		async (commentId: number | string, content: string) => {
			if (!content.trim()) return;
			const before = findComment(comments, commentId);
			setForumData((current) => ({
				...current,
				records: replaceComment(current.records, commentId, (item) => ({
					...item,
					content: content.trim(),
				})),
			}));
			try {
				const real = await updateSellerForumComment(commentId, content.trim());
				setForumData((current) => ({
					...current,
					records: replaceComment(current.records, commentId, () => real),
				}));
			} catch (err) {
				if (before) {
					setForumData((current) => ({
						...current,
						records: replaceComment(current.records, commentId, () => before),
					}));
				}
				// err 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(err, "編輯失敗"));
				throw err;
			}
		},
		[comments, setForumData],
	);

	const remove = React.useCallback(
		async (commentId: number | string) => {
			const before = comments;
			setForumData((current) => ({
				...current,
				records: removeComment(current.records, commentId),
			}));
			try {
				await deleteSellerForumComment(commentId);
			} catch (err) {
				setForumData((current) => ({ ...current, records: before }));
				// err 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(err, "刪除失敗"));
				throw err;
			}
		},
		[comments, setForumData],
	);

	const like = React.useCallback(
		async (commentId: number | string) => {
			const target = findComment(comments, commentId);
			const wasLiked = target?.isLiked === true;
			setForumData((current) => ({
				...current,
				records: replaceComment(current.records, commentId, (item) => ({
					...item,
					isLiked: !wasLiked,
					likeCount: Math.max(0, (item.likeCount ?? 0) + (wasLiked ? -1 : 1)),
				})),
			}));
			try {
				const result = await toggleSellerForumCommentLike(commentId, wasLiked);
				setForumData((current) => ({
					...current,
					records: replaceComment(current.records, commentId, (item) => ({
						...item,
						isLiked: result.liked,
					})),
				}));
			} catch (err) {
				setForumData((current) => ({
					...current,
					records: replaceComment(current.records, commentId, (item) => ({
						...item,
						isLiked: wasLiked,
						likeCount: Math.max(0, (item.likeCount ?? 0) + (wasLiked ? 1 : -1)),
					})),
				}));
				// err 目前保證是 truthy 的 Error 實例（api client 只 throw Error 子類），
				// errorMessage 的 `!error → null` 分支不會在此觸發。
				setError(errorMessage(err, "更新按讚失敗"));
			}
		},
		[comments, setForumData],
	);

	return {
		comments,
		loading: query.isLoading,
		refreshing: query.isFetching,
		loadingMore,
		hasMore: comments.length < total,
		error: error ?? (query.error instanceof Error ? query.error.message : null),
		refresh,
		loadMore,
		loadReplies,
		post,
		reply,
		update,
		remove,
		like,
	};
}
