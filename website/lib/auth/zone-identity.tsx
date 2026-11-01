"use client";

import type { Identity } from "./storage";
import { useOptionalZone } from "./zone-context";

/**
 * 受保護區內的身分：值就是 `ZoneGate` 交給 `ZoneIdentityProvider` 的、這個請求由伺服器確認過的身分，
 * 第一次 render 就有、型別上非 null，不經過 `unknown`，也不讀根層 promise。
 * 只讀身分；`signOut`、跨分頁與 401 仍由根層的 `useAuth()` 負責，`activeRole` 是 client 狀態，也留在 `useAuth()`。
 * 在 zone 外呼叫會丟錯。
 */
export function useZoneIdentity(): Identity {
	const zone = useOptionalZone();
	if (!zone) {
		throw new Error("useZoneIdentity must be used inside a <ZoneGate> zone");
	}
	return zone.identity;
}
