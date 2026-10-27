import type { Image } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import type {
	BannerAction as BannerActionPB,
	Banner as BannerPB,
} from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import { contentPublicClient } from "@/lib/connect/content";

/**
 * 首頁與 megamenu 的 banner：ContentPublicService 的 ListLiveBanners（只回啟用中、在期間內的，
 * 依顯示順序排好）轉成畫面用的形狀。點擊行為換成 union，方便畫面分支。
 */
export type BannerAction =
	| { type: "popup" }
	| { type: "url"; url: string }
	| { type: "product" | "post"; id: string };

export interface Banner {
	id: string;
	title: string;
	/** 輪播的圖。 */
	previewImage?: Image;
	/** 彈窗的大圖。 */
	popupImage?: Image;
	action?: BannerAction;
	buttonText?: string;
}

function toAction(
	action: BannerActionPB | undefined,
): BannerAction | undefined {
	const kind = action?.kind;
	switch (kind?.case) {
		case "popup":
			return { type: "popup" };
		case "url": {
			const url = kind.value.trim();
			return url ? { type: "url", url } : undefined;
		}
		case "productId":
		case "postId": {
			// ID 是不透明的字串（ADR 2026-09-25-row-identity-by-data-kind D5），組網址時再編碼。
			const id = kind.value.trim();
			if (!id) return undefined;
			return { type: kind.case === "productId" ? "product" : "post", id };
		}
	}
	return undefined;
}

/** 轉成畫面用的 banner；沒有輪播圖的不顯示。 */
export function toBanner(b: BannerPB): Banner | null {
	if (!b.id || !b.previewImage) return null;
	return {
		id: b.id,
		title: b.title.trim(),
		previewImage: b.previewImage,
		popupImage: b.popupImage,
		action: toAction(b.action),
		buttonText: b.buttonText.trim() || undefined,
	};
}

/**
 * 買家畫面的 banner（第一頁；伺服器預設每頁 50 則，上架中的 banner 不會超過這個數量）。
 * 空的清單回 `[]`；傳輸或伺服器的錯誤 reject，由 transport 依錯誤模型 D7 重試，不會把一時的故障快取成
 * 空的結果。
 */
export async function fetchBanners(): Promise<Banner[]> {
	const res = await contentPublicClient().listLiveBanners({});
	return res.banners.map(toBanner).filter((b): b is Banner => b !== null);
}
