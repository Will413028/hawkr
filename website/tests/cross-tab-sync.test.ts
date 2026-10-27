import assert from "node:assert/strict";
import { test } from "node:test";
import { ACTIVE_ROLE_STORAGE_KEY } from "../lib/auth/active-role";
import { reduceStorageEvent } from "../lib/auth/cross-tab-sync";

const KEY = ACTIVE_ROLE_STORAGE_KEY;

test("ignores storage events for unrelated keys", () => {
	assert.equal(
		reduceStorageEvent(
			{ key: "other", newValue: "seller" },
			{
				isSeller: true,
			},
		),
		null,
	);
});

test("a guest tab (no identity) ignores every event — LOGIN is not propagated", () => {
	assert.equal(
		reduceStorageEvent({ key: KEY, newValue: "seller" }, null),
		null,
	);
	assert.equal(reduceStorageEvent({ key: KEY, newValue: null }, null), null);
});

test("F1: another tab switched to a role the account holds → sync that role", () => {
	assert.equal(
		reduceStorageEvent({ key: KEY, newValue: "seller" }, { isSeller: true }),
		"seller",
	);
	assert.equal(
		reduceStorageEvent({ key: KEY, newValue: "buyer" }, { isSeller: true }),
		"buyer",
	);
});

test("F1: a switch to a role the account does NOT hold is ignored (never widens authority)", () => {
	// buyer-only account can't be pushed into the seller workspace.
	assert.equal(
		reduceStorageEvent({ key: KEY, newValue: "seller" }, { isSeller: false }),
		null,
	);
});

test("removing the active-role key does not sign out another tab", () => {
	assert.equal(
		reduceStorageEvent({ key: KEY, newValue: null }, { isSeller: true }),
		null,
	);
});

test("localStorage.clear() does not sign out another tab", () => {
	assert.equal(
		reduceStorageEvent({ key: null, newValue: null }, { isSeller: true }),
		null,
	);
});
