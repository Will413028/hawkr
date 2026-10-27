"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import BrandLogo from "@/components/shared/BrandLogo";
import ChatBubble from "@/components/shared/ChatBubble";
import Link from "@/components/shared/Link";
import NotificationBell from "@/components/shared/NotificationBell";
import {
	SITE_NAV_USER_MENU,
	SITE_NAV_USER_MENU_FOOTER,
	SITE_NAV_USER_MENU_FOOTER_BTN,
	SITE_NAV_USER_MENU_HEADER,
	SITE_NAV_USER_MENU_ITEM,
	SITE_NAV_USER_MENU_LIST,
} from "@/components/shared/navUserMenu";
import SellerAvatarImage from "@/components/shared/SellerAvatarImage";
import type {
	SellerMenuChrome,
	SignOutAction,
} from "@/components/shared/SellerNavbarLoader";
import ThemeToggle from "@/components/shared/ThemeToggle";
import { useDisclosureMenu } from "@/components/shared/useDisclosureMenu";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
import { usePathname } from "@/lib/intl/navigation";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { isStandaloneFlowPath } from "@/lib/navigation/standalone-flow";
import { cn } from "@/lib/utils/cn";

/**
 * Renders the existing site Navbar for guest/buyer (per spec: "buyer 上方跟現在一樣"),
 * and a dedicated seller header when role === 'seller'.
 *
 * The seller header replaces the public marketing nav with a workspace-style header:
 *   brand · workspace nav (商品/訂單/客戶/分析) · search · notifications · avatar
 */

/*
 * Seller-header chrome — Tailwind migration (子刀 5).
 *
 * The seller header renders only on /seller-center/* and its live styling was
 * split: navigation-shell.css (`body .seller-header*`, everywhere) supplied the
 * layout + children, and seller-center.css (`body .page-wrapper:has(.seller-
 * shell) .seller-header__inner` + a dark block) supplied the pill surface and
 * dark colours. PR-B's lessons apply: box-shadow/transform via arbitrary
 * *properties* (`[ box-shadow:… ]`/`[transform:translateY(-1px)]`), and `<a>`
 * dark text via `!` to beat the unlayered dark-link rules in
 * 01-tokens-theme.css (`body a{color:--color-ink-200}` for non-exception links; the
 * `color:inherit` exception for `a.seller-header__nav-item`). Every BEM class is
 * KEPT as a hook (gate selectors, the seller-center `@960` hide + `.auto-nav`
 * position, the `.seller-header__brand-logo` lockup sizing, the
 * `.seller-header__icon-btn .notif-bell__badge` override, and the shared
 * `.site-nav-user__menu*` dropdown deferred to 子刀 6). No backdrop-filter: the
 * pill's computed backdrop has always been `none` (same as the buyer pill).
 *
 * Colours moved to `--page-seller-nav-*` in styles/foundation/00-color-config.css
 * (identical values, light + dark) — the header used to carry 26 lines of raw
 * hex/rgb, including two dark-only hexes with no token at all.
 */
const SELLER_HEADER =
	"seller-header relative w-full h-[84px] px-11 max-[768px]:h-[56px] max-[768px]:px-0";

const SELLER_HEADER_INNER = cn(
	"seller-header__inner mx-auto flex items-center gap-x-10",
	"max-w-[var(--seller-workspace-max-width,1440px)] h-[84px] min-h-[84px] px-14 rounded-[999px]",
	"max-[768px]:h-[56px] max-[768px]:min-h-[56px] max-[768px]:gap-x-3 max-[768px]:rounded-none max-[768px]:px-2.5",
	"border border-[var(--page-seller-nav-border)]",
	"text-[var(--page-seller-nav-fg)]",
	// Pill surface: cream gradient over a translucent solid (gate reads the solid
	// as background-color); dark = primary gradient over near-black.
	"bg-[var(--page-seller-nav-pill-bg)] [background-image:var(--page-seller-nav-pill-gradient)]",
	"[box-shadow:var(--page-seller-nav-pill-shadow)]",
);

const SELLER_BRAND =
	"seller-header__brand inline-flex shrink-0 items-center gap-0 no-underline text-[var(--page-seller-nav-brand-fg)]!";

// flex-1 + overflow-x-auto: the links fill the middle and scroll horizontally
// when the viewport is too narrow, instead of overflowing into the actions/CTA
// (the brand + actions are shrink-0). scrollbar hidden to match the pill chrome.
//
// The scrollbar being hidden left NO affordance that the row scrolls at all, so
// the last tabs were discoverable only by luck. The mask fades the final 32px:
// pure CSS, no extra element, and when the row is short the fade lands on the
// empty tail instead of on a label. (`#000`/`transparent` here are mask ALPHA
// stops, not UI colour — a mask has no palette to tokenise.)
const SELLER_NAV = cn(
	"seller-header__nav flex flex-1 min-w-0 items-center gap-x-1 overflow-x-auto",
	"[scrollbar-width:none] [&::-webkit-scrollbar]:hidden max-[840px]:hidden",
	"[mask-image:linear-gradient(to_right,#000_calc(100%-32px),transparent)]",
);

const SELLER_NAV_ITEM = cn(
	"seller-header__nav-item relative inline-flex items-center justify-center min-h-11 px-[14px] rounded-[999px]",
	"text-[0.875rem] font-bold no-underline whitespace-nowrap transition-[background-color,color,transform]",
	"hover:bg-[var(--page-seller-nav-item-hover-bg)] hover:text-[var(--page-seller-nav-item-hover-fg)]! hover:[transform:translateY(-1px)]",
	"motion-reduce:hover:[transform:none]",
);
const SELLER_NAV_ITEM_DEFAULT = "text-[var(--page-seller-nav-item-fg)]!";
const SELLER_NAV_ITEM_ACTIVE = cn(
	"bg-[var(--page-seller-nav-item-active-bg)] text-[var(--page-seller-nav-item-hover-fg)]! [transform:translateY(-1px)]",
);

const SELLER_ACTIONS =
	"seller-header__actions ml-auto flex shrink-0 items-center justify-end gap-x-[10px]";

const SELLER_CTA = cn(
	"seller-header__cta inline-flex items-center gap-x-2 h-11 px-[18px] rounded-[999px]",
	"bg-[var(--page-seller-nav-cta-bg)] text-[0.875rem] font-bold no-underline whitespace-nowrap",
	"text-[var(--page-seller-nav-cta-fg)]!",
	"[box-shadow:var(--page-seller-nav-cta-shadow)] transition-[background-color,transform]",
	"hover:bg-[var(--page-seller-nav-cta-hover-bg)] hover:[transform:translateY(-1px)]",
	"motion-reduce:hover:[transform:none]",
);

// An incomplete seller must always have a direct way back to onboarding. On a
// narrow header this collapses to a 44px edit icon so the brand, theme and
// account controls keep their intrinsic sizes instead of squeezing/overflowing.
const SELLER_SETUP_CTA = cn(
	SELLER_CTA,
	"seller-header__cta--setup max-[840px]:size-11 max-[840px]:justify-center max-[840px]:gap-0 max-[840px]:px-0",
);

// The normal workspace CTA is optional chrome on small screens; product
// creation remains available from the seller navigation/menu.
const SELLER_CREATE_CTA = cn(SELLER_CTA, "max-[840px]:hidden");

// ≤640px these collapse away entirely. That is deliberate: the header is
// already carrying brand + setup/create CTA + theme + avatar at 44px each, and
// a scrolling icon row on a phone header reads as a bug. 訊息中心 and 通知 both
// have entries in the account menu below, so neither destination is lost —
// which was the actual defect (通知 previously had NO mobile path at all).
const SELLER_ICON_BTN = cn(
	"seller-header__icon-btn grid place-items-center size-11 rounded-[999px] bg-transparent no-underline",
	"max-[640px]:hidden",
	"text-[var(--page-seller-nav-item-fg)]! transition-[background-color,color,transform]",
	"hover:bg-[var(--page-seller-nav-icon-hover-bg)] hover:text-[var(--page-seller-nav-item-hover-fg)]! hover:[transform:translateY(-1px)]",
	"motion-reduce:hover:[transform:none]",
);

// 24×24 icon box — mirrors the buyer navbar's `.site-nav-action__icon`
// sizing so 訊息/通知 render at the exact same icon size across both rails.
const SELLER_ICON_BOX =
	"inline-grid place-items-center w-6 h-6 overflow-hidden [line-height:0] [&_svg]:size-6";

// Match the buyer navbar's ThemeToggle so the toggle reads the same size
// across both rails.
const SELLER_THEME_BTN = "seller-header__theme-btn size-11";

const SELLER_AVATAR = cn(
	"seller-header__avatar grid place-items-center size-11 overflow-hidden rounded-[999px] cursor-pointer",
	"border-2 border-[var(--page-seller-nav-avatar-border)] text-[var(--page-seller-nav-avatar-fg)]",
	"bg-[var(--hkui-content2)] [background-image:var(--page-seller-nav-avatar-gradient)]",
	"[box-shadow:var(--page-seller-nav-avatar-shadow)]",
);

/**
 * ONE list for the three seller workspace surfaces.
 *
 * There used to be three hand-maintained arrays (`sellerNavItems`,
 * `sellerMenuItems`, `sellerRailMenuItems`) that had already drifted: 庫存管理
 * existed only in the rail menu, 市場行情 only in the other two, and 動態 was
 * called 動態 in one place and 動態管理 in the others. Flags make the drift
 * impossible to reintroduce silently.
 *
 * `notification` is new to the account menu: on phones the header's 訊息/通知
 * icon buttons are hidden, and 通知 had no other entry point at all.
 */
interface SellerWorkspaceRoute {
	label: string;
	href: string;
	icon: AppIconName;
	/** The horizontal tab row in the workspace header. */
	inNav: boolean;
	/** The avatar dropdown inside the workspace header. */
	inMenu: boolean;
	/** The seller dropdown shown on the buyer rail before the workspace lock fires. */
	inRail: boolean;
}

const sellerWorkspaceRoutes: SellerWorkspaceRoute[] = [
	{
		label: "賣家總覽",
		href: "/seller-center/dashboard",
		icon: "chart",
		inNav: true,
		inMenu: false,
		inRail: true,
	},
	{
		label: "商品管理",
		href: "/seller-center/products",
		icon: "goods",
		inNav: true,
		inMenu: true,
		inRail: true,
	},
	{
		label: "訂單管理",
		href: "/seller-center/orders",
		icon: "box",
		inNav: true,
		inMenu: true,
		inRail: true,
	},
	{
		label: "庫存管理",
		href: "/seller-center/inventory",
		icon: "stock",
		inNav: false,
		inMenu: false,
		inRail: true,
	},
	{
		label: "客戶管理",
		href: "/seller-center/customers",
		icon: "users",
		inNav: true,
		inMenu: true,
		inRail: true,
	},
	{
		label: "訊息中心",
		href: "/seller-center/messages",
		icon: "chat",
		inNav: true,
		inMenu: true,
		inRail: false,
	},
	{
		label: "通知",
		href: "/seller-center/notifications",
		icon: "notification",
		inNav: false,
		inMenu: true,
		inRail: false,
	},
	{
		label: "動態管理",
		href: "/seller-center/posts",
		icon: "edit",
		inNav: true,
		inMenu: true,
		inRail: true,
	},
	{
		label: "市場行情",
		href: "/seller-center/insights",
		icon: "chart",
		inNav: true,
		inMenu: true,
		inRail: false,
	},
	{
		label: "商店設定",
		href: "/seller-center/me",
		icon: "settings",
		inNav: false,
		inMenu: true,
		inRail: true,
	},
];

const sellerNavItems = sellerWorkspaceRoutes.filter((route) => route.inNav);
const sellerMenuItems = sellerWorkspaceRoutes.filter((route) => route.inMenu);
const sellerRailMenuItems = sellerWorkspaceRoutes.filter(
	(route) => route.inRail,
);

const SETUP_LOCK_HINT = "完成賣場建立後開放";

// 賣場建立未完成時的鎖定樣式：去掉 hover/連結性，降透明度。The element is a real
// <button aria-disabled>, not a <span>: seven workspace destinations used to
// vanish from the tab order with nothing explaining why.
const SELLER_NAV_ITEM_LOCKED = cn(
	"relative inline-flex items-center justify-center min-h-11 px-[14px] rounded-[999px]",
	"text-[0.875rem] font-bold whitespace-nowrap select-none cursor-not-allowed opacity-40",
	"text-[var(--page-seller-nav-item-fg)] bg-transparent border-0",
);

const MENU_ITEM_LOCKED = "opacity-40 cursor-not-allowed select-none";

const MENU_SETUP_CTA = cn(
	"flex h-11 w-full items-center justify-center rounded-[999px] px-4",
	"bg-[var(--page-seller-nav-menu-cta-bg)] text-[0.8125rem] font-bold no-underline",
	"text-[var(--page-seller-nav-menu-cta-fg)]!",
	"hover:bg-[var(--page-seller-nav-menu-cta-hover-bg)]",
);

const PlusIcon = () => (
	<svg
		width="18"
		height="18"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2.4"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="M12 5v14M5 12h14" />
	</svg>
);

function SellerAccountMenu({ setupIncomplete }: { setupIncomplete: boolean }) {
	const { signOut } = useAuth();
	const { avatar, ownerName, sellerId, storeName } = useSellerIdentity();
	const [open, setOpen] = React.useState(false);
	const [isSigningOut, setIsSigningOut] = React.useState(false);
	const signOutPendingRef = React.useRef(false);
	const triggerRef = React.useRef<HTMLButtonElement>(null);
	const panelRef = React.useRef<HTMLDivElement>(null);
	const lockHintId = `${React.useId()}-seller-menu-lock`;

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
			await signOut();
		} catch {
			// 撤銷失敗＝session 仍有效（signOut 已跳 toast），保留選單供使用者重試。
			return;
		} finally {
			signOutPendingRef.current = false;
			setIsSigningOut(false);
		}
		setOpen(false);
		// 整頁重載回首頁，避免部署後 soft 導航去要舊 chunk 卡骨架。
		commitDocumentNavigation("/");
	}, [signOut]);

	return (
		<div className="seller-header__user">
			<button
				type="button"
				ref={triggerRef}
				className={SELLER_AVATAR}
				{...menu.triggerProps}
				aria-label={`${storeName} 的選單`}
				disabled={isSigningOut}
				onClick={() => setOpen((v) => !v)}
			>
				<SellerAvatarImage
					src={avatar}
					alt={ownerName || storeName}
					variant="identity"
				/>
			</button>

			{open && (
				<div
					ref={panelRef}
					className={cn(SITE_NAV_USER_MENU, "seller-header__menu")}
					{...menu.panelProps}
				>
					<div className={SITE_NAV_USER_MENU_HEADER}>
						<strong>{storeName}</strong>
						{setupIncomplete ? (
							<span>賣場建立尚未完成</span>
						) : sellerId ? (
							<span>賣家 ID：{sellerId}</span>
						) : ownerName ? (
							<span>{ownerName}</span>
						) : null}
					</div>
					{setupIncomplete && (
						<div className="px-4 pt-3 pb-1">
							<p className="mb-2.5 text-[0.8125rem] leading-relaxed text-hk-muted">
								建立賣場並驗證手機後，即可使用賣家中心功能。
							</p>
							<Link
								href="/seller/onboarding"
								{...menu.menuItemProps}
								aria-disabled={isSigningOut || undefined}
								className={cn(
									MENU_SETUP_CTA,
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
								繼續建立賣場
							</Link>
						</div>
					)}
					<ul className={SITE_NAV_USER_MENU_LIST}>
						{sellerMenuItems.map((item) =>
							setupIncomplete ? (
								<li key={item.href}>
									{/* Focusable on purpose: a disabled destination the
									    keyboard cannot reach is a destination the user is
									    never told about. */}
									<button
										type="button"
										{...menu.menuItemProps}
										aria-disabled="true"
										aria-describedby={lockHintId}
										className={cn(
											SITE_NAV_USER_MENU_ITEM,
											"w-full bg-transparent border-0 text-left",
											MENU_ITEM_LOCKED,
										)}
										onClick={(event) => event.preventDefault()}
									>
										<AppIcon name={item.icon} size={18} />
										{item.label}
									</button>
								</li>
							) : (
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
										<AppIcon name={item.icon} size={18} />
										{item.label}
									</Link>
								</li>
							),
						)}
					</ul>
					{setupIncomplete && (
						<span id={lockHintId} className="sr-only">
							{SETUP_LOCK_HINT}
						</span>
					)}
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

export function SellerHeader() {
	const pathname = usePathname() ?? "";
	const navLockHintId = `${React.useId()}-seller-nav-lock`;
	// 賣場建立未完成：工作區連結鎖定（dim、不可點），CTA 變成「繼續建立賣場」
	// 把人導去 /seller/onboarding —— 引導優先於死按鈕。
	const { hasSeller } = useAuth();
	const { setupIncomplete: profileSetupIncomplete } = useSellerIdentity();
	// buyer -> seller 升級在綁定完成前本來就沒有 seller-self 可查；不能把
	// `self=null` 當成已完成，否則 onboarding header 會洩出可點的工作區入口。
	const setupIncomplete = !hasSeller || profileSetupIncomplete;

	const isActive = (href: string) =>
		pathname === href || pathname.startsWith(`${href}/`);

	return (
		<nav
			className={SELLER_HEADER}
			aria-label="賣家工作區導覽"
			data-onboarding={isStandaloneFlowPath(pathname) || undefined}
		>
			<div className={SELLER_HEADER_INNER}>
				<Link
					href={
						setupIncomplete ? "/seller/onboarding" : "/seller-center/dashboard"
					}
					className={SELLER_BRAND}
					aria-label={setupIncomplete ? "前往建立賣場" : "回到賣家總覽"}
				>
					<BrandLogo className="seller-header__brand-logo" />
				</Link>

				<div className={SELLER_NAV}>
					{sellerNavItems.map((item) =>
						setupIncomplete ? (
							<button
								key={item.href}
								type="button"
								className={SELLER_NAV_ITEM_LOCKED}
								aria-disabled="true"
								aria-describedby={navLockHintId}
								onClick={(event) => event.preventDefault()}
							>
								{item.label}
							</button>
						) : (
							<Link
								key={item.href}
								href={item.href}
								className={cn(
									SELLER_NAV_ITEM,
									isActive(item.href)
										? `is-active ${SELLER_NAV_ITEM_ACTIVE}`
										: SELLER_NAV_ITEM_DEFAULT,
								)}
							>
								{item.label}
							</Link>
						),
					)}
					{setupIncomplete && (
						<span id={navLockHintId} className="sr-only">
							{SETUP_LOCK_HINT}
						</span>
					)}
				</div>

				<div className={SELLER_ACTIONS}>
					{setupIncomplete ? (
						<Link
							href="/seller/onboarding"
							className={SELLER_SETUP_CTA}
							aria-label="繼續建立賣場"
						>
							<AppIcon
								name="edit"
								size={19}
								className="hidden max-[840px]:block"
								aria-hidden="true"
							/>
							{/* Not aria-hidden: the visible label must be part of the
							    accessible name, and the icon-only ≤840px variant is what
							    the aria-label above exists for. */}
							<span className="max-[840px]:hidden">繼續建立賣場 →</span>
						</Link>
					) : (
						<Link
							href="/seller-center/products/new"
							className={SELLER_CREATE_CTA}
						>
							<PlusIcon /> 上架新商品
						</Link>
					)}
					<ThemeToggle className={SELLER_THEME_BTN} />
					{!setupIncomplete && (
						<>
							<ChatBubble
								href="/seller-center/messages"
								className={SELLER_ICON_BTN}
								aria-label="訊息"
							>
								<span className={SELLER_ICON_BOX} aria-hidden="true">
									<AppIcon name="message" size={24} />
								</span>
							</ChatBubble>
							<NotificationBell
								href="/seller-center/notifications"
								className={SELLER_ICON_BTN}
								aria-label="通知"
							>
								<span className={SELLER_ICON_BOX} aria-hidden="true">
									<AppIcon name="notification" size={24} />
								</span>
							</NotificationBell>
						</>
					)}
					<SellerAccountMenu setupIncomplete={setupIncomplete} />
				</div>
			</div>
		</nav>
	);
}

/**
 * Seller-side dropdown that replaces the buyer's UserMenu when a seller
 * session is the active role. SellerWorkspaceLock confines sellers to
 * /seller-center/*, so this menu mostly exists for the brief frame
 * before the lock fires (and as a manual escape if the lock fails or
 * the seller paste-navigates to a buyer URL).
 */
export function SellerMenu({
	onSignOut,
	chrome,
}: {
	onSignOut: SignOutAction;
	chrome: SellerMenuChrome;
}) {
	const { avatar, ownerName, sellerId, storeName } = useSellerIdentity();
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

	return (
		<div className={chrome.root}>
			<button
				type="button"
				ref={triggerRef}
				className={chrome.trigger}
				{...menu.triggerProps}
				aria-label={`${storeName} 的選單`}
				disabled={isSigningOut}
				onClick={() => setOpen((value) => !value)}
			>
				<SellerAvatarImage
					src={avatar}
					alt=""
					className={chrome.avatar}
					variant="identity"
				/>
				<span className={chrome.name}>{storeName}</span>
				<svg
					className={cn(chrome.chevron, open && "[transform:rotate(180deg)]")}
					viewBox="0 0 24 24"
					aria-hidden="true"
				>
					<path d="m6 9 6 6 6-6" />
				</svg>
			</button>

			{open && (
				<div ref={panelRef} className={SITE_NAV_USER_MENU} {...menu.panelProps}>
					<div className={SITE_NAV_USER_MENU_HEADER}>
						<strong>{storeName}</strong>
						{sellerId ? (
							<span>賣家 ID：{sellerId}</span>
						) : ownerName ? (
							<span>{ownerName}</span>
						) : null}
					</div>
					<ul className={SITE_NAV_USER_MENU_LIST}>
						{sellerRailMenuItems.map((item) => (
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
									<AppIcon name={item.icon} size={18} />
									{item.label}
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
