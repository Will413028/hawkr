/**
 * Canonical display-logo contract for recommended seller payloads.
 *
 * `/public/marketplace.recommended_sellers[]` and
 * `/public/sellers/recommended.records[]` expose `logoUrl`. Do not recover
 * from `logoKey`, `logo`, snake-case aliases, or a frontend placeholder here:
 * doing so hides contract drift and can display a different image than the
 * backend selected.
 */
export function recommendedSellerLogoUrl(
	seller: { logoUrl?: unknown } | null | undefined,
): string {
	return typeof seller?.logoUrl === "string" ? seller.logoUrl.trim() : "";
}
