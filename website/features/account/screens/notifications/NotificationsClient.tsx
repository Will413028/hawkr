"use client";

import { gsap } from "gsap";
import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import {
	ScEmpty,
	ScEmptyDesc,
	ScEmptyTitle,
	ScFilters,
	ScPill,
} from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { SellerMobileDisclosureList } from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiShortDate } from "@/lib/date/taipei";
import { useNotificationPreferences } from "@/lib/hooks/useNotificationPreferences";
import { useNotificationList } from "@/lib/hooks/useNotifications";
import { usePathname } from "@/lib/intl/navigation";
import { buildNotificationFilters } from "@/lib/notifications/filters";
import {
	isChatNotification,
	resolveNotification,
} from "@/lib/notifications/resolve";
import type {
	NotificationItem,
	NotificationType,
} from "@/lib/notifications/types";
import {
	useNotificationStreamStatus,
	useNotificationUnread,
} from "@/lib/notifications/unread-context";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";

/**
 * Per-type icon copied verbatim from the buyer Flutter app's
 * `assets/notification/*.svg` (see
 * `lib/features/notification/presentation/notifications_screen.dart`
 * `_iconConfig`). Each SVG is a self-contained 32×32 badge (colored
 * rounded-square fill + white glyph), so the row's icon slot just
 * needs to size it — no surrounding circle / tint required, which is
 * the visual the App ships. Mapping mirrors the App's `type` switch
 * collapsed to the web's 6 coarse categories.
 */
const TYPE_ICON: Record<NotificationType | "post", AppIconName> = {
	order: "box",
	promotion: "discount",
	message: "chat",
	system: "notification",
	review: "message",
	follow: "users",
	post: "paper",
};

function iconForType(type: NotificationType): AppIconName {
	const upper = String(type).toUpperCase();
	// 聊天私訊：後端 inbox 通知 type 是裸 "chat"（domain category），App 端另有
	// CHAT_MESSAGE / CHAT_GROUP_INVITE 細分；一律用聊天泡泡 icon，別落到系統鈴鐺。
	if (upper === "MESSAGE" || upper.startsWith("CHAT")) return TYPE_ICON.message;
	if (upper.startsWith("ORDER_") || upper === "ORDER") return TYPE_ICON.order;
	if (
		upper === "AFTER_SALE_STATUS" ||
		upper.startsWith("REFUND_") ||
		upper === "RETURN_SHIPPED"
	)
		return "shield-fail";
	if (
		upper === "PROMOTION" ||
		upper.startsWith("COUPON_") ||
		upper.startsWith("POINT_")
	)
		return TYPE_ICON.promotion;
	if (
		upper === "POST_LIKE" ||
		upper === "SELLER_POST" ||
		upper === "POST_CREATED" ||
		upper === "PRODUCT_RESTOCK" ||
		upper === "PRODUCT_PRICE_DROP" ||
		upper === "FAVORITE_UPDATE"
	)
		return TYPE_ICON.post;
	if (
		upper === "SELLER_COMMENT" ||
		upper === "POST_COMMENT" ||
		upper === "COMMENT_REPLY" ||
		upper === "MESSAGE_REPLY" ||
		upper === "REVIEW"
	)
		return TYPE_ICON.review;
	if (
		upper === "SELLER_FOLLOW" ||
		upper === "MERCHANT_UPDATE" ||
		upper === "FOLLOW"
	)
		return TYPE_ICON.follow;
	return TYPE_ICON[type as keyof typeof TYPE_ICON] ?? TYPE_ICON.system;
}

function formatTime(createdAt: string | undefined): string {
	if (!createdAt) return "";
	const ms = Date.parse(createdAt);
	if (Number.isNaN(ms)) return "";
	const diff = Date.now() - ms;
	const min = Math.floor(diff / 60_000);
	if (min < 1) return "剛剛";
	if (min < 60) return `${min} 分鐘前`;
	const hr = Math.floor(min / 60);
	if (hr < 24) return `${hr} 小時前`;
	const day = Math.floor(hr / 24);
	if (day < 7) return `${day} 天前`;
	return formatTaipeiShortDate(ms);
}

/** Collapse a run of consecutive same-source rows into one group. Source = the
 *  notification title (chat sender name / notification kind), matching the
 *  in-app toast's `groupKey`. Only consecutive items merge, so the time order
 *  is preserved and distant同類 rows don't fold together. */
function groupNotifications(items: NotificationItem[]): NotificationItem[][] {
	const groups: NotificationItem[][] = [];
	for (const item of items) {
		const key = (item.title || String(item.type)).trim();
		const last = groups[groups.length - 1];
		const lastKey = last
			? (last[0].title || String(last[0].type)).trim()
			: null;
		if (last && lastKey === key) last.push(item);
		else groups.push([item]);
	}
	return groups;
}

function notificationGroupKey(items: NotificationItem[]): string {
	return String(items[0]?.id ?? "");
}

interface NotificationsClientProps {
	/** Route-specific heading. Buyer keeps the shorter legacy label. */
	title?: string;
	/**
	 * @deprecated Back-link removed from header per design. Kept on the
	 * props so existing callers compile; the value is ignored.
	 */
	backHref?: string;
	/** @deprecated See `backHref`. */
	backLabel?: string;
	preferencesHref?: string;
}

export default function NotificationsClient(
	props: NotificationsClientProps = {},
) {
	const pathname = usePathname();
	if (!pathname) return null;
	return (
		<NotificationsContent
			{...props}
			role={notificationWorkspaceRole(pathname)}
		/>
	);
}

function NotificationsContent({
	title = "通知",
	preferencesHref = "/account/notifications/preferences",
	role,
}: NotificationsClientProps & {
	role: ReturnType<typeof notificationWorkspaceRole>;
}) {
	const [filter, setFilter] = React.useState("");
	const [pendingAction, setPendingAction] = React.useState<string | null>(null);
	const pendingActionRef = React.useRef<string | null>(null);
	const list = useNotificationList({ category: filter || undefined, role });
	const { count: unreadCount, loading: unreadCountLoading } =
		useNotificationUnread();
	const streamStatus = useNotificationStreamStatus();

	// 篩選 chips 與通知偏好同源（後端 category metadata），與偏好頁共用同一條 query。
	// 失敗時 prefs 為 null → 只有「全部」chip，不擋通知列表。
	const { prefs } = useNotificationPreferences(role);
	const filterOptions = React.useMemo(
		() => buildNotificationFilters(prefs ?? []),
		[prefs],
	);

	// 切換 workspace（買家/賣家）時重置篩選。role 是觸發依賴（值改變即重置），
	// effect 內未讀取它，故停用 exhaustive-deps 的移除建議。
	// biome-ignore lint/correctness/useExhaustiveDependencies: role 為觸發依賴
	React.useEffect(() => {
		setFilter("");
	}, [role]);

	const handleItemClick = (item: NotificationItem) => {
		if (!item.isRead) void list.markRead(item.id);
	};

	const runPendingAction = async (
		action: string,
		request: () => Promise<void>,
	) => {
		if (pendingActionRef.current) return;
		pendingActionRef.current = action;
		setPendingAction(action);
		try {
			await request();
		} catch {
			// The list hook keeps the row and exposes mutation errors inline.
		} finally {
			pendingActionRef.current = null;
			setPendingAction(null);
		}
	};

	const handleDelete = (id: NotificationItem["id"]) =>
		runPendingAction(`delete:${id}`, () => list.remove([id]));
	const deletingId = pendingAction?.startsWith("delete:")
		? pendingAction.slice("delete:".length)
		: null;

	// Collapse consecutive same-source rows (same title — chat sender or
	// notification kind) into one expandable stack, matching the in-app toast.
	const groups = React.useMemo(
		() => groupNotifications(list.items),
		[list.items],
	);

	return (
		<main className="bg-transparent">
			{!list.loading ? (
				<span data-hk-page-content-ready="notifications" hidden />
			) : null}
			<SellerPageShell
				title={title}
				subtitle={
					unreadCountLoading
						? "正在同步未讀通知…"
						: unreadCount > 0
							? `你有 ${unreadCount} 則未讀通知`
							: "目前沒有未讀通知"
				}
				actions={
					<>
						<Button
							href={preferencesHref}
							variant="bordered"
							color="default"
							size="sm"
							radius="full"
							disabled={pendingAction !== null}
						>
							通知設定
						</Button>
						<Button
							variant="light"
							size="sm"
							radius="full"
							className="dark:text-[var(--theme-text-accent)] dark:hover:bg-[var(--theme-primary-subtle)]"
							onClick={() =>
								void runPendingAction("mark-all", list.markAllRead)
							}
							disabled={
								unreadCountLoading ||
								unreadCount === 0 ||
								pendingAction !== null
							}
							isLoading={pendingAction === "mark-all"}
							loadingLabel="全部標為已讀中…"
						>
							全部標為已讀
						</Button>
					</>
				}
				toolbar={
					<ScFilters aria-label="通知分類">
						{filterOptions.map((f) => (
							<ScPill
								key={f.value || "all"}
								as="button"
								className="seller-notifications__filter-tab"
								role="tab"
								aria-selected={filter === f.value}
								active={filter === f.value}
								disabled={pendingAction !== null}
								onClick={() => setFilter(f.value)}
							>
								{f.label}
							</ScPill>
						))}
					</ScFilters>
				}
			>
				{(streamStatus === "unavailable" || streamStatus === "offline") && (
					<p
						className="my-4 px-4 py-3 rounded-hkui-md bg-warning-50 text-warning-800 text-sm dark:bg-[#a9842c]/26 dark:text-[#d6c27e]"
						role="status"
					>
						{streamStatus === "offline"
							? "網路連線中斷，即時通知已暫停；恢復連線後會自動繼續同步。"
							: "即時通知暫時中斷，我們正在重新讀取最新內容；若畫面仍未更新，請重新整理頁面。"}
					</p>
				)}
				{list.error && (
					<p
						className="my-4 px-4 py-3 rounded-hkui-md bg-danger-50 text-danger-700 text-sm"
						role="alert"
					>
						{list.error}
					</p>
				)}

				{list.loading && list.items.length === 0 ? (
					<div className="min-h-[320px]" aria-hidden="true" />
				) : list.items.length === 0 ? (
					<EmptyState filter={filter} />
				) : (
					<>
						<ul className="seller-mobile-disclosure-desktop list-none m-0 p-0 flex flex-col gap-3">
							{groups.map((group, gi) => {
								// Running index across groups drives the staggered pop-in.
								const baseIndex = groups
									.slice(0, gi)
									.reduce((n, g) => n + g.length, 0);
								return group.length === 1 ? (
									<li key={String(group[0].id)}>
										<NotificationRow
											item={group[0]}
											index={baseIndex}
											role={role}
											onClick={handleItemClick}
											onDelete={() => void handleDelete(group[0].id)}
											deletePending={deletingId === String(group[0].id)}
											deleteDisabled={pendingAction !== null}
										/>
									</li>
								) : (
									<NotificationGroup
										key={String(group[0].id)}
										items={group}
										baseIndex={baseIndex}
										role={role}
										onItemClick={handleItemClick}
										onDelete={(id) => void handleDelete(id)}
										pendingDeleteId={deletingId}
										deleteDisabled={pendingAction !== null}
									/>
								);
							})}
						</ul>
						<SellerMobileDisclosureList
							items={groups}
							getKey={notificationGroupKey}
							ariaLabel="通知清單"
							resetKey={`${role}:${filter}`}
							className="seller-notifications__mobile-list"
							getSummary={(group) => {
								const top = group[0];
								const resolved = resolveNotification(top, role);
								const unread = group.filter((item) => !item.isRead).length;
								return {
									label: `通知：${resolved.title}`,
									leading: (
										<AppIcon
											name={
												isChatNotification(top)
													? TYPE_ICON.message
													: iconForType(top.type)
											}
											size={22}
										/>
									),
									title: resolved.title,
									meta: formatTime(top.createdAt),
									status:
										unread > 0
											? `${unread} 未讀`
											: group.length > 1
												? `${group.length} 則`
												: "已讀",
								};
							}}
							renderDetails={(group) => {
								const baseIndex = groups
									.slice(0, groups.indexOf(group))
									.reduce((count, current) => count + current.length, 0);
								return (
									<div className="seller-mobile-disclosure__body seller-notifications__mobile-details">
										{group.map((item, index) => (
											<NotificationRow
												key={String(item.id)}
												item={item}
												index={baseIndex + index}
												role={role}
												onClick={handleItemClick}
												onDelete={() => void handleDelete(item.id)}
												deletePending={deletingId === String(item.id)}
												deleteDisabled={pendingAction !== null}
											/>
										))}
									</div>
								);
							}}
						/>
					</>
				)}

				{list.hasMore && (
					<div className="flex min-h-16 items-center justify-center pt-4">
						<Button
							variant="bordered"
							color="primary"
							radius="full"
							isLoading={list.loadingMore}
							loadingLabel="載入通知中…"
							onClick={() => void list.loadMore()}
						>
							載入更多通知
						</Button>
					</div>
				)}
			</SellerPageShell>
		</main>
	);
}

function NotificationRow({
	item,
	index,
	role,
	onClick,
	onDelete,
	deletePending = false,
	deleteDisabled = false,
	asPreview = false,
}: {
	item: NotificationItem;
	index: number;
	role: "buyer" | "seller";
	onClick: (i: NotificationItem) => void;
	onDelete: () => void;
	deletePending?: boolean;
	deleteDisabled?: boolean;
	/** Render as a non-interactive card (no link, no delete ✕) — used as the top
	 *  of a collapsed stack, where the surrounding pile owns the tap-to-expand. */
	asPreview?: boolean;
}) {
	// 聊天私訊一律用聊天泡泡 icon，依 type 或 refType 判定（與點擊跳轉同一個判定，
	// 避免 icon 與導向不一致）；其餘走 type 對應表。
	const iconPath = isChatNotification(item)
		? TYPE_ICON.message
		: iconForType(item.type);
	const resolved = resolveNotification(item, role);
	const href = resolved.href;

	const body = (
		<>
			<span
				className="shrink-0 w-9 h-9 grid place-items-center"
				aria-hidden="true"
			>
				<AppIcon name={iconPath} size={32} />
			</span>
			<span className="flex-1 min-w-0 flex flex-col gap-[2px]">
				<span className="text-sm font-semibold text-foreground leading-snug">
					{resolved.title}
				</span>
				{resolved.body ? (
					<span className="text-xs text-foreground-muted leading-relaxed line-clamp-2 overflow-hidden whitespace-pre-line">
						{resolved.body}
					</span>
				) : null}
				<span className="text-[0.6875rem] text-foreground-soft mt-1">
					{formatTime(item.createdAt)}
				</span>
			</span>
			{!item.isRead && (
				<span
					className="w-2 h-2 rounded-full bg-danger self-center mr-3 shrink-0"
					role="status"
					aria-label="未讀"
				/>
			)}
		</>
	);

	return (
		<div
			className={`relative flex items-stretch bg-content1 border border-divider rounded-2xl shadow-sm overflow-hidden transition-[transform,background,border-color,box-shadow] duration-200 ease-out animate-[notif-row-pop_520ms_cubic-bezier(0.34,1.56,0.64,1)_var(--notif-row-delay,0ms)_both] hover:-translate-y-px hover:border-hkui-primary-200 hover:shadow-md ${!item.isRead ? "bg-hkui-primary-50/40 border-hkui-primary-200" : ""}`}
			data-type={item.type}
			style={{
				["--notif-row-delay" as string]: `${Math.min(index, 12) * 32}ms`,
			}}
		>
			{asPreview ? (
				<div className="flex items-start gap-3 flex-1 min-w-0 px-4 py-[14px] text-left">
					{body}
				</div>
			) : href ? (
				<Link
					href={href}
					className="flex items-start gap-3 flex-1 min-w-0 px-4 py-[14px] bg-transparent border-0 text-inherit text-left cursor-pointer no-underline"
					onClick={() => onClick(item)}
				>
					{body}
				</Link>
			) : (
				<button
					type="button"
					className="flex items-start gap-3 flex-1 min-w-0 px-4 py-[14px] bg-transparent border-0 text-inherit text-left cursor-pointer no-underline"
					onClick={() => onClick(item)}
				>
					{body}
				</button>
			)}
			{asPreview ? null : (
				<Button
					type="button"
					variant="light"
					color="danger"
					size="sm"
					isIconOnly
					className="h-auto min-h-full w-11 min-w-11 shrink-0 rounded-none px-0 text-[1.375rem] leading-none text-foreground-soft transition-colors duration-150 hover:bg-danger-50 hover:text-danger"
					aria-label="刪除這則通知"
					disabled={deleteDisabled}
					isLoading={deletePending}
					loadingLabel="刪除通知中…"
					onClick={onDelete}
				>
					<AppIcon name="delete" size={18} />
				</Button>
			)}
		</div>
	);
}

/**
 * Same-source notifications collapsed into an iOS-notification-center pile: the
 * newest card on top with the older ones peeking out behind it and a count
 * badge. Tapping the pile fans them out into the full list (GSAP stagger);
 * a chevron collapses them back. Mirrors the in-app toast's stacking.
 */
function NotificationGroup({
	items,
	baseIndex,
	role,
	onItemClick,
	onDelete,
	pendingDeleteId,
	deleteDisabled,
}: {
	items: NotificationItem[];
	baseIndex: number;
	role: "buyer" | "seller";
	onItemClick: (i: NotificationItem) => void;
	onDelete: (id: NotificationItem["id"]) => void;
	pendingDeleteId: string | null;
	deleteDisabled: boolean;
}) {
	const [open, setOpen] = React.useState(false);
	const expandedRef = React.useRef<HTMLDivElement>(null);
	const collapseBtnRef = React.useRef<HTMLButtonElement>(null);
	const top = items[0];
	const rest = items.slice(1);
	const unreadInRest = rest.filter((n) => !n.isRead).length;
	const peekLayers = Math.min(rest.length, 2);

	// Fan the revealed cards out when the pile opens — they drop in from behind
	// the top card, staggered. The top card was already on screen, so skip it.
	// The collapse button then eases in on the right, once the cards have landed.
	React.useLayoutEffect(() => {
		if (!open) return;
		const list = expandedRef.current;
		const cards = list
			? (Array.from(list.children).slice(1) as HTMLElement[])
			: [];
		if (cards.length > 0) {
			gsap.from(cards, {
				y: -12,
				opacity: 0,
				duration: 0.4,
				ease: "power3.out",
				stagger: 0.06,
			});
		}
		const btn = collapseBtnRef.current;
		if (btn) {
			gsap.fromTo(
				btn,
				{ opacity: 0, x: 12 },
				{
					opacity: 1,
					x: 0,
					duration: 0.45,
					ease: "power2.out",
					delay: 0.18 + cards.length * 0.06,
				},
			);
		}
	}, [open]);

	if (!open) {
		return (
			<li className="list-none">
				<button
					type="button"
					onClick={() => setOpen(true)}
					aria-label={`展開 ${items.length} 則同類通知`}
					className="relative block w-full cursor-pointer appearance-none border-0 bg-transparent p-0 text-left"
				>
					{/* Cards peeking out behind the top one — the「堆疊」look. */}
					{Array.from({ length: peekLayers }).map((_, i) => (
						<span
							// biome-ignore lint/suspicious/noArrayIndexKey: fixed 0–1 decorative layers
							key={i}
							aria-hidden="true"
							className="absolute inset-x-0 top-0 h-full rounded-2xl border border-divider bg-content1"
							style={{
								transform: `translateY(${(i + 1) * 6}px) scaleX(${1 - (i + 1) * 0.04})`,
								opacity: 1 - (i + 1) * 0.3,
							}}
						/>
					))}
					<div className="relative z-[1]">
						<NotificationRow
							item={top}
							index={baseIndex}
							role={role}
							onClick={onItemClick}
							onDelete={() => {}}
							asPreview
						/>
					</div>
					<span
						aria-hidden="true"
						className={`absolute right-3 top-3 z-[2] grid h-5 min-w-5 place-items-center rounded-full px-1.5 text-[0.6875rem] font-bold leading-none text-white shadow-sm ${
							unreadInRest > 0 ? "bg-danger" : "bg-hkui-primary"
						}`}
					>
						{items.length}
					</span>
				</button>
			</li>
		);
	}

	return (
		<li className="list-none">
			<div ref={expandedRef} className="flex flex-col gap-3">
				{items.map((it, i) => (
					<NotificationRow
						key={String(it.id)}
						item={it}
						index={baseIndex + i}
						role={role}
						onClick={onItemClick}
						onDelete={() => onDelete(it.id)}
						deletePending={pendingDeleteId === String(it.id)}
						deleteDisabled={deleteDisabled}
					/>
				))}
			</div>
			<div className="mt-2 flex justify-end">
				<button
					ref={collapseBtnRef}
					type="button"
					onClick={() => setOpen(false)}
					aria-label="收合同類通知"
					className="inline-flex items-center gap-1 rounded-full border border-divider bg-content1 px-3 py-1 text-xs font-semibold text-foreground-soft transition-colors hover:bg-hkui-primary-50 hover:text-hkui-primary"
				>
					收合
					<AppIcon name="chevron-down" size={13} className="rotate-180" />
				</button>
			</div>
		</li>
	);
}

/**
 * Empty state uses the same `.sc-empty` pattern as /account/favorites,
 * /account/orders, /account/coupons, etc., for visual consistency
 * across the buyer workspace. Icon comes from the shared AccountEmpty
 * registry (notifications variant = bell).
 */
function EmptyState({ filter }: { filter: string }) {
	return (
		<ScEmpty>
			<AccountEmptyStateIcon variant="notifications" />
			<ScEmptyTitle>
				{filter ? "這個分類目前沒有通知" : "目前沒有任何通知"}
			</ScEmptyTitle>
			<ScEmptyDesc>我們會在這裡通知你訂單、優惠、互動與系統訊息。</ScEmptyDesc>
		</ScEmpty>
	);
}
