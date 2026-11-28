"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect, useState } from "react";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import BrandWordmark from "@/components/shared/BrandWordmark";
import { stripLocale } from "@/lib/intl/paths";
import {
	INITIAL_LOADER_REACT_FALLBACK_MS,
	INITIAL_LOADER_SKIP_CLASS,
} from "@/lib/loading/initial-loader-watchdog";
import {
	INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
	INITIAL_PUBLIC_CONTENT_READY_AT,
	INITIAL_PUBLIC_CONTENT_READY_CLASS,
} from "@/lib/loading/initial-public-content";
import {
	isPublicInitialContentReady,
	ROUTE_TERMINAL_READY_SELECTOR,
	routeContentReadySelectors,
} from "@/lib/loading/initial-route-ready";
import {
	STALE_CHUNK_RELOAD_CLASS,
	STALE_CHUNK_RELOAD_PENDING_CLASS,
} from "@/lib/loading/stale-chunk-reload";

/**
 * 非公開快路徑的最短顯示時間；公開訪客內容就緒後不追加固定等待。
 *
 * ⚠️ 匯出是為了讓測試從這裡推導預算，而不是各自寫死一個數字。
 * `tests/initial-loader-reliability.spec.ts` 曾用 500ms 當「loader 必須放開
 * 指標輸入」的上限 —— 那**低於本檔宣告的 700ms 最小值**，所以它只有在暖導航
 * （hide() 執行時 elapsed 已超過 MIN_SHOWN_MS）才碰巧會過。實測 8 次硬導航的
 * 釋放時間落在 4ms～568ms，兩次超出那個預算，而元件的行為完全正確。
 */
export const MIN_SHOWN_MS = 700;
/** splash 淡出時間；釋放的總上限是 MIN_SHOWN_MS + FADE_MS。 */
export const FADE_MS = 360;

export default function InitialPageLoader() {
	const [visible, setVisible] = useState(true);
	const [leaving, setLeaving] = useState(false);

	useEffect(() => {
		const timers: number[] = [];
		const frames: number[] = [];
		const root = document.documentElement;
		if (root.classList.contains(INITIAL_LOADER_SKIP_CLASS)) {
			setVisible(false);
			return;
		}
		const isStaleChunkReload = root.classList.contains(
			STALE_CHUNK_RELOAD_CLASS,
		);
		const reduceMotion = window.matchMedia(
			"(prefers-reduced-motion: reduce)",
		).matches;
		const shownAt = performance.now();
		const entryPathname = window.location.pathname;
		const earlyPublicHandoff = root.classList.contains(
			INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
		);
		const contentReadySelectors = routeContentReadySelectors(entryPathname);
		const waitForRouteContent = contentReadySelectors.length > 0;
		let finished = false;
		let documentReady = document.readyState !== "loading";
		let routeContentReady = !waitForRouteContent;
		let contentObserver: MutationObserver | null = null;
		let publicHandoffObserver: MutationObserver | null = null;
		let disposed = false;

		const scheduleTimeout = (callback: () => void, delay: number) => {
			const timer = window.setTimeout(() => {
				if (!disposed) callback();
			}, delay);
			timers.push(timer);
			return timer;
		};

		const hide = () => {
			if (disposed) return;
			if (isStaleChunkReload) {
				root.classList.remove(
					"initial-loader-active",
					STALE_CHUNK_RELOAD_PENDING_CLASS,
				);
				setVisible(false);
				return;
			}

			const elapsed = performance.now() - shownAt;
			const remainingMs =
				reduceMotion || earlyPublicHandoff
					? 0
					: Math.max(MIN_SHOWN_MS - elapsed, 0);

			scheduleTimeout(() => {
				root.classList.remove("initial-loader-active");
				const revealFrame = window.requestAnimationFrame(() => {
					if (disposed) return;
					setLeaving(true);
					const publicReadyAt = Number(
						root.getAttribute(INITIAL_PUBLIC_CONTENT_READY_AT),
					);
					const fadeRemaining = root.classList.contains(
						INITIAL_PUBLIC_CONTENT_READY_CLASS,
					)
						? Math.max(0, FADE_MS - (performance.now() - publicReadyAt))
						: FADE_MS;
					scheduleTimeout(
						() => setVisible(false),
						reduceMotion ? 0 : fadeRemaining,
					);
				});
				frames.push(revealFrame);
			}, remainingMs);
		};

		const releaseGateAtDeadline = () => {
			if (disposed || finished) return;
			const routeKind =
				stripLocale(window.location.pathname).split("/").filter(Boolean)[0] ||
				"home";
			Sentry.captureMessage("initial loader terminal timeout", {
				level: "warning",
				fingerprint: ["initial-loader-timeout", routeKind],
				tags: { route_kind: routeKind },
				extra: {
					pathname: window.location.pathname,
					elapsedMs: Math.round(performance.now() - shownAt),
					documentReady,
					routeContentReady,
					missingSelectors: contentReadySelectors.filter(
						(selector) => !document.querySelector(selector),
					),
				},
			});
			finished = true;
			contentObserver?.disconnect();
			publicHandoffObserver?.disconnect();
			hide();
		};

		const finish = () => {
			if (disposed || finished) return;
			if (!documentReady || !routeContentReady) return;
			finished = true;
			contentObserver?.disconnect();
			publicHandoffObserver?.disconnect();

			if (reduceMotion || isStaleChunkReload) {
				hide();
				return;
			}

			const firstFrame = window.requestAnimationFrame(() => {
				if (disposed) return;
				const secondFrame = window.requestAnimationFrame(hide);
				frames.push(secondFrame);
			});
			frames.push(firstFrame);
		};

		const onDocumentReady = () => {
			documentReady = true;
			finish();
		};

		const checkRouteContent = () => {
			if (routeContentReady) {
				return;
			}
			if (earlyPublicHandoff && !isPublicInitialContentReady(entryPathname)) {
				return;
			}
			const terminalRouteReady = document.querySelector(
				ROUTE_TERMINAL_READY_SELECTOR,
			);
			if (
				!terminalRouteReady &&
				contentReadySelectors.some(
					(selector) => !document.querySelector(selector),
				)
			) {
				return;
			}
			routeContentReady = true;
			contentObserver?.disconnect();
			finish();
		};

		const checkRouteChanged = () => {
			if (disposed || finished || routeContentReady) return;
			if (window.location.pathname === entryPathname) return;
			routeContentReady = true;
			contentObserver?.disconnect();
			finish();
		};

		const adoptPublicHandoff = () => {
			if (
				disposed ||
				finished ||
				!root.classList.contains(INITIAL_PUBLIC_CONTENT_READY_CLASS)
			)
				return;
			finished = true;
			contentObserver?.disconnect();
			publicHandoffObserver?.disconnect();
			hide();
		};
		if (earlyPublicHandoff) {
			publicHandoffObserver = new MutationObserver(adoptPublicHandoff);
			publicHandoffObserver.observe(root, {
				attributes: true,
				attributeFilter: ["class"],
			});
			adoptPublicHandoff();
		}

		if (waitForRouteContent && !finished) {
			checkRouteContent();
			if (!routeContentReady) {
				contentObserver = new MutationObserver(() => {
					checkRouteChanged();
					checkRouteContent();
				});
				contentObserver.observe(document.documentElement, {
					childList: true,
					subtree: true,
					...(earlyPublicHandoff
						? { attributes: true, attributeFilter: ["hidden"] }
						: {}),
				});
			}
		}

		scheduleTimeout(releaseGateAtDeadline, INITIAL_LOADER_REACT_FALLBACK_MS);

		document.documentElement.classList.add("initial-loader-active");

		if (documentReady) {
			finish();
		} else {
			window.addEventListener("DOMContentLoaded", onDocumentReady, {
				once: true,
			});
		}

		return () => {
			disposed = true;
			window.removeEventListener("DOMContentLoaded", onDocumentReady);
			contentObserver?.disconnect();
			publicHandoffObserver?.disconnect();
			for (const timer of timers) {
				window.clearTimeout(timer);
			}
			for (const frame of frames) {
				window.cancelAnimationFrame(frame);
			}
			document.documentElement.classList.remove("initial-loader-active");
		};
	}, []);

	useEffect(() => {
		if (!visible) {
			document.documentElement.classList.remove(STALE_CHUNK_RELOAD_CLASS);
		}
	}, [visible]);

	if (!visible) return null;

	return (
		<div
			className={`initial-loader${leaving ? " initial-loader--leaving" : ""}`}
			role="status"
			aria-live="polite"
			aria-label="Hawkr 載入中"
		>
			<div className="initial-loader__mark" aria-hidden="true">
				<BrandLogoLoader />
			</div>

			<BrandWordmark reveal />
		</div>
	);
}
