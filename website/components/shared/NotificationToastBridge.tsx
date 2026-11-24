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
import { NOTIFICATION_SIGNAL_EVENT } from "@/lib/notifications/events";
import {
	audienceForRole,
	type InboxItem,
	listInboxItems,
} from "@/lib/notifications/inbox";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { useNotificationUnread } from "@/lib/notifications/unread-context";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";

/** Cards shown for one burst before the remainder collapses into a summary. */
const MAX_TOASTS_PER_TICK = 3;
/** Inbox rows read per signal: enough to cover one burst, the rest collapses. */
const SIGNAL_PAGE_SIZE = 10;
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
	const { baselineReady } = useNotificationUnread();
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
	 * The stream's inbox signal carries no row content, so on each signal the
	 * newest page is read and every unread, non-message row not seen yet is
	 * toasted with the server-composed title, body and deep link. `seenIdsRef`
	 * is the single definition of "new": the rows that existed when the
	 * workspace's baseline was established, plus every row already handled.
	 * Message rows belong to the chat socket path above and never double-toast.
	 */
	const primedRef = React.useRef(false);
	const seenIdsRef = React.useRef(new Set<string>());
	const workspaceKeyRef = React.useRef(workspaceKey);
	workspaceKeyRef.current = workspaceKey;
	const audience = audienceForRole(activeRole ?? "buyer");

	React.useEffect(() => {
		void workspaceKey;
		// Buyer and seller have independent inboxes. A role/account change starts
		// a fresh baseline instead of reusing the previous inbox's seen set.
		primedRef.current = false;
		seenIdsRef.current = new Set();
	}, [workspaceKey]);

	React.useEffect(() => {
		void workspaceKey;
		if (!signedIn || primedRef.current || !baselineReady) return;
		// Establish the baseline only once the count is the server's answer for
		// this workspace: everything that already exists (read or unread)
		// pre-dates this session and is never toasted — the "dumps everything on
		// login" bug.
		primedRef.current = true;
		const requestedFor = workspaceKey;
		void listInboxItems({ audience, pageSize: SIGNAL_PAGE_SIZE })
			.then((page) => {
				if (workspaceKeyRef.current !== requestedFor) return;
				for (const item of page.items) seenIdsRef.current.add(item.id);
			})
			.catch(() => {
				/* best-effort — worst case the backlog can resurface once */
			});
	}, [audience, baselineReady, signedIn, workspaceKey]);

	React.useEffect(() => {
		if (!signedIn) return;
		let cancelled = false;
		const onSignal = () => {
			if (!primedRef.current) return;
			const requestedFor = workspaceKeyRef.current;
			void (async () => {
				try {
					const page = await listInboxItems({
						audience,
						pageSize: SIGNAL_PAGE_SIZE,
					});
					if (cancelled || workspaceKeyRef.current !== requestedFor) return;
					const seen = seenIdsRef.current;
					const fresh = page.items.filter(
						(n: InboxItem) =>
							!n.read &&
							n.presentation !== InboxPresentation.MESSAGE &&
							!seen.has(n.id),
					);
					for (const n of page.items) seen.add(n.id);
					if (fresh.length === 0) return;
					if (isMessagesArea(window.location.pathname)) return;
					// A burst of distinct titles used to stack one full-width card per
					// row down the screen. Show the newest few and collapse the rest
					// into a single count line.
					const shown = fresh.slice(0, MAX_TOASTS_PER_TICK);
					const overflow = fresh.slice(MAX_TOASTS_PER_TICK);
					// Oldest first so the newest ends up on top of the stack.
					for (const n of [...shown].reverse()) {
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
					/* transient fetch error — the next signal will retry */
				}
			})();
		};
		window.addEventListener(NOTIFICATION_SIGNAL_EVENT, onSignal);
		return () => {
			cancelled = true;
			window.removeEventListener(NOTIFICATION_SIGNAL_EVENT, onSignal);
		};
	}, [activeRole, audience, signedIn, show]);

	return null;
}
