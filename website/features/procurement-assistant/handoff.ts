import { LOCAL_PRIVATE_KEYS } from "@/lib/auth/local-private-keys";
import { registerLocalPrivateReset } from "@/lib/auth/local-private-resets";

/**
 * 帳號私有（裝著使用者輸入的搜尋字串），但登記為
 * `clearOn: "leaving-a-signed-in-owner"`——它的用途就是把草稿帶過「訪客 → 登入」
 * 進到受保護的 Agent 路由，所以那道轉換不清；登出／換帳號才清。
 */
export const HAWKR_AGENT_HANDOFF_STORAGE_KEY =
	LOCAL_PRIVATE_KEYS.hawkrAgentHandoff.key;
export const HAWKR_AGENT_HANDOFF_MAX_LENGTH = 500;
export const HAWKR_AGENT_HANDOFF_TTL_MS = 60 * 60 * 1000;

export type HawkrAgentHandoffSource =
	| "home-search"
	| "search-page"
	| "search-empty";

export interface HawkrAgentHandoff {
	version: 1;
	id: string;
	query: string;
	source: HawkrAgentHandoffSource;
	createdAt: number;
}

let memoryHandoff: HawkrAgentHandoff | null = null;

// sessionStorage 之外還有一份記憶體副本（storage 被擋時的 fallback），清除
// storage 不會動到它——登出／換帳號時若只清 storage，consume() 仍會吐出上一位
// 的搜尋字串。同 zustand store 的理由，一併登記進 reset 註冊表。
registerLocalPrivateReset(HAWKR_AGENT_HANDOFF_STORAGE_KEY, () => {
	memoryHandoff = null;
});

function browserSessionStorage(): Storage | null {
	if (typeof window === "undefined") return null;
	try {
		return window.sessionStorage;
	} catch {
		return null;
	}
}

function handoffId(): string {
	try {
		return globalThis.crypto.randomUUID();
	} catch {
		return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
	}
}

function parseHandoff(
	raw: string | null,
	now: number,
): HawkrAgentHandoff | null {
	if (!raw) return null;
	try {
		const value = JSON.parse(raw) as Partial<HawkrAgentHandoff>;
		if (
			value.version !== 1 ||
			typeof value.id !== "string" ||
			typeof value.query !== "string" ||
			(value.source !== "home-search" &&
				value.source !== "search-page" &&
				value.source !== "search-empty") ||
			typeof value.createdAt !== "number" ||
			!Number.isFinite(value.createdAt) ||
			now - value.createdAt > HAWKR_AGENT_HANDOFF_TTL_MS ||
			value.createdAt - now > 60_000
		) {
			return null;
		}
		const query = value.query.trim().slice(0, HAWKR_AGENT_HANDOFF_MAX_LENGTH);
		if (!query) return null;
		return { ...value, query } as HawkrAgentHandoff;
	} catch {
		return null;
	}
}

/**
 * Carries a search draft into the protected Agent route without putting the
 * user's words in the URL, navigation telemetry, referrer, or server logs.
 */
export function writeHawkrAgentHandoff(
	query: string,
	source: HawkrAgentHandoffSource,
	storage: Storage | null = browserSessionStorage(),
	now = Date.now(),
): boolean {
	const normalized = query.trim().slice(0, HAWKR_AGENT_HANDOFF_MAX_LENGTH);
	if (!normalized) return false;
	const handoff: HawkrAgentHandoff = {
		version: 1,
		id: handoffId(),
		query: normalized,
		source,
		createdAt: now,
	};
	memoryHandoff = handoff;
	try {
		storage?.setItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY, JSON.stringify(handoff));
	} catch {
		// Same-document navigation still has the in-memory fallback. If login causes
		// a full reload and storage is unavailable, the chat opens with a blank draft.
	}
	return true;
}

export function consumeHawkrAgentHandoff(
	storage: Storage | null = browserSessionStorage(),
	now = Date.now(),
): HawkrAgentHandoff | null {
	let stored: HawkrAgentHandoff | null = null;
	try {
		stored = parseHandoff(
			storage?.getItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY) ?? null,
			now,
		);
		storage?.removeItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY);
	} catch {
		// Fall through to the in-memory copy.
	}
	const handoff = stored ?? memoryHandoff;
	memoryHandoff = null;
	if (!handoff) return null;
	return parseHandoff(JSON.stringify(handoff), now);
}
