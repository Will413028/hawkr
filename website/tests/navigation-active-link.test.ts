import assert from "node:assert/strict";
import test from "node:test";
import { isNavPathActive } from "../lib/navigation/active-link";

test("matches a pathname to a navigation href with query parameters", () => {
	assert.equal(isNavPathActive("/login", "/login?role=buyer"), true);
	assert.equal(isNavPathActive("/login", "/"), false);
});

test("matches descendants without making the home link active everywhere", () => {
	assert.equal(isNavPathActive("/account/addresses", "/account"), true);
	assert.equal(isNavPathActive("/market", "/"), false);
});
