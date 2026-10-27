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
	// backend#2198 起 `GET /app/buyers/me/subscriptions/{storeId}` 回統一
	// envelope：`{ code, message, data: { subscribed } }`。旗標只在 `data`
	// 之下 —— 舊的頂層 `subscribed` 與 `followed` / `is_subscribed` 別名都已從
	// wire 消失，一次切不留相容層（10 檔 §14.5.1）。
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
	// gateway `SubscribeReq` 收 `storeId`（subscription.go:151）。這裡以前送
	// `seller_id`，與 setSellerFollow 送的 `store_id` 兩邊不一致，binding 收不到
	// 值——收斂成同一個名字，順手把那個窗口關掉。
	await buyerAppClient().followStore({ storeId: String(id) });
	return true;
}
