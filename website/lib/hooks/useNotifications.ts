import { errorText } from "@hawkr/api/connect/errors";
import type { NotificationCategory } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import {
	type InfiniteData,
	useInfiniteQuery,
	useMutation,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import { accountQueryFn, accountQueryPending } from "../auth/account-query";
import type { RoleSlot } from "../auth/active-role";
import { useAccountStatus } from "../auth/use-account-status";
import { useDefaultErrorText } from "../i18n/ui-text";
import { invalidateNotificationLists } from "../notifications/cache";
import {
	audienceForRole,
	deleteInboxItems,
	type InboxItem,
	type InboxPage,
	listInboxItems,
	markAllInboxItemsRead,
	markInboxItemRead,
} from "../notifications/inbox";
import { useNotificationUnread } from "../notifications/unread-context";
import { type QueryIdentity, queryKeys } from "../query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * Notification list hook (workspace-role scoped).
 *
 * Page-token-paginated list (`ListInboxItems`) with an explicit load-more
 * action + optimistic mark-as-read / delete. Scrolling alone never requests
 * another page. Title, body and deep link are composed by the server; the
 * client renders them as given. Mutate paths also sync the shared
 * `NotificationUnreadProvider` so the navbar bell reflects changes
 * instantly; the stream's inbox signal refetches the active list.
 *
 * The route role picks the inbox audience and scopes cache keys.
 */

/* ===== List page ===== */

interface UseNotificationListOptions {
	pageSize?: number;
	category?: NotificationCategory;
	role: RoleSlot;
}

export function notificationListQueryKey(
	identity: QueryIdentity,
	options: UseNotificationListOptions,
) {
	const pageSize = options.pageSize ?? 20;
	return queryKeys.notifications.list(identity, {
		role: options.role,
		pageSize,
		category: options.category ?? "",
	});
}

export function fetchNotificationListPage(
	options: UseNotificationListOptions,
	pageToken: string | null,
) {
	return listInboxItems({
		audience: audienceForRole(options.role),
		category: options.category,
		pageSize: options.pageSize ?? 20,
		pageToken: pageToken ?? undefined,
	});
}

export function nextNotificationListPage(lastPage: InboxPage) {
	return lastPage.nextPageToken || undefined;
}

interface UseNotificationListResult {
	items: InboxItem[];
	loading: boolean; // initial / refresh
	loadingMore: boolean;
	hasMore: boolean;
	error: string | null;
	refresh: () => Promise<void>;
	loadMore: () => Promise<void>;
	markRead: (id: InboxItem["id"]) => Promise<void>;
	markAllRead: () => Promise<void>;
	remove: (ids: Array<InboxItem["id"]>) => Promise<void>;
}

type NotificationPages = InfiniteData<InboxPage>;

function flattenNotificationPages(
	data: NotificationPages | undefined,
): InboxItem[] {
	const seen = new Set<string>();
	const items: InboxItem[] = [];
	for (const page of data?.pages ?? []) {
		for (const item of page.items) {
			if (seen.has(item.id)) continue;
			seen.add(item.id);
			items.push(item);
		}
	}
	return items;
}

export function useNotificationList(
	options: UseNotificationListOptions,
): UseNotificationListResult {
	const pageSize = options.pageSize ?? 20;
	const filterCategory = options.category;
	const role = options.role;
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const defaults = useDefaultErrorText();
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
		queryFn: accountQueryFn(
			status,
			({ pageParam }: { pageParam: string | null }) =>
				fetchNotificationListPage(listOptions, pageParam),
		),
		initialPageParam: null as string | null,
		getNextPageParam: nextNotificationListPage,
		// A category change keeps the complete previous list in place until the
		// replacement query resolves instead of clearing the workspace to a
		// loading surface.
		placeholderData: (previousData) => previousData,
	});
	const updatePages = React.useCallback(
		(updater: (item: InboxItem) => InboxItem | null) => {
			queryClient.setQueryData<NotificationPages>(listKey, (current) => {
				if (!current) return current;
				return {
					...current,
					pages: current.pages.map((page) => ({
						...page,
						items: page.items.flatMap((item) => {
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
		mutationFn: (id: InboxItem["id"]) => markInboxItemRead(id),
		onMutate: async (id) => {
			await queryClient.cancelQueries({ queryKey: listKey });
			const snapshot = queryClient.getQueryData<NotificationPages>(listKey);
			// Only nudge the shared count when the row was actually unread —
			// re-marking an already-read row would otherwise double-decrement.
			let wasUnread = false;
			updatePages((n) => {
				if (n.id !== id) return n;
				wasUnread = !n.read;
				return { ...n, read: true };
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
		mutationFn: () => markAllInboxItemsRead(audienceForRole(role)),
		onMutate: async () => {
			await queryClient.cancelQueries({ queryKey: listKey });
			const snapshot = queryClient.getQueryData<NotificationPages>(listKey);
			updatePages((n) => ({ ...n, read: true }));
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
		mutationFn: (ids: Array<InboxItem["id"]>) => deleteInboxItems(ids),
		onMutate: async (ids) => {
			await queryClient.cancelQueries({ queryKey: listKey });
			const snapshot = queryClient.getQueryData<NotificationPages>(listKey);
			const idSet = new Set(ids);
			const unreadRemoved = flattenNotificationPages(snapshot).filter(
				(n) => idSet.has(n.id) && !n.read,
			).length;
			return { idSet, unreadRemoved };
		},
		onSuccess: (_result, _ids, ctx) => {
			if (ctx) {
				updatePages((n) => (ctx.idSet.has(n.id) ? null : n));
				if (ctx.unreadRemoved > 0) decrementUnread(ctx.unreadRemoved);
			}
			void invalidateNotificationLists(queryClient, identity, "none");
		},
	});

	const items = React.useMemo(
		() =>
			status === "signedIn" ? flattenNotificationPages(listQuery.data) : [],
		[status, listQuery.data],
	);

	// Thin wrappers: the optimistic + rollback work lives in the mutation
	// lifecycle above; these just fire-and-forget (failures self-revert).
	const markRead = React.useCallback(
		(id: InboxItem["id"]) =>
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
		(ids: Array<InboxItem["id"]>) => {
			if (ids.length === 0) return Promise.resolve();
			return removeMutation.mutateAsync(ids).then(
				() => {},
				() => {},
			);
		},
		[removeMutation],
	);

	const failure =
		listQuery.error ??
		markReadMutation.error ??
		markAllReadMutation.error ??
		removeMutation.error;

	return {
		items,
		loading: accountQueryPending(status, listQuery.isLoading),
		loadingMore: listQuery.isFetchingNextPage,
		hasMore: Boolean(listQuery.hasNextPage),
		error: failure ? errorText(failure, defaults) : null,
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
