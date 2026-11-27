import type { MetadataRoute } from "next";
import { CANARY_PATH } from "@/lib/canary/sentinel";
import { getEnv } from "@/lib/config/env";
import { siteUrl } from "@/lib/seo/urls";

/**
 * Pure builder (tested directly). Driven by the single launch gate
 * (env.seoIndexable): closed — non-prod flavors AND un-launched prod — blocks
 * everything so nothing gets indexed before launch; open allows all but /api,
 * the cache canary sentinel, and points crawlers at the sitemap. Private app
 * routes are NOT disallowed here — they carry <meta robots noindex> instead,
 * which requires them to stay crawlable. The sentinel is the opposite case: an
 * infrastructure probe with no reader value, so it is both disallowed here and
 * served with `x-robots-tag: noindex` (lib/canary/sentinel.ts).
 */
export function buildRobots(indexable: boolean): MetadataRoute.Robots {
	if (!indexable) {
		return { rules: { userAgent: "*", disallow: "/" } };
	}
	return {
		rules: {
			userAgent: "*",
			allow: "/",
			disallow: ["/api/", "/adm", CANARY_PATH],
		},
		sitemap: `${siteUrl()}/sitemap.xml`,
		host: siteUrl(),
	};
}

export default function robots(): MetadataRoute.Robots {
	return buildRobots(getEnv().seoIndexable);
}
