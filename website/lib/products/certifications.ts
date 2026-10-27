/**
 * Product certification helpers.
 *
 * Backend stores `certifications[].name` as the CertificationType *value*
 * (e.g. "3701" 有機 / "3702" 產銷履歷) — the same codes the seller app sends.
 * The display label / filter routing lives on the frontend; the backend treats
 * the value as an opaque string (no enum conversion). This map mirrors the
 * seller app's `product_certification_meta.dart`.
 *
 * ⚠️ **`PRODUCT_CERT_CATALOG` is the single source for everything derived from a
 * certification code** — full label and short label. The three
 * used to be three separate tables, and the gap that opened between them is
 * exactly how the buyer PDP ended up printing `3701` with no badge for months
 * (website#507): the code→artwork mapping was a fuzzy match on *Chinese display
 * text*, so feeding it a code silently returned null and the whole `<img>` block
 * disappeared. A miss in this file must fail as a missing entry, never as a
 * silently-empty render.
 */

/** CertificationType value, as stored on `certifications[].name`. */
export type ProductCertificationCode =
	| "3701"
	| "3702"
	| "3703"
	| "3704"
	| "3705"
	| "3706";

export interface ProductCertCatalogItem {
	/** CertificationType value sent to the backend as `name`. */
	code: ProductCertificationCode;
	/** Full label shown on the seller upload form and the buyer PDP. */
	label: string;
	/**
	 * Compact label for space-constrained surfaces (product cards, filter chips,
	 * the market sidebar). Deliberately shorter than `label` — not a duplicate of
	 * it, which is why both live on the same item instead of in two tables.
	 */
	shortLabel: string;
	/** One-line helper under the label. */
	desc: string;
	/**
	 * Whether `GET /app/products/certifications/verify` can check the cert
	 * number against the 農業部 open-data platform. Only 3701/3702/3703 are
	 * supported today; 3704–3706 fall back to manual review.
	 */
	verifiable: boolean;
	/**
	 * Official public lookup page. This is only a route to the responsible
	 * authority's search UI; its presence must never be presented as proof that
	 * this particular seller-provided number has been verified.
	 */
	officialLookup?: Readonly<{
		label: string;
		href: string;
	}>;
}

/**
 * The six product certifications the seller can attach, in display order.
 * Mirrors the backend `CertificationType` enum (see the verify endpoint's
 * own mapping: "3701=有機農產品(ORGANIC)…"). Replaces the old web-only enum
 * (organic/tap/haccp/cas/iso22000/friendly_farming) so writes line up with
 * what the backend stores and the PDP reads.
 */
export const PRODUCT_CERT_CATALOG: ReadonlyArray<ProductCertCatalogItem> = [
	{
		code: "3701",
		label: "有機農產品",
		shortLabel: "有機",
		desc: "經認證機構認可的有機產品",
		verifiable: true,
		officialLookup: {
			label: "前往官方查詢有機標章",
			href: "https://epv.afa.gov.tw/Home/MarkUse",
		},
	},
	{
		code: "3702",
		label: "產銷履歷 TAP",
		shortLabel: "產銷履歷",
		desc: "可追溯生產者與生產履歷",
		verifiable: true,
		officialLookup: {
			label: "前往官方查詢產銷履歷",
			href: "https://taft.moa.gov.tw/mp-1.html",
		},
	},
	{
		code: "3703",
		label: "CAS 優良農產品",
		shortLabel: "CAS",
		desc: "國產農產品安全標章",
		verifiable: true,
		officialLookup: {
			label: "前往官方查詢 CAS 標章",
			href: "https://cas.moa.gov.tw/",
		},
	},
	{
		code: "3704",
		label: "HACCP",
		shortLabel: "HACCP",
		desc: "食品安全管制系統（人工審核）",
		verifiable: false,
	},
	{
		code: "3705",
		label: "ISO 22000",
		shortLabel: "ISO",
		desc: "食品安全管理系統（人工審核）",
		verifiable: false,
	},
	{
		code: "3706",
		label: "GMP",
		shortLabel: "GMP",
		desc: "良好作業規範（人工審核）",
		verifiable: false,
	},
];

const CERT_ITEM_BY_CODE: ReadonlyMap<string, ProductCertCatalogItem> = new Map(
	PRODUCT_CERT_CATALOG.map((item) => [item.code, item]),
);

/**
 * 碼值 → 短標籤，供空間受限的版面使用（商品卡 badge、篩選 chip、市集側欄）。
 *
 * 由 `PRODUCT_CERT_CATALOG` 導出而非另外手寫：兩份表各自演化的結果，就是新增一個
 * 碼值時其中一份漏掉，而漏掉的那一面會靜默顯示碼值本身。
 */
export const CERTIFICATION_LABEL_BY_VALUE: Record<string, string> =
	Object.fromEntries(
		PRODUCT_CERT_CATALOG.map((item) => [item.code, item.shortLabel]),
	);

/**
 * Resolve one backend code to its display metadata.
 *
 * Wire values are opaque strings and may arrive with harmless surrounding
 * whitespace from imports. Normalising at this shared boundary keeps the PDP,
 * cards, and filters from disagreeing about a visually identical code.
 */
export function getProductCertCatalogItem(
	code: string,
): ProductCertCatalogItem | null {
	return CERT_ITEM_BY_CODE.get(code.trim()) ?? null;
}

/**
 * 碼值 → 完整顯示名（買家商品頁、賣家上架表單、代行工作區 chip）。
 *
 * 取完整標籤，再退到短標籤，最後退回碼值本身 —— **不得退回空字串**：認不得的碼值
 * 多半來自匯入或更早的資料，把它顯示成空白等於邀請操作者把一個看不見的宣稱刪掉。
 */
export function certLabel(code: string): string {
	const normalized = code.trim();
	const item = getProductCertCatalogItem(normalized);
	if (item) return item.label;
	return CERTIFICATION_LABEL_BY_VALUE[normalized] ?? normalized;
}

export interface ProductCertBadge {
	/** CertificationType value as stored (e.g. "3701"). */
	code: string;
	/** Human label for the badge; falls back to the raw code when unknown. */
	label: string;
}

/** Normalise a raw certifications array into deduped display badges. */
export function toCertBadges(
	raw: Array<{ name?: string; certNo?: string }> | undefined,
): ProductCertBadge[] {
	if (!Array.isArray(raw)) return [];
	const out: ProductCertBadge[] = [];
	const seen = new Set<string>();
	for (const cert of raw) {
		const code = (cert?.name ?? "").trim();
		if (!code || seen.has(code)) continue;
		seen.add(code);
		out.push({ code, label: CERTIFICATION_LABEL_BY_VALUE[code] ?? code });
	}
	return out;
}
