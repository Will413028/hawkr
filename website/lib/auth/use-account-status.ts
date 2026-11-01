"use client";

import { useAuth } from "./context";
import type { AuthStatus } from "./session-state";
import { useOptionalZone } from "./zone-context";

/**
 * 帳號查詢用的登入狀態。受保護區內伺服器已確認 `signedIn`（`ZoneIdentityProvider`），所以區內查詢不被根層
 * 可能過期的答案擋住；區外（公開頁）是根層的 `status`。
 */
export function useAccountStatus(): AuthStatus {
	const zone = useOptionalZone();
	const { status } = useAuth();
	return zone ? "signedIn" : status;
}
