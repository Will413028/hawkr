import assert from "node:assert/strict";
import { test } from "node:test";
import { normalizeWebHref } from "../lib/navigation/web-href";

test("normalizes safe internal hrefs", () => {
	assert.equal(
		normalizeWebHref("/market/products?q=小農#results"),
		"/market/products?q=%E5%B0%8F%E8%BE%B2#results",
	);
	assert.equal(normalizeWebHref("/about"), "/about");
});

test("rejects protocol-relative, backslash, control, and script hrefs", () => {
	for (const href of [
		"//evil.example/path",
		"/\\evil.example/path",
		"/\nevil.example/path",
		"javascript:alert(1)",
		"data:text/html,test",
		123,
		{ href: "/account" },
	]) {
		assert.equal(normalizeWebHref(href, { allowExternal: true }), undefined);
	}
});

test("external destinations are explicit and HTTPS-only by default", () => {
	assert.equal(normalizeWebHref("https://example.com"), undefined);
	assert.equal(
		normalizeWebHref("https://example.com", { allowExternal: true }),
		"https://example.com/",
	);
	assert.equal(
		normalizeWebHref("http://example.com", { allowExternal: true }),
		undefined,
	);
	assert.equal(
		normalizeWebHref("http://example.com", {
			allowExternal: true,
			allowInsecureHttp: true,
		}),
		"http://example.com/",
	);
});
