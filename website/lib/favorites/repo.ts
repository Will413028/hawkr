/** BuyerAppService favorite methods; product IDs remain exact strings. */

import type { Favorite } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { buyerAppClient } from "../connect/buyer";

export async function fetchFavorites({
	pageToken = "",
	pageSize = 20,
}: {
	pageToken?: string;
	pageSize?: number;
} = {}): Promise<{ items: Favorite[]; nextPageToken: string }> {
	const result = await buyerAppClient().listFavorites({ pageSize, pageToken });
	return { items: result.favorites, nextPageToken: result.nextPageToken };
}

/** Check only the product IDs rendered on screen; IDs remain exact strings. */
export async function batchCheckFavoriteProductIds(
	ids: string[],
): Promise<Set<string>> {
	if (ids.length === 0) return new Set();
	const result = await buyerAppClient().batchCheckFavorites({
		productIds: ids,
	});
	return new Set(result.favoritedProductIds);
}

export async function checkFavorite(
	productId: number | string,
): Promise<boolean> {
	const result = await buyerAppClient().checkFavorite({
		productId: String(productId),
	});
	return result.favorited;
}

export async function toggleFavorite(
	productId: number | string,
): Promise<boolean> {
	const currentlyCollected = await checkFavorite(productId);
	if (currentlyCollected) {
		await removeFavorite(productId);
		return false;
	}
	await addFavorite(productId);
	return true;
}

export async function addFavorite(productId: number | string): Promise<void> {
	await buyerAppClient().addFavorite({ productId: String(productId).trim() });
}

export async function removeFavorite(
	productId: number | string,
): Promise<void> {
	await buyerAppClient().removeFavorite({ productId: String(productId) });
}
