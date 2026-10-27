import { revalidateTag, unstable_cache } from "next/cache";
import { type NextRequest, NextResponse } from "next/server";
import { isRevalidationSecretAuthorized } from "@/lib/api/revalidate-request";
import {
	buildSentinelBody,
	buildSentinelEntry,
	CANARY_CACHE_KEY,
	CANARY_TAG,
	CANARY_TTL_SECONDS,
	SENTINEL_RESPONSE_HEADERS,
} from "@/lib/canary/sentinel";

/**
 * Freshness canary sentinel (see lib/canary/sentinel.ts for what it measures).
 *
 *   GET  /sentinel  → the cached entry + its age at the serving PoP
 *   POST /sentinel  → purge the canary tag (x-revalidate-secret, same secret as
 *                     POST /api/revalidate)
 *
 * The writer/prober is `cloudflare/canary` (Cron Trigger); `cloudflare/canary/
 * probe.mjs` runs the same read loop from anywhere else. The purge deliberately
 * does NOT go through /api/revalidate: that route expands entity tags into the
 * aggregate tags real pages use, and the canary must never touch a visitor-
 * facing cache entry.
 *
 * The route is dynamic on purpose. Only the `unstable_cache` entry below is
 * cached — that is the object under study — while the response itself stays
 * per-request so `servedAt`/`ageMs` are always true.
 */
const readSentinelEntry = unstable_cache(
	async () =>
		buildSentinelEntry(
			Date.now(),
			process.env.OPEN_NEXT_BUILD_ID ?? process.env.DEPLOYMENT_ID,
		),
	CANARY_CACHE_KEY,
	{ tags: [CANARY_TAG], revalidate: CANARY_TTL_SECONDS },
);

export const dynamic = "force-dynamic";

export async function GET() {
	const entry = await readSentinelEntry();
	return NextResponse.json(buildSentinelBody(entry, Date.now()), {
		headers: SENTINEL_RESPONSE_HEADERS,
	});
}

export async function POST(req: NextRequest) {
	if (
		!isRevalidationSecretAuthorized(
			req.headers.get("x-revalidate-secret"),
			process.env.REVALIDATE_SECRET,
		)
	) {
		return NextResponse.json(
			{ code: 401, message: "unauthorized" },
			{ status: 401, headers: SENTINEL_RESPONSE_HEADERS },
		);
	}
	try {
		// `expire: 0` = the profile POST /api/revalidate gives an entity tag: the
		// entry is dead immediately rather than served stale while it refreshes, so
		// the measured lag is "purge → new content", not "purge → new content
		// eventually". The canary must measure the floor, so it never takes the
		// deferred profile the aggregate lists get.
		revalidateTag(CANARY_TAG, { expire: 0 });
	} catch {
		return NextResponse.json(
			{ code: 500, message: "revalidation failed" },
			{ status: 500, headers: SENTINEL_RESPONSE_HEADERS },
		);
	}
	return NextResponse.json(
		{
			code: 0,
			message: "success",
			data: { tag: CANARY_TAG, purgedAt: Date.now() },
		},
		{ headers: SENTINEL_RESPONSE_HEADERS },
	);
}
