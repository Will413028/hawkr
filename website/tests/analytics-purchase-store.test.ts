import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import {
	clearPendingPurchase,
	markPurchaseTracked,
	readPendingPurchase,
	setPendingPurchase,
} from "../lib/analytics/purchase-store";

function installSessionStorageStub(): void {
	const store = new Map<string, string>();
	(globalThis as { sessionStorage?: unknown }).sessionStorage = {
		getItem(key: string): string | null {
			return store.has(key) ? (store.get(key) as string) : null;
		},
		setItem(key: string, value: string): void {
			store.set(key, value);
		},
		removeItem(key: string): void {
			store.delete(key);
		},
	};
}

beforeEach(() => {
	installSessionStorageStub();
});

afterEach(() => {
	delete (globalThis as { sessionStorage?: unknown }).sessionStorage;
});

test("setPendingPurchase + readPendingPurchase round-trips orderNos", () => {
	setPendingPurchase(["ORD-1", "ORD-2"]);
	assert.deepEqual(readPendingPurchase(), ["ORD-1", "ORD-2"]);
});

test("readPendingPurchase stringifies non-string entries", () => {
	setPendingPurchase([123 as unknown as string, "ORD-2"]);
	assert.deepEqual(readPendingPurchase(), ["123", "ORD-2"]);
});

test("readPendingPurchase returns [] when nothing is set", () => {
	assert.deepEqual(readPendingPurchase(), []);
});

test("markPurchaseTracked returns true first, false on repeat (core dedup)", () => {
	assert.equal(markPurchaseTracked("ORD-1"), true);
	assert.equal(markPurchaseTracked("ORD-1"), false);
});

test("markPurchaseTracked returns true for a different orderNo", () => {
	assert.equal(markPurchaseTracked("ORD-1"), true);
	assert.equal(markPurchaseTracked("ORD-2"), true);
});

test("clearPendingPurchase empties the pending list", () => {
	setPendingPurchase(["ORD-1"]);
	assert.deepEqual(readPendingPurchase(), ["ORD-1"]);
	clearPendingPurchase();
	assert.deepEqual(readPendingPurchase(), []);
});
