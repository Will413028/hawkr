import { connection } from "next/server";
import { getCachedStrictProductsPage } from "@/lib/api/cached";
import { getVisitorIp } from "@/lib/http/visitor-ip";
import { defaultLocale } from "@/lib/intl/locales";
import { fetchPostsPage } from "@/lib/posts/repo";

type HeadTab = "feed" | "market";
const FEED_REFRESH_DEADLINE_MS = 3_000;

function readRequest(
	url: string,
): { tab: HeadTab; page: number; pageToken: string } | null {
	const params = new URL(url).searchParams;
	const tab = params.get("tab");
	if (tab !== "feed" && tab !== "market") return null;
	const page = Number(params.get("page") ?? "1");
	if (!Number.isInteger(page) || page < 1 || page > 100) return null;
	const pageToken = params.get("pageToken") ?? "";
	if (pageToken.length > 2048) return null;
	return { tab, page, pageToken };
}

/**
 * Same-origin freshness transport for canonical public list pages.
 *
 * The browser response is never shared or persisted. Market retains its strict
 * Cache-Components page. Feed intentionally uses a bounded direct gateway read:
 * an intermittent gateway read stall must never hold a tab-refresh response
 * (or its parent Flight) open. Each browser query keeps its visible snapshot
 * when this endpoint returns 503 and applies a retry cooldown.
 */
export async function GET(request: Request) {
	await connection();
	const input = readRequest(request.url);
	if (!input) {
		return Response.json(
			{ code: 400, message: "invalid tab page" },
			{
				status: 400,
				headers: { "cache-control": "private, no-store" },
			},
		);
	}

	try {
		const data =
			input.tab === "feed"
				? await fetchPostsPage({
						pageToken: input.pageToken,
						pageSize: 12,
						signal: AbortSignal.timeout(FEED_REFRESH_DEADLINE_MS),
						strict: true,
						locale: defaultLocale,
						visitorIp: getVisitorIp(request) ?? undefined,
					})
				: await getCachedStrictProductsPage(input.page, 24);
		return Response.json(
			{
				schemaVersion: "tab-page.v1",
				tab: input.tab,
				data,
			},
			{
				headers: {
					"cache-control": "private, no-store",
					"x-content-type-options": "nosniff",
				},
			},
		);
	} catch {
		return Response.json(
			{ code: 503, message: "tab refresh unavailable" },
			{
				status: 503,
				headers: {
					"cache-control": "private, no-store",
					"retry-after": "10",
				},
			},
		);
	}
}
