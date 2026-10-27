"use client";

/**
 * Wires the in-app toast queue to real events:
 *
 *   • New messages — subscribes to the shared chat socket and toasts inbound
 *     peer messages immediately. Connects the singleton (idempotent; nothing
 *     disconnects it) so toasts fire app-wide, not just on chat pages. Skips
 *     the user's own messages and stays quiet while they're in the messages UI.
 *
 *   • New orders / notifications — watches the notification unread count; when
 *     it climbs (after the initial prime) it pulls the latest inbox items and
 *     toasts the fresh, unread, non-message ones. Message-type inbox rows are
 *     left to the socket path above so a message never double-toasts.
 *
 * Renders nothing — mount it once inside the toast + unread providers.
 */

import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { messagePreview } from "@/lib/chat/preview";
import { getChatSocket } from "@/lib/chat/ws";
import { stripLocale } from "@/lib/intl/paths";
import {
	NOTIFICATION_NEW_EVENT,
	type NotificationNewEventDetail,
} from "@/lib/notifications/events";
import { fetchNotifications } from "@/lib/notifications/repo";
import { resolveNotification } from "@/lib/notifications/resolve";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { useNotificationUnread } from "@/lib/notifications/unread-context";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";

// Notification ids are 64-bit TSIDs (time-ordered, monotonic) — serialized
// as a string once they exceed Number.MAX_SAFE_INTEGER (see
// `useNotificationStream.ts`'s `normNotificationItem`). BigInt comparison
// is the only safe way to order them regardless of representation.
function idToBigInt(id: number | string): bigint {
	return typeof id === "number" ? BigInt(Math.trunc(id)) : BigInt(id);
}

/** Cards shown for one burst before the remainder collapses into a summary. */
const MAX_TOASTS_PER_TICK = 3;
/** Upper bound on the per-effect "already toasted" memo. */
const SEEN_MESSAGE_LIMIT = 500;

/**
 * The messaging AREA, not any path containing the word. `includes("/messages")`
 * would silently swallow every toast on a future `/help/messages-faq`.
 */
function isMessagesArea(rawPathname: string): boolean {
	const pathname = stripLocale(rawPathname);
	return (
		pathname.startsWith("/account/messages") ||
		pathname.startsWith("/seller-center/messages") ||
		pathname.startsWith("/messages")
	);
}

export default function NotificationToastBridge() {
	const { activeRole, isLoggedIn, identity } = useAuth();
	const { count, loading } = useNotificationUnread();
	const { show } = useNotificationToast();

	const myId = identity?.accountId != null ? String(identity.accountId) : "";
	const workspaceKey = isLoggedIn
		? `${myId || "unknown"}:${activeRole ?? "none"}`
		: "guest";

	/* ── New chat messages (immediate, via socket) ─────────────── */
	React.useEffect(() => {
		if (!isLoggedIn) return;
		const sock = getChatSocket();
		sock.connect(); // idempotent; the singleton is never auto-disconnected
		const seen = new Set<string>();
		const unsub = sock.onMessage((m) => {
			const isMessage =
				m.type === "text" || m.type === "image" || m.type === "chat";
			if (!isMessage) return;
			if (myId && String(m.senderId) === myId) return; // my own outgoing
			if (m.id) {
				if (seen.has(m.id)) return;
				// Insertion-ordered: dropping from the front evicts the oldest.
				if (seen.size >= SEEN_MESSAGE_LIMIT) {
					for (const oldest of seen) {
						seen.delete(oldest);
						if (seen.size < SEEN_MESSAGE_LIMIT) break;
					}
				}
				seen.add(m.id);
			}
			// Don't toast while the user is already in the messaging area. Read
			// the path at event time (window.location) rather than usePathname()
			// at render — this bridge renders null and only needs the path inside
			// this callback, so reading it here keeps the component free of any
			// request-time data access and fully static-prerenderable.
			if (isMessagesArea(window.location.pathname)) return;

			const role = notificationWorkspaceRole(window.location.pathname);
			const base =
				role === "seller" ? "/seller-center/messages" : "/account/messages";
			show({
				kind: "message",
				title: "新訊息",
				body: messagePreview(m.content, m.type),
				href: `${base}/${m.senderId}`,
				dedupeKey: m.id ? `msg:${m.id}` : undefined,
				// Collapse a burst from the same sender into one stack.
				groupKey: `chat:${m.senderId}`,
			});
		});
		return unsub;
	}, [isLoggedIn, myId, show]);

	/* ── New orders / notifications (via unread-count increase) ─── */
	const primedRef = React.useRef(false);
	const prevCountRef = React.useRef(0);
	const prevLoadingRef = React.useRef(false);
	// High-water mark: the largest notification id already accounted for
	// (toasted, or known to exist before we started watching). Anything
	// with id <= this is never "new" — same cursor concept as the SSE
	// transport's own Last-Event-ID, applied at the toast-decision layer
	// so both paths below agree on one definition of "new" instead of
	// count arithmetic guessing which rows changed.
	const highWaterMarkRef = React.useRef<bigint>(BigInt(0));

	React.useEffect(() => {
		void workspaceKey;
		// Buyer and seller have independent inboxes. A role/account change starts
		// a fresh baseline instead of reusing the previous inbox's high-water mark.
		primedRef.current = false;
		prevCountRef.current = 0;
		prevLoadingRef.current = false;
		highWaterMarkRef.current = BigInt(0);
	}, [workspaceKey]);

	/* ── New orders / notifications (via SSE notification.new event) ─────
	 * Primary path. `useUnreadCount` re-publishes the SSE payload on
	 * `hk:notification:new` so we can toast immediately with the row data
	 * the server already pushed — no need to round-trip back to the inbox
	 * REST endpoint. The legacy count-rise effect below still runs as a
	 * fallback (e.g. cross-tab mark-unread bumps the count without firing
	 * `notification.new`), and dedupes against this path via
	 * `highWaterMarkRef` so the same row never double-toasts.
	 */
	React.useEffect(() => {
		if (!isLoggedIn) return;
		const onNotificationNew = (e: Event) => {
			const detail = (e as CustomEvent<NotificationNewEventDetail>).detail;
			const item = detail?.item;
			if (!item) return;
			// The direct SSE path already accounts for this unread-count rise. Move
			// the fallback baseline forward now so the count effect below does not
			// issue a redundant inbox GET for the same event.
			prevCountRef.current = Math.max(
				prevCountRef.current,
				Math.max(0, detail.unreadCount),
			);
			// Chat-type rows are handled by the socket path above so an SSE-
			// pushed message-type inbox row doesn't double-toast with the
			// websocket one.
			if (item.type === "message") return;
			if (item.isRead) return;
			const itemId = idToBigInt(item.id);
			if (itemId <= highWaterMarkRef.current) return;
			if (isMessagesArea(window.location.pathname)) return;
			highWaterMarkRef.current = itemId;
			const role =
				activeRole ?? notificationWorkspaceRole(window.location.pathname);
			const resolved = resolveNotification(item, role);
			show({
				kind: item.type === "order" ? "order" : "system",
				title: resolved.title,
				body: resolved.body,
				imageUrl: item.imageUrl,
				href: resolved.href,
				dedupeKey: `notif:${item.id}`,
				// Same notification title (e.g.「貼文被回覆」) collapses into one stack.
				groupKey: `notif:${resolved.title}`,
			});
		};
		window.addEventListener(NOTIFICATION_NEW_EVENT, onNotificationNew);
		return () =>
			window.removeEventListener(NOTIFICATION_NEW_EVENT, onNotificationNew);
	}, [activeRole, isLoggedIn, show]);

	React.useEffect(() => {
		void workspaceKey;
		if (!isLoggedIn) return;
		let cancelled = false;
		const wasLoading = prevLoadingRef.current;
		prevLoadingRef.current = loading;
		// Establish the real baseline only after the FIRST fetch settles
		// (loading true→false). The count is 0 before that first load, so
		// priming earlier would treat the initial 0→N load as N new toasts —
		// that's the "dumps everything on login" bug.
		if (!primedRef.current) {
			if (wasLoading && !loading) {
				primedRef.current = true;
				prevCountRef.current = count;
				// Mark everything that already exists (read or unread) as
				// pre-dating this session, regardless of backlog size — the
				// single newest row's id is the cutoff, no need to page
				// through the whole unread list.
				void fetchNotifications({ pageSize: 1 })
					.then((res) => {
						if (cancelled) return;
						const latest = res.records[0];
						if (!latest) return;
						const latestId = idToBigInt(latest.id);
						if (latestId > highWaterMarkRef.current) {
							highWaterMarkRef.current = latestId;
						}
					})
					.catch(() => {
						/* best-effort — worst case the backlog can resurface once */
					});
			}
			return () => {
				cancelled = true;
			};
		}
		// Only act on settled counts; ignore the in-flight true state of polls.
		if (loading) return;
		const prev = prevCountRef.current;
		prevCountRef.current = count;
		if (count <= prev) return;

		const gained = count - prev;
		void (async () => {
			try {
				const res = await fetchNotifications({ pageSize: 10 });
				if (cancelled) return;
				const fresh = res.records
					.filter(
						(n) =>
							!n.isRead &&
							n.type !== "message" &&
							idToBigInt(n.id) > highWaterMarkRef.current,
					)
					.slice(0, gained);
				// A burst of distinct titles used to stack one full-width card per
				// row down the screen. Show the newest few and collapse the rest
				// into a single count line.
				const shown = fresh.slice(0, MAX_TOASTS_PER_TICK);
				const overflow = fresh.slice(MAX_TOASTS_PER_TICK);
				// Every row in the burst still moves the high-water mark, or the
				// collapsed ones would toast again on the next count rise.
				for (const n of fresh) {
					const nId = idToBigInt(n.id);
					if (nId > highWaterMarkRef.current) highWaterMarkRef.current = nId;
				}
				// Oldest first so the newest ends up on top of the stack.
				for (const n of [...shown].reverse()) {
					const role =
						activeRole ?? notificationWorkspaceRole(window.location.pathname);
					const resolved = resolveNotification(n, role);
					show({
						kind: n.type === "order" ? "order" : "system",
						title: resolved.title,
						body: resolved.body,
						imageUrl: n.imageUrl,
						href: resolved.href,
						dedupeKey: `notif:${n.id}`,
						groupKey: `notif:${resolved.title}`,
					});
				}
				if (overflow.length > 0) {
					const overflowRole =
						activeRole ?? notificationWorkspaceRole(window.location.pathname);
					show({
						kind: "system",
						title: `還有 ${overflow.length} 則新通知`,
						href:
							overflowRole === "seller"
								? "/seller-center/notifications"
								: "/account/notifications",
						dedupeKey: `notif:overflow:${overflow[0].id}`,
						groupKey: "notif:overflow",
					});
				}
			} catch {
				/* transient fetch error — the next increase will retry */
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [activeRole, count, isLoggedIn, loading, show, workspaceKey]);

	return null;
}
