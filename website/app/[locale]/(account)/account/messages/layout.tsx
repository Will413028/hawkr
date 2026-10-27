"use client";

import "@/styles/features/messages.css";
import "@/styles/features/procurement-assistant-sidebar.css";
import * as React from "react";
import ConversationSidebar, {
	type AgentConversationRow,
} from "@/features/messages/components/ConversationSidebar";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import {
	ASSISTANT_PROCUREMENT_PATH,
	assistantConversationPath,
	isAssistantConversationId,
} from "@/features/procurement-assistant/routes";
import { useAgentConversations } from "@/features/procurement-assistant/useAgentConversations";
import { useConversations } from "@/lib/hooks/useChat";
import { useChatSidebarCollapsed } from "@/lib/hooks/useChatSidebarCollapsed";
import { usePathname } from "@/lib/intl/navigation";
import { isAccountMessageThreadPath } from "@/lib/navigation/message-thread";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/**
 * Persistent shell for `/account/messages` and `/account/messages/[targetId]`.
 *
 * Owning the sidebar at the layout level (rather than inside each
 * page-level client component) keeps the conversation list mounted
 * across navigation between the list view and individual threads —
 * the previous structure re-mounted everything on every click, which
 * read as a "flash" because the avatar/name/list each transitioned
 * through their loading state again.
 */
const THREAD_HREF_BASE = "/account/messages";

export default function MessagesLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	const pathname = usePathname() || "";
	const router = useObservedRouter("chat_sidebar");
	const isAssistantPath =
		pathname === `${THREAD_HREF_BASE}/assistant` ||
		pathname.startsWith(`${THREAD_HREF_BASE}/assistant/`);
	const isThreadRoute = isAccountMessageThreadPath(pathname);
	const activeTargetId = React.useMemo(() => {
		if (isAssistantPath || !isThreadRoute) return undefined;
		const rest = pathname.slice(THREAD_HREF_BASE.length + 1);
		const first = rest.split("/")[0] ?? "";
		try {
			return decodeURIComponent(first);
		} catch {
			return first;
		}
	}, [isAssistantPath, isThreadRoute, pathname]);
	// 目前開著的 agent 對話 id（/assistant/{id} 的第二段）。既有的
	// isAssistantPath 是前綴比對、分不出列；per-row 高亮需要這個。
	const activeAgentConversationId = React.useMemo(() => {
		if (!isAssistantPath) return undefined;
		const segment =
			pathname.slice(`${THREAD_HREF_BASE}/assistant/`.length).split("/")[0] ??
			"";
		return isAssistantConversationId(segment) ? segment : undefined;
	}, [isAssistantPath, pathname]);
	const { items, loading, error, refresh } = useConversations();
	const agent = useAgentConversations(HAWKR_AGENT_ENABLED);
	const [collapsed, toggleCollapsed] = useChatSidebarCollapsed();

	const agentRows = React.useMemo<AgentConversationRow[]>(() => {
		if (!HAWKR_AGENT_ENABLED) return [];
		return agent.conversations.map((row) => ({
			id: row.conversationId,
			href: assistantConversationPath(row.conversationId),
			title: row.title,
			preview: row.lastMessagePreview,
			// Legacy/invalid timestamps should not silently pick January 1970's
			// winter artwork. Keep the row and use the current Taipei season.
			updatedAt: Date.parse(row.updatedAt) || Date.now(),
			primaryCategoryId: row.primaryCategoryId,
			active: row.conversationId === activeAgentConversationId,
		}));
	}, [agent.conversations, activeAgentConversationId]);

	return (
		<main
			className="chat-page"
			data-chat-view={isThreadRoute ? "thread" : "list"}
			data-chat-workspace="buyer"
		>
			<div className="chat-shell" data-sidebar-collapsed={collapsed}>
				<ConversationSidebar
					items={items}
					loading={loading}
					error={error}
					activeTargetId={activeTargetId}
					threadHrefBase={THREAD_HREF_BASE}
					peerKind="seller"
					title="訊息"
					collapsed={collapsed}
					onToggleCollapsed={toggleCollapsed}
					// 錯誤橫幅的「重試」與 header 的重新整理都 gate 在 onRefresh；
					// 之前兩個 layout 都沒傳，等於文案叫使用者再試一次卻沒東西可按。
					onRefresh={() => {
						void refresh();
						if (HAWKR_AGENT_ENABLED) agent.refresh();
					}}
					onAssistantRoute={HAWKR_AGENT_ENABLED && isAssistantPath}
					assistantEntry={
						HAWKR_AGENT_ENABLED
							? {
									// active 只供初始分頁的路由判定；既有對話列的 aria-current
									// 高亮則交給 agentConversations 管理。
									active: agent.available
										? isAssistantPath && !activeAgentConversationId
										: isAssistantPath,
								}
							: undefined
					}
					agentConversations={agentRows}
					agentLoading={HAWKR_AGENT_ENABLED && agent.loading}
					agentAvailable={HAWKR_AGENT_ENABLED && agent.available}
					agentHasMore={HAWKR_AGENT_ENABLED && agent.hasMore}
					onNewAgentChat={
						HAWKR_AGENT_ENABLED && agent.available
							? () => router.push(ASSISTANT_PROCUREMENT_PATH)
							: undefined
					}
					onAgentRowRename={agent.rename}
					onAgentRowPrefetch={(row) => void agent.prefetch(row.id)}
					onAgentRowDelete={(row) => {
						// 刪掉正在看的那個對話 → 回到「新對話」空白畫面，不要
						// 留在一個已經不存在的深連結上（那會打 404 → RESTORE_FAILED）。
						if (row.id === activeAgentConversationId)
							router.replace(ASSISTANT_PROCUREMENT_PATH);
						void agent.remove(row.id);
					}}
				/>
				{children}
			</div>
		</main>
	);
}
