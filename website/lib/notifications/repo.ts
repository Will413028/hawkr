/**
 * Notification preferences (legacy REST, pending the preferences migration).
 *
 *   GET    /app/{buyers|sellers}/me/notification-preferences
 *   PUT    /app/{buyers|sellers}/me/notification-preferences
 */

import { authedFetch } from "../api/client";
import { getActiveRole, type RoleSlot } from "../auth/storage";
import type { NotificationPreferenceItem } from "./types";

interface RoleArg {
	role?: RoleSlot;
}

/* ===== Preferences ===== */

function preferencesPath(role: RoleSlot | undefined): string {
	const r = role ?? getActiveRole() ?? "buyer";
	return r === "seller"
		? "/app/sellers/me/notification-preferences"
		: "/app/buyers/me/notification-preferences";
}

export async function fetchNotificationPreferences({
	role,
}: RoleArg = {}): Promise<NotificationPreferenceItem[]> {
	const env = await authedFetch<
		| { preferences?: NotificationPreferenceItem[] }
		| NotificationPreferenceItem[]
	>(preferencesPath(role), { role });
	const data = env.data;
	if (Array.isArray(data)) return data;
	return data?.preferences ?? [];
}

/**
 * Toggle one category on/off. Backend PUT body is `{ category, enabled }`
 * (single item per request, not a bulk array — see openapi.json
 * `/app/buyers/me/notification-preferences`). Callers wanting to save
 * multiple toggles fire this in a `Promise.all` over the diff.
 */
export async function updateNotificationPreference(
	category: string,
	enabled: boolean,
	{ role }: RoleArg = {},
): Promise<void> {
	await authedFetch(preferencesPath(role), {
		method: "PUT",
		json: { category, enabled },
		role,
	});
}
