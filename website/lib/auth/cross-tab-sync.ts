import {
	ACTIVE_ROLE_STORAGE_KEY,
	canUsePerspective,
	parseRole,
	type RoleSlot,
} from "./active-role";
import type { AuthStatus } from "./session-state";

export function reduceStorageEvent(
	event: Pick<StorageEvent, "key" | "newValue">,
	account: { isSeller: boolean } | null,
): RoleSlot | null {
	if (event.key !== ACTIVE_ROLE_STORAGE_KEY || account == null) return null;
	const next = parseRole(event.newValue);
	return next && canUsePerspective(account, next) ? next : null;
}

export const SESSION_CHANNEL_NAME = "hk-auth-session";

/** 一個分頁對同源其他分頁宣告它現在看到的帳號：`null` 是訪客。 */
export interface SessionAnnouncement {
	accountId: string | null;
}

export function parseSessionAnnouncement(
	data: unknown,
): SessionAnnouncement | null {
	if (typeof data !== "object" || data === null) return null;
	const accountId = (data as { accountId?: unknown }).accountId;
	if (accountId === null) return { accountId: null };
	if (typeof accountId === "string" && accountId !== "") return { accountId };
	return null;
}

/**
 * 其他分頁宣告的帳號和這個分頁看到的不同，才需要向伺服器重取。訊號只是「去重問」，不帶身分資料：
 * `unknown` 的初值還在路上、本來就會得到答案；`unavailable` 沒有可比的帳號，任何訊號都值得重問。
 */
export function shouldRefreshForPeer(
	announcement: SessionAnnouncement,
	current: { status: AuthStatus; accountId: string | undefined },
): boolean {
	if (current.status === "unknown") return false;
	if (current.status === "unavailable") return true;
	return announcement.accountId !== (current.accountId ?? null);
}

export interface SessionChannel {
	announce: (accountId: string | null) => void;
	close: () => void;
}

export function openSessionChannel(
	onAnnouncement: (announcement: SessionAnnouncement) => void,
): SessionChannel | null {
	if (typeof BroadcastChannel === "undefined") return null;
	const channel = new BroadcastChannel(SESSION_CHANNEL_NAME);
	channel.onmessage = (event: MessageEvent) => {
		const announcement = parseSessionAnnouncement(event.data);
		if (announcement) onAnnouncement(announcement);
	};
	return {
		announce: (accountId) => channel.postMessage({ accountId }),
		close: () => channel.close(),
	};
}
