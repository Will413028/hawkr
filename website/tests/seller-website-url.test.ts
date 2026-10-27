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

// 這一組是本檔唯一擋得住「瀏覽器裡放行、Node 裡擋掉」的斷言。
//
// Chrome 對主機名裡的非法字元是 percent-encode 後照收（實測 151：
// `https://not a url` → hostname `not%20a%20url`），Node 的 `new URL()` 則直接
// 拋。所以整串網址餵進 isValidWebsiteUrl 在 Node 下**無論修不修都回 false**，
// 那種測試對這個洞零鑑別力。要在 Node 內測到瀏覽器的那條路，只能繞過 parser、
// 把瀏覽器實際產出的主機名直接餵進述詞。
// 真正在瀏覽器引擎裡跑的版本見 tests/e2e/seller-website-url-engine.spec.ts。
test("browser-encoded hostnames are rejected by the hostname predicate", () => {
	// 左邊是 Chrome 對右側輸入實際產出的 hostname（見上方 spec 的引擎斷言）。
	assert.equal(isValidHostname("not%20a%20url"), false); // https://not a url
	assert.equal(isValidHostname("exa%20mple.com"), false); // https://exa mple.com/x
	assert.equal(isValidHostname("a%2Ab"), false); // https://a*b
	assert.equal(isValidHostname(""), false); // https://
});

// Chrome 也把 !"$&'()+,;={}~ 這類字元原樣留在主機名裡（不 encode、不拋），
// 光排除 `%` 擋不掉這一類。
test("punctuation browsers keep verbatim in a host is still rejected", () => {
	assert.equal(isValidHostname("not!a!url"), false);
	assert.equal(isValidHostname("a{b}c"), false);
	assert.equal(isValidWebsiteUrl("https://not!a!url"), false);
});

// 收緊主機名判準的反向錨：以下都是兩個引擎都解析得出、且必須維持合法的形式。
// 少了這幾條，正則寫嚴一點就會靜默擋掉真實賣家填得出來的網址。
test("legitimate hosts survive the tightened hostname check", () => {
	assert.equal(isValidWebsiteUrl("https://例子.tw"), true); // punycode 化
	assert.equal(isValidWebsiteUrl("https://example.org."), true); // 根點 FQDN
	assert.equal(isValidWebsiteUrl("https://my_site.example.org"), true);
	assert.equal(isValidWebsiteUrl("http://[::1]"), true); // IPv6 literal
	assert.equal(isValidWebsiteUrl("https://example.org:8443/shop"), true);
	assert.equal(isValidWebsiteUrl("https://example.org/my page"), true); // 空白在 path 不在 host
});

// isExternalWebsiteUrl 共用同一個破口：它決定店面頁畫不畫「訪問官網」外連。
// ⚠️ 帶空白那兩條在 Node 是靠 `new URL()` 拋出擋掉的，修不修都綠——真正的鑑別力
// 在 e2e spec 那邊；`not!a!url` 這條才是本檔測得到的（Node 也照收該主機名）。
test("external-link check rejects non-hostnames", () => {
	assert.equal(isExternalWebsiteUrl("https://not!a!url"), false);
	assert.equal(isExternalWebsiteUrl("https://not a url"), false);
	assert.equal(isExternalWebsiteUrl("not a url"), false); // 補 scheme 後同形
	assert.equal(isExternalWebsiteUrl("https://example.org"), true);
});

// 平台自己的店面網址（註冊流程種進 website_url 的那種）不算外部官網；判準取自設定的
// web origin，不是寫死的網域。
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
