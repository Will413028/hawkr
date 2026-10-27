"use client";

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
import { BUYER_MENU_ITEMS as buyerMenuItems } from "@/features/account/menu";
import HomeCompactSearch from "@/features/home/components/HomeCompactSearch";
import { resolveAuthRailMode } from "@/lib/auth/auth-rail";
import { useAuth } from "@/lib/auth/context";
import { loginPath } from "@/lib/auth/redirect";
import { getActiveRole, type RoleSlot } from "@/lib/auth/storage";
import { navItems } from "@/lib/content/data";
import { usePathname } from "@/lib/intl/navigation";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
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

/*
 * Buyer-navbar chrome — Tailwind migration (子刀 4, PR-B).
 *
 * PR-A consolidated the buyer navbar into a single CSS source spread across
 * navigation-shell/final/floating (all `body .site-nav*` at 0,1,1). PR-B moves
 * the HOST element styles here as inline utilities and deletes the matching CSS
 * declarations so the unlayered rules no longer shadow these utilities.
 *
 * The original BEM class on each element is KEPT as a hook — the computed-style
 * gate (tests/visual/buyer-navbar.spec.ts) selects on `.site-nav*`/`.nav-item`,
 * the nav-overlap regression test reads `.site-nav__inner`, and a few retained
 * rules still key off them: `.auto-nav .site-nav{position}` (子刀 1 wrapper),
 * the `.site-nav-action .notif-bell__*` descendant overrides (子刀 5),
 * `.site-nav .brand-logo` lockup sizing, and the icon/label descendant + sr-only
 * machinery. Descendant rules are intentionally NOT migrated in PR-B.
 *
 * Dark text uses `text-[var(--ink-body)]!`: --ink-body resolves to the body
 * text token (rgb(66,73,63) light / --color-ink-100 dark), and the `!`
 * is required to beat the unlayered dark-link exception
 * (`[data-theme=dark] body a.nav-item{color:inherit}` at 0,2,2) — see
 * styles/foundation/01-tokens-theme.css. Light needs no `!` (utilities beat the
 * `@layer base a` default) but `!` is harmless there. Surface/accent colours use
 * theme-aware vars so they flip with [data-theme=dark].
 *
 * Breakpoints mirror the source CSS exactly (so the gate stays byte-identical):
 * the icon-collapse is `@media (max-width:1280px)` → `[@media(max-width:1280px)]:`, the
 * center/login hide + right rail shift is `@media (max-width:840px)` →
 * `[@media(max-width:840px)]:`, and the floating-pill→flat-bar swap is
 * `@media (max-width:768px)` → `[@media(max-width:768px)]:`.
 */
const SITE_NAV =
	"site-nav relative flex items-center justify-center h-[84px] px-6 [@media(max-width:768px)]:h-[56px] [@media(max-width:768px)]:px-0";

const SITE_NAV_INNER = cn(
	"site-nav__inner flex w-full items-center justify-between max-w-[1440px]",
	"h-[84px] min-h-[84px] p-[8px_36px_8px_24px] rounded-[9999px]",
	"bg-[var(--page-nav-pill-bg)]",
	// box-shadow via an arbitrary *property* (not the shadow-* utility): Tailwind's
	// shadow utility prepends 4 transparent ring placeholder layers, changing the
	// computed string the gate diffs byte-for-byte.
	"[box-shadow:var(--page-nav-pill-shadow)]",
	// No backdrop-filter: the pill's computed backdrop-filter has always been
	// `none`. floating-navigation.css declared blur(20px) saturate(140%) on
	// .site-nav__inner, but navigation-final.css's now-removed inner rule set
	// `backdrop-filter: none` and won the cascade, so the blur never took effect.
	// PR-B preserves that observed value (re-adding blur would be a behaviour
	// change, not a faithful migration).
	"transition-[box-shadow,transform]",
	// Dark floating pill: ink-tinted gradient over a near-black base. The gate
	// reads background-color (the solid layer); the gradient is background-image.
	// Light/dark now live on the tokens (00-color-config.css), so the pill has
	// ONE box-shadow declaration per breakpoint instead of four competing ones.
	"dark:[background-image:var(--page-nav-pill-gradient-dark)]",
	// Mobile: flat top bar, square corners, single hairline shadow (dark keeps the
	// 2-layer gold ring — its non-media dark rule must still win at this width).
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
// `--theme-text-accent` + `--theme-primary-subtle` are the theme-aware
// "selected label" tokens (light = primary on ink-tint, dark = gold-cream
// on gold-tint). The old `--color-brand-primary-strong` never flipped, so the active
// tab rendered --color-brand-primary-strong primary text on the --color-ink-950 dark page — near-invisible.
const NAV_ITEM_ACTIVE = "text-[var(--theme-text-accent)]!";

const SITE_NAV_ACTION = cn(
	"site-nav-action static inline-flex items-center justify-center gap-x-2 min-h-11 px-[13px] rounded-[999px]",
	"text-[0.9375rem] font-bold leading-none no-underline whitespace-nowrap transition-[background-color,color,transform]",
	// ≤1280: collapse to a 44px icon-only pill (the label spans are sr-only'd by
	// the retained descendant rules at the same breakpoint).
	"[@media(max-width:1280px)]:w-11 [@media(max-width:1280px)]:min-w-11 [@media(max-width:1280px)]:gap-x-0 [@media(max-width:1280px)]:p-0",
	// `[ transform:… ]`, not `-translate-y-px`: Tailwind v4's translate utilities
	// drive the standalone `translate` property, but the gate (and the old CSS)
	// read the `transform` matrix.
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
	// Dark login hover keeps body text (the floating dark-login colour rule at
	// 0,2,2 beat the shared hover green), only the surface lifts.
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
// rounded-[50%], not rounded-full: v4's rounded-full computes to a giant px
// value (calc(infinity*1px)); the gate pins the literal "50%" the old CSS used.
const SITE_NAV_USER_AVATAR =
	"site-nav-user__avatar w-8 h-8 rounded-[50%] object-cover bg-[var(--page-nav-user-avatar-bg)]";
const SITE_NAV_USER_NAME = cn(
	"site-nav-user__name block max-w-[96px] overflow-hidden text-ellipsis whitespace-nowrap",
	"text-[0.875rem] font-medium text-[var(--page-nav-user-name-fg)]!",
	// ≤1440：登入後的 rail（主題・字級・App・購物車・訊息・通知・頭像）在 1281–1440
	// 之間剛好塞不下，先收起名字只留頭像；≤1280 整條 rail 都進 icon mode。
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
	// A sign-out in flight owns the panel: it is showing the pending state, so
	// neither Escape nor a stray outside click may tear it down.
	const isLocked = React.useCallback(() => signOutPendingRef.current, []);
	// Outside-click, Escape + focus return, roving Arrow/Home/End and the
	// aria-controls wiring all live in the shared hook — this dropdown, the
	// seller account menu and the seller rail menu used to carry three copies
	// of the effect and none of the keyboard behaviour.
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

	// TODO(B-189): profile endpoint — buyerName not on Identity; use nickname fallback.
	const displayName = identity?.nickname || "會員";
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
						{/* TODO(B-189): profile endpoint — phone not on Identity */}
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

interface NavbarProps {
	/** SSR-prefetched dynamic megamenu data (categories + hashtags). */
	megaMenuDynamicLinks?: MegaMenuDynamicLinks;
}

export default function Navbar({ megaMenuDynamicLinks }: NavbarProps = {}) {
	const pathname = usePathname() ?? "";
	const router = useObservedRouter("navbar");
	const { isLoggedIn, activeRole, hydrated, sessionUnavailable, signOut } =
		useAuth();
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => {
		setMounted(true);
	}, []);
	const authReady = mounted && hydrated;
	// Single decision point for the right-rail auth affordance. Pre-hydration it
	// renders the clickable 登入 link rather than a skeleton — the client-only
	// `bootRole` below must never gate it, which was the "login button does
	// nothing" race.
	const railMode = resolveAuthRailMode({
		authReady,
		isLoggedIn,
		activeRole,
		sessionUnavailable,
	});
	// Pre-hydration role HINT from localStorage (set on login/role switch), used
	// ONLY to shape the skeleton (buyer shows 訊息+通知 placeholders, others just
	// the avatar). Read in an effect so SSR + first client render agree (both
	// null → neutral avatar-only skeleton), then refined within a frame.
	const [bootRole, setBootRole] = React.useState<RoleSlot | null>(null);
	React.useEffect(() => {
		setBootRole(getActiveRole());
	}, []);
	const isHome = pathname === "/";
	// Guests keep the homepage shortcut in the navbar at every viewport. Signed-in
	// buyers keep it here on desktop, while mobile hides this copy and uses the
	// full-width row between the navbar and the home category bar instead.
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
			// 撤銷失敗＝session 仍有效（signOut 已跳 toast）。不能導航離開：導走會
			// 讓畫面看起來像登出了，實際上 cookie 還在，下次冷載入又是登入狀態。
			return false;
		}
		// 整頁重載回首頁，避免 session 期間上了新部署後 soft 導航去要舊 chunk 卡骨架。
		commitDocumentNavigation("/");
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
					{/*
					 * Header rail per the 2026-05 buyer-spec rewrite (revised
					 * 2026-05 three-page split PR2):
					 *   center nav — Hawkr 動態 · Hawkr 市集 · 搜尋
					 *     (關於我們 / 招商合作 moved to footer only, 2026-06)
					 *   right rail — 主題切換 · App 下載 · 購物車 · 登入
					 *   logged-in buyers also see 訊息 + 通知
					 *   removed — 收藏 (still reachable via user menu)
					 */}
					<ThemeToggle className="size-11" />
					{/* 字體大小 sits beside 主題切換, but only once we know who is
					    looking: the guest rail is already the widest thing on a
					    small laptop, and the setting is a returning reader's
					    preference. Signed-out visitors reach it from the footer. */}
					{/* `authReady &&` is the same hydration guard the rest of this rail
					    uses: `isLoggedIn` is false until the session resolves, so
					    gating on it bare is safe but reading it through authReady
					    keeps every auth-dependent branch in this file on one rule. */}
					{/* Hidden on phones: six 44px actions plus the brand do not fit a
					    393px viewport, and the nav clips its own overflow, so the
					    rightmost control (the account menu) becomes unreachable with no
					    scroll to recover it. Text scale stays one tap away in the footer
					    and in the settings page 顯示 section. */}
					{authReady && isLoggedIn && (
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
						<span>購物車</span>
					</CartBubble>

					{/* Auth-dependent rail. Only a request that carried a session
					    cookie (railMode "skeleton") holds these layout-stable
					    placeholders while GetSession resolves — a returner used
					    to see the guest rail paint first and then shift the whole
					    row when 訊息/通知/頭像 popped in. A no-session visitor skips
					    this entirely and gets the clickable 登入 link below. */}
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
								<span>訊息</span>
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
								<span>通知</span>
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
						// Logged in as seller. Show a primary "賣家中心" CTA
						// (sellers' main reason to be in the navbar) plus the
						// SellerMenu dropdown for everything else.
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
