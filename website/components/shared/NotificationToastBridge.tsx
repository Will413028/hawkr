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

	const ut = useUiText();
	const chatSide = chatSideOf(activeRole);
	const seenMessagesRef = React.useRef(new Set<string>());
	const peersRef = React.useRef(new Map<string, Conversation>());
	const lookupRef = React.useRef<Promise<void> | null>(null);
	const onChatEvent = React.useCallback(
		(e: ChatStreamEvent) => {
			if (e.type !== "message" || e.updated) return;
			const m: ChatMessage = e.message;
			if (m.sender === chatSide) return;
			if (m.hidden || m.erased) return;
			const seen = seenMessagesRef.current;
			if (seen.has(m.id)) return;
			if (seen.size >= SEEN_MESSAGE_LIMIT) {
				for (const oldest of seen) {
					seen.delete(oldest);
					if (seen.size < SEEN_MESSAGE_LIMIT) break;
				}
			}
			seen.add(m.id);
			if (isMessagesArea(window.location.pathname)) return;

			const role = notificationWorkspaceRole(window.location.pathname);
			const base =
				role === "seller" ? "/seller-center/messages" : "/account/messages";
			void (async () => {
				let peer = peersRef.current.get(m.conversationId);
				if (!peer) {
					lookupRef.current ??= (async () => {
						try {
							for (const c of await listConversations(chatSide)) {
								peersRef.current.set(c.id, c);
							}
						} catch {
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
					groupKey: `chat:${m.conversationId}`,
				});
			})();
		},
		[chatSide, show, ut],
	);
	useChatStream(chatSide, signedIn, onChatEvent);

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
					for (const n of [...fresh].reverse()) {
						show({
							kind:
								n.presentation === InboxPresentation.ORDER ? "order" : "system",
							title: n.title,
							body: n.body,
							imageUrl: imageUri(n.image, "thumb"),
							href: n.deepLink || undefined,
							dedupeKey: `notif:${n.id}`,
							groupKey: `notif:${n.title}`,
						});
					}
				} catch {}
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
