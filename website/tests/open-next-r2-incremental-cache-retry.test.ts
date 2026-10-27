import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

type R2Put = (key: string, value: string) => Promise<unknown>;

type R2IncrementalCache = {
	set(key: string, value: unknown, cacheType?: string): Promise<void>;
};

type Harness = {
	cache: R2IncrementalCache;
	delays: number[];
	errors: unknown[][];
	cleanup(): void;
};

let moduleSequence = 0;

async function loadR2IncrementalCache(
	put: R2Put,
	{ random = 0 }: { random?: number } = {},
): Promise<Harness> {
	let source = await readFile(
		new URL(
			"../node_modules/@opennextjs/cloudflare/dist/api/overrides/incremental-cache/r2-incremental-cache.js",
			import.meta.url,
		),
		"utf8",
	);

	const replaceImport = (pattern: RegExp, replacement: string) => {
		const nextSource = source.replace(pattern, replacement);
		assert.notEqual(
			nextSource,
			source,
			`OpenNext R2 incremental-cache import not found: ${pattern}`,
		);
		source = nextSource;
	};

	replaceImport(
		/import \{ error \} from "@opennextjs\/aws\/adapters\/logger\.js";/,
		"const error = (...args) => __fixture.errors.push(args);",
	);
	replaceImport(
		/import \{ IgnorableError \} from "@opennextjs\/aws\/utils\/error\.js";/,
		"class IgnorableError extends Error {}",
	);
	replaceImport(
		/import \{ getCloudflareContext \} from "\.\.\/\.\.\/cloudflare-context\.js";/,
		"const getCloudflareContext = () => __fixture.context;",
	);
	replaceImport(
		/import \{ computeCacheKey, debugCache \} from "\.\.\/internal\.js";/,
		`const computeCacheKey = (key, options) =>
			(options.prefix ?? "incremental-cache") + "/" +
			(options.buildId ?? "no-build-id") + "/" + key + "." +
			(options.cacheType ?? "cache");
		 const debugCache = () => {};`,
	);

	const fixtureKey = `__open_next_r2_retry_test_${moduleSequence++}`;
	const delays: number[] = [];
	const errors: unknown[][] = [];
	const testMath = Object.create(Math) as Math;
	Object.defineProperty(testMath, "random", {
		value: () => random,
	});
	const fixture = {
		context: {
			env: {
				NEXT_INC_CACHE_R2_BUCKET: { put },
			},
		},
		delays,
		errors,
		math: testMath,
		setTimeout(callback: () => void, delayMs: number) {
			delays.push(delayMs);
			queueMicrotask(callback);
			return 0;
		},
	};
	const testGlobal = globalThis as typeof globalThis & Record<string, unknown>;
	testGlobal[fixtureKey] = fixture;

	source = `const __fixture = globalThis[${JSON.stringify(fixtureKey)}];
		const Math = __fixture.math;
		const setTimeout = __fixture.setTimeout;
		${source}`;

	const moduleUrl = `data:text/javascript;base64,${Buffer.from(source).toString("base64")}#${moduleSequence}`;
	const loaded = (await import(moduleUrl)) as {
		default: R2IncrementalCache;
	};

	return {
		cache: loaded.default,
		delays,
		errors,
		cleanup() {
			delete testGlobal[fixtureKey];
		},
	};
}

function transientR2Error(code: 10043 | 10058) {
	const detail =
		code === 10058
			? "Reduce your concurrent request rate for the same object"
			: "Service is temporarily unavailable";
	return new Error(`put: ${detail} (${code})`);
}

for (const code of [10043, 10058] as const) {
	test(`OpenNext R2 retries transient ${code} writes`, async () => {
		let attempts = 0;
		const harness = await loadR2IncrementalCache(async () => {
			attempts += 1;
			if (attempts === 1) throw transientR2Error(code);
		});

		try {
			await harness.cache.set("/feed", { value: "fresh" });

			assert.equal(attempts, 2);
			assert.equal(harness.delays.length, 1);
			assert.equal(harness.errors.length, 0);
			if (code === 10058) {
				assert.ok(
					harness.delays[0] >= 1_000,
					"same-key R2 throttling must wait at least one second",
				);
			}
		} finally {
			harness.cleanup();
		}
	});
}

test("OpenNext R2 retry is exponential, jittered, and bounded to three attempts", async () => {
	let attempts = 0;
	const harness = await loadR2IncrementalCache(
		async () => {
			attempts += 1;
			throw transientR2Error(10043);
		},
		{ random: 0.999 },
	);

	try {
		await harness.cache.set("/market/products", { value: "fresh" });

		assert.equal(attempts, 3);
		assert.equal(harness.delays.length, 2);
		assert.ok(harness.delays[0] >= 250 && harness.delays[0] < 313);
		assert.ok(harness.delays[1] >= 500 && harness.delays[1] < 625);
		assert.equal(harness.errors.length, 1);
	} finally {
		harness.cleanup();
	}
});

test("OpenNext R2 only retries exact transient codes at the message suffix", async () => {
	for (const error of [
		new Error("put: metadata is invalid (10012)"),
		new Error("put: saw (10058) before trailing text"),
		Object.assign(new Error("put: rate limited"), { code: 10058 }),
	]) {
		let attempts = 0;
		const harness = await loadR2IncrementalCache(async () => {
			attempts += 1;
			throw error;
		});

		try {
			await harness.cache.set("/", { value: "fresh" });

			assert.equal(attempts, 1);
			assert.deepEqual(harness.delays, []);
			assert.equal(harness.errors.length, 1);
		} finally {
			harness.cleanup();
		}
	}
});

test("OpenNext R2 single-flights identical same-isolate writes", async () => {
	let releasePut: (() => void) | undefined;
	let attempts = 0;
	const firstPut = new Promise<void>((resolve) => {
		releasePut = resolve;
	});
	const harness = await loadR2IncrementalCache(async () => {
		attempts += 1;
		await firstPut;
	});

	try {
		const first = harness.cache.set("/feed", { value: "same" });
		const second = harness.cache.set("/feed", { value: "same" });
		await Promise.resolve();

		assert.equal(attempts, 1);
		releasePut?.();
		await Promise.all([first, second]);
		assert.equal(attempts, 1);
	} finally {
		releasePut?.();
		harness.cleanup();
	}
});

test("OpenNext R2 preserves a newer same-key value behind the in-flight write", async () => {
	let releaseFirstPut: (() => void) | undefined;
	const firstPut = new Promise<void>((resolve) => {
		releaseFirstPut = resolve;
	});
	const bodies: string[] = [];
	const harness = await loadR2IncrementalCache(async (_key, body) => {
		bodies.push(body);
		if (bodies.length === 1) await firstPut;
	});

	try {
		const first = harness.cache.set("/feed", { value: "old" });
		const second = harness.cache.set("/feed", { value: "new" });
		await Promise.resolve();

		assert.equal(bodies.length, 1);
		releaseFirstPut?.();
		await Promise.all([first, second]);
		assert.deepEqual(
			bodies.map((body) => JSON.parse(body).value),
			["old", "new"],
		);
	} finally {
		releaseFirstPut?.();
		harness.cleanup();
	}
});
