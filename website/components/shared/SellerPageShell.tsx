import type * as React from "react";
import { cn } from "@/lib/utils/cn";

interface SellerPageShellProps {
	/** h1 page title (e.g. "通知", "訂單管理"). Required. */
	title: React.ReactNode;
	/** Optional uppercase eyebrow above the title (e.g. "近 7 天"). */
	eyebrow?: React.ReactNode;
	/** Optional subtitle line under the title. */
	subtitle?: React.ReactNode;
	/** Right-side actions (CTA buttons, search box, filter dropdown, etc). */
	actions?: React.ReactNode;
	/** Optional class on the action slot for route-specific responsive layout. */
	actionsClassName?: string;
	/** Extra content slotted between the header and the children body
	 *  (e.g. filter pill bar, banner, breadcrumb). Renders inline with
	 *  the canonical gap-6 spacing. */
	toolbar?: React.ReactNode;
	/** Optional extra class on the outer wrapper for one-off tweaks. */
	className?: string;
	children?: React.ReactNode;
}

/**
 * Canonical page shell for `/seller-center/*` and the workspace-like
 * surfaces in `/account/*` (notifications, messages, etc).
 *
 * Emits the `.sc-page` markup contract documented in
 * `styles/components/sc-page.css` so every page renders the same
 * title-row scale, gutter, and spacing. Pages should ideally not
 * hand-roll `<header>` + `<h1>` markup; reach for this component.
 *
 * Layout:
 *   ┌─────────────────────────────────────────────────┐
 *   │ EYEBROW (optional)                              │
 *   │ Title                            Actions ▸ ▸ ▸ │
 *   │ Subtitle (optional)                             │
 *   ├─────────────────────────────────────────────────┤
 *   │ Toolbar (optional — filter pills, banner)        │
 *   ├─────────────────────────────────────────────────┤
 *   │ children (the page body)                         │
 *   └─────────────────────────────────────────────────┘
 *
 * Spacing between header / toolbar / children is `--hkui-space-6`
 * automatically; each child section is responsible for its own
 * inner padding (usually via `.seller-card` or `.sc-page__section`).
 */
export function SellerPageShell({
	title,
	eyebrow,
	subtitle,
	actions,
	actionsClassName,
	toolbar,
	className,
	children,
}: SellerPageShellProps) {
	return (
		<div className={cn("flex flex-col gap-[var(--hkui-space-6)]", className)}>
			<header className="flex justify-between items-end gap-[var(--hkui-space-4)] flex-wrap m-0">
				<div className="flex flex-col gap-1 min-w-0">
					{/* 12px floor (was 11px). `--hkui-foreground-muted` is defined in
					    both themes in ui-kit.css, so the old inline fallback chain only
					    made the token look unreliable. */}
					{eyebrow ? (
						<small className="text-[length:var(--hkui-text-tiny)] font-bold tracking-[0.12em] uppercase text-[var(--hkui-foreground-muted)]">
							{eyebrow}
						</small>
					) : null}
					<h1 className="font-[var(--hkui-font-display)] text-[1.75rem] font-bold leading-[1.2] tracking-[var(--hkui-tracking-tight)] text-[var(--hkui-foreground)] m-0">
						{title}
					</h1>
					{subtitle ? (
						<p className="m-0 text-[0.8125rem] leading-[1.5] text-[var(--hkui-foreground-soft)]">
							{subtitle}
						</p>
					) : null}
				</div>
				{actions ? (
					<div
						className={cn(
							"seller-page-shell__actions flex items-center gap-[var(--hkui-space-3)] flex-wrap",
							actionsClassName,
						)}
					>
						{actions}
					</div>
				) : null}
			</header>
			{toolbar}
			{children}
		</div>
	);
}

export default SellerPageShell;
