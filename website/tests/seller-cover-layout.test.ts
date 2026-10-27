import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
	calculateContainRect,
	calculateCoverCrop,
	SELLER_COVER_ASPECT,
	SELLER_COVER_BACKGROUND_PRESETS,
	SELLER_COVER_DEFAULT_BACKGROUND,
	SELLER_COVER_OUTPUT,
	shouldContainSellerCover,
} from "../lib/media/seller-cover";

test("seller cover presets stay inside the approved brand palette", () => {
	assert.deepEqual(
		SELLER_COVER_BACKGROUND_PRESETS.map(({ color }) => color),
		["#FFFDF7", "#FDF1E1", "#B8441F", "#E4572E", "#F2A541", "#9FCCDD"],
	);
	assert.equal(SELLER_COVER_DEFAULT_BACKGROUND, "#FDF1E1");
});

test("complete-image mode centers a square upload inside the canonical cover", () => {
	const rect = calculateContainRect(
		4096,
		4096,
		SELLER_COVER_OUTPUT.width,
		SELLER_COVER_OUTPUT.height,
	);

	assert.deepEqual(rect, {
		x: 600,
		y: 0,
		width: 400,
		height: 400,
	});
});

test("cover mode trims a square upload vertically around its centre", () => {
	const crop = calculateCoverCrop(
		4096,
		4096,
		SELLER_COVER_OUTPUT.width / SELLER_COVER_OUTPUT.height,
	);

	assert.equal(crop.x, 0);
	assert.equal(crop.width, 4096);
	assert.equal(crop.height, 1024);
	assert.equal(crop.y, 1536);
});

test("seller cover output and every device preview share one 4:1 aspect", () => {
	assert.equal(SELLER_COVER_ASPECT, 4);
	assert.equal(
		SELLER_COVER_OUTPUT.width / SELLER_COVER_OUTPUT.height,
		SELLER_COVER_ASPECT,
	);
	const desktop = calculateCoverCrop(1600, 400, SELLER_COVER_ASPECT);
	const mobile = calculateCoverCrop(1600, 400, SELLER_COVER_ASPECT);

	assert.deepEqual(desktop, { x: 0, y: 0, width: 1600, height: 400 });
	assert.deepEqual(mobile, desktop);
});

test("transparent or strongly mismatched covers default to complete-image mode", () => {
	assert.equal(shouldContainSellerCover(4, true), true);
	assert.equal(shouldContainSellerCover(1, false), true);
	assert.equal(shouldContainSellerCover(16 / 9, false), true);
	assert.equal(shouldContainSellerCover(4, false), false);
	assert.equal(shouldContainSellerCover(3.2, false), false);
});

test("public seller cover and loading skeleton keep 4:1 at every breakpoint", () => {
	const sellerStyles = readFileSync(
		new URL("../styles/features/seller-profile.css", import.meta.url),
		"utf8",
	);
	const loadingStyles = readFileSync(
		new URL("../styles/components/route-loading-skeleton.css", import.meta.url),
		"utf8",
	);
	const uploader = readFileSync(
		new URL("../components/ui/ImageUploader.tsx", import.meta.url),
		"utf8",
	);

	assert.match(
		sellerStyles,
		/\.hkui-seller-profile__cover \{[^}]*aspect-ratio: 4 \/ 1;/s,
	);
	assert.doesNotMatch(
		sellerStyles,
		/\.hkui-seller-profile__cover \{[^}]*height: 144px;/s,
	);
	assert.match(
		loadingStyles,
		/\.route-loading-skeleton__store-cover \{[^}]*aspect-ratio: 4 \/ 1;/s,
	);
	assert.doesNotMatch(
		loadingStyles,
		/\.route-loading-skeleton__store-cover \{[^}]*height: 144px;/s,
	);
	assert.match(
		uploader,
		/variant === "cover"[\s\S]*?"aspect-\[4\/1\] min-h-0"/,
	);
});

test("home recommended-seller cards reuse the canonical cover aspect and banner image", () => {
	const homeMerchants = readFileSync(
		new URL(
			"../features/home/components/HomeCompactMerchants.tsx",
			import.meta.url,
		),
		"utf8",
	);

	assert.match(
		homeMerchants,
		/import \{ SELLER_COVER_ASPECT \} from "@\/lib\/media\/seller-cover";/,
	);
	assert.match(
		homeMerchants,
		/style=\{\{ aspectRatio: SELLER_COVER_ASPECT \}\}/,
	);
	assert.match(homeMerchants, /preset="banner"/);
	assert.match(homeMerchants, /className="object-cover object-center"/);
	assert.doesNotMatch(homeMerchants, /aspect-\[16\/9\]/);
	assert.doesNotMatch(homeMerchants, /preset="card"/);
});
