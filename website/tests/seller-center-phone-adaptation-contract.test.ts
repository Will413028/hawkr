import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const css = source("styles/features/seller-center.css");
const orders = source("features/seller-center/orders/SellerOrdersPage.tsx");
const afterSales = source("features/seller-center/orders/AfterSalePanel.tsx");
const productForm = source(
	"features/seller-center/products/ProductFormPage.tsx",
);
const products = source("features/seller-center/products/ProductsPage.tsx");
const imageUploader = source("components/ui/ImageUploader.tsx");

test("order records use dedicated phone work cards and explicit selection mode", () => {
	assert.match(orders, /className="seller-orders__mobile-view"/);
	assert.match(orders, /function MobileOrderCard\(/);
	assert.match(orders, /mobileSelectionMode/);
	assert.match(orders, /選取訂單/);
	assert.match(orders, /seller-orders__bulk--mobile/);
	assert.match(afterSales, /seller-orders__mobile-view--aftersale/);
	assert.match(afterSales, /seller-orders__mobile-card--aftersale/);

	const phoneStyles = css.slice(
		css.indexOf("/* Dedicated order work cards"),
		css.indexOf("/* List tables opt in explicitly"),
	);
	assert.match(phoneStyles, /@media \(max-width: 767px\)/);
	assert.match(
		phoneStyles,
		/\.seller-orders__desktop-view[^{]*\{[^}]*display:\s*none/s,
	);
	assert.match(
		phoneStyles,
		/\.seller-orders__mobile-view\s*\{[^}]*display:\s*block/s,
	);
	assert.match(phoneStyles, /\.seller-orders__mobile-card\s*\{/);
	assert.match(phoneStyles, /min-height:\s*44px/);
	assert.match(phoneStyles, /safe-area-inset-bottom/);
});

test("seller list tables opt into phone cards with visible field labels", () => {
	const listTables = [
		"features/seller-center/products/ProductsPage.tsx",
		"features/seller-center/inventory/InventoryPage.tsx",
		"features/seller-center/customers/CustomersClient.tsx",
		"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx",
		"features/seller-center/invoices/SellerInvoiceOverviewPage.tsx",
		"features/seller-center/products-import/pages/JobsListPage.tsx",
		"features/seller-center/products-import/pages/JobDetailPage.tsx",
	];

	for (const file of listTables) {
		assert.match(source(file), /data-mobile-layout="cards"/, file);
		assert.match(source(file), /data-label=/, file);
	}

	const cardStyles = css.slice(css.indexOf("/* List tables opt in explicitly"));
	assert.match(cardStyles, /table\[data-mobile-layout="cards"\]/);
	assert.match(cardStyles, /content:\s*attr\(data-label\)/);
	assert.match(cardStyles, /grid-template-columns:\s*minmax\(5\.75rem, 36%\)/);
	assert.match(cardStyles, /td\[colspan\]/);
});

test("shared phone controls stay single-line and meet the touch target", () => {
	const adaptation = css.slice(
		css.indexOf("Seller workspace phone adaptation"),
	);
	assert.match(adaptation, /\.seller-shell__menu-btn,[\s\S]*width:\s*44px/);
	assert.match(adaptation, /\.seller-sidebar__close,[\s\S]*height:\s*44px/);
	assert.match(
		adaptation,
		/seller-page-shell__action-group[\s\S]*height:\s*44px/,
	);
	assert.match(
		adaptation,
		/\.seller-price-lookup__workspace-tabs[\s\S]*overflow-x:\s*auto/,
	);
	assert.match(
		adaptation,
		/\.seller-price-lookup__day-switch[\s\S]*grid-auto-flow:\s*column/,
	);
	assert.match(
		adaptation,
		/\.seller-import-detail__filter-rail[\s\S]*flex-wrap:\s*nowrap/,
	);
	assert.match(
		adaptation,
		/\.seller-import-item__action-rail[\s\S]*flex-wrap:\s*nowrap/,
	);
	assert.match(adaptation, /\.seller-card__link[\s\S]*min-height:\s*44px/);
	assert.match(
		adaptation,
		/\.seller-customers__sort select[\s\S]*min-height:\s*44px/,
	);
	assert.match(products, /className="seller-products__search-input"/);
	assert.match(products, /className="seller-products-table__check-hit"/);
	assert.match(
		adaptation,
		/\.seller-products__search-input\s*\{[^}]*min-height:\s*44px/s,
	);
	assert.match(
		adaptation,
		/\.seller-products-table__check-hit\s*\{[^}]*width:\s*44px[^}]*height:\s*44px/s,
	);
	assert.match(
		adaptation,
		/\.seller-products-table:not\(\.seller-products-table--delegated\)[\s\S]*\.seller-products-table__check\s*\{[^}]*display:\s*none/s,
	);

	const notifications = source(
		"features/account/screens/notifications/NotificationsClient.tsx",
	);
	const preferences = source(
		"features/account/screens/notifications/PreferencesClient.tsx",
	);
	const invoices = source(
		"features/seller-center/invoices/SellerInvoiceOverviewPage.tsx",
	);
	assert.match(notifications, /w-11 min-w-11/);
	assert.match(preferences, /inline-flex min-h-11 items-center/);
	assert.match(invoices, /inline-flex min-h-11 items-center self-start/);
});

test("product and profile fixed actions stay usable at 320px", () => {
	// The adapter remains the business-truth owner. Mobile only changes which
	// adapter-provided action is prominent and where the alternate is disclosed.
	assert.match(
		productForm,
		/const mobileProminentAction = secondary \?\? primary/,
	);
	assert.match(
		productForm,
		/const mobileAlternateAction = secondary \? primary : null/,
	);
	assert.match(productForm, /className="product-form__mobile-toolbar/);
	assert.match(productForm, /className="product-form__mobile-more"/);
	assert.match(productForm, /aria-label="開啟 AI 上架助手"/);
	assert.match(
		productForm,
		/submitFromButton\(mobileProminentAction\.action\)/,
	);
	assert.match(
		productForm,
		/submitFromButton\(mobileAlternateAction\.action\)/,
	);

	const productStart = css.indexOf(".product-form__title-row,");
	const productEnd = css.indexOf(".ai-panel__backdrop", productStart);
	assert.ok(productStart >= 0 && productEnd > productStart);
	const productPhoneStyles = css.slice(productStart, productEnd);
	assert.match(productPhoneStyles, /@media \(max-width: 960px\)/);
	assert.match(
		productPhoneStyles,
		/\.product-form__title-row\s*\{[^}]*flex-wrap:\s*nowrap/s,
	);
	assert.match(
		productPhoneStyles,
		/\.product-form__mobile-toolbar\s*\{[^}]*flex-wrap:\s*nowrap[^}]*safe-area-inset-bottom/s,
	);
	assert.match(
		productPhoneStyles,
		/\.product-form__mobile-more > summary\s*\{[^}]*width:\s*44px[^}]*height:\s*44px/s,
	);
	assert.match(
		productPhoneStyles,
		/@media \(max-width: 360px\)[\s\S]*\.product-form__ai-label\s*\{/,
	);

	const profileStart = css.indexOf(
		"@media (max-width: 540px)",
		css.indexOf(".seller-profile__save-bar"),
	);
	const profileEnd = css.indexOf(
		"/* ============================================================",
		profileStart,
	);
	assert.ok(profileStart >= 0 && profileEnd > profileStart);
	const profilePhoneStyles = css.slice(profileStart, profileEnd);
	assert.match(
		profilePhoneStyles,
		/\.seller-profile__save-bar\s*\{[^}]*flex-wrap:\s*nowrap[^}]*safe-area-inset-bottom/s,
	);
	assert.match(
		profilePhoneStyles,
		/\.seller-profile__save-hint\s*\{[^}]*min-width:\s*0[^}]*text-overflow:\s*ellipsis[^}]*white-space:\s*nowrap/s,
	);
	assert.match(
		profilePhoneStyles,
		/\.seller-profile__save-bar > \[data-hkui-button\]\s*\{[^}]*min-height:\s*44px[^}]*flex:\s*0 0 auto/s,
	);
});

test("seller portal drawers and certification flows keep phone-safe controls", () => {
	const dialog = source("components/ui/Dialog.tsx");
	const postModal = source(
		"features/seller-center/posts/SellerPostDetailModal.tsx",
	);
	const sellerProfileStyles = source("styles/features/seller-profile.css");
	const adaptation = css.slice(
		css.indexOf("Seller workspace phone adaptation"),
	);

	assert.match(dialog, /data-hk-overlay-panel/);
	assert.match(dialog, /data-hk-dialog-body/);
	assert.match(dialog, /data-hk-dialog-footer/);
	assert.match(dialog, /data-hk-overlay-close/);
	// 44px close button (was size-9 = 36px); the header's pr-16 clears it.
	assert.match(dialog, /right-4 top-4[\s\S]*size-11/);
	assert.match(
		adaptation,
		/body:has\(\.seller-shell\) \[data-hk-overlay-root\][\s\S]*padding:\s*8px/,
	);
	assert.match(
		adaptation,
		/\[data-hk-dialog-footer\][\s\S]*flex-wrap:\s*nowrap[\s\S]*safe-area-inset-bottom/,
	);
	assert.match(
		adaptation,
		/\[data-hk-dialog-footer\][\s\S]*:is\(button, a\[data-hkui-button\]\)[\s\S]*min-height:\s*44px/,
	);
	assert.match(
		adaptation,
		/\.shipment-dialog__chip-row\s*\{[^}]*flex-wrap:\s*nowrap[^}]*overflow-x:\s*auto/s,
	);
	assert.match(
		adaptation,
		/\.shipment-dialog__foot > \[data-hkui-button\]\s*\{[^}]*min-height:\s*44px/s,
	);
	assert.match(
		adaptation,
		/\.seller-cert--form-shell \.seller-cert__panel-anchors[\s\S]*overflow-x:\s*auto/,
	);
	assert.match(imageUploader, /data-image-uploader-actions=\{actionsVariant\}/);
	assert.match(imageUploader, /data-image-uploader-action="change"/);
	assert.match(imageUploader, /data-image-uploader-action="clear"/);
	assert.match(
		adaptation,
		/\.shipment-dialog \[data-image-uploader-action\][\s\S]*min-height:\s*44px/,
	);
	assert.match(postModal, /size-11/);
	assert.match(
		adaptation,
		/\.seller-post-modal__surface :is\(button, a\[data-hkui-button\]\)[\s\S]*min-height:\s*44px/,
	);
	assert.match(
		sellerProfileStyles,
		/@media \(max-width: 640px\)[\s\S]*\.hkui-seller-profile__cta\s*\{[^}]*height:\s*44px[^}]*min-height:\s*44px/s,
	);
	assert.match(
		sellerProfileStyles,
		/\.hkui-seller-profile__tab\s*\{[^}]*min-height:\s*44px[^}]*white-space:\s*nowrap/s,
	);
});
