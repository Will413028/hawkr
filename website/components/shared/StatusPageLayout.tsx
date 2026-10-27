import type * as React from "react";
import { cn } from "@/lib/utils/cn";

export interface StatusPageLayoutProps
	extends Omit<React.HTMLAttributes<HTMLElement>, "title"> {
	/** Icon or illustration shown inside the cream porthole disc. */
	media: React.ReactNode;
	/** Extra padding for the disc; the artwork's own bleed decides it. */
	discClassName?: string;
	eyebrow: React.ReactNode;
	title: React.ReactNode;
	description: React.ReactNode;
	/** Pill button row. */
	actions: React.ReactNode;
	/** Optional line under the actions (e.g. the error digest). */
	footnote?: React.ReactNode;
	/**
	 * Announce the panel when it replaces page content. Error boundaries set
	 * it: swapping a page for an error view on a soft navigation is otherwise
	 * completely silent to assistive tech. It lands on the inner section so
	 * the `main` landmark is not overwritten.
	 */
	announce?: boolean;
}

/**
 * Shared chrome for the app's terminal pages (crashed segment, 404). The two
 * were near line-for-line duplicates — same disc, eyebrow, clamped h1, copy
 * and pill row — differing only in artwork, copy and button count.
 *
 * The caller keeps ownership of `data-hk-route-terminal-ready`: it is passed
 * through the rest props onto the `main` element, so each terminal page still
 * declares its own marker value.
 */
export default function StatusPageLayout({
	media,
	discClassName,
	eyebrow,
	title,
	description,
	actions,
	footnote,
	announce,
	className,
	...rest
}: StatusPageLayoutProps) {
	return (
		<main
			className={cn(
				"grid min-h-[72vh] place-items-center px-6 py-16 text-hk-body",
				className,
			)}
			{...rest}
		>
			<section
				className="flex w-full max-w-[460px] flex-col items-center text-center"
				role={announce ? "alert" : undefined}
			>
				{/* Cream disc gives the themed artwork a consistent porthole ground
				    in both light and dark mode — same ground as the splash screen. */}
				<div
					className={cn(
						"hk-status-icon-disc grid size-[216px] place-items-center rounded-full max-[480px]:size-[176px]",
						discClassName ?? "p-4",
					)}
					aria-hidden="true"
				>
					{media}
				</div>
				<span className="mt-2 text-xs font-semibold tracking-[0.12em] text-hk-primary uppercase">
					{eyebrow}
				</span>
				<h1 className="m-0 mt-2 text-[clamp(1.375rem,4vw,1.75rem)] leading-[1.25] font-bold text-hk-heading">
					{title}
				</h1>
				<p className="m-0 mt-3 text-sm leading-[1.7] text-hk-muted">
					{description}
				</p>
				<div className="mt-7 flex flex-wrap items-center justify-center gap-3">
					{actions}
				</div>
				{footnote}
			</section>
		</main>
	);
}

/** Primary pill shared by both terminal pages. */
export const STATUS_PAGE_PRIMARY_ACTION_CLASS =
	"inline-flex min-h-11 cursor-pointer items-center justify-center rounded-hk-pill border border-hk-primary bg-hk-primary px-6 text-sm font-semibold text-hk-inverse no-underline transition-colors hover:border-hk-primary-hover hover:bg-hk-primary-hover";

/**
 * Secondary "escape hatch" pill shared by both terminal pages. The dark
 * colours are `--page-status-link-*`; three verbatim copies of those two
 * literals used to sit inline here and in the login modal footer.
 */
export const STATUS_PAGE_GHOST_ACTION_CLASS =
	"inline-flex min-h-11 items-center justify-center rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-6 text-sm font-semibold text-hk-body no-underline transition-colors hover:text-hk-heading dark:bg-transparent dark:text-[var(--page-status-link-fg)] dark:hover:text-[var(--page-status-link-hover-fg)]";
