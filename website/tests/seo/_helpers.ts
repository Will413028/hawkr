import type { APIRequestContext, Page } from "@playwright/test";

/** All JSON-LD objects emitted on the page (flattens arrays; unescapes <). */
export async function parseJsonLd(
	page: Page,
): Promise<Record<string, unknown>[]> {
	const blobs = await page
		.locator('script[type="application/ld+json"]')
		.allTextContents();
	const out: Record<string, unknown>[] = [];
	for (const raw of blobs) {
		const parsed = JSON.parse(raw.replace(/\\u003c/g, "<"));
		for (const obj of Array.isArray(parsed) ? parsed : [parsed]) {
			out.push(obj as Record<string, unknown>);
		}
	}
	return out;
}

/** The set of @type strings present (handles string or string[] @type). */
export function jsonLdTypes(objects: Record<string, unknown>[]): Set<string> {
	const types = new Set<string>();
	for (const o of objects) {
		const t = o["@type"];
		if (typeof t === "string") types.add(t);
		else if (Array.isArray(t)) for (const x of t) types.add(String(x));
	}
	return types;
}

/**
 * String-parsing variants for asserting against the SERVED HTML (what a crawler
 * reads) via request.get() instead of a hydrated browser DOM. Used by the
 * entity-template gate: those pages are PPR-streamed, and a browser page.goto()
 * can stall on the dynamic Suspense tail / a reused keep-alive socket, while a
 * raw HTTP GET returns the full document (head + static shell) reliably. The
 * SEO-relevant content (h1, canonical, JSON-LD) all lives in the static shell.
 */

/** JSON-LD objects parsed from a served HTML string (flattens arrays; unescapes <). */
export function parseJsonLdFromHtml(html: string): Record<string, unknown>[] {
	const out: Record<string, unknown>[] = [];
	const re = /<script type="application\/ld\+json">([\s\S]*?)<\/script>/g;
	for (const m of html.matchAll(re)) {
		const parsed = JSON.parse(m[1].replace(/\\u003c/g, "<"));
		for (const obj of Array.isArray(parsed) ? parsed : [parsed]) {
			out.push(obj as Record<string, unknown>);
		}
	}
	return out;
}

/** Count of <h1> opening tags in a served HTML string. */
export function h1CountInHtml(html: string): number {
	return (html.match(/<h1[\s/>]/g) ?? []).length;
}

/** Canonical href from a served HTML string (attribute-order tolerant), or null. */
export function canonicalFromHtml(html: string): string | null {
	for (const tag of html.match(/<link\b[^>]*>/gi) ?? []) {
		if (/rel="canonical"/i.test(tag)) {
			const href = tag.match(/href="([^"]*)"/i);
			return href ? href[1] : null;
		}
	}
	return null;
}

/**
 * A meta tag's content from a served HTML string, or null. `key` is matched
 * against `property="…"` (OpenGraph) and `name="…"` (twitter, description)
 * alike, because Next emits og:* as `property` and twitter:* as `name`.
 */
export function metaFromHtml(html: string, key: string): string | null {
	for (const tag of html.match(/<meta\b[^>]*>/gi) ?? []) {
		if (!new RegExp(`(?:property|name)="${key}"`, "i").test(tag)) continue;
		const content = tag.match(/content="([^"]*)"/i);
		if (content) return decodeEntities(content[1]);
	}
	return null;
}

/**
 * The document <title> of a served HTML string. `<svg>` blocks are stripped
 * first: an icon's accessible `<title>` is a legitimate SVG child, and under
 * streaming it is not guaranteed to come after the head's — a cold render of
 * /products/<missing> flushed the theme toggle's `<title>月亮</title>` ahead of
 * the real one, so "first <title> in the bytes" silently reads the wrong tag.
 */
export function titleFromHtml(html: string): string | null {
	const withoutSvg = html.replace(/<svg\b[\s\S]*?<\/svg>/gi, "");
	const m = withoutSvg.match(/<title[^>]*>([\s\S]*?)<\/title>/i);
	return m ? decodeEntities(m[1]) : null;
}

function decodeEntities(s: string): string {
	return s
		.replace(/&quot;/g, '"')
		.replace(/&#x27;|&#39;/g, "'")
		.replace(/&lt;/g, "<")
		.replace(/&gt;/g, ">")
		.replace(/&amp;/g, "&");
}

/** Indices of <img> tags in a served HTML string that lack an alt attribute. */
export function imgTagsMissingAlt(html: string): number[] {
	const tags = html.match(/<img\b[^>]*>/gi) ?? [];
	return tags
		.map((tag, i) => (/\salt=/i.test(tag) ? -1 : i))
		.filter((i) => i >= 0);
}

/** Canonical href, or null if absent. */
export async function getCanonical(page: Page): Promise<string | null> {
	const link = page.locator('link[rel="canonical"]');
	return (await link.count()) === 0 ? null : link.first().getAttribute("href");
}

/** robots meta content (e.g. "noindex, follow"), or null if absent. */
export async function getRobotsMeta(page: Page): Promise<string | null> {
	const m = page.locator('meta[name="robots"]');
	return (await m.count()) === 0 ? null : m.first().getAttribute("content");
}

export async function h1Count(page: Page): Promise<number> {
	return page.locator("h1").count();
}

export async function getMetaDescription(page: Page): Promise<string | null> {
	const m = page.locator('meta[name="description"]');
	return (await m.count()) === 0 ? null : m.first().getAttribute("content");
}

type LiveEntityPayload = {
	data?: {
		records?: Array<{
			id?: unknown;
			/** `/public/products` 商品列的店家鍵（camelCase）。 */
			storeId?: unknown;
		}>;
	};
};

export function extractLiveEntityId(
	json: LiveEntityPayload | null,
	kind: "product" | "seller",
): string | null {
	const rec = json?.data?.records?.[0];
	const id = kind === "product" ? rec?.id : rec?.storeId;
	return id == null ? null : String(id);
}

// Loopback by default: a spec run without env must not leave the machine.
export const SEO_API_BASE =
	process.env.API_BASE_URL ?? "http://127.0.0.1:9/api/v1";

/**
 * Discover one live product/seller id from the public products listing API
 * so the entity-template specs can navigate a real page. Returns null when the
 * backend is unreachable/empty (the caller test.skip()s with a logged reason
 * — never a silent pass). API base is process.env.API_BASE_URL, loopback by default.
 */
export async function discoverLiveEntity(
	request: APIRequestContext,
	kind: "product" | "seller",
): Promise<string | null> {
	const base = `${SEO_API_BASE}/public`;
	const url = `${base}/products?keyword=&page=1&page_size=1`;
	const res = await request.get(url).catch(() => null);
	if (!res?.ok()) return null;
	const json = (await res.json().catch(() => null)) as LiveEntityPayload | null;
	return extractLiveEntityId(json, kind);
}

/**
 * Resolves a seller's canonical store_code from the public detail endpoint,
 * the same source `getCachedSeller`/`fetchSeller` reads. `discoverLiveEntity`
 * only surfaces the storeId, so a canonical assertion against a discovered
 * seller id must
 * re-fetch the detail record to know what the page itself should self-correct
 * to. Falls back to `id` (no correction) if the detail fetch fails or the
 * seller has no store_code.
 */
export async function resolveSellerCanonicalId(
	request: APIRequestContext,
	id: string,
): Promise<string> {
	const base = `${SEO_API_BASE}/public`;
	const res = await request.get(`${base}/sellers/${id}`).catch(() => null);
	if (!res?.ok()) return id;
	// camelCase — the endpoint's DTO is the gateway's `PublicSellerInfo`
	// (`json:"storeCode"` since backend#2235). Reading `store_code` here made
	// this helper ALWAYS return `id` unchanged, which quietly disarmed every
	// canonical assertion built on it: the page was then fetched by the very
	// segment it was supposed to self-correct to, so a page that had also
	// stopped reading the field still looked correct.
	const json = (await res.json().catch(() => null)) as {
		data?: { storeCode?: unknown };
	} | null;
	const storeCode = json?.data?.storeCode;
	return typeof storeCode === "string" && storeCode ? storeCode : id;
}
