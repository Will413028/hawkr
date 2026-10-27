/**
 * The one product summary every SEO surface quotes — <meta description>,
 * og:description and the Product JSON-LD `description` — so they can never
 * disagree with each other or with the page.
 *
 * Why this exists: prod served the product NAME as its description
 * (`description: product.name`), and the JSON-LD carried none, while the
 * seller's own 詳細說明 text sat unused on the same payload. A snippet that
 * repeats the title tells a searcher nothing the title did not.
 *
 * Source order, all of it from the product record itself:
 *   1. the seller's description text (text blocks / list items, in reading
 *      order) when it says enough to stand alone;
 *   2. otherwise the facts the record states — origin, category path, seller —
 *      joined behind the name.
 * Nothing is invented: no "有機", no "免運", no delivery promise, and the
 * seller's address is never presented as the product's origin.
 */

import {
	codePointLength,
	MIN_STANDALONE_TEXT,
	SEO_DESCRIPTION_MAX,
	truncateSummary,
} from "@/lib/seo/summary-rules";

export const PRODUCT_SEO_DESCRIPTION_MAX = SEO_DESCRIPTION_MAX;

function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function str(value: unknown): string {
	return typeof value === "string" ? value.trim() : "";
}

function runsText(runs: unknown): string {
	if (!Array.isArray(runs)) return "";
	return runs.map((run) => str(asRecord(run).text)).join("");
}

function listItemsText(listJson: unknown): string[] {
	let model: unknown = listJson;
	if (typeof listJson === "string") {
		try {
			model = JSON.parse(listJson);
		} catch {
			return [];
		}
	}
	const items = asRecord(model).items;
	if (!Array.isArray(items)) return [];
	return items
		.map((item) => {
			if (typeof item === "string") return item.trim();
			const record = asRecord(item);
			return runsText(record.runs) || str(record.text) || str(record.content);
		})
		.filter((item) => item.length > 0);
}

/**
 * Plain text of the seller's description blocks, in reading order. Text
 * blocks contribute their runs (or the legacy `content` string), list blocks
 * their items; images and tables carry nothing a snippet can quote.
 */
export function productDescriptionText(description: unknown): string {
	const blocks = asRecord(description).blocks;
	if (!Array.isArray(blocks)) return "";
	const parts: string[] = [];
	for (const raw of blocks) {
		const block = asRecord(raw);
		if (block.type === "text") {
			const text = runsText(block.runs) || str(block.content);
			if (text) parts.push(text);
		} else if (block.type === "list") {
			const items = listItemsText(block.listJson);
			if (items.length) parts.push(items.join("、"));
		}
	}
	return parts.join(" ").replace(/\s+/g, " ").trim();
}

export interface ProductSummarySource {
	name?: unknown;
	description?: unknown;
	origin?: unknown;
	categoryLv1Name?: unknown;
	categoryLv2Name?: unknown;
	categoryLv3Name?: unknown;
	/** The leaf's own name (`handler.PublicProductDetailItem.categoryName`). */
	categoryName?: unknown;
	seller?: { name?: unknown } | null;
}

function categoryPath(product: ProductSummarySource): string[] {
	const path: string[] = [];
	for (const raw of [
		product.categoryLv1Name,
		product.categoryLv2Name,
		product.categoryLv3Name || product.categoryName,
	]) {
		const name = str(raw);
		if (name && path[path.length - 1] !== name) path.push(name);
	}
	return path;
}

export function productSeoDescription(product: ProductSummarySource): string {
	const name = str(product.name);
	const body = productDescriptionText(product.description);
	if (codePointLength(body) >= MIN_STANDALONE_TEXT)
		return truncateSummary(body);

	const facts: string[] = [];
	const origin = str(product.origin);
	if (origin) facts.push(`產地 ${origin}`);
	const path = categoryPath(product);
	if (path.length) facts.push(`分類 ${path.join("／")}`);
	const seller = str(product.seller?.name);
	if (seller) facts.push(`由 ${seller} 供應`);
	if (body) facts.push(body);

	if (!facts.length) return name;
	return truncateSummary(
		name ? `${name}｜${facts.join("｜")}` : facts.join("｜"),
	);
}
