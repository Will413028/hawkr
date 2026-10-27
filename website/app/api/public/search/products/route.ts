import { fetchProducts } from "@/lib/api";
import type { ProductRecord } from "@/lib/market/listFilters";
import { type SearchProductHit, searchProducts } from "@/lib/search/repo";

/** Max size of the merged "相關商品" list (keyword hits + same-category fill). */
const RELATED_CAP = 8;

/** Normalise a `/public/products` ProductRecord into the shape the search
 * popover's helpers (productImage / productMeta / productPrice) read, so
 * same-category fills render identically to keyword hits. */
function recordToHit(p: ProductRecord): SearchProductHit {
	const sellerName =
		p.seller?.name || p.sellerName || p.seller_name || undefined;
	return {
		...(p as unknown as SearchProductHit),
		id: p.id ?? "",
		name: p.name ?? "",
		// §231: the popover renders through resolveCdnUrl, so hand it the bare
		// objectKey. `/public/products` sends `imageKey` + `imageKeys` (SSOT) with
		// the Gateway-composed `imageUrl`/`imageUrls` twins.
		imageKey: p.imageKey || p.imageKeys?.[0] || undefined,
		sellerName,
		seller: p.seller
			? {
					name: p.seller.name,
					logoUrl: p.seller.logoUrl,
					city: p.seller.city,
					storeId: p.seller.storeId ?? p.seller.id,
				}
			: undefined,
		storeId: p.storeId ?? p.seller?.storeId ?? p.seller?.id ?? undefined,
		origin: p.origin || undefined,
	};
}

/**
 * Same-origin BFF proxy for product search. The homepage search popover runs
 * in the browser, so it calls this route instead of the backend directly to
 * avoid CORS and keep response normalisation in one server-side place.
 *
 * Beyond exact keyword hits, we top up the list with products from the top
 * hit's category so "相關商品" stays useful when the keyword itself has few
 * matches — the user expects to discover same-category items, not a near-empty
 * list.
 */
export async function GET(request: Request) {
	const params = new URL(request.url).searchParams;
	const q = params.get("q")?.trim() || params.get("keyword")?.trim() || "";
	if (!q) return Response.json({ records: [], total: 0 });

	const page = Number(params.get("page") || "1");
	const pageSize = Number(params.get("page_size") || "6");
	const categoryId = params.get("category_id") || undefined;
	const priceMin = params.get("price_min");
	const priceMax = params.get("price_max");
	const sortBy = params.get("sort_by") || undefined;
	const sortOrder = params.get("sort_order") || undefined;
	// Both upstream reads below are made on one visitor's behalf, so they are
	// charged to that visitor rather than to the shared SSR subject (backend#2264).
	const visitorIp = request.headers.get("cf-connecting-ip");

	let result: Awaited<ReturnType<typeof searchProducts>>;
	try {
		result = await searchProducts({
			q,
			visitorIp,
			page: Number.isFinite(page) && page > 0 ? page : 1,
			pageSize: Number.isFinite(pageSize)
				? Math.min(Math.max(pageSize, 1), 12)
				: 6,
			categoryId,
			priceMin:
				priceMin != null && priceMin !== "" ? Number(priceMin) : undefined,
			priceMax:
				priceMax != null && priceMax !== "" ? Number(priceMax) : undefined,
			sortBy,
			sortOrder,
			strict: true,
			signal: request.signal,
		});
	} catch {
		// An unavailable keyword source must not overwrite the buyer's cached
		// cards with a successful-looking empty result. Category fills below are
		// optional and may still fail without discarding real keyword matches.
		return Response.json(
			{ error: "商品搜尋暫時無法載入" },
			{ status: 502, headers: { "cache-control": "no-store" } },
		);
	}

	const merged: SearchProductHit[] = [...result.records];
	const seen = new Set(merged.map((r) => String(r.id)));

	// Top up with same-category products (keyword hits stay first). Honour an
	// explicit category filter if given, else take the top hit's category.
	const topCategoryId =
		categoryId || result.records.find((r) => r.categoryId)?.categoryId;
	if (topCategoryId && merged.length < RELATED_CAP && !request.signal.aborted) {
		try {
			const catJson = (await fetchProducts(
				"",
				1,
				RELATED_CAP,
				String(topCategoryId),
				undefined,
				{ visitorIp, signal: request.signal },
			)) as { data?: { records?: ProductRecord[] }; records?: ProductRecord[] };
			const catRecords = catJson?.data?.records || catJson?.records || [];
			for (const p of catRecords) {
				const id = String(p.id ?? "");
				if (!id || seen.has(id)) continue;
				seen.add(id);
				merged.push(recordToHit(p));
				if (merged.length >= RELATED_CAP) break;
			}
		} catch {
			// Same-category top-up is best-effort; keyword hits already returned.
		}
	}

	// 只吐 records（§16）。過去同時吐 products + records 兩份同樣的陣列，讓上游
	// 換 key 時沒有任何一端會壞——也就沒有任何一端會發現。
	return Response.json(
		{ records: merged, total: result.total },
		{ headers: { "cache-control": "public, max-age=20" } },
	);
}
