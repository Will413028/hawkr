"use client";

import * as React from "react";
import { useTheme } from "@/lib/theme/context";
import { cn } from "@/lib/utils/cn";

interface ThemeToggleProps {
	className?: string;
	verbose?: boolean;
}

/**
 * The pill chrome shared by every personalisation toggle in the navbar / footer
 * rail (主題切換 and 字體大小). It lived inline here until `TextScaleToggle`
 * needed to sit next to this button and look like the same control; exporting
 * it is what stops the two drifting apart on the next tweak.
 *
 * Behaviour-neutral: it is only geometry, surface, motion and focus. Each
 * toggle still owns its own semantics (`aria-pressed` here, a live region on
 * the 3-state cycler).
 */
export const THEME_TOGGLE_CHROME_CLASS = cn(
	// size / shape / flex — 44px touch target
	"size-11 rounded-full inline-flex items-center justify-center shrink-0 cursor-pointer",
	// frosted-glass surface; the tokens hold the previous literals
	// (light rgba(255,255,255,.55)/.70/.78, dark the amber glass set)
	"bg-[var(--page-theme-toggle-bg)] border border-[var(--page-theme-toggle-border)]",
	"text-[var(--page-theme-toggle-fg)]",
	// multi-duration transition (transform 120ms, rest 200ms)
	"[transition:background_200ms_ease,color_200ms_ease,transform_120ms_ease,border-color_200ms_ease]",
	"motion-reduce:transition-none",
	// hover
	"hover:bg-[var(--page-theme-toggle-hover-bg)] hover:-rotate-[12deg]",
	// active
	"active:scale-95 motion-reduce:scale-100 motion-reduce:rotate-0",
	// reduced motion keeps the colour feedback, drops the movement
	"motion-reduce:hover:rotate-0 motion-reduce:active:scale-100",
	// focus-visible ring
	"focus-visible:outline-2 focus-visible:outline-[var(--color-brand-primary-strong)] focus-visible:outline-offset-2",
	"dark:hover:text-[var(--theme-text-accent)]",
	// verbose variant (data-[verbose=true]: matches data-[selected=true]: pattern in codebase)
	"data-[verbose=true]:w-auto data-[verbose=true]:py-[6px] data-[verbose=true]:pr-[14px] data-[verbose=true]:pl-[10px] data-[verbose=true]:gap-[6px]",
);

export default function ThemeToggle({
	className = "",
	verbose = false,
}: ThemeToggleProps) {
	const { theme, toggle } = useTheme();
	// The navbar renders inside a <Suspense> under ThemeProvider, whose effect
	// syncs `theme` from the boot-painted `data-theme` before this (suspended)
	// subtree hydrates. Gating on a local mounted flag keeps the first client
	// render identical to SSR (light defaults), so hydration never mismatches;
	// the real theme is applied on the post-mount re-render.
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => setMounted(true), []);
	const isDark = mounted && theme === "dark";
	const actionLabel = isDark ? "切換成日間模式" : "切換成夜間模式";

	return (
		<button
			type="button"
			data-verbose={verbose ? "true" : undefined}
			className={cn(THEME_TOGGLE_CHROME_CLASS, className)}
			// APG "toggle button": a state-neutral name plus aria-pressed, so the
			// name does not contradict the state the same node announces. The
			// verbose variant already renders a visible action label, which then
			// IS the accessible name (WCAG 2.5.3 label-in-name).
			aria-label={verbose ? undefined : "主題切換"}
			aria-pressed={isDark}
			onClick={toggle}
		>
			<span className="inline-flex w-[18px] h-[18px]" aria-hidden="true">
				{isDark ? <SunIcon /> : <MoonIcon />}
			</span>
			{verbose && (
				<span className="text-[0.8125rem] font-semibold tracking-[0.02em]">
					{actionLabel}
				</span>
			)}
		</button>
	);
}

function SunIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<title>太陽</title>
			<circle cx="12" cy="12" r="4" />
			<path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
		</svg>
	);
}

function MoonIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<title>月亮</title>
			<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" />
		</svg>
	);
}
