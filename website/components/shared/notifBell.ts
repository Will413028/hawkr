/**
 * Shared skin for the notif-bell primitive (icon + unread badge), rendered by
 * NotificationBell and ChatBubble. Single source of truth so the two renderings
 * can't drift.
 *
 * The `notif-bell__inner` / `notif-bell__badge` class names are retained as
 * hooks: host-context overrides still live in CSS (`navigation-final.css` for
 * `.site-nav-action`, `navigation-shell.css` for `.seller-header__icon-btn`)
 * and target these selectors. Those host layers migrate in later knives.
 */

export const NOTIF_BELL_INNER =
	"notif-bell__inner relative inline-flex items-center gap-2";

export const NOTIF_BELL_BADGE =
	"notif-bell__badge absolute top-[-8px] right-[-10px] min-w-[18px] h-[18px] " +
	"py-0 px-[5px] rounded-[var(--radius-pill)] bg-[var(--hkui-danger)] " +
	"text-[var(--hkui-danger-foreground)] text-xs font-bold leading-[18px] " +
	"text-center border-2 border-solid border-[var(--color-white)] " +
	"pointer-events-none";
