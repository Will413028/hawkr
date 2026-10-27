import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("general async actions bind the shared loader to the initiating Button", () => {
	const productUnavailable = source(
		"features/product/ProductUnavailableView.tsx",
	);
	assert.match(productUnavailable, /isLoading=\{status === "removing"\}/);
	assert.match(productUnavailable, /loadingLabel="移除收藏中…"/);

	const orderQuickAction = source(
		"features/account/screens/orders/OrderQuickActionDrawer.tsx",
	);
	assert.match(orderQuickAction, /isLoading=\{busy\}/);
	assert.match(orderQuickAction, /loadingLabel="確認收貨中…"/);

	const accountSidebar = source(
		"features/account/components/AccountSidebar.tsx",
	);
	assert.match(accountSidebar, /const \[signingOut, setSigningOut\]/);
	assert.match(accountSidebar, /isLoading=\{signingOut\}/);
	assert.match(accountSidebar, /loadingLabel="登出帳號中…"/);

	const sellerShell = source(
		"features/seller-center/components/SellerCenterShell.tsx",
	);
	assert.match(sellerShell, /const \[signingOut, setSigningOut\]/);
	assert.match(sellerShell, /isLoading=\{signingOut\}/);
	assert.match(sellerShell, /loadingLabel="登出賣家中心中…"/);
});

test("list-like async controls retain per-action pending identity", () => {
	const addresses = source(
		"features/account/screens/addresses/AddressesClient.tsx",
	);
	assert.match(addresses, /const \[selectingId, setSelectingId\]/);
	assert.match(addresses, /disabled=\{selectingId !== null\}/);
	assert.match(addresses, /isLoading=\{selectingId === String\(a\.id\)\}/);
	assert.match(addresses, /loadingLabel="設定預設地址中…"/);

	const preview = source(
		"features/seller-center/products-import/pages/PreviewListingPage.tsx",
	);
	assert.match(preview, /"previous" \| "next" \| null/);
	assert.match(preview, /isLoading=\{paginationAction === "previous"\}/);
	assert.match(preview, /loadingLabel="載入上一頁中…"/);
	assert.match(preview, /isLoading=\{paginationAction === "next"\}/);
	assert.match(preview, /loadingLabel="載入下一頁中…"/);

	const customerDetail = source(
		"features/seller-center/customers/CustomerDetailPanel.tsx",
	);
	assert.match(customerDetail, /isLoading=\{pendingType === opt\.value\}/);
	assert.match(customerDetail, /loadingLabel=\{opt\.loadingLabel\}/);

	const tagEditor = source(
		"features/seller-center/customers/CustomerTagEditor.tsx",
	);
	assert.match(tagEditor, /kind: "add"; source: "form" \| "suggestion"/);
	assert.match(tagEditor, /kind: "remove"; key: string/);
	assert.match(
		tagEditor,
		/pendingAction\?\.kind === "remove" &&\s*pendingAction\.key === tagActionKey\(tag\)/,
	);
	assert.match(
		tagEditor,
		/pendingAction\?\.kind === "add" &&\s*pendingAction\.source === "form"/,
	);
	assert.match(
		tagEditor,
		/pendingAction\?\.kind === "add" &&\s*pendingAction\.source === "suggestion" &&\s*pendingAction\.name === name/,
	);
	const addMutation = tagEditor.slice(
		tagEditor.indexOf("const add = async"),
		tagEditor.indexOf("const remove = async"),
	);
	assert.doesNotMatch(
		addMutation,
		/setTags\(/,
		"adding a tag must not unmount the initiating suggestion while pending",
	);
	assert.ok(
		tagEditor.indexOf("if (tag.id) await removeCustomerTag") <
			tagEditor.indexOf(
				"setTags((prev) => prev.filter",
				tagEditor.indexOf("const remove"),
			),
		"removing a tag keeps its initiating Button mounted until the request settles",
	);
});

test("general loading copy names the concrete Traditional Chinese action", () => {
	const restock = source("components/shared/RestockNotifyButton.tsx");
	assert.match(restock, /"取消補貨通知中…"/);
	assert.match(restock, /"設定補貨通知中…"/);

	const sellerMessage = source(
		"features/sellers/components/SellerMessageTab.tsx",
	);
	assert.match(sellerMessage, /"送出回覆中…"/);
	assert.match(sellerMessage, /"送出留言中…"/);

	const postCompose = source(
		"features/seller-center/posts/PostComposeClient.tsx",
	);
	assert.match(postCompose, /loadingLabel="儲存貼文中…"/);

	const marketProducts = source(
		"features/market/products/MarketProductsListLayout.tsx",
	);
	assert.match(marketProducts, /loadingLabel="套用價格範圍中…"/);
});

test("optimistic removals retain their initiating row until the request settles", () => {
	const favorites = source(
		"features/account/screens/favorites/FavoritesClient.tsx",
	);
	assert.match(favorites, /pendingActionRef/);
	assert.match(
		favorites,
		/isLoading=\{\s*pendingAction === `remove-product:\$\{productId\}`/,
	);
	assert.match(favorites, /loadingLabel="取消收藏中…"/);
	assert.match(
		favorites,
		/isLoading=\{\s*pendingAction === `unfollow-seller:\$\{it\.storeId\}`/,
	);
	assert.match(favorites, /loadingLabel="取消追蹤商家中…"/);

	const accountHooks = source("lib/hooks/useAccount.ts");
	const favoriteMutation = accountHooks.slice(
		accountHooks.indexOf("const removeMutation"),
		accountHooks.indexOf("/* ===== Follow ===== */"),
	);
	assert.match(favoriteMutation, /onSuccess:/);
	assert.doesNotMatch(favoriteMutation, /onMutate:/);
	const followMutation = accountHooks.slice(
		accountHooks.indexOf("const unfollowMutation"),
		accountHooks.indexOf("/* ===== History ===== */"),
	);
	assert.match(followMutation, /onSuccess:/);
	assert.doesNotMatch(followMutation, /onMutate:/);

	const notifications = source(
		"features/account/screens/notifications/NotificationsClient.tsx",
	);
	assert.match(notifications, /pendingActionRef/);
	assert.match(notifications, /isLoading=\{pendingAction === "mark-all"\}/);
	assert.match(notifications, /loadingLabel="全部標為已讀中…"/);
	assert.match(notifications, /isLoading=\{deletePending\}/);
	assert.match(notifications, /loadingLabel="刪除通知中…"/);

	const notificationHooks = source("lib/hooks/useNotifications.ts");
	const notificationRemoveStart = notificationHooks.indexOf(
		"const removeMutation",
	);
	const removeMutation = notificationHooks.slice(
		notificationRemoveStart,
		notificationHooks.indexOf("const items", notificationRemoveStart),
	);
	assert.match(removeMutation, /onSuccess:/);
	assert.ok(
		removeMutation.indexOf("onSuccess:") <
			removeMutation.indexOf("updatePages("),
		"notification rows should be removed only after delete succeeds",
	);
});
