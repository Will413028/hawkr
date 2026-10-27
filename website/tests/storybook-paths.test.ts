import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import postcss from "postcss";
import { parser } from "storybook/internal/babel";
import {
	catalogAssetPlugin,
	catalogCssAssetPlugin,
	rebaseCatalogJavaScript,
} from "../.storybook/catalog-assets";
import {
	flutterCatalogBasePath,
	normalizeStorybookBasePath,
	rebaseCatalogAssetUrl,
	rebaseCatalogCssUrl,
} from "../scripts/storybook-paths.mjs";

const base = "/storybook/1.0.0/";

test("catalog base is a canonical path, never an origin or traversal", () => {
	assert.equal(normalizeStorybookBasePath(), "/");
	assert.equal(normalizeStorybookBasePath(base.slice(0, -1)), base);
	for (const invalid of [
		"https://example.com/catalog/",
		"//example.com/",
		"catalog/",
		"/../catalog/",
		"/catalog/./",
		"/catalog/?v=1",
		"/catalog/#hash",
		"/a\\b/",
		"/a%2fb/",
		"/a//b/",
	])
		assert.throws(() => normalizeStorybookBasePath(invalid));
});

test("only known public assets move into the versioned catalog", () => {
	assert.equal(
		rebaseCatalogAssetUrl("/brand/logo.svg", base),
		`${base}brand/logo.svg`,
	);
	assert.equal(
		rebaseCatalogAssetUrl("/placeholders/calendar.svg?size=24#glyph", base),
		`${base}placeholders/calendar.svg?size=24#glyph`,
	);
	assert.equal(
		rebaseCatalogAssetUrl("/placeholders/", base),
		`${base}placeholders/`,
	);
	for (const untouched of [
		"https://cdn.example.com/placeholders/a.svg",
		"//cdn.example.com/placeholders/a.svg",
		"data:image/svg+xml;base64,AAA",
		"blob:abc",
		"#symbol",
		"./placeholders/a.svg",
		"/api/products",
		"/app/orders",
		"/seller-center/products",
		"/public/products",
		`${base}placeholders/calendar.svg`,
	])
		assert.equal(rebaseCatalogAssetUrl(untouched, base), untouched);
	assert.equal(
		rebaseCatalogAssetUrl("/brand/logo.svg", "/"),
		"/brand/logo.svg",
	);
});

test("both Flutter catalogs receive the same versioned base as Storybook", () => {
	assert.equal(flutterCatalogBasePath(base, "buyer"), `${base}flutter/buyer/`);
	assert.equal(
		flutterCatalogBasePath(base, "seller"),
		`${base}flutter/seller/`,
	);
	assert.equal(flutterCatalogBasePath("/", "buyer"), "/flutter/buyer/");
	assert.throws(() => flutterCatalogBasePath(base, "unknown"));
});

test("JS transform preserves imports, API routes, comments and dynamic expressions", () => {
	const source = [
		'import external from "/placeholders/external-module";',
		"// /brand/comment.svg is documentation only.",
		'const icon = "/placeholders/calendar.svg";',
		`const image = \`/placeholders/\${name}.webp\`;`,
		'const request = fetch("/api/products");',
		'const route = "/seller-center/products";',
		'const remote = "https://cdn.example.com/brand/logo.svg";',
		'const view = <img src="/brand/logo.svg" />;',
	].join("\n");
	const result = rebaseCatalogJavaScript(source, base);
	assert.ok(result);
	assert.ok(
		result.includes(`const icon = "${base}placeholders/calendar.svg";`),
	);
	assert.ok(
		result.includes(`const image = \`${base}placeholders/\${name}.webp\`;`),
	);
	assert.ok(result.includes(`<img src="${base}brand/logo.svg" />`));
	assert.ok(
		result.includes('import external from "/placeholders/external-module";'),
	);
	assert.ok(result.includes("// /brand/comment.svg is documentation only."));
	assert.ok(result.includes('fetch("/api/products")'));
	assert.ok(result.includes('"/seller-center/products"'));
	assert.ok(result.includes('"https://cdn.example.com/brand/logo.svg"'));
	assert.equal(rebaseCatalogJavaScript(result, base), null);
	assert.equal(rebaseCatalogJavaScript(source, "/"), null);
});

for (const newline of ["\n", "\r\n"]) {
	test(`JS transform keeps original tab and Unicode offsets with ${JSON.stringify(newline)}`, () => {
		const source = [
			'import external from "/placeholders/external-module";',
			'export { default as icon } from "/placeholders/exported-module";',
			'export * from "/placeholders/other-module";',
			"// Hawkr — 🌿 /brand/comment.svg stays unchanged.",
			"const assets: { logo: string; image: string } = {",
			'\tlogo: "/brand/logo.svg",',
			`\timage: \`/placeholders/\${name}.webp\`,`,
			"} as const;",
			"const unchanged = '中文 /api/products';",
			"const view = (",
			'\t<img src="/placeholders/calendar.svg" title="Hawkr" />',
			");",
		].join(newline);
		const result = rebaseCatalogJavaScript(source, base);
		assert.ok(result);
		assert.doesNotThrow(() =>
			parser.parse(result, {
				sourceType: "module",
				plugins: ["typescript", "jsx"],
			}),
		);
		assert.ok(result.includes(`logo: "${base}brand/logo.svg"`));
		assert.ok(result.includes(`image: \`${base}placeholders/\${name}.webp\``));
		assert.ok(result.includes(`<img src="${base}placeholders/calendar.svg"`));
		assert.ok(
			result.startsWith(source.split(newline).slice(0, 4).join(newline)),
		);
		assert.equal(result.replaceAll(base, "/"), source);
		assert.equal(rebaseCatalogJavaScript(result, base), null);
	});
}

test("the real TypeScript asset manifest remains parseable and otherwise unchanged", () => {
	const source = readFileSync(
		new URL("../lib/media/public-assets.ts", import.meta.url),
		"utf8",
	);
	const result = rebaseCatalogJavaScript(source, base);
	assert.ok(result);
	assert.doesNotThrow(() =>
		parser.parse(result, { sourceType: "module", plugins: ["typescript"] }),
	);
	assert.ok(result.includes(`logoGrass: "${base}brand/logo-grass.svg"`));
	assert.equal(result.replaceAll(base, "/"), source);
	assert.equal(rebaseCatalogJavaScript(result, base), null);
});

test("Vite transformation is scoped to project source, never vendor modules", () => {
	const root = "/project";
	const plugin = catalogAssetPlugin(root, base);
	const hook = plugin.transform;
	assert.equal(typeof hook, "function");
	const transform = hook as (
		code: string,
		id: string,
	) => { code: string } | undefined;
	assert.equal(
		transform(
			'const url = "/placeholders/a.svg";',
			"/project/node_modules/vendor/index.js",
		),
		undefined,
	);
	assert.equal(
		transform(
			'const url = "/placeholders/a.svg";',
			"/other/components/Icon.tsx",
		),
		undefined,
	);
	assert.equal(
		transform(
			'const url = "/placeholders/a.svg";',
			"/project/.storybook/manager.ts",
		),
		undefined,
	);
	assert.ok(
		transform(
			'const url = "/placeholders/a.svg";',
			"/project/components/Icon.tsx",
		)?.code.includes(base),
	);
});

test("PostCSS rebases root assets and repository fonts without changing data URLs", async () => {
	const result = await postcss([catalogCssAssetPlugin(base)]).process(
		'.font{src:url("../../assets/fonts/example.woff2")} .icon{mask:url(/placeholders/a.svg)} .svg{background:url("data:image/svg+xml,abc")} .remote{src:url(https://cdn.example.com/f.woff2)}',
		{ from: undefined },
	);
	assert.ok(result.css.includes(`url("${base}assets/fonts/example.woff2")`));
	assert.ok(result.css.includes(`url(${base}placeholders/a.svg)`));
	assert.ok(result.css.includes('url("data:image/svg+xml,abc")'));
	assert.ok(result.css.includes("url(https://cdn.example.com/f.woff2)"));
	assert.equal(
		rebaseCatalogCssUrl("../../assets/fonts/example.woff2", "/"),
		"/assets/fonts/example.woff2",
	);
	assert.equal(
		rebaseCatalogCssUrl("../other/real-image.png", base),
		"../other/real-image.png",
	);
});

test("the real generated brand stylesheet has no font URLs escaping the catalog", async () => {
	const filename = fileURLToPath(
		new URL("../styles/foundation/openhuninn.generated.css", import.meta.url),
	);
	const source = readFileSync(filename, "utf8");
	const result = await postcss([catalogCssAssetPlugin(base)]).process(source, {
		from: filename,
	});
	assert.ok(result.css.includes(`${base}assets/fonts/hawkr-rounded-v2/`));
	assert.ok(!result.css.includes("../../assets/fonts/"));
	const fontUrls = [...result.css.matchAll(/url\("([^"]+)"\)/g)].map(
		(match) => match[1],
	);
	assert.ok(fontUrls.length > 50);
	for (const url of fontUrls) {
		assert.ok(url.startsWith(base));
		assert.ok(
			readFileSync(
				fileURLToPath(new URL(`../${url.slice(base.length)}`, import.meta.url)),
			).length > 0,
		);
	}
});
