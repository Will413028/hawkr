import { STALE_CHUNK_RELOAD_MARKER_KEY } from "@/lib/loading/stale-chunk-reload";

const DRAFT_KEY = "hk:home-search:technical-reload";
const DRAFT_TTL_MS = 60_000;
type DraftStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

/** Only checkpoint during the existing stale-chunk self-heal, never normal navigation. */
export function saveSearchDraftForTechnicalReload(
	storage: DraftStorage,
	path: string,
	draft: { query: string; open: boolean },
	now = Date.now(),
	scope = "navbar",
) {
	try {
		const reloadAt = Number(storage.getItem(STALE_CHUNK_RELOAD_MARKER_KEY));
		if (!reloadAt || now < reloadAt || now - reloadAt > DRAFT_TTL_MS) return;
		if (!draft.query || draft.query.length > 4_096) return;
		storage.setItem(
			`${DRAFT_KEY}:${scope}`,
			JSON.stringify({ ...draft, path, savedAt: now }),
		);
	} catch {
		// Storage may be blocked. Never obstruct the existing recovery mechanism.
	}
}

/** One shot, same URL, one minute. No account data and no durable search history. */
export function consumeSearchDraftAfterTechnicalReload(
	storage: DraftStorage,
	path: string,
	isTechnicalReload: boolean,
	now = Date.now(),
	scope = "navbar",
): { query: string; open: boolean } | null {
	try {
		const raw = storage.getItem(`${DRAFT_KEY}:${scope}`);
		storage.removeItem(`${DRAFT_KEY}:${scope}`);
		if (!raw || !isTechnicalReload) return null;
		const draft = JSON.parse(raw);
		if (
			draft?.path !== path ||
			typeof draft.query !== "string" ||
			!draft.query ||
			draft.query.length > 4_096 ||
			typeof draft.open !== "boolean" ||
			!Number.isFinite(draft.savedAt) ||
			now < draft.savedAt ||
			now - draft.savedAt > DRAFT_TTL_MS
		)
			return null;
		return { query: draft.query, open: draft.open };
	} catch {
		return null;
	}
}
