/**
 * 店家封鎖買家（`SellerAppService.BlockBuyer`）：以買家寫的留言指定對象，伺服器從留言解出買家並檢查那則留言
 * 在呼叫者店家的內容裡。已經封鎖時也成功（`changed` 是 false），所以重複按不是錯誤。
 * 討論區發言用同一個方法的 `forum_entry_id`。
 */

import { sellerAppClient } from "../connect/seller";

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
