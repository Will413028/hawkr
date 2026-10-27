"use client";

/**
 * next-view-transitions@0.3.5 的 in-repo fork（MIT, shuding/next-view-transitions）。
 *
 * Fork 原因：上游的 <ViewTransitions> provider 在 render 期呼叫 usePathname()，
 * 而它必須包在 root layout 最外層（所有 Suspense 之外）才能供應 context 給全站
 * 的 <Link>。Next 16 cacheComponents（#338 前）下，動態路由（/products/[id]、
 * /account/after-sales/[caseNo]…）prerender 時讀 pathname 屬於讀取 request
 * 資料 → 整條路由變 blocking route，`next build` 直接失敗。上游 0.3.5
 * （2025-12）尚無修正。
 *
 * 與上游的結構差異：
 * 1. useBrowserNativeTransitions()（popstate 返回/前進的原生 View Transition
 *    銜接）—— 已整組移除（設計決策 2026-07，見 <ViewTransitions> 內註解）：
 *    返回/前進一律瞬切。
 * 2. Link / useTransitionRouter / context 語意與上游 1:1（加上 data-vt 瞬切
 *    跳過與 TimeoutError 靜音，見 b5dffedd）。
 * 3. router 用 lib/intl/use-router.ts 的 `useRouter`、整頁導覽走 `commitDocumentNavigation`：
 *    出口加讀者的語系前綴（可重複套用），不直接碰 next/navigation 的 router 與 window.location。
 */

import type {
	AppRouterInstance,
	NavigateOptions,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import NextLink from "next/link";
import * as React from "react";
import { prefersReducedMotion } from "@/lib/a11y/reduced-motion";
import { stripLocale } from "@/lib/intl/paths";
import { useRouter } from "@/lib/intl/use-router";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import {
	canPrefetchMainNav,
	type MainNavPrefetchIntent,
} from "@/lib/navigation/main-nav-prefetch";
import { announceProductQuickViewPending } from "@/lib/product/quickViewOrigin";

type FinishViewTransition = () => void;

const ViewTransitionsContext = React.createContext<React.Dispatch<
	React.SetStateAction<FinishViewTransition | null>
> | null>(null);

// data-vt="none" 表示 PageTransition 已把這次導航定調為瞬切（Tab 切換、
// persistent shell、商品快速檢視 modal —— 見 stampViewTransitionType）。
// CSS 雖已停用 "none" 的所有 VT 動畫，但 document.startViewTransition()
// 本身仍會凍結整頁渲染與輸入，直到新 route commit 完成 —— dev 的 RSC
// refetch/編譯可達數秒，超過瀏覽器 4 秒 DOM update 上限時整頁卡死到
// TimeoutError 收場。瞬切導航必須整段跳過 View Transition，不能只關動畫。
function isInstantNav() {
	return document.documentElement.dataset.vt === "none";
}

// 轉場被瀏覽器中斷（DOM update 超過 4 秒上限、被下一個轉場搶佔）只代表
// 放棄這次動畫，導航本身照常完成，不是應用錯誤 —— 掛上空 handler，
// 避免 ready/updateCallbackDone/finished 各噴一次 unhandledRejection。
function muteTransitionAbort(transition: ViewTransition) {
	transition.updateCallbackDone.catch(() => {});
	transition.ready.catch(() => {});
	transition.finished.catch(() => {});
}

// 設計決策（2026-07）：返回/前進一律瞬切 —— 上游在 popstate 以
// startViewTransition 銜接返回動畫的 useBrowserNativeTransitions 已整組移除。
// 瀏覽器返回鍵的原生預期是「立即回去 + 捲動還原」，動畫是加分不是必要；
// 而返回目標的 RSC 時長不可控 —— startViewTransition 從呼叫起就凍結整頁
// 渲染與輸入直到新 route commit，dev/慢網路會一路頂到瀏覽器 4 秒 DOM-update
// 上限才被（已靜音的）TimeoutError 解凍，體感是安靜卡住數秒（商家頁返回
// 市集卡住即此）。data-vt 又是上一次「前進」導航的殘值，返回方向從未重新
// 定調，動畫型殘值（rise/fade/…）必踩凍結；且該元件因 selective hydration
// 比 PageTransition 更早掛 popstate listener，無法靠「返回前重新 stamp」
// 保證時序 —— 故直接移除 popstate VT。前進動畫不受影響（useTransitionRouter
// / Link 路徑照舊）。返回的捲動/清單還原由瀏覽器 + listSnapshot 機制負責。
export function ViewTransitions({ children }: { children: React.ReactNode }) {
	const [finishViewTransition, setFinishViewTransition] =
		React.useState<FinishViewTransition | null>(null);

	React.useEffect(() => {
		if (finishViewTransition) {
			finishViewTransition();
			setFinishViewTransition(null);
		}
	}, [finishViewTransition]);

	return (
		<ViewTransitionsContext.Provider value={setFinishViewTransition}>
			{children}
		</ViewTransitionsContext.Provider>
	);
}

export function useSetFinishViewTransition() {
	const context = React.use(ViewTransitionsContext);
	if (!context) {
		throw new Error(
			"useSetFinishViewTransition must be used within a ViewTransitions component",
		);
	}
	return context;
}

interface TransitionOptions {
	onTransitionReady?: () => void;
}

export interface TransitionRouter extends AppRouterInstance {
	push: (href: string, options?: NavigateOptions & TransitionOptions) => void;
	replace: (
		href: string,
		options?: NavigateOptions & TransitionOptions,
	) => void;
}

export function useTransitionRouter(): TransitionRouter {
	const router = useRouter();
	const finishViewTransition = useSetFinishViewTransition();

	const triggerTransition = React.useCallback(
		(cb: () => void, { onTransitionReady }: TransitionOptions = {}) => {
			// prefers-reduced-motion 不只是「動畫請放輕」：startViewTransition 會凍結
			// 整頁的渲染與輸入直到新 route commit 完成，即使 CSS 把動畫時間設成 0。
			// 對要求減少動態的使用者來說，那段凍結本身就是最該避免的東西 —— 整段跳過，
			// 直接換頁（等同不支援 View Transitions 的瀏覽器）。
			if (
				"startViewTransition" in document &&
				!isInstantNav() &&
				!prefersReducedMotion()
			) {
				const transition = document.startViewTransition(
					() =>
						new Promise<void>((resolve) => {
							React.startTransition(() => {
								cb();
								// state 設成 resolve 本身；provider 的 effect 會呼叫它，
								// 代表新內容已 commit，View Transition 可以收尾。
								finishViewTransition(() => resolve);
							});
						}),
				);
				muteTransitionAbort(transition);
				if (onTransitionReady) {
					// 第二參數：ready 被 abort 時放棄 callback 即可，
					// 也避免 .then 衍生的 promise 自己再 unhandledRejection。
					transition.ready.then(onTransitionReady, () => {});
				}
			} else {
				cb();
			}
		},
		[finishViewTransition],
	);

	const push = React.useCallback(
		(
			href: string,
			{
				onTransitionReady,
				...options
			}: NavigateOptions & TransitionOptions = {},
		) => {
			triggerTransition(() => router.push(href, options), {
				onTransitionReady,
			});
		},
		[triggerTransition, router],
	);

	const replace = React.useCallback(
		(
			href: string,
			{
				onTransitionReady,
				...options
			}: NavigateOptions & TransitionOptions = {},
		) => {
			triggerTransition(() => router.replace(href, options), {
				onTransitionReady,
			});
		},
		[triggerTransition, router],
	);

	return React.useMemo(
		() => ({ ...router, push, replace }),
		[push, replace, router],
	);
}

// 以下兩個 helper 抄自 next/link 的點擊判斷（與上游 fork 相同出處）。
function isModifiedEvent(event: React.MouseEvent): boolean {
	const eventTarget = event.currentTarget as HTMLElement;
	const target = eventTarget.getAttribute("target");
	return (
		Boolean(target && target !== "_self") ||
		event.metaKey ||
		event.ctrlKey ||
		event.shiftKey ||
		event.altKey ||
		Boolean(event.nativeEvent && event.nativeEvent.which === 2)
	);
}

function shouldPreserveDefault(
	e: React.MouseEvent<HTMLAnchorElement>,
): boolean {
	const { nodeName } = e.currentTarget;
	const isAnchorNodeName = nodeName.toUpperCase() === "A";
	if (!isAnchorNodeName) return false;
	if (isModifiedEvent(e) || e.currentTarget.hasAttribute("download")) {
		return true;
	}
	try {
		const url = new URL(e.currentTarget.href, window.location.href);
		return (
			(url.protocol !== "http:" && url.protocol !== "https:") ||
			url.origin !== window.location.origin
		);
	} catch {
		return true;
	}
}

/**
 * Resolve only same-document App Router destinations. The rendered anchor is
 * authoritative here (rather than the href prop) so UrlObject/as/basePath
 * callers keep NextLink's normal URL formatting.
 */
function resolveIntentPrefetchTarget(anchor: HTMLAnchorElement): string | null {
	if (
		anchor.hasAttribute("download") ||
		(anchor.target && anchor.target !== "_self")
	) {
		return null;
	}
	try {
		const url = new URL(anchor.href, window.location.href);
		if (
			(url.protocol !== "http:" && url.protocol !== "https:") ||
			url.origin !== window.location.origin
		) {
			return null;
		}
		const target = `${url.pathname}${url.search}`;
		const current = `${window.location.pathname}${window.location.search}`;
		// Hash-only/same-page anchors do not need an RSC payload.
		return target === current ? null : target;
	} catch {
		return null;
	}
}

function resolveProductQuickViewTarget(target: string): string | null {
	try {
		const url = new URL(target, window.location.href);
		if (
			url.origin === window.location.origin &&
			/^\/products\/[^/]+\/?$/.test(stripLocale(url.pathname))
		) {
			// App Router navigation should remain same-document even when an API
			// supplied a same-origin absolute action URL.
			return `${url.pathname}${url.search}${url.hash}`;
		}
		return null;
	} catch {
		return null;
	}
}

type LinkProps = React.ComponentProps<typeof NextLink> & {
	/**
	 * Leave the current document instead of asking the App Router for an RSC
	 * payload. Use this at authentication boundaries: a login page must remain
	 * reachable even when the current tab owns stale chunks after a deployment or
	 * when a slow RSC request would otherwise hold a View Transition open.
	 *
	 * The rendered element is still a real anchor, so pre-hydration clicks and
	 * modified clicks (new tab/window) keep their native browser behaviour.
	 */
	hardNavigation?: boolean;
};

/** next/link 包裝：點擊時改走 useTransitionRouter 觸發 View Transition。 */
export function Link(props: LinkProps) {
	const router = useTransitionRouter();
	const {
		hardNavigation = false,
		prefetch,
		href,
		as,
		replace,
		scroll,
		onClick: userOnClick,
		onFocus: userOnFocus,
		onMouseEnter: userOnMouseEnter,
		onPointerDown: userOnPointerDown,
		...nextLinkProps
	} = props;
	const softTarget = as ?? href;
	const intentPrefetchedTargetsRef = React.useRef(new Set<string>());

	const prefetchOnIntent = React.useCallback(
		(
			event: React.SyntheticEvent<HTMLAnchorElement>,
			intent: MainNavPrefetchIntent = "pointer",
		) => {
			if (event.defaultPrevented || hardNavigation || prefetch === false) {
				return;
			}
			if (!canPrefetchMainNav(intent)) {
				return;
			}
			const target = resolveIntentPrefetchTarget(event.currentTarget);
			if (!target || intentPrefetchedTargetsRef.current.has(target)) return;
			// A product click is owned by the intercepted quick-view route below.
			// Never overlap that navigation with a manual intent prefetch: in
			// production, clicking while a hover/focus prefetch is still in flight
			// can leave the router.push Flight stream stuck after its 200 headers. Keep
			// the source list visible and let the actual quick-view navigation own
			// the only RSC request.
			if (resolveProductQuickViewTarget(target)) {
				return;
			}
			intentPrefetchedTargetsRef.current.add(target);
			router.prefetch(target);
		},
		[hardNavigation, prefetch, router],
	);

	const onClick = React.useCallback(
		(e: React.MouseEvent<HTMLAnchorElement>) => {
			userOnClick?.(e);
			if (e.defaultPrevented) return;
			if (hardNavigation) {
				if (shouldPreserveDefault(e)) return;
				const target = as ?? href;
				// UrlObject callers stay on Next's normal path. Auth entry points use
				// string URLs generated by loginPath(), so the hard-navigation contract
				// remains explicit and cannot accidentally stringify an unsafe object.
				if (typeof target !== "string") return;
				e.preventDefault();
				commitDocumentNavigation(target, replace);
				return;
			}
			if (typeof softTarget === "string" && !shouldPreserveDefault(e)) {
				const productQuickViewTarget =
					resolveProductQuickViewTarget(softTarget);
				if (productQuickViewTarget) {
					// Product details are an intercepted parallel route. Own this
					// navigation explicitly so the underlying list remains mounted and
					// the modal can take over the pending shell.
					e.preventDefault();
					try {
						announceProductQuickViewPending();
						const navigate = replace ? router.replace : router.push;
						navigate(productQuickViewTarget, { scroll: scroll ?? true });
					} catch {
						// The native anchor was already prevented. If any client
						// router/instrumentation hook throws synchronously, retain a
						// guaranteed document-navigation escape hatch.
						commitDocumentNavigation(productQuickViewTarget);
					}
					return;
				}
			}
			if (!("startViewTransition" in document)) return;
			// PageTransition 的 capture-phase click handler 先跑，此刻
			// data-vt 已是本次導航的定調；瞬切導航走 next/link 原生路徑
			// （等同不支援 VT 的瀏覽器），完全不碰 startViewTransition。
			if (isInstantNav()) return;
			if (shouldPreserveDefault(e)) return;

			// 只攔截字串 href（全站慣例）；UrlObject 交回 next/link 原生導航。
			const target = as ?? href;
			if (typeof target !== "string") return;

			e.preventDefault();
			const navigate = replace ? router.replace : router.push;
			navigate(target, { scroll: scroll ?? true });
		},
		[
			userOnClick,
			hardNavigation,
			href,
			as,
			replace,
			scroll,
			router,
			softTarget,
		],
	);

	return (
		<NextLink
			{...nextLinkProps}
			data-hk-hard-navigation={hardNavigation ? "" : undefined}
			href={href}
			as={as}
			replace={replace}
			scroll={scroll}
			// A hard navigation must fetch the current document/build. Avoid warming
			// an RSC payload that this path intentionally never consumes. Soft links
			// also disable NextLink's viewport observer: route prefetch happens only
			// after explicit hover/focus/pointer intent below.
			prefetch={false}
			onClick={onClick}
			onFocus={(event) => {
				userOnFocus?.(event);
				prefetchOnIntent(event, "focus");
			}}
			onMouseEnter={(event) => {
				userOnMouseEnter?.(event);
				prefetchOnIntent(event, "pointer");
			}}
			onPointerDown={(event) => {
				userOnPointerDown?.(event);
				prefetchOnIntent(event, "touch");
			}}
		/>
	);
}
