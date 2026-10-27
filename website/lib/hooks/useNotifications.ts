"use client";

import {
	type InfiniteData,
	useInfiniteQuery,
	useMutation,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import type { RoleSlot } from "../auth/active-role";
import { useAuth } from "../auth/context";
import { invalidateNotificationLists } from "../notifications/cache";
import {
	deleteNotifications,
	fetchNotifications,
	markAllNotificationsRead,
	markNotificationRead,
} from "../notifications/repo";
import type {
	NotificationItem,
	NotificationListResult,
} from "../notifications/types";
import { useNotificationUnread } from "../notifications/unread-context";
import { type QueryIdentity, queryKeys } from "../query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * Notification list hook (workspace-role scoped).
 *
 * Cursor-paginated list with an explicit load-more action + optimistic
 * mark-as-read / delete. Scrolling alone never requests another page.
 * Mutate paths also sync the shared
 * `NotificationUnreadProvider` so the navbar bell reflects changes
 * instantly without waiting for its 30s poll.
 *
 * The unread badge itself lives in `lib/notifications/unread-context` —
 * import `useNotificationUnread` from there.
 *
 * The route role scopes cache keys and notification destinations. Transport
 * still follows the authenticated stored active role, enforced by the route
 * guard before this hook mounts.
 */

/* ===== List page ===== */

interface UseNotificationListOptions {
	pageSize?: number;
	category?: string;
	role?: RoleSlot;
}

export function notificationListQueryKey(
	identity: QueryIdentity,
	options: UseNotificationListOptions = {},
) {
	const pageSize = options.pageSize ?? 20;
	return queryKeys.notifications.list(identity, {
		role: options.role ?? "",
		pageSize,
		category: options.category ?? "",
	});
}

export function fetchNotificationListPage(
	options: UseNotificationListOptions,
	cursorId: string | null,
) {
	return fetchNotifications({
		pageSize: options.pageSize ?? 20,
		category: options.category,
		cursorId: cursorId ?? undefined,
		role: options.role,
	});
}

export function nextNotificationListPage(lastPage: NotificationListResult) {
	return lastPage.hasMore ? lastPage.nextCursorId : undefined;
}

interface UseNotificationListResult {
	items: NotificationItem[];
	loading: boolean; // initial / refresh
	loadingMore: boolean;
	hasMore: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
	markRead: (id: NotificationItem["id"]) => Promise<void>;
	markAllRead: () => Promise<void>;
	remove: (ids: Array<NotificationItem["id"]>) => Promise<void>;
}

type NotificationPages = InfiniteData<NotificationListResult>;

function flattenNotificationPages(
	data: NotificationPages | undefined,
): NotificationItem[] {
	const seen = new Set<string>();
	const items: NotificationItem[] = [];
	for (const page of data?.pages ?? []) {
		for (const item of page.records) {
			const id = String(item.id);
			if (seen.has(id)) continue;
			seen.add(id);
			items.push(item);
		}
	}
	return items;
}

export function useNotificationList(
	options: UseNotificationListOptions = {},
): UseNotificationListResult {
	const pageSize = options.pageSize ?? 20;
	const filterCategory = options.category;
	const role = options.role;
	const { isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const {
		decrement: decrementUnread,
		setCount: setUnreadCount,
		refresh: refreshUnread,
	} = useNotificationUnread();
	const queryClient = useQueryClient();
	const listOptions = {
		pageSize,
		category: filterCategory,
		role,
	};
	const listKey = notificationListQueryKey(identity, listOptions);
	const listQuery = useInfiniteQuery({
		queryKey: listKey,
		queryFn: ({ pageParam }) =>
			fetchNotificationListPage(listOptions, pageParam),
		initialPageParam: null as string | null,
		getNextPageParam: nextNotificationListPage,
		// A category change keeps the complete previous list in place until the
		// replacement query resolves instead of clearing the workspace to a
		// loading surface.
		placeholderData: (previousData) => previousData,
		enabled: isLoggedIn,
	});
	const updatePages = React.useCallback(
		(updater: (item: NotificationItem) => NotificationItem | null) => {
			queryClient.setQueryData<NotificationPages>(listKey, (current) => {
				if (!current) return current;
				return {
					...current,
					pages: current.pages.map((page) => ({
						...page,
						records: page.records.flatMap((item) => {
							const next = updater(item);
							return next ? [next] : [];
						}),
					})),
				};
			});
		},
		[queryClient, listKey],
	);

	// Mark-read / mark-all stay optimistic. Delete deliberately keeps the row
	// mounted until success so its initiating Button can report pending state.
	// Every path still cancels late refetches and keeps the unread bell in sync.
	const markReadMutation = useMutation({
		mutationFn: (id: NotificationItem["id"]) =>
			markNotificationRead(id, { role }),
		onMutate: async (id) => {
			await queryClient.cancelQueries({ queryKey: listKey });
			const snapshot = queryClient.getQueryData<NotificationPages>(listKey);
			// Only nudge the shared count when the row was actually unread —
			// re-marking an already-read row would otherwise double-decrement.
			let wasUnread = false;
			updatePages((n) => {
				if (n.id !== id) return n;
				wasUnread = !n.isRead;
				return { ...n, isRead: true };
			});
			if (wasUnread) decrementUnread(1);
			return { snapshot, wasUnread };
		},
		onError: (_e, _id, ctx) => {
			// Revert list + resync the bell from server (safer than hand-rolling
			// +1, since another tab may have shifted it).
			if (ctx) queryClient.setQueryData(listKey, ctx.snapshot);
			if (ctx?.wasUnread) void refreshUnread();
		},
		onSuccess: () => {
			void invalidateNotificationLists(queryClient, identity, "none");
		},
	});

	const markAllReadMutation = useMutation({
		mutationFn: () => markAllNotificationsRead({ role }),
		onMutate: async () => {
			await queryClient.cancelQueries({ queryKey: listKey });
			const snapshot = queryClient.getQueryData<NotificationPages>(listKey);
			updatePages((n) => ({ ...n, isRead: true }));
			setUnreadCount(0);
			return { snapshot };
		},
		onError: (_e, _v, ctx) => {
			if (ctx) queryClient.setQueryData(listKey, ctx.snapshot);
			void refreshUnread();
		},
		onSuccess: () => {
			void invalidateNotificationLists(queryClient, identity, "none");
		},
	});

	const removeMutation = useMutation({
		mutationFn: (ids: Array<NotificationItem["id"]>) =>
			deleteNotifications(ids, { role }),
		onMutate: async (ids) => {
			await queryClient.cancelQueries({ queryKey: listKey });
			const snapshot = queryClient.getQueryData<NotificationPages>(listKey);
			const idSet = new Set(ids.map(String));
			const unreadRemoved = flattenNotificationPages(snapshot).filter(
				(n) => idSet.has(String(n.id)) && !n.isRead,
			).length;
			return { idSet, unreadRemoved };
		},
		onSuccess: (_result, _ids, ctx) => {
			if (ctx) {
				updatePages((n) => (ctx.idSet.has(String(n.id)) ? null : n));
				if (ctx.unreadRemoved > 0) decrementUnread(ctx.unreadRemoved);
			}
			void invalidateNotificationLists(queryClient, identity, "none");
		},
	});

	const items = React.useMemo(
		() => (isLoggedIn ? flattenNotificationPages(listQuery.data) : []),
		[isLoggedIn, listQuery.data],
	);

	// Thin wrappers: the optimistic + rollback work lives in the mutation
	// lifecycle above; these just fire-and-forget (failures self-revert).
	const markRead = React.useCallback(
		(id: NotificationItem["id"]) =>
			markReadMutation.mutateAsync(id).then(
				() => {},
				() => {},
			),
		[markReadMutation],
	);

	const markAllRead = React.useCallback(
		() =>
			markAllReadMutation.mutateAsync().then(
				() => {},
				() => {},
			),
		[markAllReadMutation],
	);

	const remove = React.useCallback(
		(ids: Array<NotificationItem["id"]>) => {
			if (ids.length === 0) return Promise.resolve();
			return removeMutation.mutateAsync(ids).then(
				() => {},
				() => {},
			);
		},
		[removeMutation],
	);

	const error =
		listQuery.error ??
		markReadMutation.error ??
		markAllReadMutation.error ??
		removeMutation.error;

	return {
		items,
		loading: listQuery.isLoading,
		loadingMore: listQuery.isFetchingNextPage,
		hasMore: Boolean(listQuery.hasNextPage),
		error: error instanceof Error ? error.message : null,
		refresh: async () => {
			await listQuery.refetch();
		},
		loadMore: async () => {
			if (!listQuery.hasNextPage || listQuery.isFetchingNextPage) return;
			await listQuery.fetchNextPage();
		},
		markRead,
		markAllRead,
		remove,
	};
}
