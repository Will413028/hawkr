const NAMES = ["API_BASE_URL", "NEXT_PUBLIC_API_BASE_URL"] as const;

/**
 * Removes the API-base pins tests/setup/hermetic.mjs puts in the environment, so `getEnv()` resolves the flavor's
 * default gateway; returns the function that puts them back. For the few tests whose subject IS that default
 * (they stub `fetch`, so nothing here makes a request).
 */
export function withoutApiBasePins(): () => void {
	const saved = NAMES.map((name) => [name, process.env[name]] as const);
	for (const name of NAMES) delete process.env[name];
	return () => {
		for (const [name, value] of saved) {
			if (value === undefined) delete process.env[name];
			else process.env[name] = value;
		}
	};
}

/**
 * Points both API-base variables at `base` (what a deployment does through its env, since prod has no default
 * gateway); returns the function that restores the previous values. For tests that stub `fetch`.
 */
export function withApiBase(base: string): () => void {
	const saved = NAMES.map((name) => [name, process.env[name]] as const);
	for (const name of NAMES) process.env[name] = base;
	return () => {
		for (const [name, value] of saved) {
			if (value === undefined) delete process.env[name];
			else process.env[name] = value;
		}
	};
}
