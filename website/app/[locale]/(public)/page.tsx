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

export const unstable_dynamicStaleTime = 300;

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
	const bootstrap = await loadHomeRenderBootstrap().catch(
		prerenderSafe(EMPTY_HOME_BOOTSTRAP),
	);
	const snapshot = buildHomeSnapshot(bootstrap);
	const productUrls = homeSnapshotProductIds(snapshot).map((id) =>
		productUrl(id, locale),
	);

	return (
		<>

			{productUrls.length > 0 && <JsonLd data={itemListSchema(productUrls)} />}
			<HomePage snapshot={snapshot} />
		</>
	);
}
