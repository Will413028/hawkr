import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const rootDesignUrl = new URL("../DESIGN.md", import.meta.url);
const guidelinesUrl = new URL(
	"../docs/design-system-guidelines.md",
	import.meta.url,
);
const sidecarUrl = new URL("../.impeccable/design.json", import.meta.url);

test("root DESIGN.md is the canonical current brand entrypoint", async () => {
	const source = await readFile(rootDesignUrl, "utf8");
	const headings = [
		"## Overview",
		"## Colors",
		"## Typography",
		"## Layout",
		"## Elevation & Depth",
		"## Shapes",
		"## Components",
		"## Do's and Don'ts",
	];

	assert.match(source, /name: "Hawkr"/);
	assert.match(source, /Creative North Star: "深入職人故事的市集"/);
	assert.match(source, /品牌 owner 已確認的現行北極星/);
	assert.doesNotMatch(source, /須由品牌 owner 確認後才視為定稿/);
	assert.match(source, /brand-primary: "#e4572e"/);
	assert.match(source, /brand-primary-strong: "#b8441f"/);
	assert.match(source, /brand-primary-deep: "#8f3315"/);
	assert.match(source, /brand-secondary: "#f2a541"/);
	assert.match(source, /brand-ink: "#1f2933"/);
	assert.match(source, /circle: "50%"/);
	assert.doesNotMatch(source, /letterSpacing: "normal"/);
	assert.match(source, /### Scope and source authority/);
	assert.match(source, /Persuade、Operate、Read 或 Experience mode/);
	assert.match(source, /Truth Before Decoration/);
	assert.match(
		source,
		/touch-dominant、icon-only 與關鍵動作的實際命中區至少 44×44px/,
	);
	assert.match(
		source,
		/首頁與相鄰品牌頁有暖色 route palette，而且現況已有 dark adaptation/,
	);
	assert.match(source, /Card isPressable.*沒有 Enter／Space 啟動/);
	assert.match(source, /Textarea.*Select.*aria-describedby/);
	assert.doesNotMatch(source, /light-only warm palette/);

	let lastIndex = -1;
	for (const heading of headings) {
		const index = source.indexOf(heading);
		assert.ok(index > lastIndex, `${heading} must exist in canonical order`);
		lastIndex = index;
	}
});

test("the detailed guidelines defer to the canonical root document", async () => {
	const source = await readFile(guidelinesUrl, "utf8");

	assert.match(source, /\[`DESIGN\.md`\]\(\.\.\/DESIGN\.md\)/);
	assert.match(source, /00-color-config\.css/);
});

test("the Impeccable sidecar matches the canonical document", async () => {
	const [raw, designSource] = await Promise.all([
		readFile(sidecarUrl, "utf8"),
		readFile(rootDesignUrl, "utf8"),
	]);
	const sidecar = JSON.parse(raw) as {
		schemaVersion?: number;
		generatedAt?: string;
		title?: string;
		extensions?: {
			colorMeta?: Record<string, { canonical?: string; role?: string }>;
			breakpoints?: Array<{ name?: string; value?: string }>;
		};
		components?: Array<{
			name?: string;
			html?: string;
			css?: string;
		}>;
		narrative?: {
			northStar?: string;
			overview?: string;
			keyCharacteristics?: string[];
			rules?: Array<{ name?: string; body?: string }>;
			dos?: string[];
			donts?: string[];
		};
	};
	const colorBlock = designSource.match(
		/\ncolors:\n([\s\S]*?)\ntypography:/,
	)?.[1];
	assert.ok(colorBlock, "DESIGN.md must expose a frontmatter color block");
	const frontmatterColors = Object.fromEntries(
		[...colorBlock.matchAll(/^ {2}([a-z0-9-]+): "([^"]+)"$/gm)].map((match) => [
			match[1],
			match[2],
		]),
	);

	assert.equal(sidecar.schemaVersion, 2);
	assert.equal(sidecar.title, "Design System: Hawkr");
	assert.equal(sidecar.narrative?.northStar, "深入職人故事的市集");
	assert.ok(!Number.isNaN(Date.parse(sidecar.generatedAt ?? "")));
	assert.doesNotMatch(raw, /route-navigation-signature/);
	assert.equal(
		sidecar.extensions?.colorMeta?.["brand-primary-strong"]?.role,
		"primary",
	);
	assert.equal(
		sidecar.extensions?.colorMeta?.["brand-secondary-soft"]?.role,
		"secondary",
	);
	assert.equal(
		sidecar.extensions?.colorMeta?.["brand-secondary"]?.role,
		"secondary",
	);
	assert.equal(
		sidecar.extensions?.colorMeta?.["status-info"]?.role,
		"tertiary",
	);
	assert.deepEqual(
		Object.fromEntries(
			Object.entries(sidecar.extensions?.colorMeta ?? {}).map(([key, meta]) => [
				key,
				meta.canonical,
			]),
		),
		frontmatterColors,
	);

	assert.deepEqual(sidecar.extensions?.breakpoints, [
		{ name: "public-nav-tight", value: "1280px" },
		{ name: "public-nav-collapse-inclusive", value: "840px" },
		{
			name: "mobile-bottom-nav-exclusive",
			value: "not (min-width: 840px)",
		},
		{ name: "public-nav-flat", value: "768px" },
	]);

	assert.deepEqual(
		sidecar.components?.map((component) => component.name),
		[
			"Primary Button",
			"Secondary Button",
			"Quiet Button",
			"Default Input",
			"Default Card",
			"Semantic Chip",
			"Tabs",
			"Searchable Dropdown",
			"Dialog / Sheet Panel",
			"Public Navigation Shell",
		],
	);
	for (const component of sidecar.components ?? []) {
		assert.match(component.html ?? "", /class="ds-/);
		assert.match(component.css ?? "", /\.ds-/);
	}

	assert.ok(
		designSource.includes(
			sidecar.narrative?.overview ?? "__missing_overview__",
		),
		"sidecar overview must be copied from DESIGN.md",
	);
	const characteristicBlock = designSource.match(
		/\*\*Key Characteristics:\*\*\n\n([\s\S]*?)\n\n\*\*Truth Before Decoration/,
	)?.[1];
	assert.ok(characteristicBlock);
	assert.deepEqual(
		sidecar.narrative?.keyCharacteristics,
		[...characteristicBlock.matchAll(/^- (.+)$/gm)].map((match) => match[1]),
	);

	const markdownRules = [
		...designSource.matchAll(/^\*\*(The .+? Rule)\.\*\* (.+)$/gm),
	].map((match) => ({ name: match[1], body: match[2] }));
	assert.deepEqual(
		sidecar.narrative?.rules?.map(({ name, body }) => ({ name, body })),
		markdownRules,
	);

	assert.deepEqual(
		sidecar.narrative?.dos,
		[...designSource.matchAll(/^- \*\*Do\*\* (.+)$/gm)].map(
			(match) => `Do ${match[1]}`,
		),
	);
	assert.deepEqual(
		sidecar.narrative?.donts,
		[...designSource.matchAll(/^- \*\*Don't\*\* (.+)$/gm)].map(
			(match) => `Don't ${match[1]}`,
		),
	);
});
