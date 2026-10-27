/**
 * schema.org JSON-LD builders. Pure functions: each takes already-extracted
 * primitives and returns a schema-dts WithContext<T>. URLs come only from
 * lib/seo/urls so JSON-LD url/@id always equal the page canonical. schema-dts
 * enforces required fields at compile time.
 *
 * `inLanguage` (the page's locale tag, `zh-TW` / `en`) is set on the site-authored
 * CreativeWorks that have it — WebSite and FAQPage; Product, Organization, ItemList
 * and BreadcrumbList have no such property, and ProfilePage (a seller's own,
 * always-zh-TW text) is deliberately left without one.
 */
import type {
	BreadcrumbList,
	FAQPage,
	ItemAvailability,
	ItemList,
	Organization,
	Product,
	ProfilePage,
	SearchAction,
	WebSite,
	WithContext,
} from "schema-dts";
import type { AppLocale } from "@/lib/intl/locales";
import { siteNameFor } from "./metadata";
import { absoluteUrl, siteUrl } from "./urls";

/** Sitewide publisher identity, named the way the site's `locale` version names itself. */
export function organizationSchema(
	locale: AppLocale,
): WithContext<Organization> {
	return {
		"@context": "https://schema.org",
		"@type": "Organization",
		name: siteNameFor(locale),
		url: siteUrl(),
		logo: `${siteUrl()}/brand/app-icon.svg`,
	};
}

/** Sitewide WebSite + sitelinks search box, for the site's `locale` version. */
export function webSiteSchema(locale: AppLocale): WithContext<WebSite> {
	// schema-dts 2.x does not model the "query-input" annotation property
	// (hyphenated key, not a standard schema.org prop). Cast the potentialAction
	// value to the closest schema-dts type and add the extra key via an
	// intersection so the public return type stays WithContext<WebSite>.
	const searchAction = {
		"@type": "SearchAction" as const,
		target: {
			"@type": "EntryPoint" as const,
			urlTemplate: `${absoluteUrl("/search", locale)}?q={search_term_string}`,
		},
		// schema.org requires the literal hyphenated key here.
		"query-input": "required name=search_term_string",
	} as unknown as SearchAction;
	return {
		"@context": "https://schema.org",
		"@type": "WebSite",
		name: siteNameFor(locale),
		url: absoluteUrl("/", locale),
		inLanguage: locale,
		potentialAction: searchAction,
	};
}

export interface ProductSchemaInput {
	name: string;
	/** Canonical product URL (from productUrl(id)). */
	url: string;
	description?: string;
	/** Absolute or root-relative image URL. */
	image?: string;
	/** Single price (omit if a range is given). */
	price?: number;
	/** Price range; takes precedence over `price`. */
	priceRange?: { low: number; high: number };
	/**
	 * Only what the wire actually reports (SKU / product stock). Leave it out
	 * when stock is unknown: an unreported quantity is not "in stock", and a
	 * guessed InStock is the kind of claim Merchant Center later flags.
	 */
	availability?: "InStock" | "OutOfStock";
	/** Backend publication policy is authoritative over physical SKU stock. */
	purchasable?: boolean;
	/** Emitted only when count > 0 (never fabricated). */
	rating?: { value: number; count: number };
}

export function productSchema(input: ProductSchemaInput): WithContext<Product> {
	// schema-dts requires ItemAvailability (a branded union), not a plain string.
	// Cast the constructed full-URL string to satisfy the type checker.
	const effectiveAvailability =
		input.purchasable === false ? "OutOfStock" : input.availability;
	const availability =
		effectiveAvailability === undefined
			? undefined
			: (`https://schema.org/${effectiveAvailability}` as ItemAvailability);

	// Build offers only when price data is actually present — never fabricate a
	// price of 0 when neither price nor priceRange was supplied.
	let offers:
		| {
				"@type": "AggregateOffer";
				lowPrice: number;
				highPrice: number;
				priceCurrency: string;
				availability?: ItemAvailability;
				url: string;
		  }
		| {
				"@type": "Offer";
				price: number;
				priceCurrency: string;
				availability?: ItemAvailability;
				url: string;
		  }
		| undefined;
	const availabilityField = availability === undefined ? {} : { availability };
	if (input.priceRange) {
		offers = {
			"@type": "AggregateOffer" as const,
			lowPrice: input.priceRange.low,
			highPrice: input.priceRange.high,
			priceCurrency: "TWD",
			...availabilityField,
			url: input.url,
		};
	} else if (typeof input.price === "number") {
		offers = {
			"@type": "Offer" as const,
			price: input.price,
			priceCurrency: "TWD",
			...availabilityField,
			url: input.url,
		};
	}

	const schema: WithContext<Product> = {
		"@context": "https://schema.org",
		"@type": "Product",
		name: input.name,
		url: input.url,
	};
	if (offers !== undefined) schema.offers = offers;
	if (input.description) schema.description = input.description;
	if (input.image) schema.image = input.image;
	// No `brand`: the seller is the merchant, not the product's brand, and the
	// wire has no brand field. Emitting the store name as a Brand told Google
	// every 文旦 was branded "Hawkr" — a claim the page itself never makes.
	if (input.rating && input.rating.count > 0) {
		schema.aggregateRating = {
			"@type": "AggregateRating",
			ratingValue: input.rating.value,
			reviewCount: input.rating.count,
		};
	}
	return schema;
}

export function breadcrumbSchema(
	crumbs: Array<{ name: string; url: string }>,
): WithContext<BreadcrumbList> {
	return {
		"@context": "https://schema.org",
		"@type": "BreadcrumbList",
		itemListElement: crumbs.map((c, i) => ({
			"@type": "ListItem",
			position: i + 1,
			name: c.name,
			item: c.url,
		})),
	};
}

export interface SellerSchemaInput {
	name: string;
	url: string;
	description?: string;
	image?: string;
}

/**
 * No `inLanguage`: the page is in the visitor's language but the entity it describes is the
 * seller's own text, which stays zh-TW whatever the locale (ADR 2026-10-17 D5), so either value
 * would be wrong for half of it.
 */
export function sellerSchema(
	input: SellerSchemaInput,
): WithContext<ProfilePage> {
	const org: {
		"@type": "Organization";
		name: string;
		url: string;
		description?: string;
		image?: string;
	} = { "@type": "Organization", name: input.name, url: input.url };
	if (input.description) org.description = input.description;
	if (input.image) org.image = input.image;
	return {
		"@context": "https://schema.org",
		"@type": "ProfilePage",
		mainEntity: org,
	};
}

export function faqSchema(
	items: Array<{ q: string; a: string }>,
	locale: AppLocale,
): WithContext<FAQPage> {
	return {
		"@context": "https://schema.org",
		"@type": "FAQPage",
		inLanguage: locale,
		mainEntity: items.map((it) => ({
			"@type": "Question",
			name: it.q,
			acceptedAnswer: { "@type": "Answer", text: it.a },
		})),
	};
}

export function itemListSchema(urls: string[]): WithContext<ItemList> {
	return {
		"@context": "https://schema.org",
		"@type": "ItemList",
		itemListElement: urls.map((url, i) => ({
			"@type": "ListItem",
			position: i + 1,
			url,
		})),
	};
}
