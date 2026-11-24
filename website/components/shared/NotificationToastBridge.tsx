"use client";

/**
 * Wires the in-app toast queue to real events:
 *
 *   • New messages — subscribes to the shared chat socket and toasts inbound
 *     peer messages immediately. Connects the singleton (idempotent; nothing
 *     disconnects it) so toasts fire app-wide, not just on chat pages. Skips
 *     the user's own messages and stays quiet while they're in the messages UI.
 *
 *   • New orders / notifications — the notification stream's inbox signal names
 *     the new inbox item ids; exactly those are toasted (visible tab only).
 *     Message-type inbox rows are left to the socket path above so a message
 *     never double-toasts.
 *
 * Renders nothing — mount it once inside the toast + unread providers.
 */

import { InboxPresentation } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { previewText } from "@/lib/chat/map";
import { listConversations } from "@/lib/chat/repo";
import type { ChatStreamEvent } from "@/lib/chat/stream";
import type { ChatMessage, Conversation } from "@/lib/chat/types";
import { useChatStream } from "@/lib/chat/useChatStream";
import { chatSideOf } from "@/lib/chat/workspace";
import { useUiText } from "@/lib/i18n/ui-text";
import { stripLocale } from "@/lib/intl/paths";
import { imageUri } from "@/lib/media/image";
import {
	NOTIFICATION_SIGNAL_EVENT,
	type NotificationSignalEventDetail,
} from "@/lib/notifications/events";
import {
	audienceForRole,
	type InboxItem,
	listInboxItems,
} from "@/lib/notifications/inbox";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";

/** Inbox rows read per signal: the server sends at most 20 new ids per signal. */
const SIGNAL_PAGE_SIZE = 20;
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
	const { activeRole, status, identity } = useAuth();
	const signedIn = status === "signedIn";
	const { show } = useNotificationToast();

	const myId = identity?.accountId != null ? String(identity.accountId) : "";
	const workspaceKey = signedIn
		? `${myId || "unknown"}:${activeRole ?? "none"}`
		: "guest";

	/* ── New chat messages (immediate, via the chat stream) ─────── */
	const ut = useUiText();
	const chatSide = chatSideOf(activeRole);
	const seenMessagesRef = React.useRef(new Set<string>());
	const peersRef = React.useRef(new Map<string, Conversation>());
	const lookupRef = React.useRef<Promise<void> | null>(null);
	const onChatEvent = React.useCallback(
		(e: ChatStreamEvent) => {
			if (e.type !== "message" || e.updated) return;
			const m: ChatMessage = e.message;
			if (m.sender === chatSide) return; // my own outgoing
			if (m.hidden || m.erased) return;
			const seen = seenMessagesRef.current;
			if (seen.has(m.id)) return;
			// Insertion-ordered: dropping from the front evicts the oldest.
			if (seen.size >= SEEN_MESSAGE_LIMIT) {
				for (const oldest of seen) {
					seen.delete(oldest);
					if (seen.size < SEEN_MESSAGE_LIMIT) break;
				}
			}
			seen.add(m.id);
			// Don't toast while the user is already in the messaging area. Read
			// the path at event time (window.location) rather than usePathname()
			// at render — this bridge renders null and only needs the path inside
			// this callback, so reading it here keeps the component free of any
			// request-time data access and fully static-prerenderable.
			if (isMessagesArea(window.location.pathname)) return;

			const role = notificationWorkspaceRole(window.location.pathname);
			const base =
				role === "seller" ? "/seller-center/messages" : "/account/messages";
			void (async () => {
				let peer = peersRef.current.get(m.conversationId);
				if (!peer) {
					// 串流的訊息只帶對話 ID：連結要對方的 ID，從對話列表取。
					// 同時來一串訊息時共用同一次讀取，不為每則各打一次。
					lookupRef.current ??= (async () => {
						try {
							for (const c of await listConversations(chatSide)) {
								peersRef.current.set(c.id, c);
							}
						} catch {
							// 讀不到對話列表：通知仍顯示，連結退到收件匣。
						} finally {
							lookupRef.current = null;
						}
					})();
					await lookupRef.current;
					peer = peersRef.current.get(m.conversationId);
				}
				show({
					kind: "message",
					title: "新訊息",
					body: previewText(ut, {
						kind: m.kind,
						text: m.text,
						hidden: m.hidden,
						erased: m.erased,
						sender: m.sender,
					}),
					href: peer ? `${base}/${peer.targetUserId}` : base,
					dedupeKey: `msg:${m.id}`,
					// Collapse a burst from the same conversation into one stack.
					groupKey: `chat:${m.conversationId}`,
				});
			})();
		},
		[chatSide, show, ut],
	);
	useChatStream(chatSide, signedIn, onChatEvent);

	/* ── New orders / notifications (via the inbox signal) ───────────────
	 * The server decides what is new: an inbox signal carries `newInboxItemIds`
	 * (rows created or aggregated since the previous signal on this stream) and
	 * only those rows are toasted, with the server-composed title, body and deep
	 * link. There is no get-by-id RPC, so the newest page is read once per
	 * signal and the ids picked from it. An aggregated bump repeats an id; the
	 * toast context replaces the toast that has the same `dedupeKey`.
	 * A truncated signal (too many new rows), a signal without ids (initial,
	 * reconnect, read, delete) and a hidden tab toast nothing. Message rows
	 * belong to the chat socket path above and never double-toast.
	 */
	const workspaceKeyRef = React.useRef(workspaceKey);
	workspaceKeyRef.current = workspaceKey;
	const audience = audienceForRole(activeRole ?? "buyer");

	React.useEffect(() => {
		if (!signedIn) return;
		let cancelled = false;
		const onSignal = (event: Event) => {
			const detail = (event as CustomEvent<NotificationSignalEventDetail>)
				.detail;
			const ids = detail?.newInboxItemIds ?? [];
			if (detail?.newItemsTruncated || ids.length === 0) return;
			if (document.visibilityState !== "visible") return;
			const requestedFor = workspaceKeyRef.current;
			void (async () => {
				try {
					const page = await listInboxItems({
						audience,
						pageSize: SIGNAL_PAGE_SIZE,
					});
					if (cancelled || workspaceKeyRef.current !== requestedFor) return;
					if (document.visibilityState !== "visible") return;
					if (isMessagesArea(window.location.pathname)) return;
					const wanted = new Set(ids);
					const fresh = page.items.filter(
						(n: InboxItem) =>
							wanted.has(n.id) &&
							!n.read &&
							n.presentation !== InboxPresentation.MESSAGE,
					);
					// Oldest first so the newest ends up on top of the stack.
					for (const n of [...fresh].reverse()) {
						show({
							kind:
								n.presentation === InboxPresentation.ORDER ? "order" : "system",
							title: n.title,
							body: n.body,
							imageUrl: imageUri(n.image, "thumb"),
							href: n.deepLink || undefined,
							dedupeKey: `notif:${n.id}`,
							// Same notification title (e.g.「貼文被回覆」) collapses into one stack.
							groupKey: `notif:${n.title}`,
						});
					}
				} catch {
					/* transient fetch error — the list refetch still shows the rows */
				}
			})();
		};
		window.addEventListener(NOTIFICATION_SIGNAL_EVENT, onSignal);
		return () => {
			cancelled = true;
			window.removeEventListener(NOTIFICATION_SIGNAL_EVENT, onSignal);
		};
	}, [audience, signedIn, show]);

	return null;
}
