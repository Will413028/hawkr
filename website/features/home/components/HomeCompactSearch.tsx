"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { AssistantEntrySkeleton } from "@/features/procurement-assistant/AssistantEntryLayout";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { usePathname } from "@/lib/intl/navigation";
import { useLocalizedHref } from "@/lib/intl/use-router";
import { STALE_CHUNK_RELOAD_CLASS } from "@/lib/loading/stale-chunk-reload";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { cn } from "@/lib/utils/cn";
import type { HomeCompactSearchPanelCoords } from "./HomeCompactSearchExperience";
import {
	consumeSearchDraftAfterTechnicalReload,
	saveSearchDraftForTechnicalReload,
} from "./HomeCompactSearchRecovery";
import { HomeSearchPanelSkeleton } from "./HomeSearchSkeletons";

type SearchExperienceModule = typeof import("./HomeCompactSearchExperience");

let searchExperiencePromise: Promise<SearchExperienceModule> | null = null;

function loadSearchExperience(): Promise<SearchExperienceModule> {
	if (!searchExperiencePromise) {
		searchExperiencePromise = import("./HomeCompactSearchExperience").catch(
			(error: unknown) => {
				searchExperiencePromise = null;
				throw error;
			},
		);
	}
	return searchExperiencePromise;
}

interface HomeCompactSearchProps {
	className?: string;
	recoveryScope?: "navbar" | "member-mobile";
}

export default function HomeCompactSearch({
	className,
	recoveryScope = "navbar",
}: HomeCompactSearchProps) {
	const router = useObservedRouter("home_search");
	const searchAction = useLocalizedHref("/search");
	const pathname = usePathname() ?? "";
	const [query, setQuery] = React.useState("");
	const [committedQuery, setCommittedQuery] = React.useState("");
	const [open, setOpen] = React.useState(false);
	const [mounted, setMounted] = React.useState(false);
	const [coords, setCoords] =
		React.useState<HomeCompactSearchPanelCoords | null>(null);
	const [experience, setExperience] =
		React.useState<SearchExperienceModule | null>(null);
	const [experienceError, setExperienceError] = React.useState(false);

	const formRef = React.useRef<HTMLFormElement>(null);
	const panelRef = React.useRef<HTMLDivElement>(null);
	const inputRef = React.useRef<HTMLInputElement>(null);
	const composingRef = React.useRef(false);
	const importRequestRef = React.useRef(0);
	const aliveRef = React.useRef(true);
	const intentTimerRef = React.useRef<number | null>(null);
	const suppressNextFocusOpenRef = React.useRef(false);
	const previousPathnameRef = React.useRef(pathname);
	const latestDraftRef = React.useRef({ query, open });
	latestDraftRef.current = { query, open };

	React.useEffect(() => {
		aliveRef.current = true;
		setMounted(true);
		// The existing deploy-skew guard can refresh before an import rejects.
		// Restore only its one-shot checkpoint, without weakening global recovery.
		const currentPath = () => window.location.pathname + window.location.search;
		try {
			const restored = consumeSearchDraftAfterTechnicalReload(
				window.sessionStorage,
				currentPath(),
				document.documentElement.classList.contains(STALE_CHUNK_RELOAD_CLASS) ||
					(
						performance.getEntriesByType("navigation")[0] as
							| PerformanceNavigationTiming
							| undefined
					)?.type === "reload",
				Date.now(),
				recoveryScope,
			);
			if (restored) {
				setQuery(restored.query);
				setCommittedQuery(restored.query);
				setOpen(restored.open);
				if (restored.open) inputRef.current?.focus({ preventScroll: true });
			}
		} catch {
			/* Storage access itself can be blocked. */
		}
		const checkpoint = () => {
			try {
				saveSearchDraftForTechnicalReload(
					window.sessionStorage,
					currentPath(),
					latestDraftRef.current,
					Date.now(),
					recoveryScope,
				);
			} catch {
				/* Do not prevent the technical reload. */
			}
		};
		window.addEventListener("pagehide", checkpoint);
		return () => {
			window.removeEventListener("pagehide", checkpoint);
			aliveRef.current = false;
			importRequestRef.current += 1;
			if (intentTimerRef.current !== null) {
				window.clearTimeout(intentTimerRef.current);
				intentTimerRef.current = null;
			}
		};
	}, [recoveryScope]);

	const ensureExperience = React.useCallback(() => {
		if (intentTimerRef.current !== null) {
			window.clearTimeout(intentTimerRef.current);
			intentTimerRef.current = null;
		}
		if (experience) return;
		const requestId = ++importRequestRef.current;
		setExperienceError(false);
		void loadSearchExperience().then(
			(module) => {
				if (!aliveRef.current || importRequestRef.current !== requestId) return;
				setExperience(module);
			},
			() => {
				if (!aliveRef.current || importRequestRef.current !== requestId) return;
				setExperienceError(true);
			},
		);
	}, [experience]);

	const scheduleExperience = React.useCallback(() => {
		if (experience || intentTimerRef.current !== null) return;
		intentTimerRef.current = window.setTimeout(() => {
			intentTimerRef.current = null;
			ensureExperience();
		}, 140);
	}, [ensureExperience, experience]);

	const cancelScheduledExperience = React.useCallback(() => {
		if (intentTimerRef.current === null) return;
		window.clearTimeout(intentTimerRef.current);
		intentTimerRef.current = null;
	}, []);

	const updateCoords = React.useCallback(() => {
		const el = formRef.current;
		if (!el) return;
		const rect = el.getBoundingClientRect();
		const viewport = window.visualViewport;
		const viewportLeft = viewport?.offsetLeft ?? 0;
		const viewportTop = viewport?.offsetTop ?? 0;
		const viewportWidth = viewport?.width ?? window.innerWidth;
		const viewportHeight = viewport?.height ?? window.innerHeight;
		const gutter = 12;
		const gap = 10;
		const availableWidth = Math.max(0, viewportWidth - gutter * 2);
		const width = Math.min(
			availableWidth,
			Math.max(rect.width, Math.min(640, availableWidth)),
		);
		const left = Math.min(
			Math.max(rect.left, viewportLeft + gutter),
			Math.max(
				viewportLeft + gutter,
				viewportLeft + viewportWidth - width - gutter,
			),
		);
		const top = rect.bottom + gap;
		const maxHeight = Math.max(
			0,
			Math.min(620, viewportTop + viewportHeight - top - gutter),
		);
		setCoords({ top, left, width, maxHeight });
	}, []);

	const openSearch = React.useCallback(() => {
		updateCoords();
		setOpen(true);
		ensureExperience();
	}, [ensureExperience, updateCoords]);

	const dismiss = React.useCallback((options?: { blurInput?: boolean }) => {
		setOpen(false);
		if (options?.blurInput) inputRef.current?.blur();
	}, []);

	const restoreInputFocus = React.useCallback(() => {
		const input = inputRef.current;
		if (!input || document.activeElement === input) return;
		suppressNextFocusOpenRef.current = true;
		input.focus({ preventScroll: true });
	}, []);

	React.useEffect(() => {
		if (!open) return;
		updateCoords();
		ensureExperience();
		const onScroll = (event: Event) => {
			const target = event.target as Node | null;
			if (target && panelRef.current?.contains(target)) return;
			updateCoords();
		};
		const viewport = window.visualViewport;
		window.addEventListener("scroll", onScroll, true);
		window.addEventListener("resize", updateCoords);
		viewport?.addEventListener("resize", updateCoords);
		viewport?.addEventListener("scroll", updateCoords);
		return () => {
			window.removeEventListener("scroll", onScroll, true);
			window.removeEventListener("resize", updateCoords);
			viewport?.removeEventListener("resize", updateCoords);
			viewport?.removeEventListener("scroll", updateCoords);
		};
	}, [ensureExperience, open, updateCoords]);

	React.useEffect(() => {
		if (!open) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key !== "Escape") return;
			setOpen(false);
			restoreInputFocus();
		};
		const onClick = (event: MouseEvent) => {
			const target = event.target as Node | null;
			if (!target) return;
			if (panelRef.current?.contains(target)) return;
			if (formRef.current?.contains(target)) return;
			if (
				target instanceof Element &&
				target.closest("[data-home-search-backdrop]")
			) {
				return;
			}
			setOpen(false);
		};
		document.addEventListener("keydown", onKey);
		document.addEventListener("mousedown", onClick);
		return () => {
			document.removeEventListener("keydown", onKey);
			document.removeEventListener("mousedown", onClick);
		};
	}, [open, restoreInputFocus]);

	React.useEffect(() => {
		if (previousPathnameRef.current !== pathname) {
			setOpen(false);
			setCoords(null);
		}
		previousPathnameRef.current = pathname;
	}, [pathname]);

	const navigate = React.useCallback(
		(href: string) => {
			setOpen(false);
			router.push(href);
		},
		[router],
	);

	const goSearch = React.useCallback(() => {
		const trimmed = query.trim();
		if (!trimmed) return;
		setOpen(false);
		inputRef.current?.blur();
		router.push(`/search?q=${encodeURIComponent(trimmed)}`);
	}, [query, router]);

	const Experience = experience?.default;
	const pendingPanel =
		open && mounted && coords && !Experience
			? createPortal(
					<>
						<div
							data-home-search-backdrop
							aria-hidden="true"
							className="fixed inset-x-0 bottom-0 z-[205] hidden cursor-default touch-none bg-[rgba(45,49,45,0.2)] backdrop-blur-[5px] [@media(max-width:840px)]:block dark:bg-[rgb(var(--color-ink-950-rgb)/0.36)] [@media(prefers-reduced-transparency:reduce)]:backdrop-blur-none"
							style={{ top: Math.max(0, coords.top - 10) }}
							onPointerDown={(event) => event.stopPropagation()}
							onMouseDown={(event) => event.stopPropagation()}
							onClick={(event) => {
								event.preventDefault();
								event.stopPropagation();
								dismiss({ blurInput: true });
							}}
						/>
						<div
							ref={panelRef}
							id="home-search-panel"
							role="dialog"
							aria-label="搜尋建議"
							aria-busy={!experienceError}
							className="fixed z-[60] overflow-hidden rounded-hk-lg border border-hk-border-soft bg-hk-surface-card shadow-hk-dropdown [@media(max-width:840px)]:z-[210] [@media(max-width:840px)]:overflow-y-auto [@media(max-width:840px)]:overscroll-contain"
							style={{
								top: coords.top,
								left: coords.left,
								width: coords.width,
								maxHeight: coords.maxHeight,
							}}
						>
							{HAWKR_AGENT_ENABLED ? (
								<div className="border-b border-hk-border-subtle p-3">
									<AssistantEntrySkeleton query={query} />
								</div>
							) : null}
							{experienceError ? (
								<div
									className="flex min-h-40 flex-col items-center justify-center gap-2 p-4 text-center"
									role="alert"
								>
									<strong className="text-hk-heading text-sm">
										搜尋內容暫時載入失敗
									</strong>
									<p className="m-0 text-hk-muted text-xs">
										輸入內容已保留，可以重新載入。
									</p>
									<button
										type="button"
										data-home-search-import-retry
										className="min-h-11 rounded-hk-pill border border-hk-primary-border px-4 text-hk-accent-on-tint text-xs font-bold hover:bg-hk-primary-subtle"
										onClick={() => {
											ensureExperience();
											restoreInputFocus();
										}}
									>
										重新載入
									</button>
								</div>
							) : (
								<HomeSearchPanelSkeleton hasQuery={Boolean(query.trim())} />
							)}
						</div>
					</>,
					document.body,
				)
			: null;

	return (
		<div className={cn("relative z-[46] min-w-0 max-w-full", className)}>
			<form
				ref={formRef}
				data-home-navbar-search
				className="flex h-10 w-full min-w-0 max-w-full cursor-text items-center gap-2 rounded-hk-pill border border-hk-border-soft bg-hk-surface-card pr-1 pl-3 shadow-hk-card transition-[border-color,box-shadow] focus-within:border-hk-primary-border focus-within:ring-2 focus-within:ring-hk-primary/15 [@media(max-width:480px)]:pl-2"
				action={searchAction}
				onFocusCapture={ensureExperience}
				onPointerEnter={scheduleExperience}
				onPointerLeave={cancelScheduledExperience}
				onPointerDown={ensureExperience}
				onSubmit={(event) => {
					event.preventDefault();
					if (composingRef.current) return;
					goSearch();
				}}
				onMouseDown={(event) => {
					if ((event.target as HTMLElement).closest("button[type=submit]")) {
						return;
					}
					openSearch();
					if (event.target !== inputRef.current) {
						event.preventDefault();
						inputRef.current?.focus();
					}
				}}
			>
				<SearchGlyph
					size={18}
					className="shrink-0 text-hk-muted [@media(max-width:480px)]:hidden"
				/>
				<div className="relative min-w-0 flex-1 self-stretch">
					<input
						ref={inputRef}
						data-slot="control"
						name="q"
						value={query}
						onChange={(event) => {
							const nextQuery = event.target.value;
							setQuery(nextQuery);
							if (!composingRef.current) setCommittedQuery(nextQuery);
						}}
						onFocus={() => {
							if (suppressNextFocusOpenRef.current) {
								suppressNextFocusOpenRef.current = false;
								return;
							}
							openSearch();
						}}
						onCompositionStart={() => {
							composingRef.current = true;
							setCommittedQuery("");
						}}
						onCompositionEnd={(event) => {
							composingRef.current = false;
							setQuery(event.currentTarget.value);
							setCommittedQuery(event.currentTarget.value);
						}}
						inputMode="search"
						enterKeyHint="search"
						autoComplete="off"
						aria-label="搜尋商品"
						aria-haspopup="dialog"
						role="combobox"
						aria-expanded={open}
						aria-autocomplete="list"
						aria-controls="home-search-panel"
						onKeyDown={(event) => {
							if (
								event.key === "Enter" &&
								(composingRef.current || event.nativeEvent.isComposing)
							) {
								event.preventDefault();
								return;
							}
							if (event.key !== "ArrowDown" || !open) return;
							const firstPanelControl =
								panelRef.current?.querySelector<HTMLElement>(
									"a[href], button:not(:disabled)",
								);
							if (!firstPanelControl) return;
							event.preventDefault();
							firstPanelControl.focus();
						}}
						className="h-full w-full min-w-0 border-0 bg-transparent p-0 text-sm text-hk-heading shadow-none outline-none [@media(max-width:840px)]:text-base"
					/>
					{query ? null : (
						<span
							aria-hidden="true"
							className="pointer-events-none absolute inset-0 flex min-w-0 items-center text-sm text-hk-faint [@media(max-width:840px)]:text-base"
						>
							<span className="min-w-0 truncate [@media(max-width:840px)]:hidden">
								搜尋新鮮食材...
							</span>
							<span className="hidden whitespace-nowrap [@media(max-width:840px)]:inline">
								搜尋商品
							</span>
						</span>
					)}
				</div>
				<button
					type="submit"
					aria-label="送出搜尋"
					className="grid size-8 shrink-0 cursor-pointer place-items-center rounded-hk-pill border-0 bg-hk-primary text-hk-inverse transition-[background-color,transform] hover:bg-hk-primary-strong active:scale-[0.96] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-hk-primary"
				>
					<SearchGlyph stroke={2.2} size={17} className="shrink-0" />
				</button>
			</form>

			{Experience ? (
				<Experience
					key={pathname}
					open={open}
					query={query}
					requestQuery={committedQuery}
					coords={coords}
					panelRef={panelRef}
					onDismiss={dismiss}
					onNavigate={navigate}
				/>
			) : (
				pendingPanel
			)}
		</div>
	);
}

function SearchGlyph({
	stroke = 1.7,
	size = 16,
	className = "shrink-0 text-hk-muted",
}: {
	stroke?: number;
	size?: number;
	className?: string;
}) {
	return (
		<svg
			viewBox="0 0 24 24"
			width={size}
			height={size}
			fill="none"
			stroke="currentColor"
			strokeWidth={stroke}
			aria-hidden="true"
			className={className}
		>
			<circle cx="11" cy="11" r="7" />
			<path d="m20 20-3.5-3.5" strokeLinecap="round" />
		</svg>
	);
}
