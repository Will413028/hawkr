"use client";

import * as React from "react";
import AutoHideNavShell from "@/components/shared/AutoHideNavShell";
import type { MegaMenuDynamicLinks } from "@/components/shared/MegaMenu";
import LegacyNavbar from "@/components/shared/Navbar";
import { SellerHeaderLoader } from "@/components/shared/SellerNavbarLoader";
import { useAuth } from "@/lib/auth/context";
import { usePathname } from "@/lib/intl/navigation";
import { isDelegatedWorkspacePath } from "@/lib/navigation/delegated-workspace";
import { markAppEntry, recordPathname } from "@/lib/navigation/history-marker";
import { isStandaloneFlowPath } from "@/lib/navigation/standalone-flow";

/**
 * Renders the existing site Navbar for guest/buyer (per spec: "buyer 上方跟現在一樣"),
 * and loads the dedicated seller header only after the seller branch is selected.
 */
interface RoleAwareNavbarProps {
	/** SSR-prefetched dynamic megamenu data forwarded to the buyer Navbar. */
	megaMenuDynamicLinks?: MegaMenuDynamicLinks;
}

/**
 * Routes that opt out of the auto-hide behaviour (the bar is an anchor for the
 * page's own filtering / gallery interactions there).
 *
 * Exported as a predicate rather than inlined so a new "don't auto-hide me"
 * page has ONE place to register. It belongs next to the other route
 * predicates in `lib/navigation/route-transition-policy.ts`; that module is
 * owned by another workstream in this pass, so the move is deliberately left
 * as a follow-up and the predicate is exported from here in the meantime.
 */
export function shouldKeepNavbarVisible(effectivePathname: string): boolean {
	return (
		effectivePathname === "/market" ||
		effectivePathname.startsWith("/market/") ||
		effectivePathname === "/products" ||
		effectivePathname === "/seller/pricing" ||
		effectivePathname === "/seller-guide" ||
		/^\/products\/[^/]+/.test(effectivePathname) ||
		/^\/sellers\/[^/]+\/products\/[^/]+/.test(effectivePathname)
	);
}

export default function RoleAwareNavbar({
	megaMenuDynamicLinks,
}: RoleAwareNavbarProps = {}) {
	const pathname = usePathname() ?? "";
	const { activeRole, hydrated, isLoggedIn } = useAuth();
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => {
		setMounted(true);
	}, []);
	// Site-wide history bookkeeping for BackLink. This component is the one
	// piece of chrome the root layout mounts on every route, so it is where the
	// in-app entry stamp and the previous-route record can actually be complete.
	React.useEffect(() => {
		markAppEntry();
	}, []);
	React.useEffect(() => {
		if (pathname) recordPathname(pathname);
	}, [pathname]);
	const authReady = mounted && hydrated;
	// Never substitute browser state during render. A prerendered Suspense boundary
	// may temporarily expose a null pathname on both the server and the first
	// client pass; reading window.location here would turn that into different
	// trees and make selective hydration fail intermittently.
	const effectivePathname = pathname ?? "";

	// Height reservation, never `null`: every branch below that removes the bar
	// AFTER mount used to collapse 84px of layout and shove the page up.
	const reservation = (
		<div className="root-nav-layout-reservation" aria-hidden="true" />
	);

	if (!effectivePathname) return reservation;
	// 代行工作區自帶完整外框，且站台 navbar 讀的是 www 的登入態（代行者在 www 上
	// 通常沒有，有的話也是他自己的身分）—— 見 lib/navigation/delegated-workspace.ts。
	// 這條與下面的外部供應商 portal 是純路徑判斷，SSR 與首次 client render 一致，
	// 不會在 mount 之後才把 bar 收掉，所以維持真正的 null（保留高度反而會憑空
	// 多出一條空白帶）。
	if (isDelegatedWorkspacePath(effectivePathname)) return null;
	// Onboarding remains a focused flow (no footer/mobile bottom nav), but keeps
	// the seller header as an authenticated escape hatch. Its account menu is the
	// only safe way to sign out when someone entered with the wrong account.
	const isSellerOnboardingRoute = isStandaloneFlowPath(effectivePathname);
	const keepNavbarVisible = shouldKeepNavbarVisible(effectivePathname);
	// 認證頁（/login、/register…）是身分轉換中的獨立流程，不套賣家工作區 header：
	// 否則「還登入著的賣家」一停在 /login，賣家導覽列就會疊在登入卡上（跳轉前的
	// 競態）。/seller-center 不在此列，照常顯示。
	const isAuthRoute =
		effectivePathname.startsWith("/login") ||
		effectivePathname.startsWith("/register");
	const isSellerWorkspaceRoute = effectivePathname.startsWith("/seller-center");
	// 認證頁上的「已登入者」正在被導去自己的工作區（買家→/account、賣家→
	// /seller-center）。不渲染任何 navbar，否則買家導覽列會在跳轉落地前閃一下，
	// 接著才被目的頁的正確列取代——使用者看到的是「怪列先出現再換掉」。未登入
	// 訪客（一般登入/註冊）不受影響，照常顯示。這條在 mount 之後才成立，所以
	// 收掉的是**已經畫出來的** 84px bar：保留高度，別讓登入卡跳一下。
	if (isAuthRoute && authReady && isLoggedIn) return reservation;
	// The seller header reads client-owned identity. Keep SSR and the first client
	// render deterministic; after mount, its loader preserves the same geometry
	// while fetching the seller-only implementation.
	if ((isSellerOnboardingRoute || isSellerWorkspaceRoute) && !mounted) {
		return <div className="root-nav-layout-reservation" aria-hidden="true" />;
	}
	// 賣家身分看到的就該是賣家 header——不只 /seller-center/*。否則自助開店頁
	// （/seller/onboarding）會出現「買家導覽列＋賣家選單」的混搭。認證頁除外。
	if (
		isSellerOnboardingRoute ||
		isSellerWorkspaceRoute ||
		(authReady && activeRole === "seller" && !isAuthRoute)
	) {
		return (
			<AutoHideNavShell
				autoHide={!keepNavbarVisible && !isSellerOnboardingRoute}
			>
				<SellerHeaderLoader onboarding={isSellerOnboardingRoute} />
			</AutoHideNavShell>
		);
	}

	return (
		<AutoHideNavShell autoHide={!keepNavbarVisible}>
			<LegacyNavbar megaMenuDynamicLinks={megaMenuDynamicLinks} />
		</AutoHideNavShell>
	);
}
