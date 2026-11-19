/**
 * 店家封鎖買家（`SellerAppService.BlockBuyer`）：以買家寫的留言指定對象，伺服器從留言解出買家並檢查那則留言
 * 在呼叫者店家的內容裡。已經封鎖時也成功（`changed` 是 false），所以重複按不是錯誤。
 * 討論區發言用同一個方法的 `forum_entry_id`。
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { sellerAppClient } from "../connect/seller";
import { imageUri } from "../media/image";

type SellerClient = ReturnType<typeof sellerAppClient>;

export async function blockBuyerByComment(
	commentId: string,
	client: () => SellerClient = sellerAppClient,
): Promise<{ changed: boolean }> {
	const res = await client().blockBuyer({
		source: { case: "commentId", value: commentId },
	});
	return { changed: res.changed };
}

export async function blockBuyerByForumEntry(
	forumEntryId: string,
	client: () => SellerClient = sellerAppClient,
): Promise<{ changed: boolean }> {
	const res = await client().blockBuyer({
		source: { case: "forumEntryId", value: forumEntryId },
	});
	return { changed: res.changed };
}

/** 封鎖名單一頁的筆數（契約預設 20，上限 100）。 */
export const BLOCKED_BUYERS_PAGE_SIZE = 20;

export interface BlockedBuyerItem {
	buyerId: string;
	/** 買家已刪除帳號：只有 id，沒有名稱與頭像。 */
	deleted: boolean;
	name: string;
	avatarUrl?: string;
	/** 封鎖時間，ISO 8601；伺服器沒帶時不設。 */
	blockedAt?: string;
}

export interface BlockedBuyersPage {
	items: BlockedBuyerItem[];
	nextPageToken: string;
}

/** 店家封鎖的買家，新到舊（`ListBlockedBuyers`）。 */
export async function listBlockedBuyers(
	pageToken = "",
	client: () => SellerClient = sellerAppClient,
): Promise<BlockedBuyersPage> {
	const res = await client().listBlockedBuyers({
		pageSize: BLOCKED_BUYERS_PAGE_SIZE,
		pageToken,
	});
	return {
		items: res.blockedBuyers.map((blocked) => ({
			buyerId: blocked.buyer?.id ?? "",
			deleted: blocked.buyer?.deleted ?? false,
			name: blocked.buyer?.displayName ?? "",
			avatarUrl: imageUri(blocked.buyer?.avatar, "avatar"),
			blockedAt: blocked.createTime
				? timestampDate(blocked.createTime).toISOString()
				: undefined,
		})),
		nextPageToken: res.nextPageToken,
	};
}

/** 解除封鎖；本來就沒有封鎖時也成功（`changed` 是 false）。 */
export async function unblockBuyer(
	buyerId: string,
	client: () => SellerClient = sellerAppClient,
): Promise<{ changed: boolean }> {
	const res = await client().unblockBuyer({ buyerId });
	return { changed: res.changed };
}
