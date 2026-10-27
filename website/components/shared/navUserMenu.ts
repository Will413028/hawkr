import { cn } from "@/lib/utils/cn";

/**
 * Shared user-menu dropdown classes (步驟 6). The `.site-nav-user__menu*` panel
 * is rendered by THREE places — Navbar.tsx's BuyerMenu + SellerMenu and
 * RoleAwareNavbar.tsx's SellerAccountMenu — so its Tailwind lives here as one
 * source to avoid drift (the notifBell.ts pattern).
 *
 * BEM classes are kept as hooks: the computed-style gate selects on them, and
 * the leaf rules still in navigation-shell.css (`.site-nav-user__menu-header
 * strong`/`span`, `.site-nav-user__menu-icon`, the `[aria-hidden]` icon spans)
 * key off them — those ungated leaves are intentionally NOT migrated.
 *
 * Dark colours use `!`: menu items are `<a role="menuitem">`, which is in the
 * unlayered dark-link `color:inherit` exception in 01-tokens-theme.css, so a
 * plain utility loses. The panel keeps a real `backdrop-filter: blur(20px)`
 * (unlike the pills, whose computed backdrop is `none` from a cascade override)
 * — emitted as an arbitrary property so the computed string matches.
 */
export const SITE_NAV_USER_MENU = cn(
	"site-nav-user__menu absolute top-[calc(100%+8px)] right-0 z-[60] min-w-[260px] p-2 rounded-[24px]",
	"bg-[rgba(255,255,255,0.92)] dark:bg-hk-ink-900",
	"[backdrop-filter:blur(20px)] [-webkit-backdrop-filter:blur(20px)]",
	"[box-shadow:0_16px_48px_rgb(var(--color-ink-900-rgb)/0.14),0_4px_16px_rgba(0,0,0,0.08)]",
	"text-[var(--ink-body)] dark:text-[var(--color-ink-100)]",
);

export const SITE_NAV_USER_MENU_HEADER =
	"site-nav-user__menu-header flex flex-col gap-y-0.5 p-[12px_16px_14px] border-b border-b-[rgba(204,204,204,0.5)]";

export const SITE_NAV_USER_MENU_LIST =
	"site-nav-user__menu-list flex flex-col my-2 p-0 list-none";

export const SITE_NAV_USER_MENU_ITEM = cn(
	"flex items-center gap-x-3 px-[14px] py-[10px] rounded-[12px] no-underline text-[0.875rem]",
	"text-[rgb(85,85,85)]! dark:text-[var(--color-ink-100)]! transition-[background,color]",
	"hover:bg-[rgb(var(--color-brand-primary-strong-rgb)/0.08)] hover:text-[color:var(--color-brand-primary-strong)]!",
	"dark:hover:bg-[rgba(245,222,165,0.08)] dark:hover:text-[var(--color-ink-100)]!",
);

export const SITE_NAV_USER_MENU_FOOTER =
	"site-nav-user__menu-footer flex flex-col gap-y-0.5 px-2 pt-[6px] pb-[4px] border-t border-t-[rgba(204,204,204,0.5)]";

export const SITE_NAV_USER_MENU_FOOTER_BTN = cn(
	"w-full px-[14px] py-[10px] rounded-[12px] bg-transparent text-left text-[0.875rem] cursor-pointer border-0",
	"text-[#ee964b] dark:text-[var(--color-ink-100)] transition-[background]",
	"hover:bg-[rgba(238,150,75,0.1)] dark:hover:bg-[rgba(245,222,165,0.08)]",
);
