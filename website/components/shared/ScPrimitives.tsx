"use client";

import * as React from "react";
import { useDragScroll } from "@/lib/hooks/useDragScroll";
import { cn } from "@/lib/utils/cn";

// ─── ScFilters ────────────────────────────────────────────────────────────────

interface ScFiltersProps {
	className?: string;
	children: React.ReactNode;
	"aria-label"?: string;
}

/**
 * Horizontal pill filter bar — wraps filter `<ScPill>` tabs.
 * Replaces the global `.sc-filters` class.
 *
 * Single scrollable row (not wrap): when the tabs overflow (e.g. the 6
 * order-status tabs on a phone) the bar scrolls horizontally instead of
 * stacking onto a second line, matching the app's segmented-control feel.
 * `useDragScroll` adds mouse drag-to-scroll with momentum; touch keeps
 * native inertia. Pills are `flex-none` so they never get squeezed.
 */
export function ScFilters({
	className,
	children,
	"aria-label": ariaLabel,
}: ScFiltersProps) {
	const ref = React.useRef<HTMLDivElement | null>(null);
	useDragScroll(ref);

	// Keep the active tab in view without forcing the user to drag. On a
	// narrow phone the selected status (e.g. 待付款 at the far left, or
	// 已完成 at the far right) can land clipped at an edge; centre it on
	// mount and whenever the active pill's aria-selected/-current flips
	// (tab navigation reuses the same DOM nodes, so a MutationObserver on
	// those attributes is enough — no need to thread an activeKey prop
	// through every call site).
	React.useEffect(() => {
		const el = ref.current;
		if (!el) return;

		const centerActive = () => {
			const active = el.querySelector<HTMLElement>(
				'[aria-current="page"],[aria-selected="true"],[aria-pressed="true"]',
			);
			if (!active) return;
			const elRect = el.getBoundingClientRect();
			const aRect = active.getBoundingClientRect();
			// Already fully visible — leave the scroll position alone so we
			// don't yank the rail on every unrelated re-render.
			if (aRect.left >= elRect.left && aRect.right <= elRect.right) return;
			const target =
				el.scrollLeft +
				(aRect.left - elRect.left) -
				(el.clientWidth - aRect.width) / 2;
			const max = el.scrollWidth - el.clientWidth;
			el.scrollLeft = Math.max(0, Math.min(target, max));
		};

		centerActive();
		// Re-centre on three triggers:
		//  • aria-current/-selected flips — tab navigation reuses the same DOM
		//    nodes, so observing the attributes catches a status change.
		//  • childList/characterData — the order counts load async and their
		//    badge <span>s widen the pills *after* mount. Without this the
		//    centring computed against count-less widths leaves the active tab
		//    clipped at an edge once the counts arrive — the reported "top tab
		//    can't be tapped" (a clipped pill is a tiny edge target).
		//  • ResizeObserver — rotation / responsive width changes re-clip too.
		const observer = new MutationObserver(centerActive);
		observer.observe(el, {
			subtree: true,
			attributes: true,
			attributeFilter: ["aria-current", "aria-selected", "aria-pressed"],
			childList: true,
			characterData: true,
		});
		const resize =
			typeof ResizeObserver !== "undefined"
				? new ResizeObserver(centerActive)
				: null;
		resize?.observe(el);
		return () => {
			observer.disconnect();
			resize?.disconnect();
		};
	}, []);

	return (
		// biome-ignore lint/a11y/useSemanticElements: fieldset is a FORM grouping; this rail holds links and toggle pills, never a form control — see the role note below.
		<div
			ref={ref}
			className={cn(
				"flex flex-nowrap gap-[var(--hkui-space-2)] overflow-x-auto cursor-grab select-none [scrollbar-width:none] [overscroll-behavior-x:contain] [&::-webkit-scrollbar]:hidden [&>*]:flex-none",
				// The scrollbar is hidden and the row is drag-scrolled, which left a
				// mouse user with no sign that there was more to the right at all.
				// A mask fade on the trailing edge is that sign, and it costs no
				// layout: it only appears while the row actually overflows because a
				// non-overflowing row has nothing under the faded strip.
				"[mask-image:linear-gradient(to_right,black_0,black_calc(100%-24px),transparent_100%)]",
				className,
			)}
			// NOT `role="tablist"`: these pills are links and toggle buttons, not
			// tabs — there is no tabpanel to control and arrow keys do not move
			// between them. A tablist with zero `role="tab"` children announces a
			// keyboard model that does not exist, which is worse than no role.
			role="group"
			aria-label={ariaLabel}
		>
			{children}
		</div>
	);
}

// ─── ScPill ───────────────────────────────────────────────────────────────────

interface ScPillBaseProps {
	active?: boolean;
	className?: string;
	children: React.ReactNode;
}

type ScPillProps = ScPillBaseProps &
	(
		| ({ as: "button" } & React.ButtonHTMLAttributes<HTMLButtonElement>)
		| ({ as?: "a" } & React.AnchorHTMLAttributes<HTMLAnchorElement>)
	);

const PILL_BASE =
	"inline-flex items-center gap-1.5 h-8 pointer-coarse:min-h-11 px-[14px] border rounded-[var(--radius-pill)] text-[0.8125rem] font-bold no-underline cursor-pointer transition-[background,color,border-color] duration-[160ms] ease-out motion-reduce:transition-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--hkui-focus)]";

const PILL_INACTIVE =
	"border-divider bg-content1 text-foreground-soft hover:bg-content2 hover:text-foreground";

const PILL_ACTIVE =
	"border-transparent bg-hkui-primary text-hk-cream hover:bg-hkui-primary hover:text-hk-cream";

/**
 * A single filter/tab pill.
 * Renders as `<a>` by default, pass `as="button"` for clickable tabs.
 * Replaces `.sc-pill` / `.sc-pill--active`.
 *
 * Note: `PILL_BASE` deliberately omits text/bg/border colors. Active and
 * inactive states each declare their own complete colour set, so Tailwind's
 * arbitrary-value utility ordering can't bleed a muted text colour onto
 * the dark active pill.
 */
export function ScPill({ active, className, children, ...rest }: ScPillProps) {
	const cls = cn(PILL_BASE, active ? PILL_ACTIVE : PILL_INACTIVE, className);

	if ((rest as { as?: string }).as === "button") {
		const { as: _as, ...buttonProps } = rest as {
			as: "button";
		} & React.ButtonHTMLAttributes<HTMLButtonElement>;
		return (
			// A toggle pill: pressed state, not a selected tab.
			<button
				type="button"
				aria-pressed={active}
				{...buttonProps}
				className={cls}
			>
				{children}
			</button>
		);
	}

	const { as: _as, ...anchorProps } = rest as {
		as?: "a";
	} & React.AnchorHTMLAttributes<HTMLAnchorElement>;
	return (
		// A navigating pill: the active one IS the current page.
		<a
			aria-current={active ? "page" : undefined}
			{...anchorProps}
			className={cls}
		>
			{children}
		</a>
	);
}

// ─── ScPillCount ──────────────────────────────────────────────────────────────

interface ScPillCountProps {
	active?: boolean;
	children: React.ReactNode;
}

/**
 * Count badge inside a `<ScPill>`.
 * Replaces `.sc-pill__count`.
 */
export function ScPillCount({ active, children }: ScPillCountProps) {
	return (
		<span
			className={cn(
				"inline-flex items-center justify-center min-w-[18px] h-[18px] px-1.5 rounded-[var(--radius-pill)] text-xs font-extrabold tabular-nums leading-none",
				active
					? "bg-white/22 text-hk-cream"
					: "bg-default-200 text-foreground-soft",
			)}
		>
			{children}
		</span>
	);
}

// ─── ScEmpty ──────────────────────────────────────────────────────────────────

interface ScEmptyProps {
	className?: string;
	children: React.ReactNode;
}

/**
 * Full-width empty-state container — centers icon, title, description, CTA.
 * Replaces `.sc-empty`.
 */
export function ScEmpty({ className, children }: ScEmptyProps) {
	return (
		<div
			className={[
				"flex flex-col items-center justify-center gap-[var(--hkui-space-3)] py-[var(--hkui-space-8)] px-[var(--hkui-space-5)] text-center text-[var(--hkui-foreground-soft)]",
				className,
			]
				.filter(Boolean)
				.join(" ")}
		>
			{children}
		</div>
	);
}

// ─── ScEmptyTitle ─────────────────────────────────────────────────────────────

interface ScEmptyTitleProps {
	children: React.ReactNode;
}

/**
 * Bold empty-state heading.
 * Replaces `.sc-empty__title`.
 */
export function ScEmptyTitle({ children }: ScEmptyTitleProps) {
	return (
		<p className="text-[length:var(--text-base)] font-extrabold text-[var(--hkui-foreground)] m-0">
			{children}
		</p>
	);
}

// ─── ScEmptyDesc ──────────────────────────────────────────────────────────────

interface ScEmptyDescProps {
	children: React.ReactNode;
}

/**
 * Supporting copy in an empty state.
 * Replaces `.sc-empty__desc`.
 */
export function ScEmptyDesc({ children }: ScEmptyDescProps) {
	return (
		<p className="max-w-[32ch] text-[0.8125rem] leading-[1.6] m-0">
			{children}
		</p>
	);
}

// ─── ScSection ────────────────────────────────────────────────────────────────

interface ScSectionProps {
	className?: string;
	children: React.ReactNode;
}

/**
 * Vertical-stack section within a `<SellerPageShell>`.
 * Replaces `.sc-page__section`.
 */
export function ScSection({ className, children }: ScSectionProps) {
	return (
		<section
			className={["flex flex-col gap-[var(--hkui-space-3)]", className]
				.filter(Boolean)
				.join(" ")}
		>
			{children}
		</section>
	);
}
