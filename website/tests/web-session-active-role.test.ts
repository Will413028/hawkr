import assert from "node:assert/strict";
import { test } from "node:test";
import {
	canUsePerspective,
	effectiveActiveRole,
	roleToHeader,
} from "../lib/auth/active-role";

test("roleToHeader maps slot to the X-Active-Role value (BUYER / SELLER)", () => {
	assert.equal(roleToHeader("buyer"), "BUYER");
	assert.equal(roleToHeader("seller"), "SELLER");
	assert.equal(roleToHeader(null), undefined);
});

test("every account can use the buyer perspective; seller needs ROLE_SELLER", () => {
	assert.equal(canUsePerspective({ isSeller: true }, "buyer"), true);
	assert.equal(canUsePerspective({ isSeller: true }, "seller"), true);
	assert.equal(canUsePerspective({ isSeller: false }, "buyer"), true);
	assert.equal(canUsePerspective({ isSeller: false }, "seller"), false);
});

test("effectiveActiveRole: a seller with a stored seller workspace is seller; no seller role or nothing stored is buyer", () => {
	const seller = { isSeller: true };
	assert.equal(effectiveActiveRole(seller, "seller"), "seller");
	assert.equal(effectiveActiveRole(seller, "buyer"), "buyer");
	assert.equal(effectiveActiveRole(seller, null), "buyer");
	const buyer = { isSeller: false };
	assert.equal(effectiveActiveRole(buyer, "seller"), "buyer");
	assert.equal(effectiveActiveRole(buyer, "buyer"), "buyer");
	assert.equal(effectiveActiveRole(buyer, null), "buyer");
});
