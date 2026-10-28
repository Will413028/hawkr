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
 * ping-pong against `RoleZoneGuard required="seller"` on /seller-center
 * (which bounces non-seller views back to "/") and loop forever. Switching
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
	// 賣家操作說明：從賣家 footer 連出去的說明書，必須讓賣家讀得到。
	"/seller-guide",
	"/about",
	"/contact",
	// `/seller/onboarding` 與公開招募頁。
	"/seller",
	// Authentication is an in-flight workspace transition. Login hydrates the
	// new identity immediately before its single hard navigation; trying to
	// confine that one render can race the login owner and hijack a new seller
	// away from `/seller/onboarding`.
	"/login",
	"/register",
	// 賣家批次匯入精靈 → 商城模擬預覽（新 tab 開啟，刻意脫離 seller-center shell
	// 讓賣家用「買家視角」看暫存商品；功能鎖在 previewMode，不會誤觸購買 / 收藏）
	"/seller-product-import-preview",
];

export default function SellerWorkspaceLock() {
	const router = useObservedRouter("seller_workspace_lock");
	const pathname = usePathname() ?? "";
	const { isLoggedIn, activeRole, hydrated } = useAuth();
	// The redirect is otherwise completely silent: the page simply becomes a
	// different page. This component renders nothing visible (it is a sibling
	// guard, not a wrapper), so the announcement is the one thing it CAN add —
	// a screen-reader user is told why they left the page they asked for.
	const [redirecting, setRedirecting] = React.useState(false);

	React.useEffect(() => {
		if (!hydrated) return;
		if (!isLoggedIn || activeRole !== "seller") return;
		// Freshness check, same reason as RoleZoneGuard's: a buyer-zone deep link
		// can have just auto-switched the workspace via switchRole(), which writes
		// localStorage synchronously but commits the context state a beat later.
		// Trusting the stale in-memory "seller" here would hijack that navigation
		// back to the dashboard.
		if (getActiveRole() !== "seller") return;
		if (!pathname) return;
		if (pathname.startsWith("/seller-center")) return;
		// ADM 代行工作區：一個「自己也是賣家、且正以賣家視角瀏覽」的管理員進到
		// /adm 會被這道 lock 導去 /seller-center/dashboard，工作區根本打不開。
		if (isDelegatedWorkspacePath(pathname)) return;
		// 帳號層級的頁面（不屬於任何工作區）：賣家視角也能進。
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
	}, [hydrated, isLoggedIn, activeRole, pathname, router]);

	// Clear once the destination has committed, so a later lock announces again.
	React.useEffect(() => {
		if (pathname.startsWith("/seller-center")) setRedirecting(false);
	}, [pathname]);

	return (
		<span className="visually-hidden" role="status" aria-live="polite">
			{redirecting ? "已為您切換到賣家工作區" : ""}
		</span>
	);
}
