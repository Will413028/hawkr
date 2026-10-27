import { tryToParsePath } from "next/dist/lib/try-to-parse-path";
import { config } from "../../middleware";

/**
 * The regular expression Next compiles `config.matcher` into (the one the build writes to
 * `.next/server/middleware-manifest.json`). Tests assert on this, not on the raw pattern string:
 * path-to-regexp wraps it (`^(?:\/(…))[\/#\?]?$`) and the wrapping changes which paths match.
 */
export function compiledMatcher(): RegExp {
	const [source] = config.matcher;
	const compiled = tryToParsePath(source);
	if (!compiled.regexStr) {
		throw new Error(`config.matcher does not compile: ${source}`);
	}
	return new RegExp(compiled.regexStr);
}
