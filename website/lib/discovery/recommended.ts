import { StoreCardStatus } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { discoveryPublicClient } from "@/lib/connect/discovery";
import type { PublicTransportOptions } from "@/lib/connect/transport";
import { imageUri } from "@/lib/media/image";

/** SuggestRecommendedSellers 回傳的一家營業中的店，已轉成可以序列化進 bootstrap 的純物件。 */
export interface RecommendedStore {
	id: string;
	storeCode: string;
	name: string;
	logoUrl: string;
	coverUrl: string;
	townCode: string;
	ratingAverage?: number;
	reviewCount?: number;
	followerCount?: number;
}

export interface FetchRecommendedStoresOptions extends PublicTransportOptions {
	signal?: AbortSignal;
}

/**
 * 推薦店家的排行（伺服器已依分數排序，這裡不重排）。清單上的店都應該是營業中且有店名與店網址；
 * 不是的項目略過，不顯示成沒有連結的卡片。
 */
export async function fetchRecommendedStores(
	limit: number,
	opts: FetchRecommendedStoresOptions = {},
): Promise<RecommendedStore[]> {
	const { signal, ...transport } = opts;
	const res = await discoveryPublicClient(transport).suggestRecommendedSellers(
		{ maxResults: limit },
		{ signal },
	);
	const out: RecommendedStore[] = [];
	for (const seller of res.sellers) {
		const card = seller.store;
		const name = card?.name?.trim() ?? "";
		if (
			!card ||
			card.status !== StoreCardStatus.OPEN ||
			!card.id ||
			!card.storeCode ||
			!name
		) {
			continue;
		}
		out.push({
			id: card.id,
			storeCode: card.storeCode,
			name,
			logoUrl: imageUri(card.logo, "avatar") ?? "",
			coverUrl: imageUri(card.coverImage, "card") ?? "",
			townCode: card.townCode,
			...(card.ratingAverage !== undefined
				? { ratingAverage: card.ratingAverage }
				: {}),
			...(card.reviewCount !== undefined
				? { reviewCount: card.reviewCount }
				: {}),
			...(seller.followerCount !== undefined
				? { followerCount: seller.followerCount }
				: {}),
		});
	}
	return out;
}
