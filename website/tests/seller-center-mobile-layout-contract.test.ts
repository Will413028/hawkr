import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import postcss from "postcss";

/**
 * Source contracts for the seller workspace's compact phone layout.
 *
 * Node has no layout engine, so pixel geometry is covered by the browser pass.
 * These checks protect the shared seams and the CSS source order that make the
 * single-line action rail, device-width summary cards and compact order status
 * tabs possible. A later phone adaptation layer raises interactive targets to
 * the shared 44px minimum.
 */

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

function regionBetween(
	text: string,
	startMarker: string,
	endMarker: string,
): string {
	const start = text.indexOf(startMarker);
	assert.notEqual(start, -1, `missing source marker: ${startMarker}`);
	const end = text.indexOf(endMarker, start + startMarker.length);
	assert.notEqual(end, -1, `missing source marker: ${endMarker}`);
	return text.slice(start, end);
}

const shell = source("components/shared/SellerPageShell.tsx");
const orders = source("features/seller-center/orders/SellerOrdersPage.tsx");
const products = source("features/seller-center/products/ProductsPage.tsx");
const inventory = source("features/seller-center/inventory/InventoryPage.tsx");
const dashboard = source("features/seller-center/dashboard/DashboardPage.tsx");
const customers = source(
	"features/seller-center/customers/CustomersClient.tsx",
);
const customerDetail = source(
	"features/seller-center/customers/CustomerDetailPanel.tsx",
);
const dashboardSkeleton = source(
	"features/seller-center/components/SellerPageSkeletons.tsx",
);
const invoices = source(
	"features/seller-center/invoices/SellerInvoiceOverviewPage.tsx",
);
const messages = source(
	"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx",
);
const importList = source(
	"features/seller-center/products-import/pages/JobsListPage.tsx",
);
const importDetail = source(
	"features/seller-center/products-import/pages/JobDetailPage.tsx",
);
const importPreviewList = source(
	"features/seller-center/products-import/pages/PreviewListingPage.tsx",
);
const importPreviewDetail = source(
	"features/seller-center/products-import/pages/PreviewDetailPage.tsx",
);
const pagination = source("components/shared/Pagination.tsx");
const css = source("styles/features/seller-center.css");

test("SellerPageShell exposes one shared mobile action-rail seam", () => {
	assert.match(shell, /seller-page-shell__actions flex items-center/);
	assert.match(messages, /seller-page-shell__action-group/);
	assert.match(importList, /seller-page-shell__action-group/);
	assert.match(importDetail, /seller-page-shell__action-group/);

	const mobileActions = regionBetween(
		css,
		"/* SellerPageShell phone action rail.",
		".seller-orders__tabs {",
	);
	assert.match(mobileActions, /@media \(max-width: 960px\)/);
	assert.match(
		mobileActions,
		/\.seller-shell \.seller-page-shell__actions\s*\{[^}]*flex-wrap:\s*nowrap/s,
	);
	assert.match(mobileActions, /overflow-x:\s*auto/);
	assert.match(mobileActions, /scrollbar-width:\s*none/);
	assert.match(
		mobileActions,
		/\.seller-page-shell__actions[\s\S]*\.seller-orders__search[\s\S]*input\s*\{[^}]*height:\s*40px/s,
	);
	assert.match(
		mobileActions,
		/\[data-slot="input-wrapper"\]\s*\{[^}]*height:\s*40px[^}]*min-height:\s*40px/s,
	);
	assert.match(
		mobileActions,
		/:is\(\[data-hkui-button\], \.seller-orders__btn\)[^{]*\{[^}]*height:\s*40px[^}]*white-space:\s*nowrap/s,
	);
});

test("all seller header CTAs share the order-page phone geometry", () => {
	const mobileActions = regionBetween(
		css,
		"/* SellerPageShell phone action rail.",
		".seller-orders__tabs {",
	);
	assert.match(
		mobileActions,
		/\.seller-shell\s*\{[^}]*--seller-page-header-cta-width:\s*7\.25rem/s,
	);
	assert.match(
		mobileActions,
		/:is\(\[data-hkui-button\], \.seller-orders__btn\)[^{]*[\s\S]*?width:\s*var\(--seller-page-header-cta-width, 7\.25rem\)[^}]*min-width:\s*var\(--seller-page-header-cta-width, 7\.25rem\)[^}]*max-width:\s*var\(--seller-page-header-cta-width, 7\.25rem\)[^}]*flex:\s*0 0 var\(--seller-page-header-cta-width, 7\.25rem\)/s,
	);
	assert.match(
		mobileActions,
		/\.seller-dashboard__head > \[data-hkui-button\]/,
	);
	assert.match(dashboard, /className="seller-page-header-cta"/);
	assert.match(importPreviewList, /className="seller-page-header-cta /);
	assert.match(importPreviewDetail, /className="seller-page-header-cta /);
	assert.doesNotMatch(products, /seller-products__header-actions/);

	const phoneAdaptation = css.slice(
		css.indexOf("Seller workspace phone adaptation"),
	);
	assert.match(
		phoneAdaptation,
		/\.seller-page-header-cta,[\s\S]*?\.seller-inventory__select\s*\{[^}]*height:\s*44px[^}]*min-height:\s*44px/s,
	);
	assert.match(
		phoneAdaptation,
		/\.seller-page-header-cta\s*\{[^}]*gap:\s*var\(--hkui-control-gap-sm\)[^}]*padding-inline:\s*var\(--hkui-space-2\)[^}]*border-radius:\s*var\(--hkui-control-radius-md\)[^}]*font-size:\s*0\.8125rem[^}]*font-weight:\s*600/s,
	);
});

test("storefront preview keeps the buyer product-card control baseline", () => {
	const selector = ".seller-shell .market-product-card__buy-row button";
	const matches: postcss.Rule[] = [];
	postcss.parse(css).walkRules(selector, (rule) => matches.push(rule));
	assert.equal(matches.length, 1, "expected one scoped product-card override");

	const rule = matches[0];
	assert.ok(rule);
	assert.equal(
		rule.nodes.find((node) => node.prop === "min-height")?.value,
		"0",
	);

	const parent = rule.parent;
	assert.ok(parent && parent.type === "atrule");
	assert.equal(parent.name, "media");
	assert.equal(
		parent.params,
		"(max-width: 767px)",
		"the buyer-card exception must stay inside the phone-only seller scope",
	);
});

test("ordinary order actions shrink to the phone width without hiding Export", () => {
	assert.match(
		orders,
		/actionsClassName=\{\s*isAfterSaleTab\s*\?\s*undefined\s*:\s*"seller-orders__header-actions"\s*\}/s,
		"only the ordinary three-control toolbar should receive the compact phone scope",
	);

	const orderPhoneActions = regionBetween(
		css,
		"/* The ordinary order toolbar has exactly three controls.",
		".seller-orders__tabs {",
	);
	assert.match(orderPhoneActions, /@media \(max-width: 540px\)/);
	assert.match(
		orderPhoneActions,
		/\.seller-orders__header-actions\s*> \.seller-orders__search\s*\{[^}]*width:\s*auto[^}]*min-width:\s*0[^}]*flex:\s*1 1 0/s,
		"search must absorb the remaining phone width instead of reserving 148px",
	);
	assert.match(
		orderPhoneActions,
		/\.seller-orders__header-actions\s*> \.seller-orders__btn\s*\{[^}]*width:\s*var\(--hkui-space-24\)[^}]*min-width:\s*var\(--hkui-space-24\)[^}]*max-width:\s*var\(--hkui-space-24\)[^}]*flex:\s*0 0 var\(--hkui-space-24\)/s,
		"refresh must keep a compact, stable 96px control",
	);
	assert.match(
		orderPhoneActions,
		/\.seller-orders__header-actions\s*> \.seller-orders__btn--primary\s*\{[^}]*width:\s*calc\(var\(--hkui-space-16\) \+ var\(--hkui-space-2\)\)[^}]*flex-basis:\s*calc\(var\(--hkui-space-16\) \+ var\(--hkui-space-2\)\)/s,
		"Export must keep a compact, stable 72px control",
	);
});

test("dashboard quick actions keep each task label on one phone line", () => {
	const quickActionPhoneRail = regionBetween(
		css,
		"/* Action names are one task label, not card copy.",
		"\t.seller-dashboard__top,",
	);
	assert.match(
		quickActionPhoneRail,
		/grid-auto-columns:\s*minmax\(max-content,\s*10rem\)/,
		"phone rail columns must grow to the longest action label",
	);
	assert.match(
		quickActionPhoneRail,
		/\.seller-quickaction\s*\{[^}]*white-space:\s*nowrap/s,
		"one action name must not split across multiple lines",
	);
});

test("narrow dashboard onboarding tasks keep the full title visible", () => {
	assert.match(
		css,
		/@media \(max-width: 359px\)\s*\{[\s\S]*?\.seller-onboarding-step\s*\{[^}]*grid-template-columns:\s*34px minmax\(0, 1fr\) 22px[^}]*padding-inline:\s*var\(--hkui-space-2\)[\s\S]*?\.seller-onboarding-step__title\s*\{[^}]*font-size:\s*var\(--hkui-text-small\)/s,
		"the 320px layout must reserve enough width for the longest task title",
	);
});

test("shared pagination keeps phone navigation labels on one local rail", () => {
	assert.match(
		pagination,
		/mt-6 max-w-full min-w-0 overflow-x-auto overscroll-x-contain py-4/,
		"overflowing pagination must scroll locally instead of squeezing its labels",
	);
	assert.match(
		pagination,
		/flex w-max min-w-full flex-nowrap items-center justify-center/,
		"the rail must center when it fits and expose its leading edge when it overflows",
	);
	assert.match(
		pagination,
		/shrink-0 whitespace-nowrap rounded-hk-pill/,
		"previous and next labels must remain one line",
	);
	assert.match(
		pagination,
		/min-w-8 shrink-0 text-center/,
		"pagination ellipses must not consume label space",
	);
	assert.match(
		pagination,
		// 44px (was 40): the page numbers are the densest tap target on the
		// shared pagination rail.
		/flex h-11 min-w-11 shrink-0 items-center/,
		"page-number controls must not shrink the navigation labels",
	);
});

test("summary card groups opt into one device-width horizontal rail", () => {
	assert.match(products, /seller-inventory__stats seller-summary-rail/);
	assert.match(inventory, /seller-inventory__stats seller-summary-rail/);
	assert.match(dashboard, /seller-stats seller-summary-rail/);
	assert.match(customers, /seller-customers__stats seller-summary-rail/);
	assert.match(customerDetail, /seller-customers__stats seller-summary-rail/);
	assert.match(dashboardSkeleton, /seller-stats seller-summary-rail/);

	for (const staticRail of [inventory, dashboard, customers, customerDetail]) {
		assert.match(staticRail, /aria-label="[^"]+"[\s\S]*?tabIndex=\{0\}/);
	}

	// These contain workflow warnings or compact navigation, not overview KPI
	// cards. Keep their information visible instead of hiding it in this rail.
	assert.doesNotMatch(
		orders,
		/seller-orders__status-tabs[^"`]*seller-summary-rail/,
	);
	assert.doesNotMatch(invoices, /seller-summary-rail/);

	const summaryRail = regionBetween(
		css,
		"/* Seller summary cards use a compact multi-browse rail",
		"/* On orders these four controls are status tabs",
	);
	assert.match(summaryRail, /@media \(max-width: 960px\)/);
	assert.match(
		summaryRail,
		/:is\([\s\S]*\.seller-shell \.seller-summary-rail,[\s\S]*\.seller-customer-detail__panel \.seller-summary-rail[\s\S]*\)/,
	);
	assert.match(
		summaryRail,
		/--seller-summary-card-size:\s*min\([\s\S]*11\.625rem,[\s\S]*calc\(100% - var\(--hkui-space-12\)\)[\s\S]*\)/,
	);
	assert.match(summaryRail, /gap:\s*var\(--hkui-space-2\)/);
	assert.match(summaryRail, /grid-template-columns:\s*none/);
	assert.match(
		summaryRail,
		/grid-auto-columns:\s*var\(--seller-summary-card-size\)/,
	);
	assert.match(summaryRail, /grid-auto-flow:\s*column/);
	assert.match(summaryRail, /overflow-x:\s*auto/);
	assert.match(summaryRail, /overscroll-behavior-inline:\s*contain/);
	assert.match(summaryRail, /scroll-snap-type:\s*inline proximity/);
	assert.match(summaryRail, /scroll-snap-align:\s*start/);
	assert.match(summaryRail, /scrollbar-width:\s*none/);
	assert.match(summaryRail, /min-height:\s*144px/);
	assert.match(summaryRail, /white-space:\s*nowrap/);
	assert.match(
		summaryRail,
		/\.seller-stats\.seller-summary-rail[\s\S]*> \.seller-stat\s*\{[^}]*padding:\s*var\(--hkui-space-4\)[^}]*gap:\s*var\(--hkui-space-2\)/s,
	);
	assert.match(
		summaryRail,
		/\.seller-stats\.seller-summary-rail[\s\S]*\.seller-stat__icon\s*\{[^}]*width:\s*32px[^}]*height:\s*32px/s,
	);
});

test("product and inventory summaries reuse the order tile geometry on phones only", () => {
	assert.match(
		products,
		/seller-inventory__stats seller-summary-rail seller-summary-rail--compact-phone/,
	);
	assert.match(
		inventory,
		/seller-inventory__stats seller-summary-rail seller-summary-rail--compact-phone/,
	);

	const phoneTiles = regionBetween(
		css,
		"/* Product and inventory summaries keep the existing 186px browse rail",
		".seller-inventory__card {",
	);
	assert.match(phoneTiles, /@media \(max-width: 540px\)/);
	assert.match(
		phoneTiles,
		/grid-template-columns:\s*repeat\(4, minmax\(64px, 1fr\)\)/,
	);
	assert.match(
		phoneTiles,
		/seller-summary-rail--compact-phone\.seller-inventory__stats--5\s*\{[^}]*grid-template-columns:\s*repeat\(5, minmax\(64px, 1fr\)\)/s,
	);
	assert.match(phoneTiles, /grid-auto-columns:\s*auto/);
	assert.match(phoneTiles, /grid-auto-flow:\s*row/);
	assert.match(phoneTiles, /min-width:\s*64px/);
	assert.match(phoneTiles, /min-height:\s*68px/);
	assert.match(phoneTiles, /padding:\s*8px 4px/);
	assert.match(
		phoneTiles,
		/\.seller-inventory__stat-icon\s*\{\s*display:\s*none/s,
	);
	assert.match(
		phoneTiles,
		/\.seller-inventory__stat\s+strong\s*\{[^}]*display:\s*block[^}]*inline-size:\s*100%[^}]*overflow:\s*visible[^}]*text-overflow:\s*clip[^}]*white-space:\s*normal[^}]*overflow-wrap:\s*anywhere/s,
		"手機摘要主數值必須換行而非省略，否則較長金額會被裁切",
	);
	assert.match(
		phoneTiles,
		/\.seller-inventory__stat\s+small\s*\{[^}]*display:\s*block[^}]*inline-size:\s*100%[^}]*overflow:\s*visible[^}]*text-overflow:\s*clip[^}]*white-space:\s*normal[^}]*overflow-wrap:\s*anywhere/s,
		"手機摘要標籤也必須允許換行，避免文字縮放後被裁切",
	);

	const genericRail = css.indexOf(
		"/* Seller summary cards use a compact multi-browse rail",
	);
	const phoneOverride = css.indexOf(
		"/* Product and inventory summaries keep the existing 186px browse rail",
	);
	assert.ok(
		genericRail !== -1 && phoneOverride > genericRail,
		"phone tiles must follow the generic rail so their scoped reset wins",
	);
});

test("order status controls are a compact four-column phone rail", () => {
	assert.match(
		orders,
		/className="seller-inventory__stats seller-orders__status-tabs"/,
	);

	const statusRail = regionBetween(
		css,
		"/* On orders these four controls are status tabs",
		".seller-inventory__card {",
	);
	assert.match(statusRail, /@media \(max-width: 960px\)/);
	assert.match(
		statusRail,
		/grid-template-columns:\s*repeat\(4, minmax\(64px, 1fr\)\)/,
	);
	assert.match(statusRail, /overflow-x:\s*auto/);
	assert.match(statusRail, /min-height:\s*68px/);
	assert.match(
		statusRail,
		/\.seller-inventory__stat-icon\s*\{\s*display:\s*none/s,
	);
	assert.match(statusRail, /small\s*\{[^}]*white-space:\s*nowrap/s);
});

test("shared compact stat sizing follows the desktop declarations", () => {
	const inventorySection = css.indexOf("Seller Inventory — 4 stat grid");
	const baseCard = css.indexOf(".seller-inventory__stat {", inventorySection);
	const baseValue = css.indexOf(".seller-inventory__stat strong {", baseCard);
	const baseIcon = css.indexOf(".seller-inventory__stat-icon {", baseValue);
	const compact = css.indexOf("/* Mobile summary sizing", baseIcon);

	assert.ok(inventorySection !== -1);
	assert.ok(baseCard > inventorySection);
	assert.ok(baseValue > baseCard);
	assert.ok(baseIcon > baseValue);
	assert.ok(
		compact > baseIcon,
		"mobile stat sizing must follow the base card/value/icon declarations",
	);

	const compactRegion = regionBetween(
		css,
		"/* Mobile summary sizing",
		"/* On orders these four controls are status tabs",
	);
	assert.match(compactRegion, /min-height:\s*112px/);
	assert.match(compactRegion, /width:\s*32px/);
});

test("order and customer filters stay on one horizontal phone row", () => {
	const orderFilters = regionBetween(
		css,
		"/* Phone filters are one task rail",
		"/* ============================================================\n * Batch shipping",
	);
	assert.match(
		orderFilters,
		/\.seller-orders__quick-filters\s*\{[^}]*overflow-x:\s*auto[^}]*flex-wrap:\s*nowrap/s,
	);
	assert.match(
		orderFilters,
		/\.seller-orders__quick-filters > \*\s*\{[^}]*flex:\s*0 0 auto/s,
	);

	const customerFilters = regionBetween(
		css,
		"@media (max-width: 960px) {\n\t.seller-customers__filters,",
		".seller-customers__badge {",
	);
	assert.match(customerFilters, /overflow-x:\s*auto/);
	assert.match(customerFilters, /flex-wrap:\s*nowrap/);
	assert.match(
		customerFilters,
		/\.seller-customers__tabs,[\s\S]*\.seller-customers__sort select\s*\{[^}]*height:\s*40px/s,
	);
	assert.match(
		customerFilters,
		/\.seller-customers__tag-chip\s*\{[^}]*white-space:\s*nowrap/s,
	);
});

test("product and inventory inner toolbars do not wrap on phones", () => {
	const productToolbar = regionBetween(
		css,
		"@media (max-width: 960px) {\n\t.seller-products__toolbar {",
		".seller-products__error {",
	);
	assert.match(productToolbar, /flex-wrap:\s*nowrap/);
	assert.match(
		productToolbar,
		/\.seller-products__toolbar \.seller-orders__search input\s*\{[^}]*height:\s*40px/s,
	);

	const inventoryToolbar = regionBetween(
		css,
		"@media (max-width: 960px) {\n\t.seller-inventory__toolbar,",
		".seller-inventory__error {",
	);
	assert.match(inventoryToolbar, /flex-wrap:\s*nowrap/);
	assert.match(
		inventoryToolbar,
		/\.seller-inventory__select\s*\{[^}]*height:\s*40px/s,
	);

	const inventoryPhoneToolbar = regionBetween(
		css,
		"@media (max-width: 767px) {\n\t/* Match the product list",
		".seller-inventory__table {",
	);
	assert.match(
		inventoryPhoneToolbar,
		/\.seller-inventory__mobile-toolbar\s*\{[^}]*display:\s*flex[^}]*flex-wrap:\s*nowrap/s,
	);
	assert.match(
		inventoryPhoneToolbar,
		/\.seller-inventory__mobile-search\s*\{[^}]*flex:\s*1 1 10rem/s,
	);
	assert.match(
		inventoryPhoneToolbar,
		/\.seller-inventory__mobile-search input\s*\{[^}]*height:\s*44px/s,
	);
	assert.match(
		inventoryPhoneToolbar,
		/\.seller-inventory__mobile-toolbar \.seller-inventory__select\s*\{[^}]*width:\s*100%[^}]*height:\s*44px/s,
	);
});
