import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

// Minimal in-memory Web Storage stub.
function makeStorage() {
	const store: Record<string, string> = {};
	return {
		store,
		getItem: (k: string) => (k in store ? store[k] : null),
		setItem: (k: string, v: string) => {
			store[k] = String(v);
		},
		removeItem: (k: string) => {
			delete store[k];
		},
	};
}

let ls = makeStorage();
let ss = makeStorage();

beforeEach(() => {
	ls = makeStorage();
	ss = makeStorage();
	(globalThis as { window?: unknown }).window = {
		localStorage: ls,
		sessionStorage: ss,
	};
});

afterEach(() => {
	delete (globalThis as { window?: unknown }).window;
});

const LS_KEY = "hawkr:checkout:draft";
const SS_KEY = "hawkr:checkout:draft-pii";

function baseDraft() {
	return {
		version: 2 as const,
		createdAt: 1,
		validateAt: 1,
		seller: { key: "s1", name: "Seller" },
		selectedSkuIds: ["sku1"],
		shipping: { method: "home", deliveryTemp: "normal", label: "宅配", fee: 0 },
		note: "",
	};
}

test("writeCheckoutDraft keeps recipient PII out of localStorage", async () => {
	const { writeCheckoutDraft } = await import("../lib/checkout/draft");
	writeCheckoutDraft({
		...baseDraft(),
		recipient: { name: "A", phone: "0912000000", line: "台北市…" },
	});
	const lsRaw = ls.store[LS_KEY];
	assert.ok(lsRaw, "base draft written to localStorage");
	assert.equal(
		lsRaw.includes("0912000000"),
		false,
		"phone must NOT be persisted in localStorage",
	);
	assert.ok(
		ss.store[SS_KEY]?.includes("0912000000"),
		"phone in sessionStorage",
	);
});

test("readCheckoutDraft merges localStorage base with sessionStorage PII", async () => {
	const { writeCheckoutDraft, readCheckoutDraft } = await import(
		"../lib/checkout/draft"
	);
	writeCheckoutDraft({
		...baseDraft(),
		recipient: { name: "A", phone: "0912000000", line: "台北市…" },
	});
	const got = readCheckoutDraft();
	assert.equal(got?.recipient?.phone, "0912000000");
	assert.deepEqual(got?.selectedSkuIds, ["sku1"]);
});

test("clearCheckoutDraft removes both the base and the PII", async () => {
	const { writeCheckoutDraft, clearCheckoutDraft } = await import(
		"../lib/checkout/draft"
	);
	writeCheckoutDraft({
		...baseDraft(),
		recipient: { name: "A", phone: "0912000000", line: "台北市…" },
	});
	clearCheckoutDraft();
	assert.equal(LS_KEY in ls.store, false);
	assert.equal(SS_KEY in ss.store, false);
});

test("readCheckoutDraft falls back to inline PII from a pre-split draft", async () => {
	const { readCheckoutDraft } = await import("../lib/checkout/draft");
	// Old draft written before the split: PII still inline in localStorage.
	ls.store[LS_KEY] = JSON.stringify({
		...baseDraft(),
		recipient: { name: "Old", phone: "0900111222", line: "高雄市…" },
	});
	const got = readCheckoutDraft();
	assert.equal(got?.recipient?.phone, "0900111222");
});
