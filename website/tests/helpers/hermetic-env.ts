import { fileURLToPath, pathToFileURL } from "node:url";

const GUARD = pathToFileURL(
	fileURLToPath(new URL("../setup/hermetic.mjs", import.meta.url)),
).href;

/**
 * The environment for a subprocess a test spawns. A child process does not inherit the preload the test
 * runner loaded into its parent (`--import`, Vitest's `setupFiles`), so without this it would run with no
 * guard at all: `next start` rendering against the dev flavor's default gateway, or a script that fetches.
 *
 * `NODE_OPTIONS` makes every Node child (and grandchild) load tests/setup/hermetic.mjs first: loopback API
 * base unless `extra` names a loopback stub of its own, and no connection to any other host.
 * tests/hermetic-guard.test.ts fails a test file that imports `child_process` without calling this.
 */
type Env = Record<string, string | undefined>;

export function hermeticEnv(extra: Env = {}, base: Env = process.env): Env {
	const nodeOptions = [base.NODE_OPTIONS, `--import ${GUARD}`]
		.filter(Boolean)
		.join(" ");
	return { ...base, ...extra, NODE_OPTIONS: nodeOptions };
}
