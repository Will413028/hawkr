import { fetchSuggestions } from "@/lib/search/repo";

/**
 * Same-origin BFF proxy for keyword autocomplete. The browser can't hit the
 * backend `/public/search/suggest` directly (no CORS allow-origin for the
 * web host), so the homepage search popover calls this route instead. Server
 * runs `fetchSuggestions` (which reaches the backend with the server-side
 * API base) and returns the normalised list.
 */
export async function GET(request: Request) {
	const q = new URL(request.url).searchParams.get("q")?.trim() ?? "";
	if (!q) return Response.json({ suggestions: [] });
	// This proxy calls the gateway on one visitor's behalf, so it is charged to
	// that visitor rather than to the site's shared SSR subject (backend#2264).
	let suggestions: Awaited<ReturnType<typeof fetchSuggestions>>;
	try {
		suggestions = await fetchSuggestions(
			q,
			request.headers.get("cf-connecting-ip"),
			{ strict: true, signal: request.signal },
		);
	} catch {
		return Response.json(
			{ error: "搜尋建議暫時無法載入" },
			{ status: 502, headers: { "cache-control": "no-store" } },
		);
	}
	return Response.json(
		{ suggestions },
		// Brief shared cache — keystrokes for the same prefix dedupe at the
		// CDN/edge without making typeahead feel stale.
		{ headers: { "cache-control": "public, max-age=30" } },
	);
}
