import assert from "node:assert/strict";
import test from "node:test";
import { QueryClient } from "@tanstack/react-query";
import { invalidateNotificationLists } from "../lib/notifications/cache";
import { queryKeys } from "../lib/query/keys";

test("notification refresh invalidates every list variant for only the active identity", async () => {
	const queryClient = new QueryClient();
	const seller = { accountId: 90010001, activeRole: "seller" } as const;
	const buyer = { accountId: 90010001, activeRole: "buyer" } as const;
	const otherSeller = { accountId: 90010002, activeRole: "seller" } as const;
	const sellerAll = queryKeys.notifications.list(seller, {
		role: "seller",
		pageSize: 20,
		category: "",
	});
	const sellerSystem = queryKeys.notifications.list(seller, {
		role: "seller",
		pageSize: 20,
		category: "system",
	});
	const sellerPreferences = queryKeys.notifications.preferences(
		seller,
		"seller",
	);
	const buyerList = queryKeys.notifications.list(buyer, { role: "buyer" });
	const otherSellerList = queryKeys.notifications.list(otherSeller, {
		role: "seller",
	});

	for (const key of [
		sellerAll,
		sellerSystem,
		sellerPreferences,
		buyerList,
		otherSellerList,
	]) {
		queryClient.setQueryData(key, { fixture: true });
	}

	await invalidateNotificationLists(queryClient, seller, "none");

	assert.equal(queryClient.getQueryState(sellerAll)?.isInvalidated, true);
	assert.equal(queryClient.getQueryState(sellerSystem)?.isInvalidated, true);
	assert.equal(
		queryClient.getQueryState(sellerPreferences)?.isInvalidated,
		false,
	);
	assert.equal(queryClient.getQueryState(buyerList)?.isInvalidated, false);
	assert.equal(
		queryClient.getQueryState(otherSellerList)?.isInvalidated,
		false,
	);
});
