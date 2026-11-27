/** BuyerAppService follow methods. */

import type { Follow } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { buyerAppClient } from "../connect/buyer";

export async function fetchFollowing({
	pageToken = "",
	size = 20,
}: {
	pageToken?: string;
	size?: number;
} = {}): Promise<{ items: Follow[]; nextPageToken: string }> {
	const result = await buyerAppClient().listFollows({
		pageSize: size,
		pageToken,
	});
	return {
		items: result.follows,
		nextPageToken: result.nextPageToken,
	};
}

export async function checkFollowed(
	id: number | string,
	type?: string,
): Promise<boolean> {
	void type;
	const result = await buyerAppClient().checkFollow({ storeId: String(id) });
	return result.following;
}

export async function batchCheckFollowedStoreIds(
	ids: string[],
): Promise<Set<string>> {
	if (ids.length === 0) return new Set();
	const result = await buyerAppClient().batchCheckFollows({ storeIds: ids });
	return new Set(result.followedStoreIds);
}

/**
 * Subscribe (POST) or unsubscribe (DELETE) a seller directly. Unlike
 * `toggleFollow`, the caller states the target explicitly, so this skips the
 * extra `checkFollowed` GET — the right shape for an optimistic mutation that
 * already knows where it's heading. Returns the resulting followed boolean.
 */
export async function setSellerFollow(
	id: number | string,
	follow: boolean,
): Promise<boolean> {
	if (follow) {
		await buyerAppClient().followStore({ storeId: String(id) });
		return true;
	}
	await buyerAppClient().unfollowStore({ storeId: String(id) });
	return false;
}

export async function toggleFollow(
	id: number | string,
	type?: string,
): Promise<boolean> {
	void type;
	const followed = await checkFollowed(id);
	if (followed) {
		await buyerAppClient().unfollowStore({ storeId: String(id) });
		return false;
	}
	await buyerAppClient().followStore({ storeId: String(id) });
	return true;
}
