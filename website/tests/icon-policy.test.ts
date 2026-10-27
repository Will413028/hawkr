import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const ROOT = new URL("..", import.meta.url).pathname;
const SOURCE_DIRS = ["app", "components", "features", "lib", "styles"];
const SOURCE_EXTENSIONS = /\.(?:tsx?|mjs|css)$/;

function sourceFiles(dir: string): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(dir)) {
		const path = join(dir, entry);
		if (statSync(path).isDirectory()) found.push(...sourceFiles(path));
		else if (SOURCE_EXTENSIONS.test(entry)) found.push(path);
	}
	return found;
}

const files = SOURCE_DIRS.flatMap((dir) => sourceFiles(join(ROOT, dir)));

test("website/package.json 沒有 react-iconly 與 lottie-react", () => {
	const pkg = JSON.parse(readFileSync(join(ROOT, "package.json"), "utf8"));
	const deps = { ...pkg.dependencies, ...pkg.devDependencies };
	assert.equal(deps["react-iconly"], undefined);
	assert.equal(deps["lottie-react"], undefined);
	assert.equal(deps["@hawkr/icons"], "workspace:*");
});

test("原始碼不 import react-iconly 或 lottie-react，也不引用 assets/lottie/", () => {
	assert.ok(files.length > 0);
	const offenders = files.filter((file) => {
		const text = readFileSync(file, "utf8");
		return (
			/from\s+["'](?:react-iconly|lottie-react)["']/.test(text) ||
			/import\(\s*["'](?:react-iconly|lottie-react)["']/.test(text) ||
			text.includes("assets/lottie/")
		);
	});
	assert.deepEqual(offenders, []);
});

test("AppIcon 是 UI 圖示入口，畫圖交給 @hawkr/icons", () => {
	const path = join(ROOT, "components/shared/AppIcon.tsx");
	assert.ok(existsSync(path));
	assert.match(readFileSync(path, "utf8"), /from\s+["']@hawkr\/icons\//);
});
