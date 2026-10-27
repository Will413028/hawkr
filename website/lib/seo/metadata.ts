import type { Metadata } from "next";
import { getEnv } from "@/lib/config/env";
import { type AppLocale, defaultLocale, isAppLocale } from "@/lib/intl/locales";
import { buildImageUrl } from "@/lib/media/cdn";
import { joinTitleSegments } from "./summary-rules";
import { absoluteUrl, languageAlternates, siteUrl } from "./urls";

/** 站名、預設標題與描述，每個語系一份；標題模板（`%s | 站名`）用這裡的站名。 */
interface SiteIdentity {
	name: string;
	defaultTitle: string;
	description: string;
}

const ZH_TW_IDENTITY: SiteIdentity = {
	name: "Hawkr",
	defaultTitle: "Hawkr — 在地職人的市集",
	description:
		"在地小農與職人的生鮮市集，當季蔬果、海鮮、肉品產地直送。帶你看見產地的日常，遇見每一天的新鮮。",
};

/**
 * 英文的品牌名、標題模板與預設標題來自術語表「品牌」一列（L0 文案，
 * `docs/plans/2026-09-29-a3-buyer-seller/step10e-L0-copy.md` D7）。術語表沒有英文描述，沿用中文的，
 * 英文發佈前由發佈門檻（沒有未翻譯文字）補上。
 */
const EN_IDENTITY: SiteIdentity = {
	name: "Hawkr",
	defaultTitle: "Hawkr — A marketplace for local artisans",
	description: ZH_TW_IDENTITY.description,
};

const SITE_IDENTITY: Record<AppLocale, SiteIdentity> = {
	"zh-TW": ZH_TW_IDENTITY,
	en: EN_IDENTITY,
};

function siteIdentity(locale: AppLocale): SiteIdentity {
	return SITE_IDENTITY[locale];
}

/** 這個語系的站名：標題模板 `%s | 站名`、og:siteName、JSON-LD 的 name、商品與店家標題的後綴。 */
export function siteNameFor(locale: AppLocale): string {
	return siteIdentity(locale).name;
}

/** 這個語系的預設標題：根 layout 的 `title.default`，首頁的 `{ absolute }` 用同一個。 */
export function defaultTitleFor(locale: AppLocale): string {
	return siteIdentity(locale).defaultTitle;
}

/** zh-TW 的站名，只給沒有語系的場合（找不到任何路由的 404，`app/global-not-found.tsx`）。 */
export const SITE_NAME = ZH_TW_IDENTITY.name;

/** Open Graph 的 `og:locale`（`language_TERRITORY`）。 */
const OG_LOCALE: Record<AppLocale, string> = { "zh-TW": "zh_TW", en: "en_US" };

/** 路由參數的 `locale` 轉成 AppLocale；layout 已經對不認得的語系回 404，這裡只為型別而退回預設語系。 */
export function localeFromParam(raw: string): AppLocale {
	return isAppLocale(raw) ? raw : defaultLocale;
}

/**
 * The site-wide share card, served by the `app/opengraph-image.tsx` file
 * convention. Root-relative on purpose: `resolveImages` runs it through
 * `metadataBase`, so it lands on the right origin per flavor.
 */
export const SOCIAL_IMAGE_PATH = "/opengraph-image";

/**
 * `openGraph.images` for any page that declares an `openGraph` of its own.
 * Falls back to the site card, so the value is never empty.
 *
 * Why a page cannot just omit `images` and inherit: metadata is merged segment
 * by segment, and a segment that declares `openGraph` REPLACES the parent's
 * resolved one wholesale rather than merging into it (`mergeMetadata`, case
 * 'openGraph'). The `opengraph-image` file convention is attached only to the
 * segment whose own directory holds the file — the root — and is folded in
 * only there (`mergeStaticMetadata`). So the file convention reaches exactly
 * the pages that declare NO openGraph at all; every page that declares one to
 * get a page-specific og:url silently drops the share image on the way past.
 * `images: []` is the same thing spelled differently — the key exists, so the
 * root's images are gone and nothing replaces them.
 *
 * Measured before this helper existed: 11 of the site's 96 metadata-bearing
 * routes served NO og:image and no twitter:image, the home page among them —
 * the single most-shared URL on the site. Nothing was visibly wrong on any of
 * them, because a missing share image only shows up in someone else's chat
 * window.
 *
 * Routing every declaration through here means a page opts into the site card
 * by writing nothing, and `tests/seo-social-image.test.ts` fails the build if a
 * new `openGraph` block spells `images` any other way.
 *
 * The root layouts are in this position too (`rootSocialImage`): they sit one
 * segment BELOW `app/opengraph-image.tsx`, so the file convention does not
 * reach the pages that inherit their `openGraph`.
 */
export function socialImages(image?: string | null): { url: string }[] {
	return [{ url: image?.trim() || SOCIAL_IMAGE_PATH }];
}

/**
 * The site card as the root layouts name it: the same picture, with the
 * description the `opengraph-image` file convention would have published for it
 * (the route's `size`, `contentType` and `alt` exports; `tests/seo-social-image.test.ts`
 * keeps the two equal), so `og:image:width/height/type/alt` and the twitter
 * mirror of them are on every page that declares no `openGraph` of its own.
 * Only the content hash the file convention appends to the URL is not reproduced.
 */
function rootSocialImage(alt: string) {
	return {
		...socialImages()[0],
		width: 1200,
		height: 630,
		type: "image/png",
		alt,
	};
}

/** The seller media fields that can stand in for a store's share picture. */
export interface SellerSocialImageSource {
	/** Store cover objectKey (`coverKey` on the wire — see lib/api `fetchSeller`). */
	coverImage?: string | null;
	/** Store logo objectKey (`logoUrl` on the wire, normalised to `logo_url`). */
	logo_url?: string | null;
}

/**
 * The one picture that represents a seller on a share card, for EVERY route
 * under `/sellers/{id}` — the storefront index and each of its tabs.
 *
 *   cover  → `banner` preset (a wide store photo is what a share card wants)
 *   logo   → `avatar` preset (a square mark; the banner preset would crop it)
 *   neither → "" so `socialImages()` falls through to the site card
 *
 * Why this is shared and not four expressions: the storefront index owned the
 * full cascade while `/reviews` carried a partial copy and `/posts` +
 * `/messages` carried a shorter one still, so a seller with a logo but no cover
 * shared as an avatar card from two tabs and as the generic site card from the
 * other two — the same store, four URLs, two different pictures (#436). The
 * cascade lives here so "which field represents this seller" has one answer.
 *
 * `bannerUrl` is deliberately NOT a step: it is not a field the seller API
 * returns. `/public/sellers/{id}` wires `coverKey`/`coverUrl` +
 * `logoKey`/`logoUrl` (gateway `handler.SellerDetail`) and nothing else, which
 * is why `caac25a5` removed the `bannerUrl` read from the storefront as a dead
 * fallback. Re-adding it would be re-adding a branch that can never be taken.
 */
export function sellerSocialImage(seller: SellerSocialImageSource): string {
	const cover = seller.coverImage?.trim();
	if (cover) return buildImageUrl(cover, "banner");
	return buildImageUrl(seller.logo_url, "avatar");
}

/**
 * Root-level metadata, applied once per root layout (app/[locale]/layout.tsx,
 * app/(delegated)/layout.tsx). Establishes metadataBase, the title template
 * (per locale), default OpenGraph/Twitter, and
 * launch-gated indexability: only indexable when prod AND the launch flag is on
 * (env.seoIndexable) — web-dev, and un-launched prod, all stay
 * noindex. Pre-launch root uses follow:false (suppress all crawling, not just indexing);
 * per-route noindexMetadata uses follow:true so internal links stay discoverable.
 *
 * The share picture: `app/opengraph-image.tsx` is a file convention, and Next folds
 * it into the metadata of the segment whose directory holds it (`app/`) only. The
 * layouts that call this are one segment below (`app/[locale]/`, `app/(delegated)/`),
 * and a segment that declares `openGraph` replaces its parent's resolved one
 * wholesale — so without `images` here every page that declares no `openGraph` of
 * its own (the account, checkout, search, login and seller-center pages, the
 * route-level 404s) lost its og:image and twitter:image. They are named here, and
 * `twitter` deliberately gets none: Next fills a card's images from `openGraph`
 * only while the card has none of its own, so a root `twitter.images` would pin
 * every page that does not declare `twitter` — a product with its own photo —
 * to the site card.
 *
 * `shareImage: false` is for the one caller that shares the file's segment
 * (`app/global-not-found.tsx`): there the file convention publishes the picture,
 * with its content hash, and an `images` key would switch it off.
 */
export function buildRootMetadata(
	locale: AppLocale,
	options: { shareImage?: boolean } = {},
): Metadata {
	const indexable = getEnv().seoIndexable;
	const url = siteUrl();
	const identity = siteIdentity(locale);
	return {
		metadataBase: new URL(url),
		title: {
			template: `%s | ${identity.name}`,
			default: identity.defaultTitle,
		},
		description: identity.description,
		applicationName: identity.name,
		// Shape and identity only — deliberately no title/description, for the
		// same reason as `twitter` below and one level further up the same chain.
		//
		// Next resolves social titles by falling back: twitter → openGraph →
		// `metadata.title` (postProcessMetadata + inheritFromMetadata). Each link
		// only fires when the one before it is EMPTY. Segment merging replaces
		// `openGraph` wholesale when a segment declares one and otherwise leaves
		// the parent's in place, so a title spelled out here is inherited by every
		// descendant that does not declare its own openGraph — permanently
		// short-circuiting the fallback at the first link. That is precisely what
		// happened: 84 of the site's 98 routes served the root's DEFAULT_TITLE as
		// their og:title AND twitter:title, and the four legal pages plus /cart
		// served DEFAULT_DESCRIPTION as their og:description while their own
		// <meta name="description"> said something else entirely.
		//
		// Leaving both unset makes the fallback reach `metadata.title` /
		// `metadata.description`, so EVERY page — including ones not written yet —
		// gets a share card that matches its own <title>, with no per-page work
		// and nothing to remember. The root's own card is unaffected: this
		// layout's `title.default` / `description` above are what the fallback
		// lands on when a page supplies neither.
		openGraph: {
			type: "website",
			siteName: identity.name,
			locale: OG_LOCALE[locale],
			url: absoluteUrl("/", locale),
			...(options.shareImage === false
				? {}
				: { images: [rootSocialImage(identity.defaultTitle)] }),
		},
		// Card type only — same rule, next link down the chain. (Images follow
		// `openGraph.images`; see the note on the function.)
		twitter: { card: "summary_large_image" },
		robots: indexable
			? { index: true, follow: true }
			: { index: false, follow: false },
		// 圖示改走檔案慣例（app/icon.svg、app/apple-icon.tsx，以及賣家頁的
		// app/[locale]/(public)/sellers/[id]/apple-icon.tsx）。這裡若顯式設 metadata.icons
		// 會壓過檔案慣例，導致 apple-touch-icon 不被注入、分享連結圖示變白底。
	};
}

/** Marks a route subtree noindex while staying crawlable (equity still flows). */
export function noindexMetadata(): Metadata {
	return { robots: { index: false, follow: true } };
}

/**
 * The exact <title> a product page must serve: product, seller, brand — each
 * segment once, in that order, with no segment repeating its neighbour.
 *
 * Why the dedupe is data-driven and not a special case: the platform's own
 * store is named exactly the site name, so prod served
 * `「範例」Hawkr x 東河文旦 | Hawkr | Hawkr` for its products while
 * every third-party seller's title was fine. The AST guard in
 * tests/seo-title-brand-suffix.test.ts cannot see that — it stands in for
 * runtime data with a placeholder — so the rule lives here where a unit test
 * can feed it the colliding value (tests/product-seo-title.test.ts).
 *
 * Callers must pass the result through `{ absolute }` and reuse the same string
 * for og:title: the root `title.template` would otherwise append a second
 * brand, the very defect that `storeSeoTitle` (lib/seo/store-summary.ts) and this
 * function were extracted to stop.
 */
export function productSeoTitle(
	input: {
		name?: string | null;
		sellerName?: string | null;
	},
	locale: AppLocale,
): string {
	return joinTitleSegments([input.name, input.sellerName, siteNameFor(locale)]);
}

/**
 * Per-page metadata: page-specific title/description plus a self-canonical and
 * matching og:url, both built from lib/seo/urls (route-param based) so query
 * and tracking params can never leak into a canonical. Layered on top of the
 * root defaults from buildRootMetadata. A plain-string title flows through the
 * root "%s | Hawkr" template; pass { absolute } to opt out (home).
 */
export function buildPageMetadata(input: {
	title: string | { absolute: string };
	description: string;
	/** Root-relative, locale-free canonical path, e.g. "/faq". Query params are dropped. */
	path: string;
	/** The locale of the page being described: canonical, og:url and og:locale follow it. */
	locale: AppLocale;
	/**
	 * Optional og:image (absolute URL or root-relative path). Omitting it falls
	 * back to the site card — see `socialImages` for why inheriting is not an
	 * option once a page declares `openGraph`.
	 */
	image?: string;
	ogType?: "website" | "article" | "profile";
}): Metadata {
	const canonical = absoluteUrl(input.path, input.locale);
	const languages = languageAlternates(input.path);
	const ogTitle =
		typeof input.title === "string" ? input.title : input.title.absolute;
	return {
		title: input.title,
		description: input.description,
		alternates: { canonical, ...(languages ? { languages } : {}) },
		openGraph: {
			type: input.ogType ?? "website",
			siteName: siteIdentity(input.locale).name,
			locale: OG_LOCALE[input.locale],
			url: canonical,
			title: ogTitle,
			description: input.description,
			images: socialImages(input.image),
		},
	};
}

/**
 * `generateMetadata` for a page whose metadata is the same text in every locale except
 * the URLs: `export const generateMetadata = pageMetadata({ title, description, path })`.
 * `overrides` is shallow-merged over the result (the legal pages force `robots`).
 * Reads the `locale` route param, so the page stays static (no request data).
 */
export function pageMetadata(
	input: Omit<Parameters<typeof buildPageMetadata>[0], "locale">,
	overrides: Metadata = {},
) {
	return async ({
		params,
	}: {
		params: Promise<{ locale: string }>;
	}): Promise<Metadata> => ({
		...buildPageMetadata({
			...input,
			locale: localeFromParam((await params).locale),
		}),
		...overrides,
	});
}
