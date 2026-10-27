"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import ConversationActionsMenu from "@/features/procurement-assistant/ConversationActionsMenu";
import HawkrAgentConversationMarble from "@/features/procurement-assistant/HawkrAgentConversationMarble";
import HawkrAgentMark from "@/features/procurement-assistant/HawkrAgentMark";
import type { HawkrAgentCategoryId } from "@/lib/animation/hawkr-agent-season";
import { conversationRangeLabel } from "@/lib/chat/list-range";
import { messagePreview } from "@/lib/chat/preview";
import type { Conversation } from "@/lib/chat/types";
import { formatTaipeiShortDate } from "@/lib/date/taipei";
import Link from "@/lib/intl/native-link";

interface ConversationSidebarProps {
	items: Conversation[];
	loading?: boolean;
	error?: string | null;
	activeTargetId?: string;
	threadHrefBase: string;
	/** Account inbox peers are sellers; seller-center inbox peers are buyers. */
	peerKind: "seller" | "buyer";
	title?: string;
	subtitle?: string;
	onRefresh?: () => void;
	/** True when the parent has set `data-sidebar-collapsed="true"` on
	 * the chat-shell; the sidebar then renders an icon-rail layout. */
	collapsed?: boolean;
	/** Toggle handler — wired to the collapse / expand button in the
	 * header. Omit to hide the button entirely (legacy callers / mobile
	 * fallback). */
	onToggleCollapsed?: () => void;
	/** Presence enables the product-owned Agent tab; `active` seeds its route state. */
	assistantEntry?: {
		active?: boolean;
	};
	/** true＝目前正在看某則 Hawkr Agent 對話（含 URL 已被 replaceState 改成
	 * 對話 id 的情形）。決定分頁的預設落點，見 `routeSection`。 */
	onAssistantRoute?: boolean;
	/** Hawkr Agent 對話列（B-244）。每個對話一列，依 updatedAt 新→舊由
	 * 呼叫端排好。空陣列＝不渲染區段（後端列表端點未上線時的降級）。 */
	agentConversations?: AgentConversationRow[];
	/** agent 列表載入中。沒有這個旗標時，冷啟動會先渲染「還沒有 Agent 對話」，
	 * 等於對一個有 12 筆對話的買家宣稱他的東西不見了。 */
	agentLoading?: boolean;
	/** false＝列表端點未上線或降級。此時不能顯示「你沒有對話」——那是謊言，
	 * 要顯示服務暫時無法載入並給重試。 */
	agentAvailable?: boolean;
	/** 後端還有更多 agent 對話沒載入。用來讓「找不到」的文案誠實。 */
	agentHasMore?: boolean;
	/** 有值時「Hawkr Agent」分頁的列表頂端多一列常駐的「開新對話」。 */
	onNewAgentChat?: () => void;
	/** 有值時「⋯」選單裡有「重新命名」，選了之後由該列就地編輯。
	 * 回傳 false＝儲存失敗（呼叫端已 toast），列回到唯讀態。 */
	onAgentRowRename?: (id: string, title: string) => Promise<boolean>;
	/** 滑過／聚焦某一列時預抓該對話（冷快取切換會閃骨架，見
	 * useAgentConversations.prefetch 的說明）。 */
	onAgentRowPrefetch?: (row: AgentConversationRow) => void;
	/** hover 時直接露出的刪除鈕。刪除是樂觀＋6 秒「復原」toast（逾時才真的
	 * 送出軟刪），所以不再多一層確認 modal——多一步只會擋住可回復的動作。 */
	onAgentRowDelete?: (row: AgentConversationRow) => void;
}

export interface AgentConversationRow {
	id: string;
	href: string;
	title: string;
	preview: string;
	/** epoch ms；沿用人類對話列的相對時間格式。 */
	updatedAt: number;
	/** 後端依目前需求品項對到的九大分類；舊資料或不明確時不提供。 */
	primaryCategoryId?: HawkrAgentCategoryId | null;
	active?: boolean;
}

/**
 * 側欄只有一列分頁。有 agent 對話時它是「賣家／Hawkr Agent」兩段——agent
 * 列以前是釘在人類列表上方的不可壓縮區塊（`flex-shrink: 0` 且沒有高度
 * 上限），對話一多就把人類列表擠到 0px，而且 agent 區在捲動容器外面，
 * 捲也捲不到賣家訊息。分頁讓兩邊各自佔滿整個可捲區域。
 *
 * 沒有 agent 對話時（賣家中心收件匣）維持原本的「全部／未讀」，不為了
 * 一個永遠只有一段的清單長出一個只有單一分頁的 tablist。
 */
type FilterKey = "all" | "unread";
type SectionKey = "human" | "agent";

function formatTime(ts: number): string {
	if (!Number.isFinite(ts) || ts <= 0) return "";
	const ms = ts < 1e12 ? ts * 1000 : ts;
	const d = new Date(ms);
	if (Number.isNaN(d.getTime())) return "";
	const diff = Date.now() - d.getTime();
	const min = Math.floor(diff / 60_000);
	if (min < 1) return "剛剛";
	if (min < 60) return `${min} 分鐘前`;
	const hr = Math.floor(min / 60);
	if (hr < 24) return `${hr} 小時前`;
	const day = Math.floor(hr / 24);
	if (day < 7) return `${day} 天前`;
	return formatTaipeiShortDate(ms);
}

/**
 * Outlined chevron used for the collapse / expand toggle. Points the
 * direction the click will move the sidebar's outer edge — left when
 * expanded (collapsing leftward), right when collapsed (expanding
 * rightward). Matches the stroke vocabulary of the navbar / megamenu
 * icons elsewhere in the app.
 */
function CollapseIcon({ collapsed }: { collapsed: boolean }) {
	return (
		<AppIcon
			name="chevron-left"
			size={18}
			title={collapsed ? "展開側欄" : "收起側欄"}
			style={{
				transform: collapsed ? "rotate(180deg)" : undefined,
				transition: "transform 200ms cubic-bezier(0.16, 1, 0.3, 1)",
			}}
		/>
	);
}

/** 標題規則與後端契約一致：trim 後 1–40 runes、禁換行（超出後端回
 * `CONVERSATION_TITLE_INVALID`）。 */
const MAX_TITLE_RUNES = 40;

/** Must stay identical to the icon-rail media query in messages.css. */
const ICON_RAIL_MEDIA_QUERY = "(min-width: 960px) and (max-width: 1199px)";

/**
 * Whether CSS can actually render the 72px icon rail.
 *
 * Start false on both the server and the first client render so hydration stays
 * stable. The persisted `collapsed` preference intentionally survives every
 * viewport, but row-level content should only mount inside the one band
 * where CSS exposes them.
 */
function useIconRailViewport(): boolean {
	const [matches, setMatches] = React.useState(false);

	React.useEffect(() => {
		if (typeof window.matchMedia !== "function") return;
		const media = window.matchMedia(ICON_RAIL_MEDIA_QUERY);
		const sync = () => setMatches(media.matches);
		sync();
		media.addEventListener("change", sync);
		return () => media.removeEventListener("change", sync);
	}, []);

	return matches;
}

/**
 * 就地改名的輸入框。取代原本從右側滑出的 Drawer——改個名字不值得一整面
 * 板，而且選單彈出後緊接著滑出面板是兩段動畫疊在一起。
 *
 * Enter 儲存、Esc 取消、失焦儲存（ChatGPT 同款）。空白或超長就當取消，
 * 不擋在編輯態不放人——這是低風險欄位，不需要錯誤訊息卡住流程。
 */
function RenameField({
	initial,
	onSubmit,
	onCancel,
}: {
	initial: string;
	onSubmit: (title: string) => void | Promise<void>;
	onCancel: () => void;
}) {
	const [value, setValue] = React.useState(initial);
	const ref = React.useRef<HTMLInputElement | null>(null);
	React.useEffect(() => {
		ref.current?.select();
	}, []);

	const trimmed = value.replaceAll(/[\n\r]/g, " ").trim();
	const runes = Array.from(trimmed).length;
	// 超長以前是靜默還原——使用者打了 60 個字、按 Enter、標題變回原樣，
	// 而且沒有任何地方說過有 40 字上限。現在即時說出來。
	const tooLong = runes > MAX_TITLE_RUNES;

	const commit = () => {
		if (runes < 1 || tooLong || trimmed === initial) {
			onCancel();
			return;
		}
		void onSubmit(trimmed);
	};

	return (
		<span className="chat-sidebar__rename">
			<input
				ref={ref}
				type="text"
				className="chat-sidebar__rename-input"
				value={value}
				maxLength={MAX_TITLE_RUNES * 2}
				aria-label="對話名稱"
				aria-invalid={tooLong || undefined}
				aria-describedby={tooLong ? "chat-rename-hint" : undefined}
				onChange={(event) => setValue(event.target.value)}
				onBlur={commit}
				onKeyDown={(event) => {
					if (event.key === "Escape") {
						event.stopPropagation();
						onCancel();
						return;
					}
					if (event.key !== "Enter" || event.nativeEvent.isComposing) return;
					event.preventDefault();
					commit();
				}}
			/>
			{tooLong ? (
				<small className="chat-sidebar__rename-hint" id="chat-rename-hint">
					{`最多 ${MAX_TITLE_RUNES} 字，目前 ${runes} 字`}
				</small>
			) : null}
		</span>
	);
}

/**
 * Cold-load placeholder that keeps the conversation-list geometry stable.
 * The shapes are decorative; one concise status announcement is enough for
 * assistive technology while the panel exposes `aria-busy`.
 */
function ConversationListLoading() {
	return (
		<div className="chat-sidebar__loading" role="status" aria-live="polite">
			<span className="sr-only">載入對話中…</span>
			{[0, 1, 2].map((row) => (
				<span
					key={row}
					className="chat-sidebar__loading-row"
					aria-hidden="true"
				>
					<span className="chat-sidebar__loading-avatar" />
					<span className="chat-sidebar__loading-copy">
						<span />
						<span />
					</span>
				</span>
			))}
		</div>
	);
}

export default function ConversationSidebar({
	items,
	loading = false,
	error = null,
	activeTargetId,
	threadHrefBase,
	peerKind,
	title = "訊息中心",
	subtitle,
	onRefresh,
	collapsed = false,
	onToggleCollapsed,
	assistantEntry,
	onAssistantRoute = false,
	agentConversations = [],
	agentLoading = false,
	agentAvailable = true,
	agentHasMore = false,
	onNewAgentChat,
	onAgentRowRename,
	onAgentRowPrefetch,
	onAgentRowDelete,
}: ConversationSidebarProps) {
	const [query, setQuery] = React.useState("");
	const iconRailViewport = useIconRailViewport();
	/** 正在就地改名的那一列。同時只允許一列在編輯態。 */
	const [renamingId, setRenamingId] = React.useState<string | null>(null);
	const [filter, setFilter] = React.useState<FilterKey>("all");
	/** null＝還沒手動切過分頁，跟著當前對話走（見下方 `section`）。 */
	const [sectionChoice, setSectionChoice] = React.useState<SectionKey | null>(
		null,
	);

	// 沒有標題也沒有任何訊息的對話＝點了「＋」卻沒輸入就離開的殘骸。一整排
	// 「未命名對話」既無法分辨也點不出東西，所以不進列表。正在看的那一個
	// 例外——把使用者當下所在的列從側欄抽走會讓人以為對話被刪了。
	const agentRows = React.useMemo(
		() =>
			agentConversations.filter(
				(row) => row.active || Boolean(row.title) || Boolean(row.preview),
			),
		[agentConversations],
	);
	// 分頁的有無跟著「這個收件匣本身有沒有 Agent」，不是「現在剛好有幾個
	// agent 對話」——後者會讓同一頁在新帳號、清空對話後、或列表端點暫時
	// 降級時各長成不一樣的樣子。買家收件匣一定有 assistantEntry，賣家中心
	// 沒有，剛好就是這條界線。
	const hasAgentSection = Boolean(assistantEntry);
	const compactSectionTabs = hasAgentSection && collapsed && iconRailViewport;

	const visibleItems = React.useMemo(() => {
		const q = query.trim().toLowerCase();
		return items.filter((item) => {
			// 「未讀」只在沒有 agent 分頁時存在（見 FilterKey 的說明）。
			if (!hasAgentSection && filter === "unread" && item.unreadCount <= 0)
				return false;
			if (!q) return true;
			return (
				item.targetNickname.toLowerCase().includes(q) ||
				String(item.targetUserId).includes(q) ||
				item.lastMessage.toLowerCase().includes(q)
			);
		});
	}, [items, query, filter, hasAgentSection]);

	// Agent 列吃搜尋、但沒有未讀語意，所以不套未讀篩選。
	const visibleAgentRows = React.useMemo(() => {
		const q = query.trim().toLowerCase();
		if (!q) return agentRows;
		return agentRows.filter(
			(row) =>
				row.title.toLowerCase().includes(q) ||
				row.preview.toLowerCase().includes(q),
		);
	}, [agentRows, query]);

	// 分頁旁的數字在所有訊息產品都代表「未讀」，所以這裡就給未讀數，不給總數。
	// agent 對話沒有未讀語意，那顆分頁因此不帶數字（WhatsApp 的 chip rail 同理：
	// 只有 Unread 有數字）。
	const unreadCount = React.useMemo(
		() =>
			items.reduce((total, item) => total + Math.max(0, item.unreadCount), 0),
		[items],
	);

	// 分頁預設跟著「你現在在讀哪一則」，不是寫死「賣家」。寫死的後果是：點進
	// 一則 Agent 對話後，側欄停在賣家分頁——你正在讀的那則不在看得到的清單裡，
	// 而且「開新對話」也在另一個分頁後面（部署被擋下來時就是卡在這）。
	// 手動切過之後以使用者的選擇為準，不再被路由拉回去。
	// 用路由本身，不用資料反推。`assistantEntry.active` 只在停在
	// /assistant/procurement 時為真，而第一輪之後 URL 已被 replaceState 改成
	// 對話 id；`row.active` 則要等列表載回來、且那則真的在列表裡。兩個訊號都
	// 會在「剛開始的新對話」這個最常見的情況下同時是 false。
	const routeSection: SectionKey =
		onAssistantRoute ||
		assistantEntry?.active ||
		agentRows.some((r) => r.active)
			? "agent"
			: "human";
	const section = sectionChoice ?? routeSection;
	const setSection = setSectionChoice;

	const showAgentList = hasAgentSection && section === "agent";

	// 分頁數字改成未讀之後，「搜到的東西在另一個分頁」就不能再靠數字表達，
	// 改成在空狀態裡直接講出來並給一個可點的切換。
	const query_ = query.trim();
	const otherTabMatches = showAgentList
		? visibleItems.length
		: visibleAgentRows.length;
	const crossTabHint =
		query_ && otherTabMatches > 0
			? {
					label: showAgentList
						? peerKind === "seller"
							? "賣家"
							: "買家"
						: "Hawkr Agent",
					count: otherTabMatches,
					go: () => setSection(showAgentList ? "human" : "agent"),
				}
			: null;

	// 兩份清單都以 pageSize 50 抓取。剛好拿滿就有可能被截斷——這種時候
	// 不可以斬釘截鐵說「找不到對話」，那會對一個真的存在的對話說謊。
	const humanMaybeTruncated = items.length >= 50;
	const activeListMaybeTruncated = showAgentList
		? agentHasMore
		: humanMaybeTruncated;

	// 清單底部的「顯示 1 - 6 / 6」。兩個數字都取**當前分頁**的，不把兩份清單
	// 加總——使用者看得到的就是其中一份，加總會給出一個畫面上數不出來的數字。
	// 載入中與空清單都會落到 `visible === 0`，由 helper 自己回 null，所以這裡
	// 不需要再判一次載入旗標。
	const rangeLabel = conversationRangeLabel({
		visible: showAgentList ? visibleAgentRows.length : visibleItems.length,
		loaded: showAgentList ? agentRows.length : items.length,
		maybeTruncated: activeListMaybeTruncated,
	});

	return (
		<aside
			className="chat-sidebar"
			aria-label="聊天列表"
			data-collapsed={collapsed ? "true" : undefined}
		>
			<header className="chat-sidebar__head">
				<div className="chat-sidebar__head-meta">
					{onAssistantRoute ? <h2>{title}</h2> : <h1>{title}</h1>}
					{subtitle ? <p>{subtitle}</p> : null}
				</div>
				<div className="chat-sidebar__head-actions">
					{onRefresh ? (
						<button
							type="button"
							className="chat-sidebar__refresh"
							onClick={onRefresh}
							disabled={loading || agentLoading}
							aria-label="重新整理訊息"
						>
							<AppIcon name="refresh" size={18} />
						</button>
					) : null}
					{onToggleCollapsed ? (
						<button
							type="button"
							className="chat-sidebar__collapse"
							onClick={onToggleCollapsed}
							aria-label={collapsed ? "展開側欄" : "收起側欄"}
							aria-expanded={!collapsed}
						>
							<CollapseIcon collapsed={collapsed} />
						</button>
					) : null}
				</div>
			</header>

			<div className="chat-sidebar__search">
				<AppIcon
					name="search"
					size={19}
					className="chat-sidebar__search-icon"
				/>
				<input
					type="search"
					value={query}
					onChange={(event) => setQuery(event.target.value)}
					placeholder="搜尋對話、商家或訊息"
					aria-label="搜尋對話"
				/>
			</div>

			<div
				className="chat-sidebar__filters"
				role="tablist"
				data-kind={hasAgentSection ? "sections" : "filters"}
				aria-label={hasAgentSection ? "訊息分類" : "訊息篩選"}
				aria-orientation={compactSectionTabs ? "vertical" : "horizontal"}
				onKeyDown={(event) => {
					// Expanded tabs are horizontal; the compact icon rail is vertical.
					// Only consume the arrows promised by the current ARIA orientation,
					// otherwise ArrowUp/Down would unexpectedly block page scrolling.
					const orientationKeys = compactSectionTabs
						? ["ArrowUp", "ArrowDown"]
						: ["ArrowLeft", "ArrowRight"];
					const keys = [...orientationKeys, "Home", "End"];
					if (!keys.includes(event.key)) return;
					event.preventDefault();
					const tabs = Array.from(
						event.currentTarget.querySelectorAll<HTMLButtonElement>(
							'[role="tab"]',
						),
					);
					if (tabs.length === 0) return;
					const current = tabs.indexOf(
						document.activeElement as HTMLButtonElement,
					);
					const next =
						event.key === "Home"
							? 0
							: event.key === "End"
								? tabs.length - 1
								: (Math.max(0, current) +
										(event.key ===
										(compactSectionTabs ? "ArrowDown" : "ArrowRight")
											? 1
											: tabs.length - 1)) %
									tabs.length;
					tabs[next]?.focus();
					tabs[next]?.click();
				}}
			>
				{hasAgentSection ? (
					<>
						<button
							type="button"
							role="tab"
							id="chat-tab-human"
							aria-controls="chat-panel-human"
							aria-selected={section === "human"}
							aria-label={peerKind === "seller" ? "賣家" : "買家"}
							tabIndex={section === "human" ? 0 : -1}
							data-active={section === "human"}
							title={
								compactSectionTabs
									? peerKind === "seller"
										? "賣家"
										: "買家"
									: undefined
							}
							onClick={() => setSection("human")}
						>
							<span className="chat-sidebar__filter-label">
								<span className="chat-sidebar__filter-label-text">
									{peerKind === "seller" ? "賣家" : "買家"}
								</span>
								<AppIcon
									name="work"
									size={18}
									className="chat-sidebar__filter-icon"
									aria-hidden="true"
								/>
							</span>
							{/* 數字＝未讀，跟所有訊息產品一致；0 不印。 */}
							{unreadCount > 0 ? (
								<span className="chat-sidebar__filter-count">
									{unreadCount > 99 ? "99+" : unreadCount}
									<span className="sr-only"> 則未讀</span>
								</span>
							) : null}
						</button>
						<button
							type="button"
							role="tab"
							id="chat-tab-agent"
							aria-controls="chat-panel-agent"
							aria-selected={section === "agent"}
							aria-label="Hawkr Agent"
							tabIndex={section === "agent" ? 0 : -1}
							data-active={section === "agent"}
							title={compactSectionTabs ? "Hawkr Agent" : undefined}
							onClick={() => setSection("agent")}
						>
							<span className="chat-sidebar__filter-label">
								<span className="chat-sidebar__filter-label-text">
									Hawkr Agent
								</span>
								<HawkrAgentMark
									className="chat-sidebar__filter-agent-mark"
									hideBadge
								/>
							</span>
						</button>
					</>
				) : (
					<>
						<button
							type="button"
							role="tab"
							id="chat-tab-all"
							aria-controls="chat-panel-human"
							aria-selected={filter === "all"}
							tabIndex={filter === "all" ? 0 : -1}
							data-active={filter === "all"}
							onClick={() => setFilter("all")}
						>
							全部
						</button>
						<button
							type="button"
							role="tab"
							id="chat-tab-unread"
							aria-controls="chat-panel-human"
							aria-selected={filter === "unread"}
							tabIndex={filter === "unread" ? 0 : -1}
							data-active={filter === "unread"}
							onClick={() => setFilter("unread")}
						>
							未讀
							{unreadCount > 0 ? (
								<span className="chat-sidebar__filter-count">
									{unreadCount > 99 ? "99+" : unreadCount}
								</span>
							) : null}
						</button>
					</>
				)}
			</div>

			{error ? (
				<div className="chat-sidebar__error" role="alert">
					<span>{error}</span>
					{onRefresh ? (
						<button type="button" onClick={onRefresh}>
							重試
						</button>
					) : null}
				</div>
			) : null}

			{hasAgentSection ? (
				<div
					id={showAgentList ? "chat-panel-human" : "chat-panel-agent"}
					role="tabpanel"
					aria-labelledby={showAgentList ? "chat-tab-human" : "chat-tab-agent"}
					hidden
				/>
			) : null}
			<div
				className="chat-sidebar__panel"
				id={showAgentList ? "chat-panel-agent" : "chat-panel-human"}
				role="tabpanel"
				aria-labelledby={
					showAgentList
						? "chat-tab-agent"
						: hasAgentSection
							? "chat-tab-human"
							: filter === "unread"
								? "chat-tab-unread"
								: "chat-tab-all"
				}
				aria-busy={showAgentList ? agentLoading : loading}
			>
				{/* 開新對話是常駐的：它以前掛在 header 的方框 ＋ 上，那顆拿掉之後
				 * 入口只剩空狀態那一顆——買家一旦有了任何一則對話，側欄裡就再也開
				 * 不了新的（`onNewAgentChat` 還在傳，只是沒有地方渲染）。
				 *
				 * 放在列表頂端而不是 header：它是「這份清單的第一個項目」，跟著清
				 * 單捲動的層級關係比浮在標題列上更直接。
				 *
				 * 使用 Edit 圖示，而不是曾被移除的方框 Plus：它表達「開始撰寫」且不會
				 * 把這列誤讀成清單裡的新增資料方框。 */}
				{showAgentList && onNewAgentChat ? (
					<button
						type="button"
						className="chat-sidebar__new-chat"
						onClick={onNewAgentChat}
						aria-label="開新對話"
						title={collapsed ? "開新對話" : undefined}
					>
						<span className="chat-sidebar__new-chat-icon" aria-hidden="true">
							<AppIcon name="edit" size={18} />
						</span>
						<span className="chat-sidebar__new-chat-label">開新對話</span>
					</button>
				) : null}

				{/* 載入中／端點降級／真的沒有／搜不到，是四種不同狀態，不可以共用
				 * 同一句 placeholder（DESIGN.md：空表、載入、錯誤、無搜尋結果與無
				 * 權限是不同狀態）。 */}
				{showAgentList && visibleAgentRows.length === 0 ? (
					agentLoading ? (
						<ConversationListLoading />
					) : !agentAvailable ? (
						<div className="chat-sidebar__empty">
							<span className="chat-sidebar__empty-icon" aria-hidden="true">
								<AppIcon name="danger" size={26} />
							</span>
							<strong>Hawkr Agent 對話暫時無法載入</strong>
							<span>服務恢復後就會回來，你的對話沒有消失。</span>
							{onRefresh ? (
								<button
									type="button"
									className="chat-sidebar__empty-action"
									onClick={onRefresh}
								>
									重試
								</button>
							) : null}
						</div>
					) : (
						<div className="chat-sidebar__empty">
							<span className="chat-sidebar__empty-icon" aria-hidden="true">
								<AppIcon name="chat" size={26} />
							</span>
							<strong>{query ? "找不到對話" : "還沒有 Agent 對話"}</strong>
							<span>
								{query
									? agentHasMore
										? "目前只載入最近的對話，更早的還沒抓下來。"
										: "換個關鍵字再試一次。"
									: "說出你要找的商品與數量，Hawkr Agent 會整理成可報價的需求。"}
							</span>
							{crossTabHint ? (
								<button
									type="button"
									className="chat-sidebar__empty-action chat-sidebar__empty-action--quiet"
									onClick={crossTabHint.go}
								>
									{`${crossTabHint.label}有 ${crossTabHint.count} 筆符合`}
									<AppIcon name="arrow-right" size={16} />
								</button>
							) : null}
						</div>
					)
				) : null}

				{showAgentList && visibleAgentRows.length > 0 ? (
					<nav className="chat-sidebar__agent" aria-label="Hawkr Agent 對話">
						<ul className="chat-sidebar__list chat-sidebar__list--agent">
							{visibleAgentRows.map((row) => {
								// 後端的自動標題要「品項＋配送城市」都齊了才產生，而且
								// backfill 出來的舊對話 title 是空字串——只退回「未命名對話」
								// 的話，一整排列會長得一模一樣、無法分辨。沒有標題就用最後
								// 一則訊息當列標（那是真實內容），並且不再重複印一次預覽。
								const label = row.title || row.preview || "未命名對話";
								const showPreview = Boolean(row.title) && Boolean(row.preview);
								// 大分類真值只決定圖示；舊資料或分類不明確時顯示中性圓底，
								// 不猜標題。
								const body = (
									<>
										<HawkrAgentConversationMarble
											primaryCategoryId={row.primaryCategoryId}
										/>
										<span className="chat-sidebar__main">
											<span className="chat-sidebar__item-head">
												<strong>{label}</strong>
												<small>{formatTime(row.updatedAt)}</small>
											</span>
											{showPreview ? (
												<span className="chat-sidebar__preview">
													{row.preview}
												</span>
											) : null}
										</span>
									</>
								);
								return (
									<li key={row.id} className="chat-sidebar__agent-row">
										{renamingId === row.id && onAgentRowRename ? (
											// 就地改名時整列不是連結——輸入框巢在 <a> 裡會被
											// 導航吃掉點擊，而且巢狀互動元素是 a11y 硬傷。
											<div className="chat-sidebar__item" data-kind="agent">
												<RenameField
													initial={row.title || ""}
													onCancel={() => setRenamingId(null)}
													onSubmit={async (next) => {
														setRenamingId(null);
														await onAgentRowRename(row.id, next);
													}}
												/>
											</div>
										) : (
											<Link
												href={row.href}
												className="chat-sidebar__item"
												data-kind="agent"
												aria-current={row.active ? "page" : undefined}
												title={collapsed ? label : undefined}
												// 滑過就先把該對話抓進快取，點下去才不用等。
												onMouseEnter={() => onAgentRowPrefetch?.(row)}
												onFocus={() => onAgentRowPrefetch?.(row)}
											>
												{body}
											</Link>
										)}
										{renamingId !== row.id &&
										(onAgentRowRename || onAgentRowDelete) ? (
											// Link 的兄弟節點（絕對定位），不巢狀在 Link 內——
											// 巢狀互動元素是 a11y 硬傷。
											<span className="chat-sidebar__agent-actions">
												{onAgentRowDelete ? (
													<button
														type="button"
														className="chat-sidebar__agent-action chat-sidebar__agent-action--delete"
														onClick={() => onAgentRowDelete(row)}
														aria-label={`刪除對話「${label}」`}
													>
														<AppIcon name="delete" size={16} />
													</button>
												) : null}
												{onAgentRowRename && onAgentRowDelete ? (
													<ConversationActionsMenu
														label={label}
														onRename={() => setRenamingId(row.id)}
														onDelete={() => onAgentRowDelete(row)}
													/>
												) : null}
											</span>
										) : null}
									</li>
								);
							})}
						</ul>
					</nav>
				) : null}

				{showAgentList ? null : loading && items.length === 0 ? (
					<ConversationListLoading />
				) : visibleItems.length === 0 ? (
					(() => {
						const hasItems = items.length > 0;
						const isUnreadFilter = filter === "unread" && !query;
						const isSearch = Boolean(query);
						let title: string;
						let copy: string;
						if (!hasItems) {
							title = "還沒有任何對話";
							copy = "從商品頁或商家頁聯絡賣家後，對話會出現在這裡。";
						} else if (isUnreadFilter) {
							title = "目前沒有未讀訊息";
							copy = "所有訊息都已讀完，先去逛逛新商品吧。";
						} else if (isSearch) {
							title = "找不到對話";
							// 清單一次只抓 50 筆。剛好拿滿代表可能還有沒載入的對話，
							// 這時候說「找不到」會對一個真的存在的對話說謊。
							copy = activeListMaybeTruncated
								? "目前只載入最近的對話，更早的還沒抓下來。"
								: "換個關鍵字再試一次。";
						} else {
							title = "找不到對話";
							copy = "換個關鍵字或篩選條件再試一次。";
						}
						return (
							<div className="chat-sidebar__empty">
								<span className="chat-sidebar__empty-icon" aria-hidden="true">
									<AppIcon name={isSearch ? "search" : "message"} size={26} />
								</span>
								<strong>{title}</strong>
								<span>{copy}</span>
								{crossTabHint ? (
									<button
										type="button"
										className="chat-sidebar__empty-action chat-sidebar__empty-action--quiet"
										onClick={crossTabHint.go}
									>
										{`${crossTabHint.label}有 ${crossTabHint.count} 筆符合`}
										<AppIcon name="arrow-right" size={16} />
									</button>
								) : null}
							</div>
						);
					})()
				) : (
					<ul className="chat-sidebar__list">
						{visibleItems.map((item) => {
							const id = String(item.targetUserId);
							const active = id === activeTargetId;
							return (
								<li key={id}>
									<Link
										href={`${threadHrefBase}/${id}`}
										className="chat-sidebar__item"
										aria-current={active ? "page" : undefined}
										title={collapsed ? item.targetNickname : undefined}
									>
										<Avatar
											src={
												peerKind === "seller"
													? item.logo_url
													: item.targetAvatar
											}
											alt={item.targetNickname}
											name={item.targetNickname}
											entity={peerKind}
											className="chat-sidebar__avatar"
											fallbackClassName="chat-sidebar__avatar--fallback"
											fallback={peerKind === "seller" ? "empty" : "initial"}
										/>
										<span className="chat-sidebar__main">
											<span className="chat-sidebar__item-head">
												<strong>{item.targetNickname}</strong>
												<small>{formatTime(item.lastMessageTime)}</small>
											</span>
											<span className="chat-sidebar__preview">
												{messagePreview(item.lastMessage, item.lastMessageType)}
											</span>
										</span>
										{item.unreadCount > 0 ? (
											<span
												className="chat-sidebar__badge"
												role="status"
												aria-label={`${item.unreadCount} 則未讀`}
											>
												{item.unreadCount > 99 ? "99+" : item.unreadCount}
											</span>
										) : null}
									</Link>
								</li>
							);
						})}
					</ul>
				)}
			</div>

			{/* 清單底部的計數（設計稿 01／03／04 都有這一列）。
			 *
			 * 住在 `.chat-sidebar__panel` **外面**：面板是 `flex: 1 1 0` 且自己
			 * 捲動，放進去會跟著列一起捲走，而這一列要講的正是「捲到底之前共有
			 * 幾筆」。
			 *
			 * 收合成 icon rail 時整列隱藏（CSS）而不是這裡回 null：rail 只有
			 * 44px 寬，一段中文計數在那個寬度只會擠成兩三行。 */}
			{rangeLabel ? <p className="chat-sidebar__range">{rangeLabel}</p> : null}
		</aside>
	);
}
