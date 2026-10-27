/**
 * Active-role UI state.
 *
 * active role 是賣家帳號（每個帳號都是買家）目前在看哪個工作區：純粹是 client 的狀態，不是授權。存在
 * localStorage。預設的規則在這裡：帳號有賣家角色而且存的工作區是賣家才是賣家，其他情況是買家。Connect 不讀工作區
 * （服務名稱本身就表示以哪個身分呼叫，ADR 2026-10-07-connect-without-perspective）；只有舊 REST、SSE、AI chat 與
 * chat ws-ticket 以 X-Active-Role（`BUYER`、`SELLER`）帶上，舊 gateway 依它選擇以買家或賣家動作。伺服器不保存
 * （〈API 設計規範（Connect）〉§二）。
 */

export type RoleSlot = "buyer" | "seller";

/** localStorage key for the active workspace shared across tabs. */
export const ACTIVE_ROLE_STORAGE_KEY = "hkui:auth:active-role";

function isBrowser() {
	return typeof window !== "undefined";
}

/** Narrow a raw string to a valid RoleSlot (used for stored + storage-event values). */
export function parseRole(v: unknown): RoleSlot | null {
	return v === "buyer" || v === "seller" ? v : null;
}

/** 舊 REST 的 X-Active-Role 值：BUYER、SELLER。 */
export function roleToHeader(role: RoleSlot | null): string | undefined {
	if (role === "buyer") return "BUYER";
	if (role === "seller") return "SELLER";
	return undefined;
}

/** 帳號能不能用這個工作區：每個帳號都是買家；賣家工作區要有 ROLE_SELLER。 */
export function canUsePerspective(
	account: { isSeller: boolean },
	slot: RoleSlot,
): boolean {
	return slot === "buyer" || account.isSeller;
}

/**
 * 目前的工作區：本機存的工作區帳號還能用時沿用它；沒有存、或帳號已經不是賣家時是買家（每個帳號都是買家）。
 */
export function effectiveActiveRole(
	account: { isSeller: boolean },
	stored: RoleSlot | null = getStoredActiveRole(),
): RoleSlot {
	return stored && canUsePerspective(account, stored) ? stored : "buyer";
}

export function getStoredActiveRole(): RoleSlot | null {
	if (!isBrowser()) return null;
	// ponytail: storage access can throw even when the object exists (Safari private
	// mode, quota) or be a backendless global (Node 22+ test env) — guard, don't detect.
	try {
		return parseRole(localStorage.getItem(ACTIVE_ROLE_STORAGE_KEY));
	} catch {
		return null;
	}
}

export function setStoredActiveRole(role: RoleSlot | null): void {
	if (!isBrowser()) return;
	try {
		if (role) localStorage.setItem(ACTIVE_ROLE_STORAGE_KEY, role);
		else localStorage.removeItem(ACTIVE_ROLE_STORAGE_KEY);
	} catch {
		/* storage unavailable — best-effort, role still applied in-memory */
	}
}
