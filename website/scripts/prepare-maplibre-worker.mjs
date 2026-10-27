import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const require = createRequire(import.meta.url);
const packageRoot = dirname(require.resolve("maplibre-gl/package.json"));
const { version } = JSON.parse(
	readFileSync(join(packageRoot, "package.json"), "utf8"),
);
const workerFiles = ["maplibre-gl-worker.mjs", "maplibre-gl-shared.mjs"];

/**
 * MapLibre 6's module worker imports its adjacent shared module. A bundler URL
 * import emits the worker as a raw asset without that sibling. Prepare both
 * exact installed files before Next snapshots public assets, for every build
 * and dev bundler. The package version keeps cached worker pairs immutable.
 * @param {string} projectRoot
 */
export function prepareMapLibreWorker(projectRoot) {
	if (typeof version !== "string" || !/^\d+\.\d+\.\d+$/.test(version)) {
		throw new Error("MapLibre worker assets require a stable package version");
	}
	const publicDirectory = `/generated/maplibre/${version}`;
	const outputDirectory = join(projectRoot, "public", publicDirectory);
	mkdirSync(outputDirectory, { recursive: true });
	for (const filename of workerFiles) {
		const source = readFileSync(join(packageRoot, "dist", filename));
		const destination = join(outputDirectory, filename);
		let existing;
		try {
			existing = readFileSync(destination);
		} catch (error) {
			if (error.code !== "ENOENT") throw error;
		}
		if (!existing?.equals(source)) writeFileSync(destination, source);
	}
	return `${publicDirectory}/maplibre-gl-worker.mjs`;
}
