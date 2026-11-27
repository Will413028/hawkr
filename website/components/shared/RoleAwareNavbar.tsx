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
	const { activeRole, status } = useAuth();
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => {
		setMounted(true);
	}, []);
	React.useEffect(() => {
		markAppEntry();
	}, []);
	React.useEffect(() => {
		if (pathname) recordPathname(pathname);
	}, [pathname]);
	const authReady = status !== "unknown";
	const effectivePathname = pathname ?? "";

	const reservation = (
		<div className="root-nav-layout-reservation" aria-hidden="true" />
	);

	if (!effectivePathname) return reservation;
	if (isDelegatedWorkspacePath(effectivePathname)) return null;
	const isSellerOnboardingRoute = isStandaloneFlowPath(effectivePathname);
	const keepNavbarVisible = shouldKeepNavbarVisible(effectivePathname);
	const isAuthRoute =
		effectivePathname.startsWith("/login") ||
		effectivePathname.startsWith("/register");
	const isSellerWorkspaceRoute = effectivePathname.startsWith("/seller-center");
	if (isAuthRoute && authReady && status === "signedIn") return reservation;
	if ((isSellerOnboardingRoute || isSellerWorkspaceRoute) && !mounted) {
		return <div className="root-nav-layout-reservation" aria-hidden="true" />;
	}
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
