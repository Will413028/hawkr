"use client";

import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import SellerAvatarImage from "@/components/shared/SellerAvatarImage";
import SellerSidebarIcon from "@/components/shared/SellerSidebarIcon";
import ThemeToggle from "@/components/shared/ThemeToggle";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import TerminationNoticeBanner from "@/features/seller/measures/TerminationNoticeBanner";
import { SELLER_SUSPENDED_EVENT } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { useChatUnread } from "@/lib/chat/unread-context";
import { useSellerDashboard } from "@/lib/hooks/useSellerDashboard";
import { useSellerInvoiceOverview } from "@/lib/hooks/useSellerInvoices";
import { useSellerPosts } from "@/lib/hooks/useSellerPosts";
import { useSellerProducts } from "@/lib/hooks/useSellerProducts";
import { useInvalidateSellerSelf } from "@/lib/hooks/useSellerSelf";
import { useSellerWorkspaceRealtime } from "@/lib/hooks/useSellerWorkspaceRealtime";
import { usePathname } from "@/lib/intl/navigation";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { useNotificationUnread } from "@/lib/notifications/unread-context";
import { hasCapability } from "@/lib/seller/store";
import { framedNavItem, isReceiptOnlySeller } from "../invoices/receiptFraming";
import { isBlockedByCapabilities } from "./pageCapabilities";
import {
	type NavBadgeMap,
	type NavItem,
	SELLER_FOOTER_ITEMS,
	SELLER_NAV_GROUPS,
} from "./sellerNav";

const NAV_GROUPS = SELLER_NAV_GROUPS;
const FOOTER_ITEMS = SELLER_FOOTER_ITEMS;

const ALL_NAV_ITEMS: NavItem[] = [
	...NAV_GROUPS.flatMap((group) => group.items),
	...FOOTER_ITEMS,
];

export default function SellerCenterShell({
	children,
}: {
	children: React.ReactNode;
}) {
	const router = useObservedRouter("seller_center_shell");
	const { signOut } = useAuth();
	const {
		avatar,
		isPendingDeletion,
		isSuspended,
		loading: identityLoading,
		seller,
		ownerName,
		sellerId,
		setupIncomplete,
		storeName: displayName,
	} = useSellerIdentity();

	// 賣場建立未完成（沒店鋪 404，或 needs_onboarding／手機未驗）：整個工作區
	// 的資料都拿不到，把人送回 /seller/onboarding 完成驗證，禁止進入這些頁面。
	React.useEffect(() => {
		if (setupIncomplete) router.replace("/seller/onboarding");
	}, [setupIncomplete, router]);
	// backend#3066 裁決 B：免用統一發票的賣家在賣家中心看到的是「收據」而不是「發票」。
	// 端點 16 已被 S01 用同一把 queryKey 查過 ⇒ 走 TanStack 快取，不會多打一次 API。
	// 停權時 hook 自己不發查詢（blockedBySuspension），此時退回發票措辭即可。
	const { overview } = useSellerInvoiceOverview();
	const receiptOnly = isReceiptOnlySeller(overview);
	const { count: unreadMessageCount } = useChatUnread();
	const { count: unreadNotificationCount } = useNotificationUnread();
	// 賣家事件與串流漏事件的訊號來自根 layout 唯一的那條串流；這裡只訂閱，不開連線。
	useSellerWorkspaceRealtime();
	const pathname = usePathname() ?? "";
	const isMessagesRoute =
		pathname === "/seller-center/messages" ||
		pathname.startsWith("/seller-center/messages/");
	const isNotificationsRoute =
		pathname === "/seller-center/notifications" ||
		pathname.startsWith("/seller-center/notifications/");

	// 進入的頁面需要的能力不在 `GetMyStore.capabilities` 時，主動導去公告頁（不等撞錯誤才反應）。
	const restrictedBlocked = isBlockedByCapabilities(seller, pathname);
	React.useEffect(() => {
		if (restrictedBlocked) router.replace("/seller-center/suspended");
	}, [restrictedBlocked, router]);

	// 補即時性落差：店家查詢 30s staleTime 快取還沒反映最新停權狀態前，任何 authed
	// 呼叫先撞到 SELLER_SUSPENDED（REST 9240 或 Connect）→ 廣播全域事件，這裡監聽
	// 並 invalidate 讓快取盡快追上；這一頁到底開不開放，由更新後的能力判斷決定
	// （不在事件裡猜，否則背景 badge 輪詢一觸發就會把停留在開放頁的賣家彈回公告頁）。
	const invalidateSellerSelf = useInvalidateSellerSelf();
	React.useEffect(() => {
		const onSuspended = () => {
			void invalidateSellerSelf();
		};
		window.addEventListener(SELLER_SUSPENDED_EVENT, onSuspended);
		return () =>
			window.removeEventListener(SELLER_SUSPENDED_EVENT, onSuspended);
	}, [invalidateSellerSelf]);
	// The nav badges stay fresh without polling: `useSellerWorkspaceRealtime`
	// invalidates the order/after-sale/dashboard families when the stream pushes a
	// seller event, and each query refetches when the tab regains focus (30s
	// staleTime). Stock and post counts have no push signal; focus is their only
	// refresh.
	// 這三支 badge 查詢各自需要一種能力（dashboard 與 posts 是 MARKETING、products 是
	// PRODUCT_MANAGE）：沒有該能力時伺服器必拒絕，直接跳過，避免無謂的錯誤噪音與觸發
	// SELLER_SUSPENDED_EVENT 的背景輪詢風暴。店家還沒讀到時照常查詢。
	const canUse = (capability: SellerCapability) =>
		seller === null || hasCapability(seller, capability);
	const { data: dashboard } = useSellerDashboard({
		enabled: canUse(SellerCapability.MARKETING),
	});
	// Same single-cache treatment for the low-stock badge: one identity-scoped
	// query instead of refetching the whole product table on every navigation.
	// Product mutations invalidate this via useInvalidateSellerProducts.
	const { data: products } = useSellerProducts(
		{ page: 1, pageSize: 100 },
		{ enabled: canUse(SellerCapability.PRODUCT_MANAGE) },
	);
	const lowStockCount = React.useMemo(
		() =>
			(products?.records ?? []).filter(
				(product) => Number(product.stock ?? 0) <= 5,
			).length,
		[products],
	);
	// Same identity-scoped cache entry as SellerPostsClient's list query, so
	// opening the posts page doesn't trigger a second fetch.
	const { count: postsCount } = useSellerPosts({
		enabled: canUse(SellerCapability.MARKETING),
	});

	const [open, setOpen] = React.useState(false);
	const [signingOut, setSigningOut] = React.useState(false);
	const identitySubtitle = sellerId
		? `賣家 ID：${sellerId}`
		: ownerName || "賣家中心";

	// Auto-close the drawer when the user navigates to a different
	// section (clicking a NavLink doesn't unmount the shell so we
	// need an explicit hook).
	React.useEffect(() => {
		void pathname;
		setOpen(false);
	}, [pathname]);

	// Lock body scroll while the drawer is open.
	React.useEffect(() => {
		if (!open) return;
		const releaseScrollLock = acquireBodyScrollLock();
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") setOpen(false);
		};
		document.addEventListener("keydown", onKey);
		return () => {
			releaseScrollLock();
			document.removeEventListener("keydown", onKey);
		};
	}, [open]);

	const navBadges = React.useMemo<NavBadgeMap>(() => {
		const orderCounts = dashboard?.orderStatusCounts;
		return {
			orders: orderCounts
				? orderCounts.pendingShipmentCount + orderCounts.aftersalesCount
				: undefined,
			inventory: lowStockCount,
			messages: unreadMessageCount,
			notifications: unreadNotificationCount,
			posts: postsCount,
		};
	}, [
		dashboard,
		lowStockCount,
		unreadMessageCount,
		unreadNotificationCount,
		postsCount,
	]);

	const handleSignOut = async () => {
		if (signingOut) return;
		setSigningOut(true);
		// "登出" in the seller workspace footer signs out of the active
		// session only and routes back to the public guest homepage —
		// matches the SellerWorkspaceLock contract that sellers must
		// re-authenticate from the public site to view buyer surfaces.
		try {
			await signOut();
		} catch {
			// 撤銷失敗＝session 仍有效（signOut 已跳 toast），留在賣家工作區。
			setSigningOut(false);
			return;
		}
		// 用整頁重載（非 soft router.push）落回首頁：若 session 期間剛好上了新
		// 部署，這個分頁仍握著舊 chunk manifest，soft 導航會去要已被刪除的舊
		// chunk 而卡在首頁 skeleton；整頁載入會抓當前 build（同 login 的手法）。
		commitDocumentNavigation("/");
	};

	// seller-self 首抓未回前，onboarding 狀態未知。Hard entry 仍由
	// InitialPageLoader 持有；soft entry 會由 shared Link 先暖好同一 query，
	// 因此這裡不畫任何 page-local skeleton。
	if (identityLoading) {
		return null;
	}

	// 驗證未完成：別渲染工作區內容（會打一堆 404 API、閃破圖）。顯示過場，
	// 上面的 effect 會把人導去 /seller/onboarding。
	if (setupIncomplete) {
		return (
			<div className="grid min-h-[60vh] place-items-center px-6 text-center">
				<div className="flex flex-col items-center gap-3">
					<p className="text-[0.9375rem] font-medium text-foreground">
						請先建立賣場並完成手機驗證
					</p>
					<p className="text-[0.8125rem] text-foreground-soft">
						正在前往建立賣場頁…
					</p>
				</div>
			</div>
		);
	}

	// 限制中的店家進入能力不足的頁面：別閃一下原本內容才被上面的 effect 導走。
	if (restrictedBlocked) {
		return (
			<div className="grid min-h-[60vh] place-items-center px-6 text-center">
				<div className="flex flex-col items-center gap-3">
					<p className="text-[0.9375rem] font-medium text-foreground">
						{isSuspended ? "商店已暫停營業" : "帳號刪除處理中"}
					</p>
					<p className="text-[0.8125rem] text-foreground-soft">
						{isSuspended ? "正在前往停權公告頁…" : "正在前往公告頁…"}
					</p>
				</div>
			</div>
		);
	}

	return (
		<div
			className="seller-shell"
			data-hk-seller-workspace-ready
			data-sidebar-open={open}
		>
			{/* Mobile-only top bar with hamburger + the active section name.
			    Hidden on desktop via CSS (display: none) — the sticky
			    sidebar makes this redundant on wider viewports. */}
			<header className="seller-shell__mobile-bar">
				<button
					type="button"
					className="seller-shell__menu-btn"
					aria-label={open ? "關閉選單" : "開啟選單"}
					aria-expanded={open}
					onClick={() => setOpen((v) => !v)}
				>
					<span className="seller-shell__menu-icon" aria-hidden="true" />
				</button>
				<strong className="seller-shell__mobile-title">賣家中心</strong>
				<nav
					className="seller-shell__mobile-quick-tabs"
					aria-label="賣家中心快速入口"
				>
					<Link
						href="/seller-center/messages"
						className={`seller-shell__mobile-quick-tab seller-shell__mobile-message-tab${isMessagesRoute ? " is-active" : ""}`}
						aria-label={
							unreadMessageCount > 0
								? `訊息中心（${unreadMessageCount > 99 ? "99+" : unreadMessageCount} 則未讀）`
								: "訊息中心"
						}
						aria-current={isMessagesRoute ? "page" : undefined}
						data-hkui-button
					>
						<span
							className="seller-shell__mobile-quick-icon seller-shell__mobile-message-icon"
							aria-hidden="true"
						>
							<SellerSidebarIcon name="chat" />
						</span>
						<span className="seller-shell__mobile-quick-label">訊息</span>
						{unreadMessageCount > 0 ? (
							<span className="seller-shell__mobile-quick-badge seller-shell__mobile-message-badge">
								{unreadMessageCount > 99 ? "99+" : unreadMessageCount}
							</span>
						) : null}
					</Link>
					<Link
						href="/seller-center/notifications"
						className={`seller-shell__mobile-quick-tab seller-shell__mobile-notification-tab${isNotificationsRoute ? " is-active" : ""}`}
						aria-label={
							unreadNotificationCount > 0
								? `通知中心（${unreadNotificationCount > 99 ? "99+" : unreadNotificationCount} 則未讀）`
								: "通知中心"
						}
						aria-current={isNotificationsRoute ? "page" : undefined}
						data-hkui-button
					>
						<span
							className="seller-shell__mobile-quick-icon seller-shell__mobile-notification-icon"
							aria-hidden="true"
						>
							<SellerSidebarIcon name="notification" />
						</span>
						<span className="seller-shell__mobile-quick-label">通知</span>
						{unreadNotificationCount > 0 ? (
							<span className="seller-shell__mobile-quick-badge seller-shell__mobile-notification-badge">
								{unreadNotificationCount > 99 ? "99+" : unreadNotificationCount}
							</span>
						) : null}
					</Link>
				</nav>
				<ThemeToggle className="max-[960px]:ml-auto max-[960px]:size-11" />
				<div className="seller-shell__mobile-avatar" aria-hidden="true">
					<SellerAvatarImage src={avatar} alt="" variant="identity" />
				</div>
			</header>

			{/* Backdrop — only paints on mobile while the drawer is open. */}
			<button
				type="button"
				className="seller-shell__backdrop"
				aria-label="關閉側邊選單"
				aria-hidden={!open}
				tabIndex={open ? 0 : -1}
				onClick={() => setOpen(false)}
			/>

			<aside className="seller-sidebar" aria-hidden={false}>
				<div className="seller-sidebar__profile">
					<div className="seller-sidebar__avatar" aria-hidden="true">
						<SellerAvatarImage src={avatar} alt="" variant="identity" />
					</div>
					<div className="seller-sidebar__profile-meta">
						<strong title={displayName}>{displayName}</strong>
						<small title={identitySubtitle}>{identitySubtitle}</small>
					</div>
					<button
						type="button"
						className="seller-sidebar__close"
						aria-label="關閉選單"
						onClick={() => setOpen(false)}
					>
						<IconClose />
					</button>
				</div>

				<nav className="seller-sidebar__nav" aria-label="賣家中心導覽">
					{NAV_GROUPS.map((group) => (
						<div key={group.heading} className="seller-sidebar__group">
							<div className="seller-sidebar__heading">{group.heading}</div>
							{group.items.map((item) => (
								<NavLink
									key={item.href}
									item={framedNavItem(item, receiptOnly)}
									pathname={pathname}
									badges={navBadges}
								/>
							))}
						</div>
					))}
				</nav>

				<div className="seller-sidebar__footer">
					{FOOTER_ITEMS.map((item) => (
						<NavLink
							key={item.href}
							item={item}
							pathname={pathname}
							badges={navBadges}
						/>
					))}
					<Button
						type="button"
						variant="light"
						color="danger"
						className="seller-sidebar__item seller-sidebar__item--signout h-auto justify-start"
						disabled={signingOut}
						isLoading={signingOut}
						loadingLabel="登出賣家中心中…"
						onClick={() => void handleSignOut()}
					>
						<span className="seller-sidebar__icon" aria-hidden="true">
							<SellerSidebarIcon name="logout" />
						</span>
						<span className="seller-sidebar__label">登出</span>
					</Button>
				</div>
			</aside>

			<main className="seller-main">
				<TerminationNoticeBanner />
				{children}
			</main>
		</div>
	);
}

function NavLink({
	item,
	pathname,
	badges,
}: {
	item: NavItem;
	pathname: string;
	badges: NavBadgeMap;
}) {
	const isActive = isNavItemActive(item, pathname);
	const badge = resolveBadge(item, badges);

	return (
		<Link
			href={item.href}
			className={
				isActive
					? "seller-sidebar__item seller-sidebar__item--active"
					: "seller-sidebar__item"
			}
			aria-current={isActive ? "page" : undefined}
		>
			<span className="seller-sidebar__icon" aria-hidden="true">
				{item.icon}
			</span>
			<span className="seller-sidebar__label">{item.label}</span>
			{badge !== undefined && (
				<span className="seller-sidebar__badge">{badge}</span>
			)}
		</Link>
	);
}

function navMatchTarget(item: NavItem): string {
	return item.matchPrefix ?? item.href.split("?")[0];
}

function pathMatchesItem(item: NavItem, pathname: string): boolean {
	const target = navMatchTarget(item);
	return pathname === target || pathname.startsWith(`${target}/`);
}

// Active = this item matches the current path AND no other nav item is a more
// specific (longer) match. Longest-match keeps detail routes (e.g.
// /seller-center/orders/ORD123) highlighting their parent while avoiding the
// double-highlight of nested siblings like /me and /me/shipping.
function isNavItemActive(item: NavItem, pathname: string): boolean {
	if (!pathMatchesItem(item, pathname)) return false;
	const targetLength = navMatchTarget(item).length;
	return !ALL_NAV_ITEMS.some(
		(other) =>
			other !== item &&
			pathMatchesItem(other, pathname) &&
			navMatchTarget(other).length > targetLength,
	);
}

function resolveBadge(
	item: NavItem,
	badges: NavBadgeMap,
): string | number | undefined {
	const value = item.badgeKey ? badges[item.badgeKey] : item.badge;
	if (value == null || value === "" || value === 0) return undefined;
	if (typeof value === "number") return value > 99 ? "99+" : value;
	return value;
}

function IconClose() {
	return <AppIcon name="close" />;
}
