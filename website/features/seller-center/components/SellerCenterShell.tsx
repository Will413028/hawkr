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
import { usePendingInquiryCount } from "@/lib/hooks/useSellerInquiries";
import { useSellerInvoiceOverview } from "@/lib/hooks/useSellerInvoices";
import { useSellerPosts } from "@/lib/hooks/useSellerPosts";
import { useSellerProducts } from "@/lib/hooks/useSellerProducts";
import { useInvalidateSellerSelf } from "@/lib/hooks/useSellerSelf";
import {
	SellerWorkspaceRealtimeStatusContext,
	useSellerWorkspaceRealtime,
} from "@/lib/hooks/useSellerWorkspaceRealtime";
import { usePathname } from "@/lib/intl/navigation";
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

	React.useEffect(() => {
		if (setupIncomplete) router.replace("/seller/onboarding");
	}, [setupIncomplete, router]);
	const { overview } = useSellerInvoiceOverview();
	const receiptOnly = isReceiptOnlySeller(overview);
	const { count: unreadMessageCount } = useChatUnread();
	const { count: unreadNotificationCount } = useNotificationUnread();
	const realtimeStatus = useSellerWorkspaceRealtime(!setupIncomplete);
	const pathname = usePathname() ?? "";
	const isMessagesRoute =
		pathname === "/seller-center/messages" ||
		pathname.startsWith("/seller-center/messages/");
	const isNotificationsRoute =
		pathname === "/seller-center/notifications" ||
		pathname.startsWith("/seller-center/notifications/");

	const restrictedBlocked = isBlockedByCapabilities(seller, pathname);
	React.useEffect(() => {
		if (restrictedBlocked) router.replace("/seller-center/suspended");
	}, [restrictedBlocked, router]);

	const invalidateSellerSelf = useInvalidateSellerSelf();
	React.useEffect(() => {
		const onSuspended = () => {
			void invalidateSellerSelf();
		};
		window.addEventListener(SELLER_SUSPENDED_EVENT, onSuspended);
		return () =>
			window.removeEventListener(SELLER_SUSPENDED_EVENT, onSuspended);
	}, [invalidateSellerSelf]);
	const canUse = (capability: SellerCapability) =>
		seller === null || hasCapability(seller, capability);
	const { data: dashboard } = useSellerDashboard({
		enabled: canUse(SellerCapability.MARKETING),
	});
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
	const { count: postsCount } = useSellerPosts({
		enabled: canUse(SellerCapability.MARKETING),
	});

	const pendingInquiryCount = usePendingInquiryCount();

	const [open, setOpen] = React.useState(false);
	const [signingOut, setSigningOut] = React.useState(false);
	const identitySubtitle = sellerId
		? `賣家 ID：${sellerId}`
		: ownerName || "賣家中心";

	React.useEffect(() => {
		void pathname;
		setOpen(false);
	}, [pathname]);

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
			inquiries: pendingInquiryCount,
		};
	}, [
		dashboard,
		lowStockCount,
		unreadMessageCount,
		unreadNotificationCount,
		postsCount,
		pendingInquiryCount,
	]);

	const handleSignOut = async () => {
		if (signingOut) return;
		setSigningOut(true);
		try {
			await signOut();
		} catch {
			setSigningOut(false);
		}
	};

	if (identityLoading) {
		return null;
	}

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
		<SellerWorkspaceRealtimeStatusContext.Provider value={realtimeStatus}>
			<div
				className="seller-shell"
				data-hk-seller-workspace-ready
				data-sidebar-open={open}
			>

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
									{unreadNotificationCount > 99
										? "99+"
										: unreadNotificationCount}
								</span>
							) : null}
						</Link>
					</nav>
					<ThemeToggle className="max-[960px]:ml-auto max-[960px]:size-11" />
					<div className="seller-shell__mobile-avatar" aria-hidden="true">
						<SellerAvatarImage src={avatar} alt="" variant="identity" />
					</div>
				</header>


				<button
					type="button"
					className="seller-shell__backdrop"
					aria-label="關閉側邊選單"
					aria-hidden={!open}
					tabIndex={open ? 0 : -1}
					onClick={() => setOpen(false)}
				/>

				<aside
					className="seller-sidebar"
					aria-hidden={false}
					aria-label="賣家中心側邊欄"
				>
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

				<div className="seller-main">
					<TerminationNoticeBanner />
					{children}
				</div>
			</div>
		</SellerWorkspaceRealtimeStatusContext.Provider>
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
