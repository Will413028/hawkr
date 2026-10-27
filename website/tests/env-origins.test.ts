import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { afterEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const NAMES = ["NEXT_PUBLIC_WEB_ORIGIN", "NEXT_PUBLIC_ADM_ORIGIN"] as const;
const saved = new Map(NAMES.map((name) => [name, process.env[name]]));

function resolve(
	flavor: "dev" | "prod",
	values: Partial<Record<(typeof NAMES)[number], string>>,
) {
	for (const name of NAMES) {
		const value = values[name];
		if (value === undefined) delete process.env[name];
		else process.env[name] = value;
	}
	__setFlavorForTest(flavor);
	__setEnvForTest(null);
	return getEnv();
}

afterEach(() => {
	for (const [name, value] of saved) {
		if (value === undefined) delete process.env[name];
		else process.env[name] = value;
	}
	__setFlavorForTest(null);
	__setEnvForTest(null);
});

const PROD_ORIGINS = {
	NEXT_PUBLIC_WEB_ORIGIN: "https://www.example.org",
	NEXT_PUBLIC_ADM_ORIGIN: "https://admin.example.org",
};

test("prod takes its web and admin origins from env", () => {
	const env = resolve("prod", PROD_ORIGINS);
	assert.equal(env.webOrigin, "https://www.example.org");
	assert.equal(env.admOrigin, "https://admin.example.org");
});

test("prod has no default web origin", () => {
	assert.throws(
		() => resolve("prod", { NEXT_PUBLIC_ADM_ORIGIN: "https://a.example.org" }),
		/NEXT_PUBLIC_WEB_ORIGIN is required for the production flavor/,
	);
});

test("prod has no default admin origin", () => {
	assert.throws(
		() => resolve("prod", { NEXT_PUBLIC_WEB_ORIGIN: "https://w.example.org" }),
		/NEXT_PUBLIC_ADM_ORIGIN is required for the production flavor/,
	);
});

test("dev falls back to the local servers when the origins are unset", () => {
	const env = resolve("dev", {});
	assert.equal(env.webOrigin, "http://localhost:3000");
	assert.equal(env.admOrigin, "http://localhost:3001");
});

test("an origin env value is canonicalised, and anything but a bare origin is refused", () => {
	assert.equal(
		resolve("dev", { NEXT_PUBLIC_WEB_ORIGIN: "HTTPS://Shop.Example:443/" })
			.webOrigin,
		"https://shop.example",
	);
	for (const bad of [
		"shop.example",
		"ftp://shop.example",
		"https://user:pass@shop.example",
		"https://shop.example/path",
		"https://shop.example?",
		"https://shop.example#",
	]) {
		assert.throws(
			() => resolve("dev", { NEXT_PUBLIC_WEB_ORIGIN: bad }),
			/NEXT_PUBLIC_WEB_ORIGIN must be/,
			bad,
		);
	}
});

test("lineUrl is empty when unset and the configured https URL otherwise", () => {
	const previous = process.env.NEXT_PUBLIC_LINE_URL;
	try {
		delete process.env.NEXT_PUBLIC_LINE_URL;
		assert.equal(resolve("prod", PROD_ORIGINS).lineUrl, "");
		process.env.NEXT_PUBLIC_LINE_URL = "https://line.example.com/R/ti/p/@demo";
		__setEnvForTest(null);
		assert.equal(getEnv().lineUrl, "https://line.example.com/R/ti/p/@demo");
		for (const bad of [
			"http://line.example.com/x",
			"not a url",
			"javascript:alert(1)",
		]) {
			process.env.NEXT_PUBLIC_LINE_URL = bad;
			__setEnvForTest(null);
			assert.throws(() => getEnv(), /NEXT_PUBLIC_LINE_URL/, bad);
		}
	} finally {
		if (previous === undefined) delete process.env.NEXT_PUBLIC_LINE_URL;
		else process.env.NEXT_PUBLIC_LINE_URL = previous;
	}
});

test("ContactPage renders its LINE entry only from the configured URL", () => {
	const source = readFileSync(
		new URL("../features/content/contact/ContactPage.tsx", import.meta.url),
		"utf8",
	);
	assert.doesNotMatch(source, /lin\.ee|line\.me/);
	assert.match(source, /getEnv\(\)/);
	assert.match(source, /lineUrl \? \[\{ \.\.\.item, href: lineUrl \}\] : \[\]/);
});

test("pnpm dev is plain `next dev` on localhost, with no custom host or self-signed HTTPS", () => {
	const pkg = JSON.parse(
		readFileSync(new URL("../package.json", import.meta.url), "utf8"),
	) as { scripts: Record<string, string> };
	assert.equal(pkg.scripts.dev, "next dev");
});
