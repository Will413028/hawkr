import type { LoginProvider } from "@/lib/api/auth";
import type { RoleSlot } from "./active-role";

/** Clerk 完成 OAuth 之後回到的頁面（features/auth/screens/login/SsoCallbackPage.tsx）。 */
export const SSO_CALLBACK_PATH = "/login/sso-callback";

/** 登入失敗時給使用者看的文字。 */
export const LOGIN_FAILED_MESSAGE = "登入失敗，請稍後再試";

/** SSO callback 的網址，帶著登入頁的脈絡：選的工作區、登入後要去的地方、登入方式。 */
export function ssoCallbackPath(params: {
	role: RoleSlot;
	redirect: string;
	provider: LoginProvider | null;
}): string {
	const qs = new URLSearchParams({ role: params.role });
	if (params.redirect) qs.set("redirect", params.redirect);
	if (params.provider) qs.set("provider", params.provider);
	return `${SSO_CALLBACK_PATH}?${qs}`;
}

/** callback 網址上的登入方式；不認得的值是 null。 */
export function parseLoginProvider(
	value: string | null | undefined,
): LoginProvider | null {
	return value === "google" || value === "apple" || value === "line"
		? value
		: null;
}
