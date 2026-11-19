import { normalizePostTag } from "@/lib/posts/tags";

/**
 * `/feed` 的 URL 篩選範圍。伺服器的貼文列表只接受「店」或「標籤」其中一種範圍（兩個一起給是 invalid_argument），
 * 所以兩者同時出現在網址時店優先、標籤不生效；站內連結不會同時產生兩者（標籤連結是 `?hashtag=`、店是 `?sellerId=`），
 * 同時出現只會來自手改的網址。
 */
export interface FeedScope {
	sellerId: string;
	/** 已正規化；店存在時一律是空字串。 */
	hashtag: string;
	topic: string;
	postId: string;
}

export function feedScopeOf(params: URLSearchParams): FeedScope {
	const sellerId = params.get("sellerId") || params.get("seller") || "";
	return {
		sellerId,
		hashtag: sellerId ? "" : normalizePostTag(params.get("hashtag") || ""),
		topic: params.get("topic") || "",
		postId: params.get("post") || "",
	};
}
