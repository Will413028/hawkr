"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import AppDownloadCTA from "@/components/shared/AppDownloadCTA";
import Avatar from "@/components/shared/Avatar";
import BrandLogo from "@/components/shared/BrandLogo";
import CartBubble from "@/components/shared/CartBubble";
import ChatBubble from "@/components/shared/ChatBubble";
import LazyAppIcon from "@/components/shared/LazyAppIcon";
import Link from "@/components/shared/Link";
import MegaMenu, {
	type MegaMenuDynamicLinks,
} from "@/components/shared/MegaMenu";
import NotificationBell from "@/components/shared/NotificationBell";
import {
	SITE_NAV_USER_MENU,
	SITE_NAV_USER_MENU_FOOTER,
	SITE_NAV_USER_MENU_FOOTER_BTN,
	SITE_NAV_USER_MENU_HEADER,
	SITE_NAV_USER_MENU_ITEM,
	SITE_NAV_USER_MENU_LIST,
} from "@/components/shared/navUserMenu";
import {
	type SellerMenuChrome,
	SellerMenuLoader,
	type SignOutAction,
} from "@/components/shared/SellerNavbarLoader";
import TextScaleToggle from "@/components/shared/TextScaleToggle";
import ThemeToggle from "@/components/shared/ThemeToggle";
import { useDisclosureMenu } from "@/components/shared/useDisclosureMenu";
import Button from "@/components/ui/Button";
import AccountMenuIcon from "@/features/account/components/AccountMenuIcon";
import {
	BUYER_MENU_ITEMS as buyerMenuItems,
	buyerMenuLabel,
} from "@/features/account/menu";
import HomeCompactSearch from "@/features/home/components/HomeCompactSearch";
import { resolveAuthRailMode } from "@/lib/auth/auth-rail";
import { useAuth } from "@/lib/auth/context";
import { loginPath } from "@/lib/auth/redirect";
import { getActiveRole, type RoleSlot } from "@/lib/auth/storage";
import { navItems } from "@/lib/content/data";
import { usePathname } from "@/lib/intl/navigation";
import {
	cancelMainNavPointerIntent,
	prefetchMainNavOnIntent,
} from "@/lib/navigation/main-nav-prefetch";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { cn } from "@/lib/utils/cn";

function isActive(pathname: string, href: string) {
	if (href === "/") return pathname === "/";
	return pathname === href || pathname.startsWith(`${href}/`);
}

const SITE_NAV =
	"site-nav relative flex items-center justify-center h-[84px] px-6 [@media(max-width:768px)]:h-[56px] [@media(max-width:768px)]:px-0";

const SITE_NAV_INNER = cn(
	"site-nav__inner flex w-full items-center justify-between max-w-[1440px]",
	"h-[84px] min-h-[84px] p-[8px_36px_8px_24px] rounded-[9999px]",
	"bg-[var(--page-nav-pill-bg)]",
	"[box-shadow:var(--page-nav-pill-shadow)]",
	"transition-[box-shadow,transform]",
	"dark:[background-image:var(--page-nav-pill-gradient-dark)]",
	"[@media(max-width:768px)]:h-[56px] [@media(max-width:768px)]:min-h-[56px] [@media(max-width:768px)]:p-[8px_10px] [@media(max-width:768px)]:rounded-none",
	"[@media(max-width:768px)]:[box-shadow:var(--page-nav-pill-shadow-mobile)]",
);

const SITE_NAV_CENTER =
	"site-nav__center relative z-[42] flex items-center gap-x-[6px] ml-[34px] mr-auto [@media(max-width:1280px)]:gap-x-4 [@media(max-width:840px)]:hidden";

const SITE_NAV_RIGHT =
	"site-nav__right relative z-[45] flex items-center gap-x-[10px] [@media(max-width:1280px)]:gap-x-2 [@media(max-width:840px)]:gap-x-[2px] [@media(max-width:840px)]:ml-auto";

const SITE_NAV_LOGO =
	"site-nav__logo relative z-[45] flex flex-none items-center no-underline";

const NAV_ITEM = cn(
	"nav-item relative isolate inline-flex items-center h-[40px] min-h-[44px] px-[13px] rounded-[999px]",
	"text-[0.9375rem] font-bold leading-[40px] tracking-[0.04em] whitespace-nowrap cursor-pointer",
	"transition-colors hover:bg-[var(--page-nav-item-hover-bg)] hover:text-[var(--page-nav-item-hover-fg)]!",
);
const NAV_ITEM_DEFAULT = "text-[var(--ink-body)]!";
const NAV_ITEM_ACTIVE = "text-[var(--theme-text-accent)]!";

const SITE_NAV_ACTION = cn(
	"site-nav-action static inline-flex items-center justify-center gap-x-2 min-h-11 px-[13px] rounded-[999px]",
	"text-[0.9375rem] font-bold leading-none no-underline whitespace-nowrap transition-[background-color,color,transform]",
	"[@media(max-width:1280px)]:w-11 [@media(max-width:1280px)]:min-w-11 [@media(max-width:1280px)]:gap-x-0 [@media(max-width:1280px)]:p-0",
	"hover:bg-[var(--page-nav-action-hover-bg)] hover:text-[var(--page-nav-item-hover-fg)]! hover:[transform:translateY(-1px)]",
	"motion-reduce:hover:[transform:none]",
);
const ACTION_DEFAULT =
	"text-[var(--ink-body)]! [@media(max-width:840px)]:dark:text-[var(--page-nav-action-mobile-fg-dark)]!";
const ACTION_ACTIVE =
	"bg-[var(--surface-card)] text-[var(--color-brand-primary-strong)]! [transform:translateY(-1px)]";

const SITE_NAV_LOGIN = cn(
	"site-nav-login static inline-flex items-center justify-center gap-x-2 min-h-11 px-[13px] rounded-[999px]",
	"text-[0.9375rem] font-bold leading-none no-underline whitespace-nowrap transition-[background-color,color,transform]",
	"dark:border-[var(--page-nav-login-border-dark)]",
	"[@media(max-width:1280px)]:w-11 [@media(max-width:1280px)]:min-w-11 [@media(max-width:1280px)]:gap-x-0 [@media(max-width:1280px)]:p-0 [@media(max-width:840px)]:hidden",
	"hover:bg-[var(--surface-card)] hover:text-[var(--color-brand-primary-strong)]! hover:[transform:translateY(-1px)]",
	"dark:hover:text-[var(--page-nav-login-hover-fg-dark)]!",
	"motion-reduce:hover:[transform:none]",
);
const LOGIN_DEFAULT = "text-[var(--ink-body)]!";

/*
 * Logged-in user-menu trigger + 賣家中心 CTA (步驟 6). The shared dropdown panel
 * classes live in navUserMenu.ts. The trigger is a <button> (no dark-link
 * repaint); only the name <span> needs `!` (house-dark `body span`). The trigger
 * collapses to icon-only at ≤1280 (name + chev hidden). The seller CTA is not
 * gate-able — SellerWorkspaceLock redirects a logged-in seller off buyer routes
 * so it only flashes pre-lock — so it is migrated CSS-faithfully (its dark colour
 * is the global `body a` repaint, matching the prior cascade) and verified by
 * inspection.
 */
const SITE_NAV_USER = "site-nav-user relative";
const SITE_NAV_USER_TRIGGER = cn(
	"site-nav-user__trigger inline-flex items-center justify-center gap-x-2 h-11 min-w-11 pt-1 pr-3 pb-1 pl-1 rounded-[999px] border-0 cursor-pointer",
	"bg-[var(--page-nav-user-trigger-bg)] text-[var(--page-nav-user-trigger-fg)] transition-[background]",
	"[@media(max-width:1280px)]:gap-x-0 [@media(max-width:1280px)]:p-1",
	"hover:bg-[var(--page-nav-user-trigger-hover-bg)] hover:text-[var(--color-brand-primary-strong)]",
	"aria-expanded:bg-[var(--page-nav-user-trigger-hover-bg)] aria-expanded:text-[var(--color-brand-primary-strong)]",
);
const SITE_NAV_USER_AVATAR =
	"site-nav-user__avatar w-8 h-8 rounded-[50%] object-cover bg-[var(--page-nav-user-avatar-bg)]";
const SITE_NAV_USER_NAME = cn(
	"site-nav-user__name block max-w-[96px] overflow-hidden text-ellipsis whitespace-nowrap",
	"text-[0.875rem] font-medium text-[var(--page-nav-user-name-fg)]!",
	"[@media(max-width:1440px)]:hidden",
);
const SITE_NAV_USER_CHEV =
	"site-nav-user__chev w-[14px] h-[14px] fill-none stroke-current [stroke-width:2] [stroke-linecap:round] transition-transform motion-reduce:transition-none [@media(max-width:1440px)]:hidden";

const SELLER_MENU_CHROME = {
	root: SITE_NAV_USER,
	trigger: SITE_NAV_USER_TRIGGER,
	avatar: SITE_NAV_USER_AVATAR,
	name: SITE_NAV_USER_NAME,
	chevron: SITE_NAV_USER_CHEV,
} satisfies SellerMenuChrome;

const SITE_NAV_SELLER_CTA = cn(
	"site-nav-seller-cta inline-flex items-center h-11 px-[18px] rounded-[999px] whitespace-nowrap no-underline",
	"border-[1.818px] border-[var(--page-nav-seller-cta-accent)] bg-[var(--color-neutral-cream)] text-[var(--page-nav-seller-cta-fg)] text-[0.875rem] font-medium",
	"transition-[background,color,transform]",
	"hover:bg-[var(--page-nav-seller-cta-accent)] hover:text-[var(--color-neutral-cream)] hover:[transform:translateY(-1px)]",
	"motion-reduce:hover:[transform:none]",
);

function BuyerMenu({ onSignOut }: { onSignOut: SignOutAction }) {
	const { identity } = useAuth();
	const [open, setOpen] = React.useState(false);
	const [isSigningOut, setIsSigningOut] = React.useState(false);
	const signOutPendingRef = React.useRef(false);
	const triggerRef = React.useRef<HTMLButtonElement>(null);
	const panelRef = React.useRef<HTMLDivElement>(null);

	const close = React.useCallback(() => setOpen(false), []);
	const openMenu = React.useCallback(() => setOpen(true), []);
	const isLocked = React.useCallback(() => signOutPendingRef.current, []);
	const menu = useDisclosureMenu({
		open,
		onClose: close,
		onOpen: openMenu,
		triggerRef,
		panelRef,
		isLocked,
	});

	const handleSignOut = React.useCallback(async () => {
		if (signOutPendingRef.current) return;
		signOutPendingRef.current = true;
		setIsSigningOut(true);
		try {
			if (await onSignOut()) setOpen(false);
		} finally {
			signOutPendingRef.current = false;
			setIsSigningOut(false);
		}
	}, [onSignOut]);

	const tCommon = useTranslations("common");
	const tMenu = useTranslations("account.menu");
	const displayName = identity?.nickname || tCommon("memberFallback");
	const avatar = identity?.avatarKey;

	return (
		<div className={SITE_NAV_USER}>
			<button
				type="button"
				ref={triggerRef}
				data-testid="account-menu-trigger"
				className={SITE_NAV_USER_TRIGGER}
				{...menu.triggerProps}
				aria-label={`${displayName} 的選單`}
				disabled={isSigningOut}
				onClick={() => setOpen((v) => !v)}
			>
				<Avatar
					src={avatar}
					alt={displayName}
					entity="buyer"
					className={SITE_NAV_USER_AVATAR}
				/>
				<span className={SITE_NAV_USER_NAME}>{displayName}</span>
				<svg
					className={cn(
						SITE_NAV_USER_CHEV,
						open && "[transform:rotate(180deg)]",
					)}
					viewBox="0 0 24 24"
					aria-hidden="true"
				>
					<path d="m6 9 6 6 6-6" />
				</svg>
			</button>

			{open && (
				<div ref={panelRef} className={SITE_NAV_USER_MENU} {...menu.panelProps}>
					<div className={SITE_NAV_USER_MENU_HEADER}>
						<strong>{displayName}</strong>

					</div>
					<ul className={SITE_NAV_USER_MENU_LIST}>
						{buyerMenuItems.map((item) => (
							<li key={item.href}>
								<Link
									href={item.href}
									{...menu.menuItemProps}
									aria-disabled={isSigningOut || undefined}
									className={cn(
										SITE_NAV_USER_MENU_ITEM,
										isSigningOut && "pointer-events-none opacity-50",
									)}
									onClick={(event) => {
										if (isSigningOut) {
											event.preventDefault();
											return;
										}
										setOpen(false);
									}}
								>
									<AccountMenuIcon
										name={item.icon}
										className="site-nav-user__menu-icon"
									/>
									{buyerMenuLabel(item, tMenu)}
								</Link>
							</li>
						))}
					</ul>
					<div className={SITE_NAV_USER_MENU_FOOTER}>
						<Button
							data-testid="logout-button"
							{...menu.menuItemProps}
							className={SITE_NAV_USER_MENU_FOOTER_BTN}
							variant="light"
							color="danger"
							fullWidth
							isLoading={isSigningOut}
							loadingLabel="登出中…"
							onClick={() => void handleSignOut()}
						>
							登出
						</Button>
					</div>
				</div>
			)}
		</div>
	);
}

interface NavbarProps {
	/** SSR-prefetched dynamic megamenu data (categories + hashtags). */
	megaMenuDynamicLinks?: MegaMenuDynamicLinks;
}

export default function Navbar({ megaMenuDynamicLinks }: NavbarProps = {}) {
	const pathname = usePathname() ?? "";
	const router = useObservedRouter("navbar");
	const { activeRole, status, signOut } = useAuth();
	const railMode = resolveAuthRailMode({ status, activeRole });
	const [bootRole, setBootRole] = React.useState<RoleSlot | null>(null);
	React.useEffect(() => {
		setBootRole(getActiveRole());
	}, []);
	const isHome = pathname === "/";
	const showHomeSearch =
		isHome && (railMode === "guest" || railMode === "buyer");
	const moveHomeSearchBelowNavbar = isHome && railMode === "buyer";
	const visibleMenuItems = React.useMemo(
		() =>
			showHomeSearch
				? navItems.filter((item) => item.href !== "/search")
				: navItems,
		[showHomeSearch],
	);

	const prefetchHome = React.useCallback(
		(intent: "pointer" | "focus" | "touch") => {
			prefetchMainNavOnIntent({
				router,
				href: "/",
				currentPathname: pathname,
				intent,
			});
		},
		[pathname, router],
	);

	const handleSignOut = async (): Promise<boolean> => {
		try {
			await signOut();
		} catch {
			return false;
		}
		return true;
	};

	return (
		<nav className={SITE_NAV} aria-label="主要導覽">
			<div className={SITE_NAV_INNER}>
				<Link
					href="/"
					prefetch={false}
					className={SITE_NAV_LOGO}
					aria-label="回到 Hawkr 首頁"
					onPointerEnter={(event) => {
						if (event.pointerType !== "touch") prefetchHome("pointer");
					}}
					onPointerLeave={() => cancelMainNavPointerIntent("/")}
					onPointerDown={(event) => {
						if (event.pointerType === "touch") prefetchHome("touch");
					}}
					onFocus={(event) => {
						if (event.currentTarget.matches(":focus-visible")) {
							prefetchHome("focus");
						}
					}}
					onClick={() => cancelMainNavPointerIntent("/")}
				>
					<BrandLogo className="site-nav-logo--full" />
					<BrandLogo variant="mark" className="site-nav-logo--tablet" />
				</Link>

				<div className={cn(SITE_NAV_CENTER, showHomeSearch && "mr-0")}>
					<MegaMenu
						items={visibleMenuItems}
						isItemActive={(item) => isActive(pathname, item.href)}
						triggerClassName={(item) =>
							cn(
								NAV_ITEM,
								isActive(pathname, item.href)
									? NAV_ITEM_ACTIVE
									: NAV_ITEM_DEFAULT,
							)
						}
						dynamicLinks={megaMenuDynamicLinks}
					/>
				</div>

				{showHomeSearch ? (
					<HomeCompactSearch
						className={cn(
							"mx-[10px] w-auto min-w-[180px] flex-1",
							"[@media(max-width:1280px)]:mx-[2px]",
							"[@media(max-width:840px)]:mx-[8px] [@media(max-width:840px)]:w-auto [@media(max-width:840px)]:min-w-0 [@media(max-width:840px)]:flex-1",
							moveHomeSearchBelowNavbar && "[@media(max-width:840px)]:hidden",
						)}
					/>
				) : null}

				<div className={SITE_NAV_RIGHT}>

					<ThemeToggle className="size-11" />



					{status === "signedIn" && (
						<TextScaleToggle className="size-11 [@media(max-width:840px)]:hidden" />
					)}

					<AppDownloadCTA
						variant="compact"
						className={cn("site-nav-app-cta", showHomeSearch && "h-10!")}
					/>

					<CartBubble
						href="/cart"
						className={cn(
							SITE_NAV_ACTION,
							isActive(pathname, "/cart") ? ACTION_ACTIVE : ACTION_DEFAULT,
						)}
						aria-label="購物車"
					>
						<span
							className="site-nav-action__icon site-nav-action__icon--cart"
							aria-hidden="true"
						>
							<LazyAppIcon name="cart" size={24} />
						</span>
						<span aria-hidden="true">購物車</span>
					</CartBubble>


					{railMode === "skeleton" ? (
						<>
							{bootRole === "buyer" && (
								<>
									<span
										className="h-11 w-14 animate-pulse rounded-full bg-default-200/70"
										aria-hidden="true"
									/>
									<span
										className="h-11 w-14 animate-pulse rounded-full bg-default-200/70"
										aria-hidden="true"
									/>
								</>
							)}
							<span
								className="size-11 animate-pulse rounded-full bg-default-200/70"
								aria-hidden="true"
							/>
						</>
					) : null}
					{railMode === "buyer" && (
						<>
							<ChatBubble
								href="/account/messages"
								className={cn(
									SITE_NAV_ACTION,
									isActive(pathname, "/account/messages")
										? ACTION_ACTIVE
										: ACTION_DEFAULT,
								)}
								aria-label="訊息"
							>
								<span
									className="site-nav-action__icon site-nav-action__icon--message"
									aria-hidden="true"
								>
									<LazyAppIcon name="message" size={24} />
								</span>
								<span aria-hidden="true">訊息</span>
							</ChatBubble>
							<NotificationBell
								href="/account/notifications"
								className={cn(
									SITE_NAV_ACTION,
									isActive(pathname, "/account/notifications")
										? ACTION_ACTIVE
										: ACTION_DEFAULT,
								)}
								aria-label="通知"
							>
								<span
									className="site-nav-action__icon site-nav-action__icon--notification"
									aria-hidden="true"
								>
									<LazyAppIcon name="notification" size={22} />
								</span>
								<span aria-hidden="true">通知</span>
							</NotificationBell>
						</>
					)}

					{railMode === "skeleton" ? null : railMode === "buyer" ? (
						<BuyerMenu onSignOut={handleSignOut} />
					) : railMode === "guest" ? (
						<Link
							href={loginPath({
								redirect: pathname === "/login" ? "" : pathname,
							})}
							hardNavigation
							className={cn(
								SITE_NAV_LOGIN,
								isActive(pathname, "/login") ? ACTION_ACTIVE : LOGIN_DEFAULT,
							)}
						>
							<span className="site-nav-login__icon" aria-hidden="true">
								<LazyAppIcon name="login" size={24} />
							</span>
							<span>登入</span>
						</Link>
					) : (
						<>
							<Link
								href="/seller-center/dashboard"
								className={SITE_NAV_SELLER_CTA}
								data-testid="seller-center-link"
							>
								賣家中心
							</Link>
							<SellerMenuLoader
								onSignOut={handleSignOut}
								chrome={SELLER_MENU_CHROME}
							/>
						</>
					)}
				</div>
			</div>
		</nav>
	);
}
