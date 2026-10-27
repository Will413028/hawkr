import assert from "node:assert/strict";
import test from "node:test";
import {
	consumeSearchDraftAfterTechnicalReload,
	saveSearchDraftForTechnicalReload,
} from "../features/home/components/HomeCompactSearchRecovery";
import { STALE_CHUNK_RELOAD_MARKER_KEY } from "../lib/loading/stale-chunk-reload";

function storage() {
	const values = new Map<string, string>();
	return {
		getItem: (key: string) => values.get(key) ?? null,
		setItem: (key: string, value: string) => {
			values.set(key, value);
		},
		removeItem: (key: string) => {
			values.delete(key);
		},
	};
}

test("technical reload restores a same-page draft once without creating normal search history", () => {
	const cache = storage();
	const draft = { query: "小白菜", open: true };
	saveSearchDraftForTechnicalReload(cache, "/", draft, 2_000);
	assert.equal(
		consumeSearchDraftAfterTechnicalReload(cache, "/", true, 2_000),
		null,
	);
	cache.setItem(STALE_CHUNK_RELOAD_MARKER_KEY, "2000");
	saveSearchDraftForTechnicalReload(cache, "/", draft, 2_100);
	assert.deepEqual(
		consumeSearchDraftAfterTechnicalReload(cache, "/", true, 2_200),
		draft,
	);
	assert.equal(
		consumeSearchDraftAfterTechnicalReload(cache, "/", true, 2_200),
		null,
	);
});

test("expired, wrong-page, normal navigation and unavailable storage cannot restore a draft", () => {
	for (const [path, technical, now] of [
		["/?other=1", true, 3_000],
		["/", false, 3_000],
		["/", true, 63_000],
	] as const) {
		const cache = storage();
		cache.setItem(STALE_CHUNK_RELOAD_MARKER_KEY, "2000");
		saveSearchDraftForTechnicalReload(
			cache,
			"/",
			{ query: "小白菜", open: false },
			2_100,
		);
		assert.equal(
			consumeSearchDraftAfterTechnicalReload(cache, path, technical, now),
			null,
		);
	}
	const blocked = {
		getItem() {
			throw new Error("blocked");
		},
		setItem() {
			throw new Error("blocked");
		},
		removeItem() {
			throw new Error("blocked");
		},
	};
	assert.doesNotThrow(() =>
		saveSearchDraftForTechnicalReload(blocked, "/", {
			query: "小白菜",
			open: true,
		}),
	);
	assert.equal(
		consumeSearchDraftAfterTechnicalReload(blocked, "/", true),
		null,
	);
});

test("a hidden navbar instance cannot consume the member mobile search checkpoint", () => {
	const cache = storage();
	cache.setItem(STALE_CHUNK_RELOAD_MARKER_KEY, "2000");
	const draft = { query: "手機的小白菜", open: true };
	saveSearchDraftForTechnicalReload(cache, "/", draft, 2_100, "member-mobile");
	assert.equal(
		consumeSearchDraftAfterTechnicalReload(cache, "/", true, 2_200),
		null,
	);
	assert.deepEqual(
		consumeSearchDraftAfterTechnicalReload(
			cache,
			"/",
			true,
			2_200,
			"member-mobile",
		),
		draft,
	);
});
