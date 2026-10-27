import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { checkCrossPlatformTokens } from "../scripts/check-cross-platform-tokens.mjs";

function fixture(t) {
	const root = mkdtempSync(join(tmpdir(), "hawkr-token-gate-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	mkdirSync(join(root, "buyer"));
	mkdirSync(join(root, "seller"));
	const web = (content) => writeFileSync(join(root, "tokens.css"), content);
	const app = (name, content) =>
		writeFileSync(join(root, name, "tokens.dart"), content);
	web(
		":root { --forest: #334c0f; --action: var(--forest); --space: .25rem; } [data-theme='dark'] { --action: #ffffff; }",
	);
	for (const name of ["buyer", "seller"]) {
		app(
			name,
			"class AppColors { static const Color forest = Color(0xFF334C0F); static const Color action = forest; } class HkSpace { static const double xs = 4; }",
		);
	}
	const sources = {
		web: { platform: "web", path: "tokens.css", format: "css" },
	};
	for (const name of ["buyer", "seller"])
		sources[name] = {
			platform: name,
			path: `${name}/tokens.dart`,
			format: "dart",
		};
	const bindings = (webKey, dartKey) => [
		{ source: "web", key: webKey },
		...["buyer", "seller"].map((source) => ({ source, key: dartKey })),
	];
	const contract = {
		schemaVersion: 1,
		sources,
		tokens: [
			{
				id: "brand.action",
				type: "color",
				value: "#334c0f",
				bindings: bindings("--action", "AppColors.action"),
			},
			{
				id: "space.xs",
				type: "dimension",
				value: 4,
				bindings: bindings("--space", "HkSpace.xs"),
			},
		],
		exceptions: [],
	};
	return {
		root,
		web,
		app,
		contract,
		check: () =>
			checkCrossPlatformTokens({ webRoot: root, appsRoot: root, contract }),
	};
}

test("checks Web alone by default without claiming app verification", (t) => {
	const f = fixture(t);
	rmSync(join(f.root, "buyer"), { recursive: true });
	const result = checkCrossPlatformTokens({
		webRoot: f.root,
		contract: f.contract,
	});
	assert.equal(result.passed, true);
	assert.deepEqual(result.platforms, ["web"]);
	assert.equal(result.checked, 2);
});

test("resolves source aliases, normalizes rem, and excludes dark or route overrides", (t) => {
	const f = fixture(t);
	f.web(
		":root { --forest: #334c0f; --action: var(--forest); --space: .25rem; } [data-theme='dark'] { --action: #ffffff; } .seller { --action: #000000; } @media (max-width: 600px) { :root { --action: #ffffff; } }",
	);
	assert.equal(f.check().passed, true);
});

test("fails an actual app color change behind a semantic alias", (t) => {
	const f = fixture(t);
	f.app(
		"seller",
		"class AppColors { static const Color forest = Color(0xFF719244); static const Color action = forest; } class HkSpace { static const double xs = 4; }",
	);
	const result = f.check();
	assert.equal(result.passed, false);
	assert.ok(
		result.failures.some(
			(failure) =>
				failure.includes("seller:AppColors.action") &&
				failure.includes("#719244"),
		),
	);
});

test("requires both app repositories when cross-platform verification was requested", (t) => {
	const f = fixture(t);
	rmSync(join(f.root, "seller"), { recursive: true });
	assert.equal(f.check().passed, false);
	assert.ok(
		f
			.check()
			.failures.some((failure) => failure.includes("Source unavailable")),
	);
});

test("does not silently discard a changed Dart color alpha", (t) => {
	const f = fixture(t);
	f.app(
		"buyer",
		"class AppColors { static const Color forest = Color(0x80334C0F); static const Color action = forest; } class HkSpace { static const double xs = 4; }",
	);
	assert.ok(
		f
			.check()
			.failures.some((failure) => failure.includes("Expected opaque color")),
	);
});

test("fails missing and circular CSS aliases with actionable diagnostics", (t) => {
	const f = fixture(t);
	f.web(":root { --action: var(--missing); --space: .25rem; }");
	assert.ok(
		f
			.check()
			.failures.some((failure) =>
				failure.includes("Missing CSS alias --missing"),
			),
	);
	f.web(
		":root { --action: var(--forest); --forest: var(--action); --space: .25rem; }",
	);
	assert.ok(
		f
			.check()
			.failures.some((failure) => failure.includes("Circular CSS alias")),
	);
});

test("tracks known exceptions so stale divergence values also fail", (t) => {
	const f = fixture(t);
	f.contract.exceptions.push({
		id: "app.compactDensity",
		type: "dimension",
		bindings: [{ source: "seller", key: "HkSpace.xs", value: 4 }],
	});
	assert.equal(f.check().passed, true);
	f.contract.exceptions[0].bindings[0].value = 8;
	assert.ok(
		f
			.check()
			.failures.some((failure) =>
				failure.includes("review and update the tracked exception"),
			),
	);
});

test("reads nominal TextStyle size and weight without inferring runtime text scaling", (t) => {
	const f = fixture(t);
	f.app(
		"seller",
		"class AppTypography { static TextStyle get body => TextStyle(fontSize: 16.sp, fontWeight: FontWeight.w400, height: 24 / 16); }",
	);
	f.contract.tokens = [
		{
			id: "size",
			type: "dimension",
			value: 16,
			bindings: [
				{ source: "seller", key: "AppTypography.body", property: "fontSize" },
			],
		},
		{
			id: "weight",
			type: "number",
			value: 400,
			bindings: [
				{ source: "seller", key: "AppTypography.body", property: "fontWeight" },
			],
		},
		{
			id: "line",
			type: "number",
			value: 1.5,
			bindings: [
				{ source: "seller", key: "AppTypography.body", property: "height" },
			],
		},
	];
	assert.equal(f.check().passed, true);
	f.contract.tokens[0].value = 18;
	assert.ok(
		f
			.check()
			.failures.some(
				(failure) =>
					failure.includes("AppTypography.body.fontSize") &&
					failure.includes("found 16"),
			),
	);
});
