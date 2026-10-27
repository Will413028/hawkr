import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const disclosure = source(
	"features/seller-center/components/SellerMobileDisclosureList.tsx",
);
const css = source("styles/components/seller-mobile-disclosure.css");
const messagesCss = source("styles/features/messages.css");

test("共用手機清單預設全收合，而且任何時候只保留一筆展開", () => {
	assert.match(
		disclosure,
		/useState<string \| null>\(null\)/,
		"default expanded key must be null",
	);
	assert.match(
		disclosure,
		/current === itemKey \? null : itemKey/,
		"toggling a row must replace the single expanded key",
	);
	assert.match(disclosure, /const expanded = expandedKey === itemKey/);
	assert.match(disclosure, /expanded=\{expanded\}/);
	assert.match(disclosure, /aria-expanded=\{expanded\}/);
	assert.match(disclosure, /aria-controls=\{detailsId\}/);
	assert.match(disclosure, /hidden=\{!expanded\}/);
	assert.match(disclosure, /setExpandedKey\(null\)/);
});

test("收合列外高 58px，文字層級與觸控尺寸遵守賣家中心規格", () => {
	const sharedStyles = css.slice(
		css.indexOf("Shared seller-center phone disclosure contract"),
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure\s*\{[\s\S]*?border:\s*1px/s,
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure__summary\s*\{[\s\S]*?height:\s*56px[\s\S]*?min-height:\s*56px/s,
		"56px content plus the card's 1px top and bottom borders is 58px",
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure__title\s*\{[\s\S]*?font-size:\s*0\.875rem/s,
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure__meta,[\s\S]*?font-size:\s*0\.75rem/s,
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure__details\s*\{[\s\S]*?font-size:\s*0\.875rem/s,
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure__chevron\s*\{[\s\S]*?width:\s*44px[\s\S]*?height:\s*44px/s,
	);
	assert.match(
		sharedStyles,
		/\.seller-mobile-disclosure__title,[\s\S]*?text-overflow:\s*ellipsis[\s\S]*?white-space:\s*nowrap/s,
	);
});

test("賣家中心的主要紀錄頁共用同一套手機 disclosure，而不是各自發明卡片", () => {
	const recordPages = [
		"features/seller-center/dashboard/DashboardPage.tsx",
		"features/seller-center/products/ProductsPage.tsx",
		"features/seller-center/inventory/InventoryPage.tsx",
		"features/seller-center/posts/SellerPostsClient.tsx",
		"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx",
		"features/seller-center/customers/CustomersClient.tsx",
		"features/seller-center/coupons/SellerCouponsClient.tsx",
		"features/account/screens/notifications/NotificationsClient.tsx",
		"features/seller/measures/SellerMeasuresPage.tsx",
		"features/seller-center/invoices/SellerInvoiceOverviewPage.tsx",
		"features/seller-center/invoices/documents/SellerInvoiceDocumentsPage.tsx",
		"features/seller-center/invoices/documents/SellerReceivedInvoicesPanel.tsx",
		"features/seller-center/invoices/appeals/SellerInvoiceAppealsPage.tsx",
		"features/seller-center/products-import/pages/JobsListPage.tsx",
		"features/seller-center/wallet/WalletClient.tsx",
	];

	for (const file of recordPages) {
		const page = source(file);
		assert.match(page, /<SellerMobileDisclosureList/, file);
		assert.match(page, /getKey=\{[a-zA-Z]+/, file);
	}
});

test("商店預覽只收合賣家提示，買家實際看到的內容仍維持忠實預覽", () => {
	const banner = source(
		"features/seller-center/preview/SellerStorefrontPreviewBanner.tsx",
	);
	const layout = source(
		"app/[locale]/(seller)/seller-center/preview/[id]/layout.tsx",
	);
	assert.match(banner, /seller-mobile-disclosure-desktop/);
	assert.match(banner, /<SellerMobileDisclosureList/);
	assert.match(layout, /<SellerStorefrontPreviewBanner/);
	assert.doesNotMatch(layout, /SellerMobileDisclosureList/);
});

test("聊天室清單保留原頭像並以 58px 單行直接開啟唯一對話", () => {
	const conversationSidebar = source(
		"features/messages/components/ConversationSidebar.tsx",
	);
	const phoneStart = messagesCss.lastIndexOf("@media (max-width: 959px)");
	assert.notEqual(phoneStart, -1);
	const phoneStyles = messagesCss.slice(phoneStart);

	assert.match(conversationSidebar, /className="chat-sidebar__avatar"/);
	assert.match(conversationSidebar, /className="chat-sidebar__badge"/);
	assert.match(
		conversationSidebar,
		/href=\{`\$\{threadHrefBase\}\/\$\{id\}`\}/,
	);
	assert.match(
		phoneStyles,
		/\.chat-sidebar__item\s*\{[\s\S]*?height:\s*58px[\s\S]*?min-height:\s*58px[\s\S]*?padding:\s*7px 10px/s,
	);
	assert.match(
		phoneStyles,
		/\.chat-sidebar__avatar\s*\{[\s\S]*?width:\s*44px[\s\S]*?height:\s*44px/s,
	);
	assert.match(
		phoneStyles,
		/\.chat-sidebar__preview\s*\{[^}]*display:\s*none/s,
	);
});
