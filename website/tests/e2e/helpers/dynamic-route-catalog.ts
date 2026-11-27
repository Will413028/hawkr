export type DynamicRouteAccess =
	| "public"
	| "buyer"
	| "seller"
	| "account"
	| "delegated";

export type DynamicRouteFixture =
	| "protected-placeholder"
	| "delegated-workspace"
	| "order-callback"
	| "product"
	| "seller"
	| "seller-thread";

export interface DynamicRouteCatalogEntry {
	pattern: string;
	access: DynamicRouteAccess;
	fixture: DynamicRouteFixture;
}

export interface PublicDynamicFixtureIds {
	productId: string;
	/**
	 * CANONICAL storefront path segment (the seller's `store_code`). Routes are
	 * materialized with this so a document assertion lands on the URL it asked
	 * for instead of following the `308` in front of a `store_id` URL.
	 */
	sellerId: string;
	/**
	 * The key the public product feed actually exposes (`store_id`), kept so the
	 * consolidation itself can be asserted. Equal to `sellerId` when the seller
	 * has no distinct store_code to be sent to.
	 */
	sellerStoreId?: string;
}

/**
 * Every independently addressable dynamic App Router page.
 *
 * Keep this list explicit: the set-equality contract in site-links.spec.ts
 * intentionally fails whenever a dynamic page is added or removed without a
 * deliberate access/runtime test decision here.
 *
 * The intercepted product modal under `app/[locale]/(public)/@modal` is not an
 * independently addressable document, so it belongs in an interaction test
 * rather than this route catalog.
 */
export const DYNAMIC_ROUTE_CATALOG: readonly DynamicRouteCatalogEntry[] = [
	{
		pattern: "/adm/[storeId]/[...area]",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/dashboard",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/orders",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/orders/[orderNo]",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/posts",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/posts/new",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products/[productId]/edit",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products/[productId]/preview",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products/new",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/storefront",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/account/addresses/[id]/edit",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/after-sales/[caseNo]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/invoices/[documentId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/messages/[targetId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/messages/assistant/[conversationId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]/return",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]/review",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]/tracking",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/reports/[id]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/wallet/[sellerId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/orders/[orderNo]",
		access: "public",
		fixture: "order-callback",
	},
	{
		pattern: "/products/[id]",
		access: "public",
		fixture: "product",
	},
	{
		pattern: "/seller-center/coupons/[id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/customers/[buyerId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller/inquiries/[inquiryId]",
		access: "account",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller/measures/[measureId]",
		access: "account",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller/measures/[measureId]/appeal",
		access: "account",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller/measures/[measureId]/remediation",
		access: "account",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/invoices/[documentId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/invoices/appeals/[complaintId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/invoices/jobs/[jobId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/messages/[targetId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/orders/[orderNo]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/posts/[id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/products/[id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/sellers/[id]",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/card",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/messages",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/messages/[threadId]",
		access: "public",
		fixture: "seller-thread",
	},
	{
		pattern: "/sellers/[id]/posts",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/reviews",
		access: "public",
		fixture: "seller",
	},
];

function requireFixtureId(
	fixtures: PublicDynamicFixtureIds | undefined,
	key: keyof PublicDynamicFixtureIds,
	pattern: string,
): string {
	const value = fixtures?.[key];
	if (!value) {
		throw new Error(`${pattern} requires a non-empty ${key} fixture`);
	}
	return encodeURIComponent(value);
}

export function materializeDynamicRoute(
	entry: DynamicRouteCatalogEntry,
	fixtures?: PublicDynamicFixtureIds,
): string {
	let route = entry.pattern;

	switch (entry.fixture) {
		case "protected-placeholder":
			route = route.replace(/\[[^\]]+\]/g, "route-contract-id");
			break;
		case "delegated-workspace":
			route = route
				.replace("[storeId]", "9000000000000000000")
				.replace(/\[[^\]]+\]/g, "route-contract-id");
			break;
		case "order-callback":
			route = route.replace("[orderNo]", "route-contract-order");
			break;
		case "product":
			route = route.replace(
				"[id]",
				requireFixtureId(fixtures, "productId", entry.pattern),
			);
			break;
		case "seller":
			route = route.replace(
				"[id]",
				requireFixtureId(fixtures, "sellerId", entry.pattern),
			);
			break;
		case "seller-thread":
			route = route
				.replace("[id]", requireFixtureId(fixtures, "sellerId", entry.pattern))
				.replace("[threadId]", "00000000-0000-0000-0000-000000000000");
			break;
	}

	if (route.includes("[")) {
		throw new Error(`unresolved dynamic route parameters: ${route}`);
	}
	return route;
}
