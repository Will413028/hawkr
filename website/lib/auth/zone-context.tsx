"use client";

import * as React from "react";
import type { Identity, RoleSlot } from "./storage";

interface ZoneValue {
	identity: Identity;
	workspace: RoleSlot | null;
}

/** 根層 `AuthProvider` 提供的計數：目前掛著幾個受保護區。區內有伺服器確認的身分，401 要清它，即使根層的舊答案不是 `signedIn`。 */
export const ZoneMountRegistryContext =
	React.createContext<React.MutableRefObject<number> | null>(null);

const ZoneContext = React.createContext<ZoneValue | null>(null);

/**
 * 受保護區的身分來源：`ZoneGate` 在 `requireZone` 通過後，把這個請求的 `verifySession()` 結果與區的工作區
 * 當 prop 交給它（與 Auth.js `SessionProvider` 的 `session` prop 同型）。client 不從根層 promise 推導，
 * 所以軟導覽進區時根層的舊答案（例如載入頁面時的 `unavailable`）不影響區內。
 */
export function ZoneIdentityProvider({
	identity,
	workspace,
	children,
}: ZoneValue & { children: React.ReactNode }) {
	const value = React.useMemo(
		() => ({ identity, workspace }),
		[identity, workspace],
	);
	const registry = React.useContext(ZoneMountRegistryContext);
	React.useEffect(() => {
		if (!registry) return;
		registry.current += 1;
		return () => {
			registry.current -= 1;
		};
	}, [registry]);
	return <ZoneContext.Provider value={value}>{children}</ZoneContext.Provider>;
}

/** 在區內是該區的身分與工作區，區外是 null。 */
export function useOptionalZone(): ZoneValue | null {
	return React.useContext(ZoneContext);
}
