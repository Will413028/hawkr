import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import tailwind from "@tailwindcss/postcss";
import postcss from "postcss";

/**
 * Tailwind v4's candidate scanner is a plain regex over raw file bytes — it does
 * not parse the language and therefore does not skip comments. Any text shaped
 * like a utility becomes a real CSS rule, wherever it appears: a JSDoc block, a
 * lint script's error message, a test's explanatory prose.
 *
 * `NotificationToast.tsx` already carried a comment warning about exactly this
 * (dead utilities resurrecting from comments, web#417) — and that warning was
 * itself written as `pt-[ … ]` (without the spaces), so it was one of the
 * offenders. Prose does not
 * enforce itself; this test does.
 *
 * The failure mode with a placeholder like `…` is worse than a resurrected dead
 * class: the generated declaration is `padding-top: …`, which is not valid CSS
 * at all. Lightning CSS then aborts with `Unexpected token Ident("…")` and
 * `styles/globals.css` fails to compile. `next build` tolerates it, so CI stays
 * green — but `next dev` does not, which is what `pnpm test:e2e` runs locally.
 * The result was ~98 local E2E failures reading as broken product code while CI
 * was green (2026-08-03).
 *
 * To quote a class shape in prose, put spaces inside the brackets —
 * `pt-[ … ]` — whitespace disqualifies it as a candidate.
 */
test("no source text leaks an unparseable Tailwind candidate into globals.css", async () => {
	const entry = new URL("../styles/globals.css", import.meta.url);
	const css = await readFile(entry, "utf8");

	const result = await postcss([tailwind()]).process(css, {
		from: "styles/globals.css",
	});

	const offenders = result.css
		.split("\n")
		.filter(
			(line) =>
				line.includes("\u2026") ||
				/@media \(width (?:<=|>=|<|>) [A-Za-z][\w-]*px\)/.test(line),
		)
		.slice(0, 10);

	assert.deepEqual(
		offenders,
		[],
		`generated CSS contains an unresolved placeholder, so Lightning CSS cannot parse it.\n` +
			`Find the source with:\n` +
			`  rg -n 'NNNpx|\\[[^]]*\u2026' --glob '!node_modules/**' .\n` +
			`and rewrite the quoted class with spaces inside the brackets.\n` +
			`Offending lines:\n${offenders.join("\n")}`,
	);
});
