"use client";

import * as React from "react";
import { useAuth } from "@/lib/auth/context";
import { ACCESS_CLASSES, requiredAccessForPath } from "@/lib/auth/redirect";
import { getActiveRole } from "@/lib/auth/storage";
import { usePathname } from "@/lib/intl/navigation";
import { isDelegatedWorkspacePath } from "@/lib/navigation/delegated-workspace";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/**
 * While a user is VIEWING the seller workspace, confine them to
 * /seller-center/*; anywhere else (buyer storefront, /account, /products…)
 * redirect to the dashboard — a guardrail so the seller workspace stays
 * self-contained.
 *
 * Keys off the active-role UI state (`activeRole === "seller"`), NOT the
 * account's role SET (`hasSeller`). A both-role account viewing as a buyer
 * (activeRole "buyer", the default) must NOT be locked here: it would
 * ping-pong against `WorkspaceAlign` on /seller-center (which bounces
 * non-seller views back to the default page) and loop forever. Switching
 * workspace is a client-side activeRole toggle, no re-login.
 *
 * Skips /seller-center itself (already there).
 *
 * Also skips a set of universal informational pages (legal, contact, help)
 * reachable from the global Footer: confining sellers to the storefront and
 * account flows is the point, but they must still be able to read the privacy
 * policy / terms and contact support from anywhere.
 */
const PUBLIC_ALLOWLIST = [
	"/privacy",
	"/terms",
	"/user-policy",
	"/return-policy",
	"/seller-terms",
	"/faq",
	"/seller-guide",
	"/about",
	"/contact",
	"/seller",
	"/login",
	"/register",
];

export default function SellerWorkspaceLock() {
	const router = useObservedRouter("seller_workspace_lock");
	const pathname = usePathname() ?? "";
	const { activeRole, status } = useAuth();
	const [redirecting, setRedirecting] = React.useState(false);

	React.useEffect(() => {
		if (status === "unknown") return;
		if (status !== "signedIn" || activeRole !== "seller") return;
		if (getActiveRole() !== "seller") return;
		if (!pathname) return;
		if (pathname.startsWith("/seller-center")) return;
		if (isDelegatedWorkspacePath(pathname)) return;
		const access = requiredAccessForPath(pathname);
		if (access && ACCESS_CLASSES[access].workspace === null) return;
		if (
			PUBLIC_ALLOWLIST.some(
				(p) => pathname === p || pathname.startsWith(`${p}/`),
			)
		)
			return;
		setRedirecting(true);
		router.replace("/seller-center/dashboard");
	}, [status, activeRole, pathname, router]);

	React.useEffect(() => {
		if (pathname.startsWith("/seller-center")) setRedirecting(false);
	}, [pathname]);

	return (
		<span className="visually-hidden" role="status" aria-live="polite">
			{redirecting ? "已為您切換到賣家工作區" : ""}
		</span>
	);
}
