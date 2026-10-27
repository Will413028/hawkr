import { MarketProductsListPage } from "@/features/market/products";
import { localeFromParam, pageMetadata } from "@/lib/seo/metadata";

// Keep the already-rendered tab tree in Next's in-memory router cache so a tab
// revisit paints its last snapshot without another RSC flight (live data
// refreshes in the background on activation).
export const unstable_dynamicStaleTime = 300;

// ISR (#338): the server renders the canonical no-filter first page (SEO body +
// ItemList JSON-LD) with no request-time input; all filter / sort / tab params
// are read on the client (MarketProductsListClient), so the route no longer
// server-renders per request. Product data comes from the shared `unstable_cache`
// layer; `revalidate` is the static-shell TTL safety net, aligned with the
// product list data window.
export const revalidate = 60;

export const generateMetadata = pageMetadata({
	title: { absolute: "Hawkr 市集 — 生鮮蔬果產地直送" },
	description:
		"在 Hawkr 市集瀏覽當季蔬果、海鮮與肉品，由在地小農與職人產地直送，每日上架最新鮮、最當令的食材。你可以依分類、主題、產地與關鍵字輕鬆挑選，搭配商家認證與交易保障機制，安心比較與下單，採買產地到餐桌的每一份新鮮好味道，讓每一餐都吃得到當季的真實風味與安心。",
	path: "/market/products",
});

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const locale = localeFromParam((await params).locale);
	return (
		<>
			<h1 className="sr-only">Hawkr 市集 — 生鮮商品總覽</h1>
			<MarketProductsListPage locale={locale} />
		</>
	);
}
