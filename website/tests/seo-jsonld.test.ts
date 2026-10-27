import assert from "node:assert/strict";
import { test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import {
	breadcrumbSchema,
	faqSchema,
	itemListSchema,
	organizationSchema,
	productSchema,
	sellerSchema,
	webSiteSchema,
} from "../lib/seo/jsonld";
import { siteUrl } from "../lib/seo/urls";

// The prod flavor requires its own web origin (NEXT_PUBLIC_WEB_ORIGIN); these cases describe a deployment at this one.
process.env.NEXT_PUBLIC_WEB_ORIGIN = "https://example.com";

function withProdEnv(fn: () => void) {
	__setFlavorForTest("prod");
	__setEnvForTest(null);
	try {
		fn();
	} finally {
		__setFlavorForTest(null);
		__setEnvForTest(null);
	}
}

test("organizationSchema has @context, @type, absolute url + logo", () => {
	withProdEnv(() => {
		const org = organizationSchema("zh-TW");
		assert.equal(org["@context"], "https://schema.org");
		assert.equal(org["@type"], "Organization");
		assert.equal(org.name, "Hawkr");
		assert.equal(org.url, "https://example.com");
		assert.equal(org.logo, "https://example.com/brand/app-icon.svg");
	});
});

test("webSiteSchema exposes a SearchAction pointing at /search?q=", () => {
	withProdEnv(() => {
		const site = webSiteSchema("zh-TW");
		assert.equal(site["@type"], "WebSite");
		assert.equal(site.inLanguage, "zh-TW");
		assert.equal(site.url, siteUrl());
		const action = site.potentialAction as {
			"@type": string;
			target: { urlTemplate: string };
			"query-input": string;
		};
		assert.equal(action["@type"], "SearchAction");
		assert.equal(
			action.target.urlTemplate,
			"https://example.com/search?q={search_term_string}",
		);
		assert.equal(action["query-input"], "required name=search_term_string");
	});
});

test("every builder output is JSON-serializable", () => {
	withProdEnv(() => {
		assert.doesNotThrow(() => JSON.stringify(organizationSchema("zh-TW")));
		assert.doesNotThrow(() => JSON.stringify(organizationSchema("en")));
		assert.doesNotThrow(() => JSON.stringify(webSiteSchema("zh-TW")));
		assert.doesNotThrow(() => JSON.stringify(webSiteSchema("en")));
		assert.doesNotThrow(() =>
			JSON.stringify(
				productSchema({
					name: "x",
					url: "https://example.com/products/1",
					price: 10,
					availability: "InStock",
				}),
			),
		);
		assert.doesNotThrow(() =>
			JSON.stringify(
				breadcrumbSchema([{ name: "首頁", url: "https://example.com" }]),
			),
		);
		assert.doesNotThrow(() =>
			JSON.stringify(
				sellerSchema({
					name: "農場",
					url: "https://example.com/sellers/1",
				}),
			),
		);
		assert.doesNotThrow(() =>
			JSON.stringify(faqSchema([{ q: "Q?", a: "A." }], "zh-TW")),
		);
		assert.doesNotThrow(() =>
			JSON.stringify(itemListSchema(["https://example.com/products/1"])),
		);
	});
});

test("productSchema with a single price emits a plain Offer in TWD", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "有機紅蘿蔔 1kg",
			url: "https://example.com/products/42",
			image: "https://img.example/42.jpg",
			description: "宜蘭三星有機紅蘿蔔，整箱 1 公斤。",
			price: 120,
			availability: "InStock",
		});
		assert.equal(p["@type"], "Product");
		assert.equal(p.name, "有機紅蘿蔔 1kg");
		const offer = p.offers as {
			"@type": string;
			price: number;
			priceCurrency: string;
			availability: string;
		};
		assert.equal(offer["@type"], "Offer");
		assert.equal(offer.price, 120);
		assert.equal(offer.priceCurrency, "TWD");
		assert.equal(offer.availability, "https://schema.org/InStock");
		assert.equal(p.description, "宜蘭三星有機紅蘿蔔，整箱 1 公斤。");
		// The seller is the merchant, not the product's brand: no Brand is
		// fabricated from the store name.
		assert.equal("brand" in p, false);
	});
});

test("productSchema leaves availability out when the wire reports no stock", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "庫存未回報的商品",
			url: "https://example.com/products/43",
			price: 120,
		});
		const offer = p.offers as { price: number; availability?: string };
		assert.equal(offer.price, 120);
		// Unknown is not "in stock": nothing is claimed either way.
		assert.equal("availability" in offer, false);
	});
});

test("productSchema with a price range emits an AggregateOffer", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "綜合蔬菜箱",
			url: "https://example.com/products/7",
			priceRange: { low: 200, high: 480 },
			availability: "OutOfStock",
		});
		const offer = p.offers as {
			"@type": string;
			lowPrice: number;
			highPrice: number;
			priceCurrency: string;
			availability: string;
		};
		assert.equal(offer["@type"], "AggregateOffer");
		assert.equal(offer.lowPrice, 200);
		assert.equal(offer.highPrice, 480);
		assert.equal(offer.availability, "https://schema.org/OutOfStock");
	});
});

test("productSchema never advertises InStock when backend says the product is not purchasable", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "季節性暫停販售商品",
			url: "https://example.com/products/42",
			price: 120,
			availability: "InStock",
			purchasable: false,
		});
		const offer = p.offers as { availability: string };
		assert.equal(offer.availability, "https://schema.org/OutOfStock");
	});
});

test("productSchema omits aggregateRating when there are no reviews", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "x",
			url: "https://example.com/products/1",
			price: 10,
			availability: "InStock",
			rating: { value: 4.5, count: 0 },
		});
		assert.equal(p.aggregateRating, undefined);
	});
});

test("productSchema includes aggregateRating only with real reviews", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "x",
			url: "https://example.com/products/1",
			price: 10,
			availability: "InStock",
			rating: { value: 4.5, count: 23 },
		});
		assert.deepEqual(p.aggregateRating, {
			"@type": "AggregateRating",
			ratingValue: 4.5,
			reviewCount: 23,
		});
	});
});

test("breadcrumbSchema numbers positions from 1 with absolute urls", () => {
	withProdEnv(() => {
		const bc = breadcrumbSchema([
			{ name: "Hawkr 市集", url: "https://example.com/market/products" },
			{ name: "有機紅蘿蔔", url: "https://example.com/products/42" },
		]);
		assert.equal(bc["@type"], "BreadcrumbList");
		const items = bc.itemListElement as Array<{
			"@type": string;
			position: number;
			name: string;
			item: string;
		}>;
		assert.equal(items.length, 2);
		assert.equal(items[0].position, 1);
		assert.equal(items[0].item, "https://example.com/market/products");
		assert.equal(items[1].position, 2);
	});
});

test("sellerSchema wraps an Organization in a ProfilePage", () => {
	withProdEnv(() => {
		const s = sellerSchema({
			name: "陽光農場",
			url: "https://example.com/sellers/9",
			description: "南投在地有機蔬果",
			image: "https://img.example/cover.jpg",
		});
		assert.equal(s["@type"], "ProfilePage");
		// the seller's own text is zh-TW whatever the page locale: no language claim either way
		assert.equal("inLanguage" in s, false);
		const org = s.mainEntity as {
			"@type": string;
			name: string;
			url: string;
			description: string;
			image: string;
		};
		assert.equal(org["@type"], "Organization");
		assert.equal(org.name, "陽光農場");
		assert.equal(org.url, "https://example.com/sellers/9");
		assert.equal(org.description, "南投在地有機蔬果");
		assert.equal(org.image, "https://img.example/cover.jpg");
	});
});

test("faqSchema maps Q&A into Question/acceptedAnswer pairs", () => {
	withProdEnv(() => {
		const f = faqSchema(
			[
				{ q: "如何付款？", a: "支援信用卡與 ATM。" },
				{ q: "多久到貨？", a: "下單後 1-2 個工作天。" },
			],
			"zh-TW",
		);
		assert.equal(f["@type"], "FAQPage");
		assert.equal(f.inLanguage, "zh-TW");
		const items = f.mainEntity as Array<{
			"@type": string;
			name: string;
			acceptedAnswer: { "@type": string; text: string };
		}>;
		assert.equal(items.length, 2);
		assert.equal(items[0]["@type"], "Question");
		assert.equal(items[0].name, "如何付款？");
		assert.equal(items[0].acceptedAnswer["@type"], "Answer");
		assert.equal(items[0].acceptedAnswer.text, "支援信用卡與 ATM。");
	});
});

test("itemListSchema numbers urls from position 1", () => {
	withProdEnv(() => {
		const list = itemListSchema([
			"https://example.com/products/1",
			"https://example.com/products/2",
		]);
		assert.equal(list["@type"], "ItemList");
		const items = list.itemListElement as Array<{
			"@type": string;
			position: number;
			url: string;
		}>;
		assert.equal(items[0].position, 1);
		assert.equal(items[0].url, "https://example.com/products/1");
		assert.equal(items[1].position, 2);
	});
});

test("productSchema omits offers entirely when no price or range is given", () => {
	withProdEnv(() => {
		const p = productSchema({
			name: "x",
			url: "https://example.com/products/1",
			availability: "InStock",
		});
		assert.equal(p.offers, undefined);
	});
});

test("the en site is the /en WebSite: its own name, url, search target and inLanguage", () => {
	withProdEnv(() => {
		const site = webSiteSchema("en");
		assert.equal(site.name, "Hawkr");
		assert.equal(organizationSchema("en").name, "Hawkr");
		assert.equal(
			organizationSchema("en").logo,
			`${siteUrl()}/brand/app-icon.svg`,
		);
		assert.equal(site.inLanguage, "en");
		assert.equal(site.url, `${siteUrl()}/en`);
		const action = site.potentialAction as {
			target: { urlTemplate: string };
		};
		assert.equal(
			action.target.urlTemplate,
			`${siteUrl()}/en/search?q={search_term_string}`,
		);
		assert.equal(faqSchema([{ q: "q", a: "a" }], "en").inLanguage, "en");
	});
});

test("types without an inLanguage property do not get one", () => {
	withProdEnv(() => {
		assert.equal("inLanguage" in organizationSchema("en"), false);
		assert.equal("inLanguage" in sellerSchema({ name: "x", url: "u" }), false);
		assert.equal("inLanguage" in itemListSchema(["https://x"]), false);
		assert.equal("inLanguage" in breadcrumbSchema([]), false);
		assert.equal(
			"inLanguage" in productSchema({ name: "x", url: "https://x", price: 1 }),
			false,
		);
	});
});
