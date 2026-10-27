import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

let calls: unknown[][] = [];

function installWindow() {
	calls = [];
	(globalThis as Record<string, unknown>).window = {
		gtag: (...args: unknown[]) => calls.push(args),
	};
}

function enable(gaId: string) {
	process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID = gaId;
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	getEnv();
}

beforeEach(installWindow);

afterEach(() => {
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID;
	delete (globalThis as Record<string, unknown>).window;
	delete (globalThis as Record<string, unknown>).document;
});

test("track fires gtag event when enabled", async () => {
	enable("G-AAA");
	const { track } = await import("../lib/analytics/gtag");
	track("add_to_cart", { value: 100, currency: "TWD" });
	assert.deepEqual(calls[0], [
		"event",
		"add_to_cart",
		{ value: 100, currency: "TWD" },
	]);
});

test("every event overrides GA page context with identifier-free values", async () => {
	enable("G-AAA");
	(globalThis as Record<string, unknown>).window = {
		gtag: (...args: unknown[]) => calls.push(args),
		location: {
			origin: "https://example.com",
			pathname: "/account/messages/assistant/843856384063471760",
			search: "?requestId=req-secret-123",
		},
	};
	const { track } = await import("../lib/analytics/gtag");
	track("hawkr_agent_terminal_rendered", {
		visible_step_count: 3,
		page_location: "https://attacker.invalid/raw",
		page_referrer: "https://example.com/raw-secret",
	});

	assert.deepEqual(calls[0], [
		"event",
		"hawkr_agent_terminal_rendered",
		{
			visible_step_count: 3,
			page_path: "/account/messages/assistant/:conversationId",
			page_location:
				"https://example.com/account/messages/assistant/:conversationId",
			page_referrer: "",
			page_title: "/account/messages/assistant/:conversationId",
		},
	]);
	assert.doesNotMatch(
		JSON.stringify(calls[0]),
		/843856384063471760|req-secret/,
	);
});

test("bootstrap gives automatic events a sanitized page context", async () => {
	enable("G-AAA");
	(globalThis as Record<string, unknown>).window = {
		location: {
			origin: "https://example.com",
			pathname: "/adm/2685914682/orders/HK260830001",
			search: "?token=secret&utm_source=google",
		},
	};
	(globalThis as Record<string, unknown>).document = {
		title: "訂單 HK260830001｜Hawkr",
		referrer: "https://example.com/account/orders/HK260829001?buyerName=王小明",
	};
	const { __resetGtagForTest, bootstrapGtag } = await import(
		"../lib/analytics/gtag"
	);
	__resetGtagForTest();
	bootstrapGtag("G-AAA");

	const dataLayer = (
		(globalThis as Record<string, unknown>).window as {
			dataLayer: unknown[];
		}
	).dataLayer;
	const config = Array.from(dataLayer[2] as ArrayLike<unknown>);
	assert.deepEqual(config, [
		"config",
		"G-AAA",
		{
			send_page_view: false,
			page_path: "/adm/:storeId/orders/:orderNo?utm_source=google",
			page_location:
				"https://example.com/adm/:storeId/orders/:orderNo?utm_source=google",
			page_referrer: "https://example.com/account/orders/:orderNo",
			page_title: "/adm/:storeId/orders/:orderNo",
		},
	]);
	assert.doesNotMatch(
		JSON.stringify(config),
		/2685914682|HK260830001|HK260829001|王小明|secret/,
	);
});

test("SPA page context updates without emitting an automatic page view", async () => {
	enable("G-AAA");
	const { updatePageContext } = await import("../lib/analytics/gtag");
	updatePageContext({
		page_path: "/seller/pricing",
		page_location: "https://example.com/seller/pricing",
		page_referrer: "https://www.google.com/",
		page_title: "賣家方案",
	});

	assert.deepEqual(calls[0], [
		"config",
		"G-AAA",
		{
			update: true,
			send_page_view: false,
			page_path: "/seller/pricing",
			page_location: "https://example.com/seller/pricing",
			page_referrer: "https://www.google.com/",
			page_title: "賣家方案",
		},
	]);
});

test("track is a no-op when gaId is empty", async () => {
	enable("");
	const { track } = await import("../lib/analytics/gtag");
	track("add_to_cart", { value: 100 });
	assert.equal(calls.length, 0);
});

test("setUserProperties uses the user_properties set form", async () => {
	enable("G-AAA");
	const { setUserProperties } = await import("../lib/analytics/gtag");
	setUserProperties({ role: "buyer" });
	assert.deepEqual(calls[0], ["set", "user_properties", { role: "buyer" }]);
});

test("the auth bridge never sends a stable account identifier to GA4", () => {
	const source = readFileSync(
		new URL("../lib/analytics/useAnalyticsIdentity.ts", import.meta.url),
		"utf8",
	);
	assert.doesNotMatch(source, /accountId|setUserId|clearUserId|user_id/);
});

test("pre-bootstrap page views flush after js and config commands", async () => {
	enable("G-AAA");
	(globalThis as Record<string, unknown>).window = {};
	const { __resetGtagForTest, bootstrapGtag, track } = await import(
		"../lib/analytics/gtag"
	);
	__resetGtagForTest();

	track("page_view", { page_path: "/" });
	bootstrapGtag("G-AAA");

	const dataLayer = (
		(globalThis as Record<string, unknown>).window as {
			dataLayer: unknown[];
		}
	).dataLayer;
	assert.equal(
		Object.prototype.toString.call(dataLayer[0]),
		"[object Arguments]",
	);
	const commands = dataLayer.map((command) =>
		Array.from(command as ArrayLike<unknown>),
	);
	assert.deepEqual(commands[0], [
		"set",
		"allow_ad_personalization_signals",
		false,
	]);
	assert.equal(commands[1]?.[0], "js");
	assert.deepEqual(commands[2], ["config", "G-AAA", { send_page_view: false }]);
	assert.deepEqual(commands[3], ["event", "page_view", { page_path: "/" }]);
});
