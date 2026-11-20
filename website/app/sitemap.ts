import * as Sentry from "@sentry/nextjs";
import type { MetadataRoute } from "next";
import { getCachedSitemapStores } from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { type AppLocale, publishedFor, type Surface } from "@/lib/intl/locales";
import { surfaceOfPath } from "@/lib/intl/paths";
import type { StoreSitemapRow } from "@/lib/seller/list-stores";
import { INDEXABLE_STATIC_ROUTES, isIndexable } from "@/lib/seo/indexability";
import { absoluteUrl, languageAlternates } from "@/lib/seo/urls";

/**
 * Static public routes → sitemap entries (pure; tested directly). Store URLs
 * come from `storeSitemapEntries` (enumerated by `ListStores`); product URLs
 * are not listed yet.
 *
 * One sitemap file, no sharding: past `SITEMAP_SHARD_THRESHOLD` stores an error
 * is reported (the trigger to add `generateSitemaps`), the entries are still all emitted.
 *
 * One entry per PUBLISHED locale of each route (lib/intl/locales.ts): an
 * unpublished `/en` is never announced, and a non-default locale's UGC pages
 * (`isIndexable`) are left out. With a single published locale the entries are
 * exactly the zh-TW list and carry no `alternates`.
 *
 * No `lastModified` here, on purpose. These are hand-written pages with no
 * content date the build knows about; stamping `new Date()` on them told
 * crawlers all 13 changed at the very same instant on every deploy (prod on
 * 2026-09-08 listed 13 identical timestamps), which is a signal Google learns
 * to ignore for the whole file. A date goes in only when it is the content's
 * own — store entries carry the store's `update_time`.
 */
export function staticSitemapEntries(
	published: (surface: Surface) => readonly AppLocale[] = publishedFor,
): MetadataRoute.Sitemap {
	return INDEXABLE_STATIC_ROUTES.flatMap((route) => {
		const locales = published(surfaceOfPath(route));
		const languages = languageAlternates(route, locales);
		return locales
			.filter((locale) => isIndexable(route, locale))
			.map((locale) => ({
				url: absoluteUrl(route, locale),
				changeFrequency: "daily" as const,
				priority: route === "/" ? 1 : 0.7,
				...(languages ? { alternates: { languages } } : {}),
			}));
	});
}

/** sitemap 單檔上限 50,000 筆，45,000 起要改成分檔。 */
export const SITEMAP_SHARD_THRESHOLD = 45_000;

/** 每家店每個已發佈語系一筆，規則同 `staticSitemapEntries`（UGC 頁只列預設語系）。 */
export function storeSitemapEntries(
	stores: readonly StoreSitemapRow[],
	published: (surface: Surface) => readonly AppLocale[] = publishedFor,
	report: (message: string) => void = (m) => {
		Sentry.captureMessage(m, "error");
	},
): MetadataRoute.Sitemap {
	if (stores.length > SITEMAP_SHARD_THRESHOLD) {
		report(
			`sitemap has ${stores.length} stores, over ${SITEMAP_SHARD_THRESHOLD}: shard it with generateSitemaps`,
		);
	}
	return stores.flatMap((store) => {
		const path = `/sellers/${encodeURIComponent(store.storeCode)}`;
		const locales = published(surfaceOfPath(path));
		const languages = languageAlternates(path, locales);
		return locales
			.filter((locale) => isIndexable(path, locale))
			.map((locale) => ({
				url: absoluteUrl(path, locale),
				...(store.updatedAtMs !== undefined
					? { lastModified: new Date(store.updatedAtMs) }
					: {}),
				changeFrequency: "daily" as const,
				priority: 0.6,
				...(languages ? { alternates: { languages } } : {}),
			}));
	});
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
	const stores = await getCachedSitemapStores().catch((err: unknown) => {
		Sentry.captureException(err);
		return prerenderSafe<StoreSitemapRow[]>([])(err);
	});
	return [...staticSitemapEntries(), ...storeSitemapEntries(stores)];
}
