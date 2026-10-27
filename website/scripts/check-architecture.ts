#!/usr/bin/env node

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.cwd();
const failures = [];

function walk(dir, predicate = () => true) {
	const out = [];
	for (const entry of readdirSync(join(root, dir))) {
		if (entry === "node_modules" || entry === ".next" || entry === ".git") {
			continue;
		}
		const abs = join(root, dir, entry);
		const rel = relative(root, abs);
		const stat = statSync(abs);
		if (stat.isDirectory()) out.push(...walk(rel, predicate));
		else if (predicate(rel)) out.push(rel);
	}
	return out;
}

function read(path) {
	return readFileSync(join(root, path), "utf8");
}

function fail(message) {
	failures.push(message);
}

const appTsx = walk("app", (path) => path.endsWith(".tsx"));
const appClients = appTsx.filter((path) => path.endsWith("Client.tsx"));
if (appClients.length > 0) {
	fail(
		`app/ must not own route-adjacent Client files:\n${appClients.join("\n")}`,
	);
}

for (const path of appTsx) {
	const text = read(path);
	if (/from\s+["']\.\.?\//.test(text)) {
		fail(`${path} imports route-local code; move implementation to features/.`);
	}
}

const codeFiles = walk(
	".",
	(path) =>
		/\.(ts|tsx|mjs|js)$/.test(path) &&
		!path.startsWith("node_modules/") &&
		!path.startsWith(".next/") &&
		!path.startsWith(".open-next/"),
);

const allowedLibRootFiles = new Set(["README.md"]);
const libRootFiles = readdirSync(join(root, "lib"))
	.filter((entry) => statSync(join(root, "lib", entry)).isFile())
	.filter((entry) => !allowedLibRootFiles.has(entry));
if (libRootFiles.length > 0) {
	fail(
		`lib/ root should stay as navigation only. Move files into owned subfolders:\n${libRootFiles
			.map((entry) => `lib/${entry}`)
			.join("\n")}`,
	);
}

const strayCssFiles = walk(
	".",
	(path) =>
		path.endsWith(".css") &&
		/^(app|components|features|lib)\//.test(path) &&
		!path.startsWith("node_modules/") &&
		!path.startsWith(".next/"),
);
if (strayCssFiles.length > 0) {
	fail(
		`CSS files should live under styles/ during the Tailwind migration:\n${strayCssFiles.join(
			"\n",
		)}`,
	);
}

const retiredLibImports = [
	"auth-client",
	"auth-context",
	"auth-redirect",
	"auth-routes",
	"auth-storage",
	"phone-auth",
	"assets",
	"cdn",
	"cn",
	"data",
	"site-content",
	"origin-content",
	"openapi-types",
	"public-assets",
	"theme-context",
];
const retiredLibImportPattern = new RegExp(
	`@/lib/(${retiredLibImports.join("|")})(["'/])`,
);

for (const path of codeFiles) {
	const text = read(path);
	if (retiredLibImportPattern.test(text)) {
		fail(`${path} imports a retired root lib path. Use the owning lib folder.`);
	}
	if (path !== "scripts/check-architecture.ts" && text.includes("@/app/")) {
		fail(`${path} imports from app/. Import the owning feature instead.`);
	}
	if (path.startsWith("lib/")) {
		const upwardRuntimeImport = text
			.split("\n")
			.some((line) =>
				/^import(?!\s+type).*@\/(app|features|components)\//.test(line),
			);
		if (upwardRuntimeImport) {
			fail(`${path} imports upward into UI/app layers.`);
		}
	}
	if (
		path !== "lib/browser/body-scroll-lock.ts" &&
		/(?:document\.(?:body|documentElement)|\b(?:body|html|root))\.style\.(?:overflow|overscrollBehavior|position|top|left|right|width)\s*=/.test(
			text,
		)
	) {
		fail(
			`${path} writes global scroll styles directly. Use acquireBodyScrollLock() so overlapping overlays cannot strand the page.`,
		);
	}
	if (
		path !== "scripts/check-architecture.ts" &&
		path !== "open-next.config.ts" &&
		!path.startsWith("tests/") &&
		/\brevalidatePath\s*\(/.test(text)
	) {
		fail(
			`${path} calls revalidatePath(), but OpenNext filters implicit path tags with softTagFilter. Use explicit cache tags or remove that filter first.`,
		);
	}
}

const tailwindConvertedUi = [
	"components/ui/Button.tsx",
	"components/ui/Badge.tsx",
	"components/ui/Chip.tsx",
	"components/ui/Card.tsx",
];
for (const path of tailwindConvertedUi) {
	const text = read(path);
	if (/hkui-(btn|badge|chip|card)/.test(text)) {
		fail(
			`${path} should stay Tailwind-first and not depend on hkui CSS classes.`,
		);
	}
}

if (failures.length > 0) {
	console.error(failures.join("\n\n"));
	process.exit(1);
}

console.log("Architecture checks passed.");
