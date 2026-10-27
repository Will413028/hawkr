"use client";

import * as React from "react";
import LegacyFooter from "@/components/shared/Footer";
import SellerFooter from "@/components/shared/SellerFooter";
import SiteBackgroundToggle from "@/components/shared/SiteBackgroundToggle";
import { useAuth } from "@/lib/auth/context";
import { usePathname } from "@/lib/intl/navigation";
import { isDelegatedWorkspacePath } from "@/lib/navigation/delegated-workspace";
import { footerPerspectiveForPathname } from "@/lib/navigation/footer-perspective";
import { isStandaloneFlowPath } from "@/lib/navigation/standalone-flow";

/**
 * Reserved height for the not-yet-resolved footer. Returning null used to
 * remove several hundred px from the document at the very bottom, so the total
 * scroll height changed mid-session on long pages (and any bottom-anchored
 * scroll position jumped).
 *
 * 560px is the buyer footer's desktop stack computed from its own layout, not
 * a browser measurement (no browser in this environment): 60px top padding +
 * the tallest column (客戶服務 = heading 48px + 7 links × 44px + 6 gaps × 12px)
 * + the 48px column/bottom gap + the ~50px bottom bar. Override per surface
 * with `--footer-reserve` if a footer variant ever differs materially.
 */
const FOOTER_RESERVE = "min-h-[var(--footer-reserve,560px)]";

/**
 * Picks one canonical footer for the active workspace. The buyer-facing
 * storefront preview nested under /seller-center is the intentional exception.
 */
export default function RoleAwareFooter({
	categoryColumns,
}: {
	/**
	 * The buyer footer's data columns, rendered on the server and passed
	 * through this client boundary as an element. This component is client-only
	 * (it picks a footer from pathname + auth), so the columns cannot fetch
	 * their own data here without becoming a browser request — which is exactly
	 * what site-stability forbids. Rendering them above and passing them down
	 * keeps the read on the server.
	 */
	categoryColumns: React.ReactNode;
}) {
	const pathname = usePathname() ?? "";
	const { activeRole, hydrated } = useAuth();
	const [mounted, setMounted] = React.useState(false);
	// Pathname and active role are both browser state. Wait for both so a seller
	// opening a public/legal page never flashes the buyer footer first.
	React.useEffect(() => setMounted(true), []);
	// No role-specific links on focused/delegated flows, but the shared
	// background pause action remains reachable without covering page actions.
	// Decided before the auth gate below: it depends on the pathname alone
	// (both predicates return false for the empty prerender pathname), so
	// reserving a full-height footer here would only be undone a frame later.
	if (isStandaloneFlowPath(pathname) || isDelegatedWorkspacePath(pathname)) {
		return (
			<footer className="site-background-footer">
				<SiteBackgroundToggle variant="page" />
			</footer>
		);
	}
	if (!mounted || !hydrated || !pathname) {
		return (
			<div
				className={`${FOOTER_RESERVE} w-full bg-[var(--brand-text-black)]`}
				aria-hidden="true"
			/>
		);
	}
	if (footerPerspectiveForPathname(pathname, activeRole) === "seller") {
		return <SellerFooter />;
	}
	return <LegacyFooter categoryColumns={categoryColumns} />;
}
