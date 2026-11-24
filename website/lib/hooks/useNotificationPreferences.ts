"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import type { RoleSlot } from "@/lib/auth/active-role";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import {
	audienceForRole,
	fetchNotificationPreferences,
	type PreferenceEntry,
} from "@/lib/notifications/inbox";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * Notification preference entries (category x channel) for the active workspace
 * role, keyed by role. Shared read for the preferences screen (toggle UI) and
 * the notifications list (derives its filter chips). Read-only — the
 * preferences screen owns the optimistic toggle locally.
 */
export function useNotificationPreferences(role: RoleSlot): {
	entries: PreferenceEntry[] | null;
	loading: boolean;
	error: string | null;
} {
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const defaults = useDefaultErrorText();
	const query = useQuery({
		queryKey: queryKeys.notifications.preferences(identity, role),
		queryFn: accountQueryFn(status, () =>
			fetchNotificationPreferences(audienceForRole(role)),
		),
	});
	return {
		entries: query.data ?? null,
		loading: accountQueryPending(status, query.isPending),
		error: query.error ? errorText(query.error, defaults) : null,
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
