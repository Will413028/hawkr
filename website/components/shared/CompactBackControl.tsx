/**
 * Shared visual contract for compact page-level back controls.
 *
 * Navigation semantics stay with each caller: product detail uses a button so
 * it can trigger the reverse zoom, while seller storefront keeps a real link.
 *
 * Height is 44px (was 36px): this is one of the most-tapped controls on the
 * two busiest detail pages, and both callers render it as a free-floating pill
 * over a cover image / gallery where nothing else constrains its box.
 */
export const COMPACT_BACK_CONTROL_CLASSNAME =
	"inline-flex items-center gap-1.5 h-11 min-w-11 justify-center pl-2.5 pr-3.5 rounded-hk-pill border border-hk-border-soft bg-hk-surface-glass text-hk-body text-sm font-semibold no-underline transition-colors motion-reduce:transition-none hover:text-hk-primary hover:bg-hk-surface-glass-hover";

/**
 * The one accessible name for this control. Icon-only callers must still label
 * the control, and a shared constant keeps every call site saying the same
 * thing (both current callers had their own copy of the string).
 */
export const COMPACT_BACK_CONTROL_LABEL = "返回";

export function CompactBackIcon() {
	return (
		<svg
			width="18"
			height="18"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<path d="m15 18-6-6 6-6" />
		</svg>
	);
}
