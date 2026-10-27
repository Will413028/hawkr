import type { MetadataRoute } from "next";
import { type AppLocale, publishedFor, type Surface } from "@/lib/intl/locales";
import { surfaceOfPath } from "@/lib/intl/paths";
import { INDEXABLE_STATIC_ROUTES, isIndexable } from "@/lib/seo/indexability";
import { absoluteUrl, languageAlternates } from "@/lib/seo/urls";

/**
 * Static public routes → sitemap entries (pure; tested directly). Dynamic
 * product / seller URLs are added in a later PR once full-enumeration
 * endpoints + generateSitemaps sharding (50k cap) are wired — they need real
 * lastModified from entity updatedAt, which the current recommended-only
 * endpoints don't provide.
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
 * own — the entity entries above will carry `updatedAt`.
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

export default function sitemap(): MetadataRoute.Sitemap {
	return staticSitemapEntries();
}
