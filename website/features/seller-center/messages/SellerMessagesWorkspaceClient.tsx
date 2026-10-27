"use client";

import { parseAsString, useQueryState } from "nuqs";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import SellerPageShell from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import {
	SellerEmptyState,
	SellerTableSkeletonRows,
} from "@/features/seller-center";
import { SellerMobileDisclosureList } from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiDate, formatTaipeiMonthDayTime } from "@/lib/date/taipei";
import { useConversations } from "@/lib/hooks/useChat";
import { fetchOrders } from "@/lib/orders/repo";
import {
	type CustomerTag,
	fetchSellerCustomers,
	fetchSellerTags,
	type SellerCustomer,
	type SellerCustomerType,
} from "@/lib/seller/repo";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import CustomerDetailDrawer from "../customers/CustomerDetailDrawer";

type MessageStateFilter = "all" | "unread";
type QuickRange = "all" | "today" | "yesterday" | "7d" | "30d" | "custom";

interface PurchaseSummary {
	total: number;
	lastOrderAt: number;
	lastOrderNo: string;
	lastAmount: number;
}

interface MessageWorkbenchRow {
	targetId: string;
	customer?: SellerCustomer;
	name: string;
	avatar?: string;
	phone?: string;
	customerType: SellerCustomerType;
	tags: string[];
	orderCount: number;
	totalSpent: number;
	lastOrderAt: number;
	lastOrderNo: string;
	lastAmount: number;
	lastMessage: string;
	lastMessageType?: string;
	lastMessageAt: number;
	unreadCount: number;
}

const CUSTOMER_TYPE_TABS: Array<{
	value: "" | SellerCustomerType;
	label: string;
}> = [
	{ value: "", label: "全部客戶" },
	{ value: "normal", label: "一般" },
	{ value: "vip", label: "VIP" },
	{ value: "blocklist", label: "黑名單" },
];

const RANGE_OPTIONS: Array<{ value: QuickRange; label: string }> = [
	{ value: "all", label: "不限時間" },
	{ value: "today", label: "今天" },
	{ value: "yesterday", label: "昨天" },
	{ value: "7d", label: "近 7 天" },
	{ value: "30d", label: "近 30 天" },
	{ value: "custom", label: "自訂" },
];

const TYPE_BADGE: Partial<Record<SellerCustomerType, string>> = {
	vip: "VIP",
	blocklist: "黑名單",
};

function coerceTimeMs(raw: unknown): number {
	if (raw == null || raw === "") return 0;
	if (typeof raw === "number") return raw < 1e12 ? raw * 1000 : raw;
	if (typeof raw === "string") {
		const n = Number(raw);
		if (Number.isFinite(n) && n > 0) return n < 1e12 ? n * 1000 : n;
		const parsed = Date.parse(raw);
		return Number.isNaN(parsed) ? 0 : parsed;
	}
	return 0;
}

function formatDateTime(ts: number): string {
	return formatTaipeiMonthDayTime(ts) || "—";
}

function formatDateOnly(ts: number): string {
	return formatTaipeiDate(ts) || "尚未下單";
}

function dayStart(d: Date): Date {
	const next = new Date(d);
	next.setHours(0, 0, 0, 0);
	return next;
}

function dayEnd(d: Date): Date {
	const next = new Date(d);
	next.setHours(23, 59, 59, 999);
	return next;
}

function resolveRange(
	range: QuickRange,
	customStart: string,
	customEnd: string,
): { start: number; end: number } | null {
	const now = new Date();
	if (range === "all") return null;
	if (range === "today") {
		return { start: dayStart(now).getTime(), end: dayEnd(now).getTime() };
	}
	if (range === "yesterday") {
		const y = new Date(now);
		y.setDate(y.getDate() - 1);
		return { start: dayStart(y).getTime(), end: dayEnd(y).getTime() };
	}
	if (range === "7d" || range === "30d") {
		const start = dayStart(now);
		start.setDate(start.getDate() - (range === "7d" ? 6 : 29));
		return { start: start.getTime(), end: dayEnd(now).getTime() };
	}
	const start = customStart ? new Date(`${customStart}T00:00:00`).getTime() : 0;
	const end = customEnd ? new Date(`${customEnd}T23:59:59`).getTime() : 0;
	if (!start && !end) return null;
	return {
		start: start || Number.NEGATIVE_INFINITY,
		end: end || Number.POSITIVE_INFINITY,
	};
}

function rowToCustomerSummary(row: MessageWorkbenchRow): SellerCustomer | null {
	if (!row.targetId || row.targetId === "0") return null;
	return (
		row.customer ?? {
			id: row.targetId,
			name: row.name,
			avatar: row.avatar,
			phone: row.phone,
			orderCount: row.orderCount,
			totalSpent: row.totalSpent,
			tags: row.tags,
			customerType: row.customerType,
		}
	);
}

function messageRowKey(row: MessageWorkbenchRow): string {
	return row.targetId;
}

function purchaseAmountLabel(row: MessageWorkbenchRow): string {
	if (row.totalSpent > 0) {
		return `累計 NT$ ${Math.round(row.totalSpent).toLocaleString()}`;
	}
	if (row.lastAmount > 0) {
		return `最近 NT$ ${Math.round(row.lastAmount).toLocaleString()}`;
	}
	return "尚無消費金額";
}

export default function SellerMessagesWorkspaceClient() {
	const conversations = useConversations();
	const [customers, setCustomers] = React.useState<SellerCustomer[]>([]);
	const [tags, setTags] = React.useState<CustomerTag[]>([]);
	const [purchaseByBuyer, setPurchaseByBuyer] = React.useState<
		Record<string, PurchaseSummary>
	>({});
	const [customersLoading, setCustomersLoading] = React.useState(true);
	const customersRequestRef = React.useRef<Promise<void> | null>(null);
	const [customersError, setCustomersError] = React.useState<string | null>(
		null,
	);
	const [purchaseSummariesLoading, setPurchaseSummariesLoading] =
		React.useState(false);
	const purchaseLoadGenerationRef = React.useRef(0);
	const [refreshingAll, setRefreshingAll] = React.useState(false);
	const refreshAllPromiseRef = React.useRef<Promise<void> | null>(null);
	const [query, setQuery] = React.useState("");
	const [customerType, setCustomerType] = React.useState<
		"" | SellerCustomerType
	>("");
	const [tagId, setTagId] = React.useState<string | null>(null);
	const [messageState, setMessageState] =
		React.useState<MessageStateFilter>("all");
	const [range, setRange] = React.useState<QuickRange>("all");
	const [customStart, setCustomStart] = React.useState("");
	const [customEnd, setCustomEnd] = React.useState("");
	const [activeCustomer, setActiveCustomer] =
		React.useState<SellerCustomer | null>(null);
	const openChat = useChatDrawerStore((state) => state.openChat);

	// 外部連結（如聊天通知）帶 ?chat={targetId} 進來時，自動開啟聊天半彈窗。
	// 消費後立即清掉參數讓網址乾淨，再次點同一則通知仍會重新觸發開啟。
	const [chatParam, setChatParam] = useQueryState("chat", parseAsString);
	React.useEffect(() => {
		const t = chatParam?.trim();
		if (!t) return;
		openChat(t);
		void setChatParam(null);
	}, [chatParam, setChatParam, openChat]);

	const loadCustomers = React.useCallback(() => {
		if (customersRequestRef.current) return customersRequestRef.current;
		const pending = (async () => {
			setCustomersLoading(true);
			setCustomersError(null);
			try {
				const [customerRows, tagRows] = await Promise.all([
					fetchSellerCustomers({ pageSize: 200, sortBy: "total_spent" }),
					fetchSellerTags().catch(() => []),
				]);
				setCustomers(customerRows);
				setTags(tagRows);
			} catch (error) {
				setCustomersError(
					error instanceof Error ? error.message : "載入客戶失敗",
				);
			} finally {
				setCustomersLoading(false);
			}
		})();
		customersRequestRef.current = pending;
		void pending.finally(() => {
			if (customersRequestRef.current === pending) {
				customersRequestRef.current = null;
			}
		});
		return pending;
	}, []);

	React.useEffect(() => {
		void loadCustomers();
	}, [loadCustomers]);

	const customerById = React.useMemo(() => {
		const map = new Map<string, SellerCustomer>();
		for (const customer of customers) map.set(String(customer.id), customer);
		return map;
	}, [customers]);

	const conversationIds = React.useMemo(
		() =>
			conversations.items
				.map((item) => String(item.targetUserId))
				.filter((id) => id && id !== "0"),
		[conversations.items],
	);

	React.useEffect(() => {
		let alive = true;
		const ids = conversationIds.filter((id) => !purchaseByBuyer[id]);
		if (ids.length === 0) {
			setPurchaseSummariesLoading(false);
			return;
		}
		const generation = purchaseLoadGenerationRef.current + 1;
		purchaseLoadGenerationRef.current = generation;
		setPurchaseSummariesLoading(true);
		void Promise.all(
			ids.map(async (id) => {
				try {
					const result = await fetchOrders({
						view: "seller",
						buyerId: id,
						pageSize: 20,
					});
					const sorted = [...result.records].sort(
						(a, b) => (b.createdAt || 0) - (a.createdAt || 0),
					);
					const latest = sorted[0];
					return [
						id,
						{
							total: result.total || sorted.length,
							lastOrderAt: latest?.createdAt || 0,
							lastOrderNo: latest?.orderNo || "",
							lastAmount: Number(latest?.totalActualAmount ?? 0) || 0,
						},
					] as const;
				} catch {
					// Purchase enrichment is best-effort. Record an empty settled value
					// so a failed buyer lookup cannot create an automatic retry loop.
					return [
						id,
						{ total: 0, lastOrderAt: 0, lastOrderNo: "", lastAmount: 0 },
					] as const;
				}
			}),
		)
			.then((results) => {
				if (!alive) return;
				setPurchaseByBuyer((prev) => {
					const next = { ...prev };
					for (const [id, summary] of results) {
						next[id] = summary;
					}
					return next;
				});
			})
			.finally(() => {
				if (alive && purchaseLoadGenerationRef.current === generation) {
					setPurchaseSummariesLoading(false);
				}
			});
		return () => {
			alive = false;
		};
	}, [conversationIds, purchaseByBuyer]);

	const selectedTag = tagId ? tags.find((tag) => tag.id === tagId) : null;

	const rows = React.useMemo<MessageWorkbenchRow[]>(() => {
		return conversations.items.map((conversation) => {
			const targetId = String(conversation.targetUserId);
			const customer = customerById.get(targetId);
			const purchase = purchaseByBuyer[targetId];
			const customerOrderCount = customer?.orderCount ?? 0;
			const purchaseOrderCount = purchase?.total ?? 0;
			return {
				targetId,
				customer,
				name:
					customer?.name ||
					conversation.targetNickname ||
					`客戶 #${conversation.targetUserId}`,
				avatar: customer?.avatar || conversation.targetAvatar,
				phone: customer?.phone,
				customerType: customer?.customerType ?? "normal",
				tags: customer?.tags ?? [],
				orderCount: Math.max(customerOrderCount, purchaseOrderCount),
				totalSpent: Number(customer?.totalSpent ?? 0),
				lastOrderAt: purchase?.lastOrderAt ?? 0,
				lastOrderNo: purchase?.lastOrderNo ?? "",
				lastAmount: purchase?.lastAmount ?? 0,
				lastMessage: conversation.lastMessage,
				lastMessageType: conversation.lastMessageType,
				lastMessageAt: coerceTimeMs(conversation.lastMessageTime),
				unreadCount: conversation.unreadCount,
			};
		});
	}, [conversations.items, customerById, purchaseByBuyer]);

	const dateRange = React.useMemo(
		() => resolveRange(range, customStart, customEnd),
		[customEnd, customStart, range],
	);

	const filteredRows = React.useMemo(() => {
		const q = query.trim().toLowerCase();
		const selectedTagName = selectedTag?.name ?? "";
		return rows
			.filter((row) => {
				if (messageState === "unread" && row.unreadCount <= 0) return false;
				if (customerType && row.customerType !== customerType) return false;
				if (tagId) {
					if (!selectedTagName) return false;
					if (!row.tags.includes(selectedTagName)) return false;
				}
				if (q) {
					const haystack = [
						row.name,
						row.phone,
						row.lastMessage,
						row.lastOrderNo,
						...row.tags,
					]
						.filter(Boolean)
						.join(" ")
						.toLowerCase();
					if (!haystack.includes(q)) return false;
				}
				if (dateRange) {
					const ts = row.lastMessageAt;
					if (!ts || ts < dateRange.start || ts > dateRange.end) return false;
				}
				return true;
			})
			.sort((a, b) => {
				return (b.lastMessageAt || 0) - (a.lastMessageAt || 0);
			});
	}, [customerType, dateRange, messageState, query, rows, selectedTag, tagId]);

	const totalUnread = rows.reduce((sum, row) => sum + row.unreadCount, 0);
	const loadingInitial =
		(conversations.loading && conversations.items.length === 0) ||
		(customersLoading && customers.length === 0);
	const error = conversations.error || customersError;
	const hasFilters =
		query.trim() ||
		customerType ||
		tagId !== null ||
		messageState !== "all" ||
		range !== "all";

	const resetFilters = () => {
		setQuery("");
		setCustomerType("");
		setTagId(null);
		setMessageState("all");
		setRange("all");
		setCustomStart("");
		setCustomEnd("");
	};

	const refreshAll = React.useCallback(() => {
		if (refreshAllPromiseRef.current) return refreshAllPromiseRef.current;
		setRefreshingAll(true);
		if (conversationIds.length > 0) setPurchaseSummariesLoading(true);
		setPurchaseByBuyer({});
		const pending = Promise.allSettled([
			conversations.refresh(),
			loadCustomers(),
		])
			.then(() => undefined)
			.finally(() => {
				if (refreshAllPromiseRef.current === pending) {
					refreshAllPromiseRef.current = null;
					setRefreshingAll(false);
				}
			});
		refreshAllPromiseRef.current = pending;
		return pending;
	}, [conversationIds.length, conversations.refresh, loadCustomers]);

	const workspaceBusy =
		conversations.loading ||
		customersLoading ||
		refreshingAll ||
		purchaseSummariesLoading;

	const closeCustomerDrawer = () => {
		setActiveCustomer(null);
	};

	return (
		<>
			<SellerPageShell
				className="seller-messages"
				title="客戶訊息"
				subtitle={`共 ${rows.length.toLocaleString()} 則對話，${totalUnread.toLocaleString()} 則未讀`}
				actionsClassName="seller-messages__actions-slot"
				actions={
					<div className="seller-messages__head-actions seller-page-shell__action-group">
						<div className="seller-orders__search">
							<AppIcon name="chat" size={16} />
							<input
								type="search"
								value={query}
								onChange={(event) => setQuery(event.target.value)}
								placeholder="搜尋客戶、電話或訊息…"
								aria-label="搜尋客戶訊息"
							/>
						</div>
						<Button
							type="button"
							variant="bordered"
							color="default"
							size="md"
							className="seller-orders__btn"
							onClick={() => void refreshAll()}
							disabled={workspaceBusy}
							isLoading={workspaceBusy}
							loadingLabel="重新整理中…"
						>
							<AppIcon name="spinner" size={14} />
							重新整理
						</Button>
					</div>
				}
				toolbar={
					<div className="seller-messages__filters">
						<div
							className="seller-orders__segment"
							role="tablist"
							aria-label="訊息狀態"
						>
							<button
								type="button"
								className={`seller-orders__segment-btn${messageState === "all" ? " is-active" : ""}`}
								role="tab"
								aria-selected={messageState === "all"}
								onClick={() => setMessageState("all")}
							>
								全部
								<span className="seller-orders__segment-count">
									{rows.length}
								</span>
							</button>
							<button
								type="button"
								className={`seller-orders__segment-btn${messageState === "unread" ? " is-active" : ""}`}
								role="tab"
								aria-selected={messageState === "unread"}
								onClick={() => setMessageState("unread")}
							>
								未讀
								<span className="seller-orders__segment-count">
									{totalUnread}
								</span>
							</button>
						</div>

						<div className="seller-messages__filter-group">
							<span className="seller-messages__filter-label">客戶分類</span>
							<div className="seller-messages__chip-row">
								{CUSTOMER_TYPE_TABS.map((tab) => (
									<button
										key={tab.value || "all"}
										type="button"
										className={`seller-orders__chip${customerType === tab.value ? " is-selected" : ""}`}
										aria-pressed={customerType === tab.value}
										onClick={() => setCustomerType(tab.value)}
									>
										{tab.label}
									</button>
								))}
							</div>
						</div>

						{tags.length > 0 ? (
							<div className="seller-messages__filter-group">
								<span className="seller-messages__filter-label">客戶標籤</span>
								<div className="seller-messages__chip-row">
									<button
										type="button"
										className={`seller-orders__chip${tagId === null ? " is-selected" : ""}`}
										aria-pressed={tagId === null}
										onClick={() => setTagId(null)}
									>
										全部標籤
									</button>
									{tags.map((tag) => (
										<button
											key={tag.id || tag.name}
											type="button"
											className={`seller-orders__chip${tagId === tag.id ? " is-selected" : ""}`}
											aria-pressed={tagId === tag.id}
											onClick={() => setTagId(tagId === tag.id ? null : tag.id)}
										>
											{tag.name}
										</button>
									))}
								</div>
							</div>
						) : null}

						<div className="seller-messages__filter-group seller-messages__filter-group--time">
							<span className="seller-messages__filter-label">對話時間</span>
							<div className="seller-messages__chip-row">
								{RANGE_OPTIONS.map((option) => (
									<button
										key={option.value}
										type="button"
										className={`seller-orders__chip${range === option.value ? " is-selected" : ""}`}
										aria-pressed={range === option.value}
										onClick={() => setRange(option.value)}
									>
										{option.label}
									</button>
								))}
							</div>
							{range === "custom" ? (
								<div className="seller-messages__date-inputs">
									<input
										type="date"
										value={customStart}
										onChange={(event) => setCustomStart(event.target.value)}
										aria-label="開始日期"
									/>
									<span>至</span>
									<input
										type="date"
										value={customEnd}
										onChange={(event) => setCustomEnd(event.target.value)}
										aria-label="結束日期"
									/>
								</div>
							) : null}
						</div>

						<button
							type="button"
							className={`seller-messages__reset${hasFilters ? "" : " is-hidden"}`}
							onClick={resetFilters}
							disabled={!hasFilters}
							aria-hidden={!hasFilters}
							tabIndex={hasFilters ? 0 : -1}
						>
							清除篩選
						</button>
					</div>
				}
			>
				{error ? (
					<p className="seller-customers__error" role="alert">
						{error}
					</p>
				) : null}

				<div
					className={[
						"seller-orders__card seller-messages__card",
						!loadingInitial && filteredRows.length > 0
							? "seller-mobile-disclosure-desktop"
							: "",
					]
						.filter(Boolean)
						.join(" ")}
					data-loading={workspaceBusy ? "true" : undefined}
				>
					<div className="seller-orders__table-wrap seller-messages__table-wrap">
						{!loadingInitial && filteredRows.length === 0 ? (
							<SellerEmptyState
								variant="messages"
								searchQuery={query}
								seamless
								className="seller-messages__empty-state"
								title={hasFilters ? "找不到符合條件的對話" : undefined}
								description={
									hasFilters
										? "調整客戶分類、標籤或時間範圍後再試一次。"
										: undefined
								}
							/>
						) : (
							<table
								className="seller-orders__table seller-messages__table"
								data-mobile-layout="cards"
							>
								<colgroup>
									<col className="seller-messages__col-customer" />
									<col className="seller-messages__col-order-time" />
									<col className="seller-messages__col-purchase" />
									<col className="seller-messages__col-reply-time" />
									<col className="seller-messages__col-actions" />
								</colgroup>
								<thead>
									<tr>
										<th>客戶名稱</th>
										<th className="is-center">下單時間</th>
										<th className="is-center">購買紀錄</th>
										<th className="is-center">上次回覆時間</th>
										<th className="is-center">操作</th>
									</tr>
								</thead>
								<tbody>
									{loadingInitial ? (
										<SellerTableSkeletonRows
											columns={5}
											rows={6}
											labels={[
												"客戶名稱",
												"下單時間",
												"購買紀錄",
												"上次回覆",
												"操作",
											]}
										/>
									) : (
										filteredRows.map((row) => (
											<MessageRow
												key={row.targetId}
												row={row}
												onOpenChat={() =>
													openChat(row.targetId, {
														name: row.name,
														avatar: row.avatar,
													})
												}
												onOpenCustomer={() =>
													setActiveCustomer(rowToCustomerSummary(row))
												}
											/>
										))
									)}
								</tbody>
							</table>
						)}
					</div>
				</div>

				{!loadingInitial && filteredRows.length > 0 ? (
					<SellerMobileDisclosureList
						items={filteredRows}
						getKey={messageRowKey}
						ariaLabel="客戶訊息清單"
						resetKey={JSON.stringify([
							query,
							customerType,
							tagId,
							messageState,
							range,
							customStart,
							customEnd,
						])}
						getSummary={(row) => ({
							label: `與 ${row.name} 的對話`,
							leading: (
								<Avatar
									src={row.avatar}
									name={row.name}
									alt=""
									entity="buyer"
									className="seller-orders__buyer-avatar"
									fallbackClassName="seller-customers__avatar-fallback"
									fallback="initial"
									loading="eager"
								/>
							),
							title: row.name,
							meta:
								row.unreadCount > 0
									? `${row.unreadCount.toLocaleString()} 未讀`
									: formatDateTime(row.lastMessageAt),
						})}
						renderDetails={(row) => (
							<>
								<div className="seller-mobile-disclosure__body">
									<div className="seller-mobile-disclosure__detail-row">
										<span>最新訊息</span>
										<strong>{row.lastMessage || "—"}</strong>
									</div>
									{row.phone || row.tags.length > 0 || row.lastOrderNo ? (
										<div className="seller-mobile-disclosure__facts">
											{row.phone ? (
												<span className="seller-mobile-disclosure__fact">
													電話 {row.phone}
												</span>
											) : null}
											{row.tags.map((tag) => (
												<span
													key={tag}
													className="seller-mobile-disclosure__fact"
												>
													{tag}
												</span>
											))}
											{row.lastOrderNo ? (
												<span className="seller-mobile-disclosure__fact">
													訂單 {row.lastOrderNo}
												</span>
											) : null}
										</div>
									) : null}
									<div className="seller-mobile-disclosure__detail-row">
										<span>下單時間</span>
										<strong>{formatDateOnly(row.lastOrderAt)}</strong>
									</div>
									<div className="seller-mobile-disclosure__detail-row">
										<span>購買紀錄</span>
										<strong>
											{row.orderCount.toLocaleString()} 筆 ·{" "}
											{purchaseAmountLabel(row)}
										</strong>
									</div>
									<div className="seller-mobile-disclosure__detail-row">
										<span>上次回覆</span>
										<strong>{formatDateTime(row.lastMessageAt)}</strong>
									</div>
								</div>
								<div className="seller-mobile-disclosure__actions">
									<button
										type="button"
										className="seller-orders__action"
										onClick={() =>
											openChat(row.targetId, {
												name: row.name,
												avatar: row.avatar,
											})
										}
									>
										開啟聊天室
									</button>
									<button
										type="button"
										className="seller-orders__action seller-messages__secondary-action"
										onClick={() => setActiveCustomer(rowToCustomerSummary(row))}
									>
										客戶資料
									</button>
								</div>
							</>
						)}
					/>
				) : null}
			</SellerPageShell>
			<CustomerDetailDrawer
				open={activeCustomer !== null}
				customer={activeCustomer}
				onClose={closeCustomerDrawer}
				// 分類 / 標籤變更當下即時重載客戶清單，不再等關閉抽屜。
				onMutated={() => {
					void loadCustomers();
				}}
			/>
		</>
	);
}

function MessageRow({
	row,
	onOpenChat,
	onOpenCustomer,
}: {
	row: MessageWorkbenchRow;
	onOpenChat: () => void;
	onOpenCustomer: () => void;
}) {
	const typeBadge = TYPE_BADGE[row.customerType];
	return (
		<tr className="seller-orders__row seller-messages__row">
			<td data-label="客戶名稱">
				<button
					type="button"
					className="seller-orders__buyer seller-messages__buyer"
					onClick={onOpenChat}
					aria-label={`開啟與 ${row.name} 的聊天室`}
				>
					<Avatar
						src={row.avatar}
						name={row.name}
						alt={row.name}
						entity="buyer"
						className="seller-orders__buyer-avatar"
						fallbackClassName="seller-customers__avatar-fallback"
						fallback="initial"
					/>
					<div className="seller-orders__buyer-text seller-messages__buyer-text">
						<strong>
							{row.name}
							{typeBadge ? (
								<span
									className={`seller-customers__badge is-${row.customerType}`}
								>
									{typeBadge}
								</span>
							) : null}
						</strong>
						{row.phone ? <small>{row.phone}</small> : null}
						{row.tags.length > 0 ? (
							<div className="seller-messages__tags">
								{row.tags.slice(0, 3).map((tag) => (
									<span key={tag} className="seller-customers__tag">
										{tag}
									</span>
								))}
								{row.tags.length > 3 ? (
									<span className="seller-customers__tag">
										+{row.tags.length - 3}
									</span>
								) : null}
							</div>
						) : null}
					</div>
				</button>
			</td>
			<td className="is-center seller-orders__shipping" data-label="下單時間">
				<strong>{formatDateOnly(row.lastOrderAt)}</strong>
				{row.lastOrderNo ? <small>{row.lastOrderNo}</small> : null}
			</td>
			<td className="is-center" data-label="購買紀錄">
				<strong>{row.orderCount.toLocaleString()} 筆</strong>
				<small>
					{row.totalSpent > 0
						? `累計 NT$ ${Math.round(row.totalSpent).toLocaleString()}`
						: row.lastAmount > 0
							? `最近 NT$ ${Math.round(row.lastAmount).toLocaleString()}`
							: "尚無消費金額"}
				</small>
			</td>
			<td
				className="is-center seller-orders__shipping seller-messages__reply-time"
				data-label="上次回覆"
			>
				<strong>{formatDateTime(row.lastMessageAt)}</strong>
			</td>
			<td className="is-center" data-label="操作">
				<div className="seller-orders__actions">
					<button
						type="button"
						className="seller-orders__action"
						onClick={onOpenChat}
					>
						開啟聊天室
					</button>
					<button
						type="button"
						className="seller-orders__action seller-messages__secondary-action"
						onClick={onOpenCustomer}
					>
						客戶資料
					</button>
				</div>
			</td>
		</tr>
	);
}
