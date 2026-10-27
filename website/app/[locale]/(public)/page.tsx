import type { Metadata } from "next";
import JsonLd from "@/components/seo/JsonLd";
import HomePage from "@/features/home/HomePage";
import {
	buildHomeSnapshot,
	homeSnapshotProductIds,
} from "@/features/home/homeSnapshot";
import {
	EMPTY_HOME_BOOTSTRAP,
	loadHomeRenderBootstrap,
} from "@/lib/api/home-bootstrap.server";
import { prerenderSafe } from "@/lib/api/prerender";
import { itemListSchema } from "@/lib/seo/jsonld";
import {
	buildPageMetadata,
	defaultTitleFor,
	localeFromParam,
} from "@/lib/seo/metadata";
import { productUrl } from "@/lib/seo/urls";

// Keep the already-rendered public tab tree in Next's in-memory BFCache. Live
// Home data has its own shorter freshness window and refreshes in the
// background on activation, so returning here can paint the stale snapshot
// immediately without blocking on another RSC flight.
export const unstable_dynamicStaleTime = 300;

// Regenerate the prerendered home HTML at most once a minute — the same
// live-commerce window the underlying product/marketplace cache entries use.
// A write-side POST /api/revalidate still busts sooner via cache tags.
export const revalidate = 60;

// 描述只有中文（術語表沒有英文描述，英文發佈前由發佈門檻補上）；標題是這個語系的預設標題。
const HOME_DESCRIPTION =
	"Hawkr 是在地小農與職人的線上市集，當季蔬果、海鮮、肉品由產地直送到你家餐桌。我們以商家認證與交易保障建立信任，讓你看見每樣食材背後的產地故事與用心，遇見每一天最新鮮的味道，安心採買、輕鬆下單，把產地到餐桌的距離拉得更近，也讓飲食回到對土地與生產者的理解與信任。";

export async function generateMetadata({
	params,
}: {
	params: Promise<{ locale: string }>;
}): Promise<Metadata> {
	const locale = localeFromParam((await params).locale);
	return buildPageMetadata({
		title: { absolute: defaultTitleFor(locale) },
		description: HOME_DESCRIPTION,
		path: "/",
		locale,
	});
}

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const locale = localeFromParam((await params).locale);
	// Server-render the home content into the initial HTML so crawlers (and the
	// first paint) see the real product / seller / post cards instead of an empty
	// shell filled in later by a client fetch.
	//
	// Strict-by-design: `loadHomeRenderBootstrap` throws on a backend outage, the
	// render fails, and ISR keeps serving the last good prerender rather than
	// caching a fresh empty homepage (SEO regression). `prerenderSafe` softens
	// ONLY the first `next build` (no prior prerender to keep); at runtime it
	// re-throws so the strict contract holds.
	const bootstrap = await loadHomeRenderBootstrap().catch(
		prerenderSafe(EMPTY_HOME_BOOTSTRAP),
	);
	const snapshot = buildHomeSnapshot(bootstrap);
	const productUrls = homeSnapshotProductIds(snapshot).map((id) =>
		productUrl(id, locale),
	);

	return (
		<>
			{/* ItemList of every product the home HTML lists — same SEO contract as
			    /market/products. Skipped when the (build-fallback) snapshot is empty
			    so a zero-item list is never emitted. */}
			{productUrls.length > 0 && <JsonLd data={itemListSchema(productUrls)} />}
			<HomePage snapshot={snapshot} />
		</>
	);
}
