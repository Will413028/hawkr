import type { LoginProvider } from "@/lib/api/auth";

/**
 * 單體發起登入的入口（`backend/internal/httpapi/identity/weblogin.go`）：整頁導向它，它再把瀏覽器送去
 * Google 或 LINE；callback 設好 `__Host-hk_session` 之後 302 回 `return_to`。不經 Connect，是純 HTTP
 * GET，經同源反代到單體（部署環境的第一方反代、本機的 app/api/auth）。
 */
export const OAUTH_START_PATH = "/api/auth";

/** 登入失敗時單體 302 回登入頁帶的 `?error=` 代碼（weblogin.go 的 Error* 常數）。 */
export const LOGIN_ERROR_CODES: ReadonlySet<string> = new Set([
	"access_denied",
	"invalid_flow",
	"provider_error",
	"login_failed",
	"unavailable",
	"login_required",
]);

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
