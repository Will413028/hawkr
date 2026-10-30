/**
 * 顧客 session 的 cookie 名字。權威在單體（`backend/internal/httpapi/rpc/session.go` 的 `SessionCookie`）：
 * `__Host-` 前綴要求 Secure、Path=/、沒有 Domain，值是 opaque token，HttpOnly，所以 JS 讀不到值，只有
 * middleware 與伺服器端（`session-cookie.server.ts`）看得到它在不在。edge-safe（middleware 也 import）。
 */
export const SESSION_COOKIE_NAME = "__Host-hk_session";
