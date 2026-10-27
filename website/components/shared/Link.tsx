"use client";

import type { KYCApplication } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClientContext } from "@tanstack/react-query";
import * as React from "react";
import { Link as ViewTransitionLink } from "@/components/shared/view-transitions";
import type { RoleSlot } from "@/lib/auth/active-role";
import { useOptionalAuth } from "@/lib/auth/context";
import { workspaceOfPath } from "@/lib/auth/redirect";
import { localizeTarget } from "@/lib/intl/navigation";
import { stripLocale } from "@/lib/intl/paths";
import { useRoutedLocale } from "@/lib/intl/routed-locale";
import { useRouter } from "@/lib/intl/use-router";
import { beginNavPending } from "@/lib/loading/nav-pending";
import {
	beginNavigationIntent,
	cancelNavigationIntent,
	isNavigationIntentCurrent,
	navigateDocument,
} from "@/lib/navigation/observability";
import { armProgrammaticRouteMotion } from "@/lib/navigation/programmatic-route-motion";
import { queryKeys } from "@/lib/query/keys";
import type { MyStore } from "@/lib/seller/store";

/**
 * 全站統一的頁面連結，轉出 view-transitions fork 的 Link（fork 原因見
 * components/shared/view-transitions.tsx 檔頭）。
 *
 * 點擊導航時以瀏覽器原生 View Transitions API 過渡：舊頁/新頁由合成器
 * 截圖 crossfade，取代 PageTransition 早前的 wrapper transform 進場動畫
 * （transform 會劫持 fixed 定位、把整頁提升成合成層）。轉場「型態」由
 * PageTransition 在點擊當下蓋到 <html data-vt>（fade/rise/slide/zoom/none），
 * 對應 CSS 在 styles/components/page-transition.css；Tab 切換、persistent
 * shell、商品快速檢視等瞬切場景蓋 data-vt="none" 跳過動畫。
 *
 * 不支援 View Transitions 的瀏覽器自動降級為無動畫直接切頁。之後若遷移
 * 到 Next 原生 viewTransition，只需改這一個檔案。
 */
type LinkProps = React.ComponentProps<typeof ViewTransitionLink>;

function isPlainPrimaryClick(event: React.MouseEvent<HTMLAnchorElement>) {
	return (
		event.button === 0 &&
		!event.metaKey &&
		!event.ctrlKey &&
		!event.shiftKey &&
		!event.altKey &&
		(!event.currentTarget.target || event.currentTarget.target === "_self")
	);
}

function notificationRoleForTarget(
	target: LinkProps["href"] | LinkProps["as"],
): RoleSlot | null {
	if (typeof target !== "string") return null;
	try {
		const url = new URL(target, window.location.href);
		if (url.origin !== window.location.origin || url.search) return null;
		const path = stripLocale(url.pathname);
		if (path === "/account/notifications") return "buyer";
		if (path === "/seller-center/notifications") return "seller";
		return null;
	} catch {
		return null;
	}
}

function isSellerWalletTarget(target: LinkProps["href"] | LinkProps["as"]) {
	if (typeof target !== "string") return false;
	try {
		const url = new URL(target, window.location.href);
		return (
			url.origin === window.location.origin &&
			stripLocale(url.pathname) === "/seller-center/wallet" &&
			url.search === ""
		);
	} catch {
		return false;
	}
}

function isSellerWorkspaceTarget(target: LinkProps["href"] | LinkProps["as"]) {
	if (typeof target !== "string") return false;
	try {
		const url = new URL(target, window.location.href);
		const path = stripLocale(url.pathname);
		return (
			url.origin === window.location.origin &&
			(path === "/seller-center" || path.startsWith("/seller-center/"))
		);
	} catch {
		return false;
	}
}

function protectedRoleForTarget(
	target: LinkProps["href"] | LinkProps["as"],
): RoleSlot | null {
	if (typeof target !== "string") return null;
	try {
		const url = new URL(target, window.location.href);
		if (url.origin !== window.location.origin) return null;
		// `account` pages belong to no workspace, so there is no role to switch to.
		return workspaceOfPath(url.pathname);
	} catch {
		return null;
	}
}

const SELLER_WALLET_ORDER_PARAMS = {
	view: "seller",
	status: "completed",
	pageSize: 20,
} as const;

/**
 * 錢包預取的判斷在動態載入的模組裡：Link 在每一頁的 bundle 裡，不能為了一個判斷把 seller 生成碼拉進來。
 * 還沒載入（第一次）時，目的地一律視為需要預取，預取會先載入它。
 */
let walletTransactionRule:
	| typeof import("@/lib/seller/wallet-transactions").walletShowsTransactions
	| undefined;

// One user intent owns delayed atomic navigation across every Link instance.
// A later click invalidates an earlier prefetch completion so the first link
// cannot pull the user back after they have already chosen another page.
let latestNavigationIntent = 0;

const COMMIT_POLL_INTERVAL_MS = 120;
/** Under navigation-watchdog's 10s hard-reload so its recovery, not a stuck
 * rail, owns the truly-hung case. */
const COMMIT_POLL_MAX_MS = 8_000;

/**
 * Keep the delayed-atomic rail alive until the navigation actually commits
 * (App Router updates `location` at commit). Ending at data-settle would play
 * a completion sweep while the screen is still on the old page — for tab and
 * same-shell targets PageTransition intentionally never takes over, so that
 * premature sweep would be followed by pure silence for the whole RSC fetch.
 */
function endNavPendingWhenCommitted(
	end: (options?: { abort?: boolean }) => void,
	sourceHref: string,
) {
	const startedAt = Date.now();
	const tick = () => {
		if (window.location.href !== sourceHref) {
			end();
			return;
		}
		if (Date.now() - startedAt >= COMMIT_POLL_MAX_MS) {
			end({ abort: true });
			return;
		}
		window.setTimeout(tick, COMMIT_POLL_INTERVAL_MS);
	};
	window.setTimeout(tick, COMMIT_POLL_INTERVAL_MS);
}

/**
 * Warms client-query destinations (seller wallet, notifications, seller
 * workspace) while a link is hovered/focused; if the user clicks before the
 * warm settles, keep the current committed page visible and navigate only
 * once the atomic ensure resolves. The home route is server-rendered and
 * needs no client-side warm-up — it streams its own content per request.
 */
export default function Link(props: LinkProps) {
	const { onNavigate: _onNavigate, ...forwardedProps } = props;
	// Shared links are also rendered in isolated SSR/component contexts (email
	// previews, tests, and future server-owned shells). Data warming is an
	// enhancement, so absence of AppProviders must degrade to a plain link
	// instead of making the anchor itself impossible to render.
	const queryClient = React.useContext(QueryClientContext);
	const router = useRouter();
	const routedLocale = useRoutedLocale();
	const auth = useOptionalAuth();
	const activeRole = auth?.activeRole ?? null;
	const isLoggedIn = auth?.isLoggedIn ?? false;
	const hasSeller = auth?.hasSeller ?? false;
	const identity = React.useMemo(
		() => ({
			accountId: auth?.identity?.accountId ?? null,
			activeRole,
		}),
		[activeRole, auth?.identity?.accountId],
	);
	const navigationPendingRef = React.useRef(false);
	const observedIntentRef = React.useRef<string | null>(null);
	// The swallowed-click window (below) can last until COMMIT_POLL_MAX_MS. The
	// global rail reports that a navigation is happening, but not that THIS link
	// is the one waiting — so the anchor says so itself.
	const [deferred, setDeferred] = React.useState(false);
	const target = props.as ?? props.href;
	const targetRole = protectedRoleForTarget(target);
	const ownsTargetRole =
		targetRole === "buyer"
			? isLoggedIn
			: targetRole === "seller"
				? hasSeller
				: false;

	const ensureNotificationList = React.useCallback(
		async (role: RoleSlot) => {
			if (!queryClient) return;
			const module = await import("@/lib/hooks/useNotifications");
			const options = { pageSize: 20, role };
			return queryClient.ensureInfiniteQueryData({
				queryKey: module.notificationListQueryKey(identity, options),
				queryFn: ({ pageParam }) =>
					module.fetchNotificationListPage(options, pageParam as string | null),
				initialPageParam: null as string | null,
				getNextPageParam: module.nextNotificationListPage,
				staleTime: 30_000,
			});
		},
		[identity, queryClient],
	);

	const ensureSellerWallet = React.useCallback(async () => {
		if (!queryClient) return;
		const [sellerRepo, kycRepo, ordersRepo, walletRule] = await Promise.all([
			import("@/lib/seller/repo"),
			import("@/lib/seller/kyc"),
			import("@/lib/orders/repo"),
			import("@/lib/seller/wallet-transactions"),
		]);
		walletTransactionRule = walletRule.walletShowsTransactions;
		const [seller, kyc] = await Promise.all([
			queryClient.ensureQueryData({
				queryKey: queryKeys.seller.self(identity),
				queryFn: sellerRepo.fetchMyStore,
				staleTime: 30_000,
			}),
			queryClient.ensureQueryData({
				queryKey: queryKeys.seller.kyc(identity),
				queryFn: () => kycRepo.fetchMyKyc(),
				staleTime: 30_000,
			}),
		]);
		if (!walletRule.walletShowsTransactions(seller, kyc?.flow)) return;
		await Promise.all([
			queryClient.ensureQueryData({
				queryKey: queryKeys.seller.orders(identity, SELLER_WALLET_ORDER_PARAMS),
				queryFn: () => ordersRepo.fetchOrders(SELLER_WALLET_ORDER_PARAMS),
				staleTime: 30_000,
			}),
		]);
	}, [identity, queryClient]);

	const ensureSellerWorkspace = React.useCallback(async () => {
		if (!queryClient) return;
		const sellerRepo = await import("@/lib/seller/repo");
		return queryClient.ensureQueryData({
			queryKey: queryKeys.seller.self(identity),
			queryFn: sellerRepo.fetchMyStore,
			staleTime: 30_000,
		});
	}, [identity, queryClient]);

	const ensureDestination = React.useCallback(async () => {
		if (isSellerWalletTarget(target)) {
			return ensureSellerWallet();
		}
		const notificationRole = notificationRoleForTarget(target);
		if (notificationRole) {
			await Promise.all([
				ensureNotificationList(notificationRole),
				notificationRole === "seller"
					? ensureSellerWorkspace()
					: Promise.resolve(),
			]);
			return;
		}
		if (isSellerWorkspaceTarget(target)) {
			return ensureSellerWorkspace();
		}
		return undefined;
	}, [
		ensureNotificationList,
		ensureSellerWallet,
		ensureSellerWorkspace,
		target,
	]);

	const destinationNeedsData = React.useCallback(() => {
		if (!queryClient) return false;
		if (targetRole && activeRole !== targetRole) return false;
		if (isSellerWalletTarget(target)) {
			const seller = queryClient.getQueryData<MyStore | null>(
				queryKeys.seller.self(identity),
			);
			const kyc = queryClient.getQueryData<KYCApplication | null>(
				queryKeys.seller.kyc(identity),
			);
			if (
				seller === undefined ||
				kyc === undefined ||
				walletTransactionRule === undefined
			) {
				return true;
			}
			if (!walletTransactionRule(seller, kyc?.flow)) {
				return false;
			}
			return (
				queryClient.getQueryData(
					queryKeys.seller.orders(identity, SELLER_WALLET_ORDER_PARAMS),
				) === undefined
			);
		}
		const notificationRole = notificationRoleForTarget(target);
		if (notificationRole) {
			const notificationKey = queryKeys.notifications.list(identity, {
				role: notificationRole,
				pageSize: 20,
				category: "",
			});
			return (
				queryClient.getQueryData(notificationKey) === undefined ||
				(notificationRole === "seller" &&
					queryClient.getQueryData(queryKeys.seller.self(identity)) ===
						undefined)
			);
		}
		if (isSellerWorkspaceTarget(target)) {
			return (
				queryClient.getQueryData(queryKeys.seller.self(identity)) === undefined
			);
		}
		return false;
	}, [activeRole, identity, queryClient, target, targetRole]);

	const warmDestination = React.useCallback(() => {
		if (
			typeof window === "undefined" ||
			props.hardNavigation ||
			(targetRole && activeRole !== targetRole) ||
			!destinationNeedsData()
		) {
			return;
		}
		void ensureDestination().catch(() => undefined);
	}, [
		activeRole,
		destinationNeedsData,
		ensureDestination,
		props.hardNavigation,
		targetRole,
	]);

	const handleClick = React.useCallback(
		(event: React.MouseEvent<HTMLAnchorElement>) => {
			props.onClick?.(event);
			if (
				event.defaultPrevented ||
				!isPlainPrimaryClick(event) ||
				event.currentTarget.hasAttribute("download")
			) {
				return;
			}

			const roleSwitchRequired =
				Boolean(targetRole) && activeRole !== targetRole && ownsTargetRole;
			const needsData = destinationNeedsData();
			if (
				needsData &&
				navigationPendingRef.current &&
				isNavigationIntentCurrent(observedIntentRef.current)
			) {
				// Only reuse a still-owned wait. A trusted new interaction cancels
				// old recovery in capture phase; that click must create a new intent.
				// QueryClient deduplicates any still-pending destination request.
				event.preventDefault();
				return;
			}
			const navigationIntent = ++latestNavigationIntent;
			const navigationSource = window.location.href;
			const navigationMetadata = props as LinkProps & {
				"data-nav-action"?: string;
				"data-nav-component"?: string;
			};
			const renderedTarget = event.currentTarget.href;
			const renderedTargetPath = stripLocale(
				new URL(renderedTarget, window.location.href).pathname,
			);
			const observedIntent = beginNavigationIntent({
				// The rendered anchor is authoritative for UrlObject/as/basePath and
				// same-origin absolute URLs.
				target: renderedTarget,
				trigger: "link",
				mode: props.hardNavigation
					? "hard"
					: /^\/products\/[^/]+\/?$/.test(renderedTargetPath)
						? "intercepted"
						: "soft",
				actionId: navigationMetadata["data-nav-action"],
				componentId: navigationMetadata["data-nav-component"],
				replace: props.replace,
			});
			observedIntentRef.current = observedIntent;

			// The VT/deferred paths prevent NextLink's click before its onNavigate
			// callback runs. Own that callback here once for every local soft link.
			if (
				!props.hardNavigation &&
				new URL(renderedTarget).origin === window.location.origin
			) {
				props.onNavigate?.({
					preventDefault() {
						event.preventDefault();
						cancelNavigationIntent(observedIntent);
					},
				});
				if (event.defaultPrevented) return;
			}

			if (props.hardNavigation) return;
			if (roleSwitchRequired && targetRole && typeof target === "string") {
				if (auth?.switchRole(targetRole)) {
					event.preventDefault();
					navigateDocument(renderedTarget, {
						componentId:
							navigationMetadata["data-nav-component"] ?? "shared_link",
						actionId: navigationMetadata["data-nav-action"] ?? "switch_role",
						replace: props.replace,
					});
				}
				return;
			}
			if (!needsData) return;

			event.preventDefault();
			navigationPendingRef.current = true;
			setDeferred(true);
			// The click is swallowed until the data wave settles — surface the
			// wait on the global progress rail. PageTransition's own signal
			// doesn't cover this window for same-shell/tab targets (its history
			// patch skips startLoading for them), so the rail is the only
			// acknowledgement the tap got through.
			const endNavPending = beginNavPending();
			void ensureDestination()
				// A failed wave still navigates: the destination owns its own
				// empty/error surface. No Link-level rescue — a hard reload
				// here would trade a recoverable in-app state for a browser
				// error page and lose all SPA state when offline.
				.catch(() => undefined)
				.then(() => {
					if (
						navigationIntent !== latestNavigationIntent ||
						!isNavigationIntentCurrent(observedIntent) ||
						window.location.href !== navigationSource
					) {
						endNavPending();
						return;
					}
					const href = renderedTarget;
					armProgrammaticRouteMotion(href);
					if (props.replace) {
						router.replace(href, { scroll: props.scroll ?? true });
					} else {
						router.push(href, { scroll: props.scroll ?? true });
					}
					// Don't end the rail at data-settle: the RSC commit is still
					// in flight and the screen hasn't changed — for tab/same-shell
					// targets nothing else takes over, so ending here would play
					// a completion sweep followed by seconds of silence. Poll the
					// URL (App Router updates location at commit) and hand the
					// >cap case to navigation-watchdog's recovery.
					endNavPendingWhenCommitted(endNavPending, navigationSource);
				})
				.finally(() => {
					if (observedIntentRef.current !== observedIntent) return;
					navigationPendingRef.current = false;
					setDeferred(false);
				});
		},
		[
			activeRole,
			destinationNeedsData,
			ensureDestination,
			auth,
			ownsTargetRole,
			props,
			router,
			target,
			targetRole,
		],
	);

	return (
		<ViewTransitionLink
			{...forwardedProps}
			href={localizeTarget(props.href, routedLocale)}
			as={localizeTarget(props.as, routedLocale)}
			aria-busy={deferred || props["aria-busy"]}
			data-pending={deferred ? "" : undefined}
			onClick={handleClick}
			onFocus={(event) => {
				props.onFocus?.(event);
				if (!event.defaultPrevented) warmDestination();
			}}
			onMouseEnter={(event) => {
				props.onMouseEnter?.(event);
				if (!event.defaultPrevented) warmDestination();
			}}
			onPointerDown={(event) => {
				props.onPointerDown?.(event);
				if (event.defaultPrevented) return;
				// Touch: `pointerdown` fires on every tap, INCLUDING the tap that is
				// about to navigate and the taps that are really scroll starts, so
				// warming here fans out multi-query prefetches a phone never uses.
				// `mouseenter`/`focus` above already cover the intent signals that
				// exist on a pointer device.
				if (event.pointerType === "touch") return;
				warmDestination();
			}}
		/>
	);
}
