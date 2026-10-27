"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import type { RoleSlot } from "@/lib/auth/active-role";
import { useAuth } from "@/lib/auth/context";
import { fetchNotificationPreferences } from "@/lib/notifications/repo";
import type { NotificationPreferenceItem } from "@/lib/notifications/types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * Notification preference list for the active workspace role, keyed by role.
 * Shared read for the preferences screen (toggle UI) and the notifications list
 * (derives its filter chips) so the two stop double-fetching the same endpoint.
 * Read-only — the preferences screen owns the optimistic toggle locally.
 */
export function useNotificationPreferences(role: RoleSlot): {
	prefs: NotificationPreferenceItem[] | null;
	loading: boolean;
	error: string | null;
} {
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.notifications.preferences(identity, role),
		queryFn: () => fetchNotificationPreferences({ role }),
		enabled: hydrated && isLoggedIn,
	});
	return {
		prefs: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error.message : null,
	};
}

/** Invalidate the preference list for one role after a write elsewhere. */
export function useInvalidateNotificationPreferences(): (
	role: RoleSlot,
) => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		(role) =>
			queryClient.invalidateQueries({
				queryKey: queryKeys.notifications.preferences(identity, role),
			}),
		[queryClient, identity],
	);
}
