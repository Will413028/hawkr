"use client";

/**
 * Realtime notification stream hook — subscribes to
 * `/app/notifications/stream` and surfaces typed payloads (new inbox item
 * + unread-count deltas) to the caller. Authed mode rides the Clerk session
 * token + `X-Active-Role` selection — same auth wiring as
 * `apiClient.authed.*` (lib/sse/client.ts).
 *
 * Design book §3 (notification event schema) / §10 (lifecycle) / §10
 * (failure):
 *   - `enabled=false` keeps the SSE closed entirely (signed-out users,
 *     guest paths, role-switching grace periods).
 *   - `resetSignal` (typically `activeRole`) restarts the stream when
 *     the viewer switches workspace so the new identity's notifications
 *     replace the old without retaining the previous role's last id.
 *   - Last-Event-ID is cached per resetSignal so a hook unmount/remount
 *     during the same SPA session resumes from the most recent id.
 *
 * One open() per (resetSignal, enabled=true) cycle. Cleanup closes the
 * underlying connection so React Strict Mode's double-invoke doesn't
 * leave dangling streams.
 */

import * as React from "react";
import { normalizeNotificationStreamItem } from "@/lib/notifications/stream-item";
import type { NotificationItem } from "@/lib/notifications/types";
import { SseClient, type SseConnectionStatus } from "./client";
import { getLastEventId, setLastEventId } from "./lastEventIdCache";
import { asNum, asStr, unpackBatch } from "./utils";

export interface NotificationNewPayload {
	item: NotificationItem;
	unreadCount: number;
	eventId: string;
}

export interface NotificationUnreadChangedPayload {
	unreadCount: number;
	eventId: string;
}

/**
 * Seller workspace realtime payloads — ride the same `/app/notifications/stream`
 * connection on topic `notification:user:{sellerId}` (gateway
 * SellerWorkspaceChannel).
 */
export interface SellerOrderUpdatedPayload {
	eventId: string;
	orderNo: string;
	source: string;
	previousStatus?: string;
	status?: string;
	shipmentStatus?: string;
	changedAt?: string;
}

export interface SellerAfterSaleUpdatedPayload {
	eventId: string;
	orderNo: string;
	afterSaleCaseNo: string;
	source: string;
	status?: string;
	changedAt?: string;
}

export interface UseNotificationStreamHandlers {
	onNew?: (payload: NotificationNewPayload) => void;
	onUnreadChanged?: (payload: NotificationUnreadChangedPayload) => void;
	/** Seller workspace realtime handlers (seller role topics only). */
	onSellerOrderUpdated?: (payload: SellerOrderUpdatedPayload) => void;
	onSellerAfterSaleUpdated?: (payload: SellerAfterSaleUpdatedPayload) => void;
	/** Fired when server `ready` carried bufferMiss=true — caller should re-fetch REST list/count. */
	onBufferMiss?: () => void;
}

export interface UseNotificationStreamOptions {
	enabled: boolean;
	/** Any value whose change should restart the stream (e.g. active role / accountId). */
	resetSignal?: unknown;
	handlers: UseNotificationStreamHandlers;
}

export interface UseNotificationStreamResult {
	status: SseConnectionStatus;
}

const cacheKey = (resetSignal: unknown) =>
	`notifications:${resetSignal == null ? "" : String(resetSignal)}`;

export function useNotificationStream({
	enabled,
	resetSignal,
	handlers,
}: UseNotificationStreamOptions): UseNotificationStreamResult {
	const [status, setStatus] = React.useState<SseConnectionStatus>("idle");
	const handlersRef = React.useRef(handlers);
	handlersRef.current = handlers;

	React.useEffect(() => {
		if (!enabled) return;
		const key = cacheKey(resetSignal);

		const dispatch = (
			eventName: string,
			data: unknown,
			id: string | null,
		): void => {
			switch (eventName) {
				case "notification.batch": {
					for (const inner of unpackBatch(data)) {
						dispatch(inner.event, inner.data, inner.id);
					}
					break;
				}
				case "notification.new": {
					const o = (data ?? {}) as { item?: unknown; unreadCount?: unknown };
					handlersRef.current.onNew?.({
						item: normalizeNotificationStreamItem(o.item),
						unreadCount: asNum(o.unreadCount),
						eventId: id ?? "",
					});
					break;
				}
				case "notification.unread_changed": {
					const o = (data ?? {}) as { unreadCount?: unknown };
					handlersRef.current.onUnreadChanged?.({
						unreadCount: asNum(o.unreadCount),
						eventId: id ?? "",
					});
					break;
				}
				case "seller.order.updated": {
					// 賣家工作區訂單狀態變更（gateway 已將 sellerId 序列化為 string）
					const o = (data ?? {}) as Record<string, unknown>;
					handlersRef.current.onSellerOrderUpdated?.({
						eventId: id ?? asStr(o.eventId),
						orderNo: asStr(o.orderNo),
						source: asStr(o.source),
						previousStatus:
							o.previousStatus != null ? asStr(o.previousStatus) : undefined,
						status: o.status != null ? asStr(o.status) : undefined,
						shipmentStatus:
							o.shipmentStatus != null ? asStr(o.shipmentStatus) : undefined,
						changedAt: o.changedAt != null ? asStr(o.changedAt) : undefined,
					});
					break;
				}
				case "seller.after_sale.updated": {
					// 賣家工作區售後案件狀態變更
					const o = (data ?? {}) as Record<string, unknown>;
					handlersRef.current.onSellerAfterSaleUpdated?.({
						eventId: id ?? asStr(o.eventId),
						orderNo: asStr(o.orderNo),
						afterSaleCaseNo: asStr(o.afterSaleCaseNo),
						source: asStr(o.source),
						status: o.status != null ? asStr(o.status) : undefined,
						changedAt: o.changedAt != null ? asStr(o.changedAt) : undefined,
					});
					break;
				}
				default:
					// Unknown event — forward-compat.
					break;
			}
			if (id) setLastEventId(key, id);
		};

		const client = new SseClient({
			path: "/notifications/stream",
			mode: "authed",
			initialLastEventId: getLastEventId(key),
			handlers: {
				onEvent: dispatch,
				onReady: (info) => {
					if (info.bufferMiss) handlersRef.current.onBufferMiss?.();
				},
				onStatusChange: setStatus,
			},
		});

		client.open();
		return () => {
			client.close();
		};
	}, [enabled, resetSignal]);

	return { status };
}
