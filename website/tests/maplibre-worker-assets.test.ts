import assert from "node:assert/strict";
import {
	mkdtempSync,
	readdirSync,
	readFileSync,
	rmSync,
	statSync,
	utimesSync,
} from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { prepareMapLibreWorker } from "../scripts/prepare-maplibre-worker.mjs";

const require = createRequire(import.meta.url);
const packageRoot = dirname(require.resolve("maplibre-gl/package.json"));
const { version } = JSON.parse(
	readFileSync(join(packageRoot, "package.json"), "utf8"),
);
const projectRoot = join(import.meta.dirname, "..");

test("prepares the exact installed worker with every adjacent module it imports", (t) => {
	const root = mkdtempSync(join(tmpdir(), "hawkr-maplibre-assets-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	const workerPath = prepareMapLibreWorker(root);
	assert.equal(
		workerPath,
		`/generated/maplibre/${version}/maplibre-gl-worker.mjs`,
	);
	const outputDirectory = dirname(join(root, "public", workerPath));
	const files = readdirSync(outputDirectory).sort();
	assert.deepEqual(files, ["maplibre-gl-shared.mjs", "maplibre-gl-worker.mjs"]);
	let adjacentImports = 0;
	for (const filename of files) {
		const prepared = readFileSync(join(outputDirectory, filename));
		assert.deepEqual(
			prepared,
			readFileSync(join(packageRoot, "dist", filename)),
			`${filename} must retain the installed security patch byte for byte`,
		);
		for (const match of prepared
			.toString()
			.matchAll(/\bfrom\s*["'](\.\/[^"']+)["']/g)) {
			adjacentImports++;
			assert.ok(
				files.includes(match[1].slice(2)),
				`${filename} imports missing sibling ${match[1]}`,
			);
		}
	}
	assert.ok(adjacentImports > 0, "exercise the real worker/shared dependency");
});

test("repeated Next configuration loads do not rewrite unchanged worker assets", (t) => {
	const root = mkdtempSync(join(tmpdir(), "hawkr-maplibre-idempotent-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	const workerPath = prepareMapLibreWorker(root);
	const outputDirectory = dirname(join(root, "public", workerPath));
	const historicalTime = new Date("2000-01-01T00:00:00.000Z");
	for (const filename of readdirSync(outputDirectory)) {
		utimesSync(join(outputDirectory, filename), historicalTime, historicalTime);
	}
	assert.equal(prepareMapLibreWorker(root), workerPath);
	for (const filename of readdirSync(outputDirectory)) {
		assert.equal(
			statSync(join(outputDirectory, filename)).mtimeMs,
			historicalTime.getTime(),
		);
	}
});

test("Next prepares ignored vendor artifacts and the map selects the same package version", () => {
	const config = readFileSync(join(projectRoot, "next.config.ts"), "utf8");
	const map = readFileSync(
		join(projectRoot, "features/seller-center/insights/TaiwanPriceMap.tsx"),
		"utf8",
	);
	assert.match(config, /prepareMapLibreWorker\(projectRoot\)/);
	assert.match(
		map,
		/maplibregl\.setWorkerUrl\(\s*`\/generated\/maplibre\/\$\{maplibregl\.getVersion\(\)\}\/maplibre-gl-worker\.mjs`/,
	);
	assert.match(
		readFileSync(join(projectRoot, ".gitignore"), "utf8"),
		/^\/public\/generated\/maplibre\/$/m,
	);
});
