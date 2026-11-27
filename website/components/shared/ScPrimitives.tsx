"use client";

import * as React from "react";
import { useDragScroll } from "@/lib/hooks/useDragScroll";
import { cn } from "@/lib/utils/cn";


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
			if (aRect.left >= elRect.left && aRect.right <= elRect.right) return;
			const target =
				el.scrollLeft +
				(aRect.left - elRect.left) -
				(el.clientWidth - aRect.width) / 2;
			const max = el.scrollWidth - el.clientWidth;
			el.scrollLeft = Math.max(0, Math.min(target, max));
		};

		centerActive();
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
				"[mask-image:linear-gradient(to_right,black_0,black_calc(100%-24px),transparent_100%)]",
				className,
			)}
			role="group"
			aria-label={ariaLabel}
		>
			{children}
		</div>
	);
}


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
		<a
			aria-current={active ? "page" : undefined}
			{...anchorProps}
			className={cls}
		>
			{children}
		</a>
	);
}


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
