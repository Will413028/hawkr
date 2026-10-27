import assert from "node:assert/strict";
import test from "node:test";
import {
	galleryIndexAfterSwipe,
	PRODUCT_GALLERY_SWIPE_THRESHOLD,
} from "../lib/product/gallery-swipe";

test("product gallery uses the same 48px snap threshold as the home carousel", () => {
	assert.equal(PRODUCT_GALLERY_SWIPE_THRESHOLD, 48);
	assert.equal(galleryIndexAfterSwipe(0, 3, -47), 0);
	assert.equal(galleryIndexAfterSwipe(0, 3, -48), 1);
	assert.equal(galleryIndexAfterSwipe(1, 3, 48), 0);
});

test("product gallery swipe clamps at both image edges", () => {
	assert.equal(galleryIndexAfterSwipe(0, 3, 120), 0);
	assert.equal(galleryIndexAfterSwipe(2, 3, -120), 2);
	assert.equal(galleryIndexAfterSwipe(0, 1, -120), 0);
});
