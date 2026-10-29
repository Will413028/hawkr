import type { StoryItem } from "@/features/feed/components/StoryRail";
import type { RecommendedStore } from "@/lib/discovery/recommended";
import { storeLocation } from "@/lib/seller/store-location";

const RAIL_SIZE = 12;

/** 推薦店家排行轉成故事 rail 的項目，順序照伺服器；縣市與鄉鎮市區由 town_code 轉出。 */
export function recommendedStoriesFromStores(
	stores: readonly RecommendedStore[],
): StoryItem[] {
	return stores.slice(0, RAIL_SIZE).map((store) => {
		const location = storeLocation(store.townCode);
		return {
			id: store.id,
			storeCode: store.storeCode,
			name: store.name,
			avatar: store.logoUrl,
			city: location?.city,
			district: location?.district,
		} satisfies StoryItem;
	});
}
