import type { LoginProvider } from "@/lib/api/auth";

/**
 * 單體發起登入的入口（`backend/internal/httpapi/identity/weblogin.go`）：整頁導向它，它再把瀏覽器送去
 * Google 或 LINE；callback 設好 `__Host-hk_session` 之後 302 回 `return_to`。不經 Connect，是純 HTTP
 * GET，經同源反代到單體（部署環境的第一方反代、本機的 app/api/auth）。
 */
export const OAUTH_START_PATH = "/api/auth";

/** 登入錯誤文字的 key（`messages/*.json` 的 `auth.error.*`）。 */
export type LoginErrorKey = "accessDenied" | "unavailable" | "failed";

/**
 * 登入失敗時單體 302 回來帶的錯誤代碼（weblogin.go 的 Error* 常數）對到文字 key：取消授權與服務暫時不能用各有一句，
 * 其他認得的代碼共用「登入失敗」；不認得的代碼是 null（不顯示）。登入頁的 `?error=` 與連結區的 `?link_error=` 共用。
 */
const LOGIN_ERROR_KEYS: Readonly<Record<string, LoginErrorKey>> = {
	access_denied: "accessDenied",
	unavailable: "unavailable",
	invalid_flow: "failed",
	provider_error: "failed",
	login_failed: "failed",
	login_required: "failed",
};

export function loginErrorKey(
	code: string | null | undefined,
): LoginErrorKey | null {
	if (!code || !Object.hasOwn(LOGIN_ERROR_KEYS, code)) return null;
	return LOGIN_ERROR_KEYS[code];
}

/** 連結錯誤文字的 key：`auth.error.*`，或連結才有的 `auth.linkError.alreadyLinked`。 */
export type LinkErrorKey =
	| { ns: "auth.error"; key: LoginErrorKey }
	| { ns: "auth.linkError"; key: "alreadyLinked" };

/** 連結流程（`link=1`）失敗時 302 回 return_to 帶的 `?link_error=` 代碼；不認得的是 null。 */
export function linkErrorKey(
	code: string | null | undefined,
): LinkErrorKey | null {
	if (code === "identity_already_linked") {
		return { ns: "auth.linkError", key: "alreadyLinked" };
	}
	const key = loginErrorKey(code);
	return key ? { ns: "auth.error", key } : null;
}

/**
 * 已登入的帳號連結另一家的入口：`GET /api/auth/{provider}/start?link=1&return_to=…`（整頁導覽）。`returnTo` 是
 * 成功或失敗後單體 302 回來的站內路徑（含語系）。
 */
export function linkStartUrl(
	provider: LoginProvider,
	returnTo: string,
): string {
	return `${OAUTH_START_PATH}/${provider}/start?${new URLSearchParams({ link: "1", return_to: returnTo })}`;
}

/**
 * `GET /api/auth/{provider}/start?return_to=…` 的網址。`returnTo` 是登入成功後單體 302 回來的站內
 * 路徑（登入頁自己，帶著工作區、目的地與登入方式；單體只收 `/` 開頭且非 `//` 的相對路徑）。
 */
export function oauthStartUrl(
	provider: LoginProvider,
	returnTo: string,
): string {
	return `${OAUTH_START_PATH}/${provider}/start?${new URLSearchParams({ return_to: returnTo })}`;
}

/** 登入頁網址上的登入方式（登入成功回來時用來記 `login` 事件）；不認得的值是 null。 */
export function parseLoginProvider(
	value: string | null | undefined,
): LoginProvider | null {
	return value === "google" || value === "line" ? value : null;
}
