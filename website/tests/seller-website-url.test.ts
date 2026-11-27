import assert from "node:assert/strict";
import test from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import {
	isExternalWebsiteUrl,
	isValidHostname,
	isValidWebsiteUrl,
} from "../lib/seller/website-url";

test("empty string is valid (clears the field)", () => {
	assert.equal(isValidWebsiteUrl(""), true);
});

test("valid https url", () => {
	assert.equal(isValidWebsiteUrl("https://example.org"), true);
});

test("valid http url with path", () => {
	assert.equal(isValidWebsiteUrl("http://example.org/shop"), true);
});

test("missing scheme is invalid", () => {
	assert.equal(isValidWebsiteUrl("example.org"), false);
});

test("non-http(s) scheme is invalid", () => {
	assert.equal(isValidWebsiteUrl("javascript:alert(1)"), false);
});

test("scheme without host is invalid", () => {
	assert.equal(isValidWebsiteUrl("https://"), false);
});

test("not a url at all is invalid", () => {
	assert.equal(isValidWebsiteUrl("not a url"), false);
});

test("browser-encoded hostnames are rejected by the hostname predicate", () => {
	assert.equal(isValidHostname("not%20a%20url"), false);
	assert.equal(isValidHostname("exa%20mple.com"), false);
	assert.equal(isValidHostname("a%2Ab"), false);
	assert.equal(isValidHostname(""), false);
});

test("punctuation browsers keep verbatim in a host is still rejected", () => {
	assert.equal(isValidHostname("not!a!url"), false);
	assert.equal(isValidHostname("a{b}c"), false);
	assert.equal(isValidWebsiteUrl("https://not!a!url"), false);
});

test("legitimate hosts survive the tightened hostname check", () => {
	assert.equal(isValidWebsiteUrl("https://例子.tw"), true);
	assert.equal(isValidWebsiteUrl("https://example.org."), true);
	assert.equal(isValidWebsiteUrl("https://my_site.example.org"), true);
	assert.equal(isValidWebsiteUrl("http://[::1]"), true);
	assert.equal(isValidWebsiteUrl("https://example.org:8443/shop"), true);
	assert.equal(isValidWebsiteUrl("https://example.org/my page"), true);
});

test("external-link check rejects non-hostnames", () => {
	assert.equal(isExternalWebsiteUrl("https://not!a!url"), false);
	assert.equal(isExternalWebsiteUrl("https://not a url"), false);
	assert.equal(isExternalWebsiteUrl("not a url"), false);
	assert.equal(isExternalWebsiteUrl("https://example.org"), true);
});

test("external-link check treats the configured web origin as our own platform", () => {
	const base = getEnv();
	__setEnvForTest({ ...base, webOrigin: "https://shop.example.net" });
	try {
		assert.equal(
			isExternalWebsiteUrl("https://shop.example.net/sellers/1"),
			false,
		);
		assert.equal(isExternalWebsiteUrl("shop.example.net/sellers/1"), false);
		assert.equal(isExternalWebsiteUrl("https://www.shop.example.net"), false);
		assert.equal(isExternalWebsiteUrl("https://m.shop.example.net"), false);
		assert.equal(isExternalWebsiteUrl("https://notshop.example.net"), true);
		assert.equal(
			isExternalWebsiteUrl("https://shop.example.net.evil.test"),
			true,
		);
		assert.equal(isExternalWebsiteUrl("https://example.org"), true);
	} finally {
		__setEnvForTest(null);
	}
});
