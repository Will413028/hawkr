"use client";

import "@/styles/features/messages.css";
import * as React from "react";
import ConversationSidebar from "@/features/messages/components/ConversationSidebar";
import { useConversations } from "@/lib/hooks/useChat";
import { useChatSidebarCollapsed } from "@/lib/hooks/useChatSidebarCollapsed";
import { usePathname } from "@/lib/intl/navigation";

/**
 * Seller-center variant of `app/account/messages/layout.tsx`. Same
 * persistent-sidebar shell; only the href base differs so links point
 * back into the seller workspace.
 */
const THREAD_HREF_BASE = "/seller-center/messages";

export default function SellerMessagesLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	const pathname = usePathname() || "";
	const isThreadRoute = pathname.startsWith(`${THREAD_HREF_BASE}/`);
	const activeTargetId = React.useMemo(() => {
		if (!isThreadRoute) return undefined;
		const rest = pathname.slice(THREAD_HREF_BASE.length + 1);
		const first = rest.split("/")[0] ?? "";
		try {
			return decodeURIComponent(first);
		} catch {
			return first;
		}
	}, [isThreadRoute, pathname]);
	const { items, loading, error, refresh } = useConversations();
	const [collapsed, toggleCollapsed] = useChatSidebarCollapsed();

	if (!isThreadRoute) {
		return <>{children}</>;
	}

	return (
		<div
			className="chat-page"
			data-chat-view="thread"
			data-chat-workspace="seller"
		>
			<div className="chat-shell" data-sidebar-collapsed={collapsed}>
				<ConversationSidebar
					items={items}
					loading={loading}
					error={error}
					activeTargetId={activeTargetId}
					threadHrefBase={THREAD_HREF_BASE}
					peerKind="buyer"
					title="訊息"
					collapsed={collapsed}
					onToggleCollapsed={toggleCollapsed}
					onRefresh={() => void refresh()}
				/>
				{children}
			</div>
		</div>
	);
}
