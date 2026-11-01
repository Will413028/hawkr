import { type AppLocale, defaultLocale } from "@/lib/intl/locales";
import { splitLocale } from "@/lib/intl/paths";
import { effectiveActiveRole } from "./active-role";
import {
	ACCESS_CLASSES,
	defaultPathForRole,
	loginPath,
	type RequiredAccess,
	safeRedirectPath,
} from "./redirect";
import type { SessionState } from "./session-state";

export type ZoneDecision =
	| { kind: "allow" }
	| { kind: "unavailable" }
	| { kind: "redirect"; href: string; locale: AppLocale };

/**
 * 受保護區（`/account`、`/checkout`、`/seller-center`、`/seller-product-import-preview`、`/seller/measures`）
 * 的存取判定，只看帳號層級：伺服器知道的是「有沒有登入、帳號有沒有賣家角色」；買家或賣家工作區是 client 的
 * 狀態（`lib/auth/active-role.ts`），不在這裡判斷。授權的唯一執行點仍是單體（`withSession`），這裡只決定導向。
 *
 * - 訪客（沒有 cookie，或 cookie 被單體拒絕）：登入頁，帶工作區與回來的網址，與 middleware 的導向同一個 URL。
 * - 登入了但沒有這區要的角色（非賣家進賣家區）：帳號目前工作區的預設頁（`defaultPathForRole`，舊 `roleZoneFallbackPath` 的伺服器端對應）。
 *
 * 導向目標是不含語系前綴的 `href` 與 `locale`，由 `@/lib/intl/navigation` 的 `redirect` 加前綴。
 * - 單體暫時答不出來：不導向（不是登出），由呼叫端顯示「暫時無法確認」。
 *
 * `returnPath` 是 middleware 帶來的原始請求 URL（含語系前綴與 query）。
 */
export function decideZone(
	required: RequiredAccess,
	session: SessionState,
	returnPath: string | null,
): ZoneDecision {
	const safeReturn = safeRedirectPath(returnPath);
	const locale =
		splitLocale(safeReturn.split(/[?#]/)[0] ?? "").locale ?? defaultLocale;
	if (session.status === "unavailable") return { kind: "unavailable" };
	const { workspace, owns } = ACCESS_CLASSES[required];
	if (session.status === "guest") {
		return {
			kind: "redirect",
			href: loginPath({
				...(workspace ? { role: workspace } : {}),
				...(safeReturn ? { redirect: safeReturn } : {}),
			}),
			locale,
		};
	}
	const allowed = owns({
		isLoggedIn: true,
		hasSeller: session.identity.isSeller,
	});
	if (allowed) return { kind: "allow" };
	return {
		kind: "redirect",
		href: defaultPathForRole(effectiveActiveRole(session.identity, null)),
		locale,
	};
}
