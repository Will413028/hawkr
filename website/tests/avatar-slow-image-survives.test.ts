import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import Avatar from "../components/shared/Avatar";

/**
 * Slowness is not failure.
 *
 * `Avatar` used to arm a 4-second timer and, when it elapsed, render the
 * fallback INSTEAD of the image. That swap unmounts the <img>: the in-flight
 * request is abandoned and the photo can never appear, however close it was to
 * arriving. A cold CDN image-transform miss routinely takes longer than four
 * seconds, so the protection was firing on healthy avatars and permanently
 * destroying them (#410). Measured on a throttled SPA navigation, the timer
 * armed on every mount and the <img> count went to 0 and stayed there — an
 * image delivered at 6s never came back.
 *
 * The fix is that the <img> stays mounted for the whole life of the request and
 * the fallback is painted UNDER it as a background layer, so the two are no
 * longer alternatives. Only a real `error` — terminal for a given src, and
 * reset when the src changes — replaces the element.
 *
 * These assertions pin both halves. The timer check is deliberately a source
 * check: the regression is not a wrong value, it is the mechanism existing at
 * all, and "add a slightly longer timeout" is the tempting wrong fix.
 */

const avatarSource = () =>
	readFile(new URL("../components/shared/Avatar.tsx", import.meta.url), "utf8");

test("a pending avatar renders the image, not the fallback element", () => {
	// Buyer with a name: the fallback strategy is `initial`, i.e. the case where
	// the old code had the most to swap in. Nothing may pre-empt the image.
	const markup = renderToStaticMarkup(
		createElement(Avatar, {
			src: "public/avatar/buyer-1.png",
			name: "陳小美",
			alt: "陳小美",
			entity: "buyer",
			className: "size-10 rounded-full object-cover",
		}),
	);

	assert.match(markup, /<img/, `expected an <img>, got: ${markup}`);
	assert.doesNotMatch(
		markup,
		/<span/,
		"the fallback element must not stand in for a src that is merely loading",
	);
	// The initial is the fallback element's text content, and that element
	// carries `role="img"`. Neither may be on screen while a real avatar is on
	// its way. (`陳` on its own would match the alt attribute, which is correct.)
	assert.doesNotMatch(markup, /role="img"/);
	assert.doesNotMatch(markup, />陳</);
});

test("the placeholder fallback is painted under the image, not in its place", () => {
	const markup = renderToStaticMarkup(
		createElement(Avatar, {
			src: "public/avatar/buyer-2.png",
			// No name → the buyer strategy resolves to `placeholder`.
			entity: "buyer",
			className: "size-10 rounded-full object-cover",
		}),
	);

	assert.match(markup, /<img/);
	assert.match(
		markup,
		/background-image:url\(\/placeholders\/avatar\.svg\)/,
		`placeholder layer missing from: ${markup}`,
	);
	assert.match(markup, /background-size:cover/);
	// The layer must ride on the image element itself. A second element would
	// take `className` away from the node every caller's CSS sizes and targets.
	assert.doesNotMatch(markup, /<span|<div/);
});

test("a seller avatar keeps its className on the image element", () => {
	const markup = renderToStaticMarkup(
		createElement(Avatar, {
			src: "public/avatar/seller-1.png",
			name: "陳記",
			entity: "seller",
			className: "seller-hero__avatar-img",
			fallbackClassName: "seller-hero__avatar--fallback",
		}),
	);

	assert.match(markup, /class="seller-hero__avatar-img"/);
	// `fallbackClassName` belongs to the fallback ELEMENT only. Merging it onto
	// the image would hand callers' sizing to whichever rule the stylesheet
	// happens to declare later: `.seller-customers__avatar-fallback` hard-codes
	// 40px while its caller's `.seller-customer-detail__avatar` is 64px.
	assert.doesNotMatch(markup, /seller-hero__avatar--fallback/);
});

test("no timer may treat a slow avatar as a broken one", async () => {
	const source = await avatarSource();

	assert.doesNotMatch(
		source,
		/setTimeout/,
		"Avatar must not schedule anything: a timeout cannot tell a slow image from a broken one, and acting on one destroys the other (#410)",
	);
	assert.doesNotMatch(source, /clearTimeout|stallTimer/);
});

test("the hydration-race guard survives", async () => {
	const source = await avatarSource();

	// A server-rendered <img> can finish or fail before hydration attaches the
	// handlers, and React never replays the event it missed. Without this read
	// of the element's own state, an avatar that already failed sits blank
	// forever (#427, commit 40f63e9c).
	assert.match(source, /\.complete/);
	assert.match(source, /naturalWidth === 0/);
	assert.match(source, /onError/);
});
