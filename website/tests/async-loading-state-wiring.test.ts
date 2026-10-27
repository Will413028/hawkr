import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

test("cart background refresh and clear use real request state without a logistics loader", () => {
	const hook = source("lib/hooks/useCart.ts");
	const client = source("features/cart/CartClient.tsx");
	const header = source("features/cart/components/CartPageHeader.tsx");

	assert.match(hook, /const refreshing = cartQuery\.isFetching;/);
	assert.match(hook, /\n\t\trefreshing,\n/);
	assert.match(
		hook,
		/if \(clearPendingRef\.current\) return clearPendingRef\.current;/,
	);
	assert.match(client, /if \(\s*clearCartPendingRef\.current \|\|/);
	// Refresh is automatic now; its real request state blocks selection and
	// checkout rather than presenting a separate refresh action.
	assert.match(
		client,
		/const selectionBusy = clearing \|\| hasRowPending \|\| cart\.refreshing;/,
	);
	assert.match(client, /selectionBusy=\{selectionBusy\}/);
	assert.match(client, /: selectionBusy\s*\? "購物車更新中"/);
	assert.match(client, /if \(!canCheckout \|\| selectionIsBusy\(\)\)/);
	assert.match(client, /<CartPageHeader[\s\S]*?clearing=\{clearing\}/);
	assert.match(header, /isLoading=\{clearing\}/);
	assert.match(header, /loadingLabel="清空中…"/);
	assert.match(
		client,
		/setClearing\(true\);\s*try \{\s*const outcome = await cart\.clear\(\);[\s\S]*?finally \{\s*clearCartPendingRef\.current = false;\s*setClearing\(false\);/,
	);
	assert.doesNotMatch(client, /TruckLoadingOverlay/);
	assert.doesNotMatch(client, /setTimeout\(\(\) => setClearing/);
});

test("cart row mutations keep per-SKU initiating identity until every request settles", () => {
	const hook = source("lib/hooks/useCart.ts");
	const client = source("features/cart/CartClient.tsx");
	const tile = source("features/cart/components/CartProductTile.tsx");

	assert.match(client, /rowPendingRef\.current\.has\(skuId\)/);
	assert.match(client, /rowPendingRef\.current\.set\(skuId, action\)/);
	// Each retained row action owns its pending state until its request settles;
	// the redundant bulk-remove action no longer exists.
	assert.match(
		client,
		/setRowPendingBySku\([\s\S]*?\[skuId\]: action[\s\S]*?try \{\s*await operation\(\);\s*\} finally \{\s*rowPendingRef\.current\.delete\(skuId\);[\s\S]*?delete next\[skuId\];/,
	);
	assert.match(client, /pendingBySku: rowPendingBySku/);
	assert.match(client, /selectionBlocked: selectionBusy/);
	assert.doesNotMatch(client, /handleRemoveSelected|bulkRemoving/);
	assert.match(client, /savedRestorePendingRef\.current\.has\(item\.id\)/);
	assert.match(client, /loadingLabel="加回購物車中…"/);
	assert.match(client, /const outcome = await cart\.remove\(item\.id\);/);

	assert.match(tile, /isLoading=\{pendingAction === "decrement"\}/);
	assert.match(tile, /isLoading=\{pendingAction === "increment"\}/);
	assert.match(tile, /isLoading=\{pendingAction === "save-for-later"\}/);
	assert.match(tile, /isLoading=\{pendingAction === "remove"\}/);
	assert.match(
		tile,
		/isLoading=\{pendingAction === "save-for-later"\}\s*loadingLabel="移動中…"/,
	);
	assert.match(
		tile,
		/isLoading=\{pendingAction === "remove"\}\s*loadingLabel="移除中…"/,
	);

	// Destructive row mutations settle before the query cache drops the row,
	// otherwise React unmounts the exact button that should report progress.
	assert.match(
		hook,
		/const remove = React\.useCallback\([\s\S]*?\(prev\) => prev,[\s\S]*?removeFromCart\(skuId\)/,
	);
});

test("query-backed load-more buttons use fetchNextPage state", () => {
	const ordersHook = source("lib/hooks/useOrders.ts");
	const ordersClient = source(
		"features/account/screens/orders/OrdersClient.tsx",
	);
	const reviewsHook = source("lib/hooks/useMyReviews.ts");
	const reviewedList = source(
		"features/account/screens/reviews/ReviewedList.tsx",
	);

	assert.match(ordersHook, /loadingMore:\s*query\.isFetchingNextPage/);
	assert.match(ordersHook, /await query\.fetchNextPage\(\)/);
	assert.match(ordersClient, /isLoading=\{list\.loadingMore\}/);
	assert.match(ordersClient, /loadingLabel="載入訂單中…"/);

	assert.match(reviewsHook, /loadingMore:\s*query\.isFetchingNextPage/);
	assert.match(reviewsHook, /await query\.fetchNextPage\(\)/);
	assert.match(reviewedList, /isLoading=\{loadingMore\}/);
	assert.match(reviewedList, /loadingLabel="載入評論中…"/);
});

test("manual seller refreshes remain locked until their aggregate work settles", () => {
	const ordersHook = source("lib/hooks/useOrders.ts");
	const ordersPage = source(
		"features/seller-center/orders/SellerOrdersPage.tsx",
	);
	const forumHook = source("lib/hooks/useSellerForum.ts");
	const forumTab = source("features/sellers/components/SellerMessageTab.tsx");

	assert.match(ordersHook, /refreshing:\s*query\.isFetching/);
	assert.match(ordersPage, /refreshOrderWorkspacePromiseRef/);
	assert.match(ordersPage, /Promise\.allSettled\(\[/);
	assert.match(ordersPage, /isLoading=\{headerBusy\}/);
	assert.match(ordersPage, /loadingLabel=\{\s*isAfterSaleTab/);

	assert.match(forumHook, /refreshing:\s*query\.isFetching/);
	assert.match(forumHook, /refreshPromiseRef/);
	assert.match(forumTab, /isLoading=\{forum\.refreshing\}/);
	assert.match(forumTab, /loadingLabel="重新載入留言中…"/);

	// SellerOrdersPage eagerly owns the complete queue; its button only widens
	// an in-memory slice and therefore must not reuse initial-query loading.
	const ordersTabBody = ordersPage.slice(
		ordersPage.indexOf("function OrdersTabBody"),
		ordersPage.indexOf("function BulkActionBar"),
	);
	assert.match(ordersTabBody, /onClick=\{\(\) => void loadMore\(\)\}/);
	assert.doesNotMatch(ordersTabBody, /isLoading=\{loading\}/);
	assert.doesNotMatch(ordersTabBody, /disabled=\{loading\}/);
	assert.match(ordersPage, /TruckSpinner/);
});

test("seller message workspace includes enrichment in aggregate loading", () => {
	const workspace = source(
		"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx",
	);

	assert.match(workspace, /refreshAllPromiseRef/);
	assert.match(workspace, /Promise\.allSettled\(\[/);
	assert.match(workspace, /purchaseLoadGenerationRef/);
	assert.match(
		workspace,
		/const workspaceBusy =\s*conversations\.loading \|\|\s*customersLoading \|\|\s*refreshingAll \|\|\s*purchaseSummariesLoading;/,
	);
	assert.match(workspace, /isLoading=\{workspaceBusy\}/);
	assert.match(
		workspace,
		/data-loading=\{workspaceBusy \? "true" : undefined\}/,
	);
	assert.doesNotMatch(
		workspace,
		/void conversations\.refresh\(\);\s*void loadCustomers\(\);/,
	);
});
