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

function isInstantNav() {
	return document.documentElement.dataset.vt === "none";
}

function muteTransitionAbort(transition: ViewTransition) {
	transition.updateCallbackDone.catch(() => {});
	transition.ready.catch(() => {});
	transition.finished.catch(() => {});
}

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
								finishViewTransition(() => resolve);
							});
						}),
				);
				muteTransitionAbort(transition);
				if (onTransitionReady) {
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
				if (typeof target !== "string") return;
				e.preventDefault();
				commitDocumentNavigation(target, replace);
				return;
			}
			if (typeof softTarget === "string" && !shouldPreserveDefault(e)) {
				const productQuickViewTarget =
					resolveProductQuickViewTarget(softTarget);
				if (productQuickViewTarget) {
					e.preventDefault();
					try {
						announceProductQuickViewPending();
						const navigate = replace ? router.replace : router.push;
						navigate(productQuickViewTarget, { scroll: scroll ?? true });
					} catch {
						commitDocumentNavigation(productQuickViewTarget);
					}
					return;
				}
			}
			if (!("startViewTransition" in document)) return;
			if (isInstantNav()) return;
			if (shouldPreserveDefault(e)) return;

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
