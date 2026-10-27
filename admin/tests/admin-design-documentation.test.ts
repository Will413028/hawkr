import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const read = (path: string) => readFile(path, "utf8");

test("DESIGN.md keeps the canonical section order and implemented brand tokens", async () => {
	const [design, visualCss, globals] = await Promise.all([
		read("DESIGN.md"),
		read("src/styles/AdminVisualRefresh.css"),
		read("src/app/globals.css"),
	]);

	assert.ok(design.startsWith("---\n"), "DESIGN.md must start with YAML");
	const sections = [
		"## Overview",
		"## Colors",
		"## Typography",
		"## Layout",
		"## Elevation & Depth",
		"## Shapes",
		"## Components",
		"## Do's and Don'ts",
	];
	let previousIndex = -1;
	for (const section of sections) {
		const index = design.indexOf(section);
		assert.ok(index > previousIndex, `${section} must stay in canonical order`);
		previousIndex = index;
	}

	const adminTokens = {
		"primary-strong": "#b8441f",
		primary: "#e4572e",
		cream: "#f8f6ef",
		paper: "#fffdf7",
		ink: "#191c18",
		body: "#42493f",
		"muted-aa": "#5f675b",
		border: "#d9ddd2",
		success: "#2d7f50",
		warning: "#8a5a00",
		danger: "#b3261e",
		info: "#3f6f9e",
	};
	for (const [token, value] of Object.entries(adminTokens)) {
		assert.match(design, new RegExp(`${token}: ["']${value}["']`, "u"));
		assert.match(visualCss, new RegExp(`--admin-${token}: ${value}`, "u"));
	}
	assert.match(design, /delegated-secondary: ["']#f2a541["']/u);
	assert.match(globals, /--hk-secondary: #f2a541/u);
});

test("AI and human entry documents point to the same design authority", async () => {
	const [agents, readme, surface] = await Promise.all([
		read("AGENTS.md"),
		read("README.md"),
		read(".impeccable/surfaces/src-app-admin-merchants-id-operate-page-tsx.md"),
	]);

	assert.match(agents, /所有 UI／UX 任務先讀 \[`DESIGN\.md`\]/u);
	assert.match(agents, /產品與安全契約 → `DESIGN\.md` 通用規範/u);
	assert.match(readme, /\[`DESIGN\.md`\]\(\.\/DESIGN\.md\)/u);
	assert.match(readme, /\[`AGENTS\.md`\]\(\.\/AGENTS\.md\)/u);
	assert.match(readme, /\[`PRODUCT\.md`\]\(\.\/PRODUCT\.md\)/u);
	assert.match(surface, /一律遵循根目錄 `DESIGN\.md`/u);

	assert.doesNotMatch(readme, /Google OAuth\*\* 或 \*\*email\/password/u);
	assert.doesNotMatch(readme, /JWT 同時寫入 `localStorage`/u);
	assert.doesNotMatch(agents, /gateway-token/u);
	assert.match(agents, /後台網域由 Cloudflare Access 保護/u);
	assert.match(agents, /不照抄瀏覽器的 header/u);
});

test("Impeccable sidecar renders the documented system instead of generic UI", async () => {
	const sidecar = JSON.parse(await read(".impeccable/design.json")) as {
		schemaVersion: number;
		title: string;
		extensions: { colorMeta: Record<string, unknown> };
		components: Array<{ name: string; html: string; css: string }>;
		narrative: { northStar: string };
	};

	assert.equal(sidecar.schemaVersion, 2);
	assert.equal(sidecar.title, "Design System: Hawkr Admin");
	assert.equal(sidecar.narrative.northStar, "沉穩的生鮮營運台");
	assert.ok(sidecar.extensions.colorMeta["primary-strong"]);
	assert.ok(sidecar.extensions.colorMeta["delegated-secondary"]);
	assert.ok(sidecar.components.length >= 8);
	for (const component of sidecar.components) {
		assert.match(component.html, /class="ds-/u, component.name);
		assert.match(component.css, /\.ds-/u, component.name);
	}
});
