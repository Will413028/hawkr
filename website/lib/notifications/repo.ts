/**
 * Notification HTTP repository.
 *
 * All endpoints are authed via lib/api/client.ts and the stored active
 * workspace role. The legacy `role` option is retained for query scoping and
 * for choosing the buyer/seller preferences path; it does not override the
 * `X-Active-Role` transport header.
 *
 * Endpoint contract (matches hawkr_buyer/seller `notification_repository.dart`):
 *   GET    /app/notifications?pageSize=20&cursorId=N
 *   GET    /app/notifications/unread-count
 *   PATCH  /app/notifications/{id}/read       (idempotent)
 *   PATCH  /app/notifications/read-all        → { updated: N }
 *   DELETE /app/notifications                 body { ids: [...] } → { deleted: N }
 *   GET    /app/{buyers|sellers}/me/notification-preferences
 *   PUT    /app/{buyers|sellers}/me/notification-preferences
 */

import { type ApiEnvelope, authedFetch } from "../api/client";
import { getActiveRole, type RoleSlot } from "../auth/storage";
import type {
	DeleteNotificationsBody,
	NotificationItem,
	NotificationListResult,
	NotificationPreferenceItem,
} from "./types";

const BASE = "/app/notifications";

interface RoleArg {
	role?: RoleSlot;
}

/* ===== List ===== */

export interface FetchNotificationsParams extends RoleArg {
	cursorId?: string | number;
	pageSize?: number;
	category?: string;
}

interface ServerListEnvelope {
	records?: NotificationItem[];
	list?: NotificationItem[];
	total?: number | string;
	pageSize?: number | string;
	hasMore?: boolean;
	nextCursorId?: number | string | null;
}

function notificationCursor(value: unknown): string | null {
	if (value == null) return null;
	const cursor = String(value).trim();
	return cursor && cursor !== "0" ? cursor : null;
}

export async function fetchNotifications({
	cursorId,
	pageSize = 20,
	category,
	role,
}: FetchNotificationsParams = {}): Promise<NotificationListResult> {
	const qs = new URLSearchParams({ pageSize: String(pageSize) });
	const cursor = notificationCursor(cursorId);
	if (cursor) qs.set("cursorId", cursor);
	if (category) qs.set("category", category);

	const env = await authedFetch<ServerListEnvelope | NotificationItem[]>(
		`${BASE}?${qs.toString()}`,
		{ role },
	);

	// Two known shapes: `{records, total, pageSize, hasMore}` (per Flutter repo)
	// vs a flat array (per openapi.json `/user/notifications`). Normalize both.
	const data = env.data;
	if (Array.isArray(data)) {
		return {
			records: data,
			total: data.length,
			pageSize,
			hasMore: data.length >= pageSize,
			nextCursorId:
				data.length > 0 ? notificationCursor(data[data.length - 1]?.id) : null,
		};
	}
	const records = data?.records ?? data?.list ?? [];
	const total = Number(data?.total ?? records.length);
	const explicitNext = data?.nextCursorId;
	const lastId =
		records.length > 0
			? notificationCursor(records[records.length - 1]?.id)
			: null;
	return {
		records,
		total,
		pageSize: Number(data?.pageSize ?? pageSize),
		hasMore: data?.hasMore ?? records.length >= pageSize,
		nextCursorId: notificationCursor(explicitNext) ?? lastId,
	};
}

/* ===== Unread count ===== */

export async function fetchUnreadCount({
	role,
}: RoleArg = {}): Promise<number> {
	type UnreadShape = { count?: number; unread?: number };
	const env = (await authedFetch<UnreadShape>(`${BASE}/unread-count`, {
		role,
	})) as ApiEnvelope<UnreadShape> & { count?: number };
	const fromData = env.data?.count ?? env.data?.unread;
	const fromTopLevel = env.count;
	const n = Number(fromData ?? fromTopLevel ?? 0);
	return Number.isFinite(n) ? n : 0;
}

/* ===== Mark read ===== */

export async function markNotificationRead(
	id: number | string,
	{ role }: RoleArg = {},
): Promise<void> {
	await authedFetch(`${BASE}/${id}/read`, { method: "PATCH", role });
}

export async function markAllNotificationsRead({
	role,
}: RoleArg = {}): Promise<number> {
	const env = await authedFetch<{ updated?: number }>(`${BASE}/read-all`, {
		method: "PATCH",
		role,
	});
	return Number(env.data?.updated ?? 0);
}

/* ===== Delete ===== */

export async function deleteNotifications(
	ids: Array<string | number>,
	{ role }: RoleArg = {},
): Promise<number> {
	if (ids.length === 0) return 0;
	const body: DeleteNotificationsBody = { ids };
	const env = await authedFetch<{ deleted?: number }>(BASE, {
		method: "DELETE",
		json: body,
		role,
	});
	return Number(env.data?.deleted ?? ids.length);
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

/** Bulk save — sends one PUT per category in parallel. */
export async function updateNotificationPreferences(
	preferences: NotificationPreferenceItem[],
	{ role }: RoleArg = {},
): Promise<void> {
	await Promise.all(
		preferences.map((p) =>
			updateNotificationPreference(p.category, p.enabled, { role }),
		),
	);
}
