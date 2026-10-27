import assert from "node:assert/strict";
import { test } from "node:test";
import { create } from "@bufbuild/protobuf";
import { FavoriteSchema } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import {
	displayableFavorites,
	isDisplayableFavorite,
} from "../lib/favorites/visible";

function favorite(id: string, product: Record<string, unknown> = {}) {
	return create(FavoriteSchema, {
		product: {
			id,
			name: "彰化溪州珍珠芭樂",
			image: { uriTemplate: "https://media.example/i/{preset}/product.png" },
			available: true,
			...product,
		},
	});
}

test("only active products with a name and image appear in the overview rail", () => {
	assert.equal(isDisplayableFavorite(favorite("1")), true);
	assert.equal(isDisplayableFavorite(favorite("2", { deleted: true })), false);
	assert.equal(
		isDisplayableFavorite(favorite("3", { available: false })),
		false,
	);
	assert.equal(isDisplayableFavorite(favorite("4", { name: " " })), false);
	assert.equal(
		isDisplayableFavorite(favorite("5", { image: undefined })),
		false,
	);
});

test("the overview count matches its rendered list", () => {
	const items = [
		favorite("1"),
		favorite("2", { deleted: true }),
		favorite("3"),
	];
	assert.deepEqual(
		displayableFavorites(items).map((item) => item.product?.id),
		["1", "3"],
	);
});
