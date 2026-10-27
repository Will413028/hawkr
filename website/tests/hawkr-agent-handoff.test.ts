import assert from "node:assert/strict";
import test from "node:test";
import {
	consumeHawkrAgentHandoff,
	HAWKR_AGENT_HANDOFF_MAX_LENGTH,
	HAWKR_AGENT_HANDOFF_STORAGE_KEY,
	HAWKR_AGENT_HANDOFF_TTL_MS,
	writeHawkrAgentHandoff,
} from "../features/procurement-assistant/handoff";
import { ASSISTANT_PROCUREMENT_PATH } from "../features/procurement-assistant/routes";

class MemoryStorage implements Storage {
	readonly #values = new Map<string, string>();

	get length() {
		return this.#values.size;
	}

	clear() {
		this.#values.clear();
	}

	getItem(key: string) {
		return this.#values.get(key) ?? null;
	}

	key(index: number) {
		return [...this.#values.keys()][index] ?? null;
	}

	removeItem(key: string) {
		this.#values.delete(key);
	}

	setItem(key: string, value: string) {
		this.#values.set(key, value);
	}
}

test("a search draft is consumed exactly once from session storage", () => {
	const storage = new MemoryStorage();
	const now = Date.UTC(2026, 7, 6, 10, 0, 0);

	assert.equal(
		writeHawkrAgentHandoff("  台南水果禮盒  ", "home-search", storage, now),
		true,
	);
	assert.ok(storage.getItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY));

	const consumed = consumeHawkrAgentHandoff(storage, now + 500);
	assert.equal(consumed?.query, "台南水果禮盒");
	assert.equal(consumed?.source, "home-search");
	assert.equal(storage.getItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY), null);
	assert.equal(consumeHawkrAgentHandoff(storage, now + 1_000), null);
});

test("expired and implausibly future-dated handoffs are rejected and removed", () => {
	const now = Date.UTC(2026, 7, 6, 10, 0, 0);
	for (const createdAt of [
		now - HAWKR_AGENT_HANDOFF_TTL_MS - 1,
		now + 60_001,
	]) {
		const storage = new MemoryStorage();
		storage.setItem(
			HAWKR_AGENT_HANDOFF_STORAGE_KEY,
			JSON.stringify({
				version: 1,
				id: "handoff-test",
				query: "過期搜尋",
				source: "search-page",
				createdAt,
			}),
		);

		assert.equal(consumeHawkrAgentHandoff(storage, now), null);
		assert.equal(storage.getItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY), null);
	}
});

test("handoff trims and caps an oversized query before storing it", () => {
	const storage = new MemoryStorage();
	const oversized = `  ${"梨".repeat(HAWKR_AGENT_HANDOFF_MAX_LENGTH + 25)}  `;

	assert.equal(
		writeHawkrAgentHandoff(oversized, "search-empty", storage, 100),
		true,
	);
	const raw = storage.getItem(HAWKR_AGENT_HANDOFF_STORAGE_KEY);
	assert.ok(raw);
	const stored = JSON.parse(raw) as { query: string };
	assert.equal(stored.query.length, HAWKR_AGENT_HANDOFF_MAX_LENGTH);
	assert.equal(
		consumeHawkrAgentHandoff(storage, 100)?.query,
		"梨".repeat(HAWKR_AGENT_HANDOFF_MAX_LENGTH),
	);
});

test("the live chat URL carries no search text or handoff identifier", () => {
	const privateQuery = "公司中秋送禮 預算 50000";
	// 設計書 v6 §V.10.12-4：唯一正式新對話入口是 procurement，legacy 的
	// /assistant/new 已改為 redirect。搜尋草稿要不外洩的是這條 URL。
	const route = new URL(ASSISTANT_PROCUREMENT_PATH, "https://example.com");

	assert.equal(route.pathname, "/account/messages/assistant/procurement");
	assert.equal(route.search, "");
	assert.equal(route.hash, "");
	assert.doesNotMatch(route.href, new RegExp(encodeURIComponent(privateQuery)));
	assert.doesNotMatch(route.href, /handoff|session|query/i);
});
