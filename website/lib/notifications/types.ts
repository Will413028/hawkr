/**
 * Notification feature types.
 *
 * Shapes mirror the unified gateway contract under
 * `/api/v1/app/notifications` (see gateway `notification_inbox.go`
 * `inboxItemDTO`). Gateway emits camelCase per project API spec, and
 * `createdAt` is an RFC3339 string (not unix seconds).
 */

import type { RoleSlot } from "@/lib/auth/active-role";

export type NotificationType =
	| "order"
	| "promotion"
	| "system"
	| "message"
	| "review"
	| "follow"
	| "restock" // 02 §5.13b / backend issue #942：補貨通知
	| string;

export interface NotificationItem {
	id: number | string;
	accountId?: string;
	/** Present on notification.new SSE items; REST rows are already role-scoped. */
	userType?: RoleSlot;
	type: NotificationType;
	title: string;
	body?: string;
	imageUrl?: string;
	actionUrl?: string;
	refType?: string;
	refId?: string | number;
	isRead: boolean;
	readAt?: string;
	createdAt: string;
	data?: Record<string, string>;
}

/** Shape returned by GET /app/notifications (cursor-paginated). */
export interface NotificationListResult {
	records: NotificationItem[];
	total: number;
	pageSize: number;
	hasMore: boolean;
	/** Opaque TSID cursor. Keep as text so 64-bit ids never lose precision. */
	nextCursorId: string | null;
}

/**
 * Per-category preference toggle. Backend (`GET/PUT
 * /app/{buyers|sellers}/me/notification-preferences`) tracks a single
 * `enabled` flag per category — no per-channel (push/email/in_app)
 * granularity is wired through the gateway. `title` + `description`
 * come from the backend so categories can be added server-side without
 * a client redeploy.
 *
 * `forced` flags categories the backend won't let the user disable
 * (notification design doc §3.1 — e.g. `shipment_anomaly` for sellers).
 * The UI renders the toggle locked-on for these rows. Some backend
 * builds may not include the flag yet, so callers should additionally
 * fall back to a hard-coded `FORCED_BY_ROLE` map.
 */
export interface NotificationPreferenceItem {
	category: string;
	title?: string;
	description?: string;
	enabled: boolean;
	forced?: boolean;
}

/** Body for DELETE /app/notifications */
export interface DeleteNotificationsBody {
	ids: Array<string | number>;
}
