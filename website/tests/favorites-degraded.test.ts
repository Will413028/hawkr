import assert from "node:assert/strict";
import { test } from "node:test";
import { create } from "@bufbuild/protobuf";
import { FavoriteSchema } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import {
	browsableFavorites,
	isBrowsableFavorite,
} from "../lib/favorites/visible";

test("deleted products remain in the full list for the existing unavailable card", () => {
	const deleted = create(FavoriteSchema, {
		product: { id: "841713639471484333", deleted: true },
	});
	assert.equal(isBrowsableFavorite(deleted), true);
	assert.deepEqual(browsableFavorites([deleted]), [deleted]);
	assert.equal(deleted.product?.deleted, true);
});

test("a product without an ID cannot link to a detail page", () => {
	const missing = create(FavoriteSchema, { product: { deleted: true } });
	assert.equal(isBrowsableFavorite(missing), false);
});

test("available products remain browsable", () => {
	const available = create(FavoriteSchema, {
		product: {
			id: "841713639471484333",
			name: "彰化溪州珍珠芭樂",
			available: true,
		},
	});
	assert.equal(isBrowsableFavorite(available), true);
});
