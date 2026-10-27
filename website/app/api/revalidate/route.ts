import { revalidateTag } from "next/cache";
import { type NextRequest, NextResponse } from "next/server";
import {
	applyRevalidation,
	isRevalidationSecretAuthorized,
	MAX_REVALIDATION_BODY_BYTES,
	parseRevalidateRequest,
} from "@/lib/api/revalidate-request";

/**
 * On-demand cache invalidation. The Java order/product service POSTs here on
 * price/stock/publish mutations with the shared secret + the affected entity
 * tags (e.g. ["product:123"], ["seller:456"]). The parser derives the domain
 * tags of the aggregates that embed that entity at this trusted boundary, so a
 * producer cannot accidentally invalidate only the entity page. Each
 * unstable_cache entry's `revalidate` window is the TTL safety net for when this
 * hasn't fired. Tag strings come from lib/seo/tags.ts.
 *
 *   POST /api/revalidate
 *   Header: x-revalidate-secret: <REVALIDATE_SECRET>
 *   Body:   {
 *     "tags": ["product:123", "seller:456"],
 *     "event_id": "inventory-123-revision-9"
 *   }
 *
 * How hard a tag expires follows its shape, not the caller (see
 * `revalidationProfileFor`): entity tags expire on the spot so the writer's own
 * next read shows their change, while the aggregate lists stay
 * stale-while-revalidate to keep OpenNext's regeneration dedupe and the
 * stale-if-error fallback.
 *
 * Two bounds stay honest even for the immediate half: the tag lookup is cached
 * per PoP for ~5s and the purge only drops the writer PoP's copy, so a reader
 * elsewhere can lag that long; and a 200 means "accepted", not "landed" — Next
 * flushes tag writes on `pendingWaitUntil` after the response leaves, which is
 * also why warming a page from inside this request would race the write instead
 * of helping. Checkout still validates price and inventory against the
 * authoritative backend.
 */
export async function POST(req: NextRequest) {
	const secretHeader = req.headers.get("x-revalidate-secret");
	const expectedSecret = process.env.REVALIDATE_SECRET;
	// Authenticate before reading the body so an unauthorised caller cannot make
	// the Worker parse an arbitrarily large JSON payload.
	if (!isRevalidationSecretAuthorized(secretHeader, expectedSecret)) {
		return NextResponse.json(
			{ code: 401, message: "unauthorized" },
			{ status: 401 },
		);
	}

	const declaredLength = Number(req.headers.get("content-length"));
	if (
		Number.isFinite(declaredLength) &&
		declaredLength > MAX_REVALIDATION_BODY_BYTES
	) {
		return NextResponse.json(
			{ code: 413, message: "payload too large" },
			{ status: 413 },
		);
	}
	const rawBody = await req.text().catch(() => "");
	if (
		new TextEncoder().encode(rawBody).byteLength > MAX_REVALIDATION_BODY_BYTES
	) {
		return NextResponse.json(
			{ code: 413, message: "payload too large" },
			{ status: 413 },
		);
	}
	let body: unknown = null;
	try {
		body = rawBody ? JSON.parse(rawBody) : null;
	} catch {
		// The parser below returns the same stable 400 contract as an absent body.
	}
	const result = parseRevalidateRequest(secretHeader, body, expectedSecret);
	if (!result.ok) {
		return NextResponse.json(
			{ code: result.status, message: result.error },
			{ status: result.status },
		);
	}
	try {
		applyRevalidation(result.tags, revalidateTag);
	} catch {
		return NextResponse.json(
			{ code: 500, message: "revalidation failed" },
			{ status: 500 },
		);
	}
	return NextResponse.json({
		code: 0,
		message: "success",
		data: {
			revalidated: result.tags,
			...(result.eventId ? { eventId: result.eventId } : {}),
		},
	});
}
