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

// Render the brand splash in the initial HTML. Eligible guest public routes
// hand its visual fade to the document bootstrap as soon as SSR content is
// ready; React separately owns the hydration/document-settled barrier.
// Most routes fade once the DOM is interactive; request-time landing/detail
// routes additionally publish a content marker after their complete first tree
// is committed. `window.load` also waits for images, fonts and third-party
// resources, so it is not a stable signal for when users can interact.
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
		// Keep every async handle local to this exact effect setup. React Strict
		// Mode deliberately runs setup → cleanup → setup in development; a shared
		// ref lets an old setup cancel or append work owned by the new one.
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
				// Reveal the already-complete application tree while the splash is
				// still fully opaque. Give the browser one frame to paint that tree
				// behind the loader, then begin the fade. Keeping the page hidden
				// until the end of the opacity transition exposes the bare body
				// through a progressively transparent splash and finishes with a
				// one-frame content pop.
				root.classList.remove("initial-loader-active");
				const revealFrame = window.requestAnimationFrame(() => {
					if (disposed) return;
					setLeaving(true);
					// Adopt a bootstrap fade already in progress; never restart it
					// when delayed hydration or a Strict Mode replay arrives.
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
			// Telemetry only — the message text and fingerprint are kept stable so
			// existing Sentry issues keep aggregating (#338 closeout metric).
			// Fingerprint by route kind (not the full dynamic path) so e.g. every
			// slow seller page groups into one issue instead of one per seller id.
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
			// Gate convergence (#338): a missing marker must never hold the page.
			// Release the splash and expose the server-rendered tree as-is; failed
			// data belongs to error boundaries, not to this overlay.
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

		// A client-side navigation away from `entryPathname` voids the content
		// gate: the route it was waiting on has unmounted, so its marker can never
		// render and the splash would sit until the deadline — on a page that had
		// loaded fine. Soft navigations belong to PageTransition's contract, not
		// this document splash's, so release the content half and let `finish()`
		// apply the document half as usual.
		// (#1819: SellerWorkspaceLock bounces a seller landing on "/" to
		// /seller-center/dashboard, and the home marker never arrives.)
		//
		// Event-driven, not polled: a client-side navigation always commits new
		// DOM, so the same MutationObserver that watches for content markers also
		// observes the route change — no interval needed.
		//
		// Read the URL directly rather than `usePathname()`. This always-mounted
		// splash renders in the root layout OUTSIDE any <Suspense> so it ships in
		// the initial HTML; keeping it decoupled from next/navigation avoids
		// coupling first paint to the router (and, historically under Cache
		// Components, a dynamic read here made every route blocking). The URL is
		// the same fact the hook would report.
		const checkRouteChanged = () => {
			if (disposed || finished || routeContentReady) return;
			if (window.location.pathname === entryPathname) return;
			routeContentReady = true;
			contentObserver?.disconnect();
			finish();
		};

		// The inline bootstrap can finish before this effect or while its content
		// observer is waiting. It never clears initial-loader-active: only this
		// mounted React effect may release the third-party DOM-injection barrier.
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

		// These request-time routes can become DOM-ready while their first content
		// tree is still resolving. Wait for both signals there. Other routes leave
		// as soon as the DOM is interactive; slow images/fonts never gate the
		// splash. The deadline below remains an escape hatch for failed
		// data/hydration.
		if (waitForRouteContent && !finished) {
			checkRouteContent();
			if (!routeContentReady) {
				// Deliberately `subtree: true` on <html>, and deliberately NOT
				// narrowed to `childList` on <body>: the markers this waits for are
				// published deep inside the route tree (a route-ready <span>, a
				// `hidden` flip on the public handoff), not as direct children of
				// anything stable. It is the widest observer in the app and it runs
				// during hydration — but it is also disconnected the moment the
				// splash finishes, and every cheaper scope was tried and misses the
				// signal. Do not narrow it without moving the markers first.
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

		// A route with no complete content at the deadline releases the gate
		// instead of locking it (#338): report to Sentry, then fade the splash and
		// hand the viewport to whatever the server rendered.
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
		// Keep the first-paint CSS guard until React has actually removed the
		// loader node. Clearing it in the same effect as setVisible(false) can
		// expose a one-frame flash between the state update and commit.
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
				{/* Two stacked layers: the line-art outline stays put while each of
			    the colour blocks (awning, then counter) blooms out of the dead
			    centre — see `.il-blade` / `@keyframes ilBlade*`.
			    Markup lives in the shared BrandLogoLoader (no size prop here:
			    the `.il-logo` CSS default is this splash's 104px). */}
				<BrandLogoLoader />
			</div>
			{/* Wordmark: a faint dark-grey base with its real two-tone green
		    colour wiping in left → right, like a loading bar. Decorative — the
		    wrapper's `aria-label` already names this surface once. */}
			<BrandWordmark reveal />
		</div>
	);
}
