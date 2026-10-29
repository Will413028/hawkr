import type { StoreCard } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { StoreCardStatus } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { imageUri } from "@/lib/media/image";
import { sellerPath } from "@/lib/seo/urls";

export interface StoreCardText {
	paused: string;
	closed: string;
	closedStore: string;
}

export type StoreCardLabel = "paused" | "closed";

export interface StoreCardView {
	id: string;
	name: string;
	logoUrl: string;
	label?: StoreCardLabel;
	labelText?: string;
	/** 空字串表示不可點：暫停、已關閉、狀態不明或沒有店網址的店都沒有公開的店面頁。 */
	href: string;
}

/**
 * 參照入口上一家店的呈現（B11）：營業中可點並以 store_code 導頁；暫停營業與已關閉顯示標籤、不可點；
 * 名稱沒有設定（個資清除之後）顯示「已關閉的店家」。卡片整個沒有（沒公開過或不存在）時回 undefined，
 * 由呼叫端決定怎麼處理這一列，不在這裡猜。
 */
export function storeCardView(
	card: StoreCard | undefined,
	text: StoreCardText,
): StoreCardView | undefined {
	if (!card) return undefined;
	const name = card.name?.trim() || text.closedStore;
	const logoUrl = imageUri(card.logo, "avatar") ?? "";
	switch (card.status) {
		case StoreCardStatus.OPEN:
			return {
				id: card.id,
				name,
				logoUrl,
				href: card.storeCode ? sellerPath(card.storeCode) : "",
			};
		case StoreCardStatus.PAUSED:
			return {
				id: card.id,
				name,
				logoUrl,
				label: "paused",
				labelText: text.paused,
				href: "",
			};
		case StoreCardStatus.CLOSED:
			return {
				id: card.id,
				name,
				logoUrl,
				label: "closed",
				labelText: text.closed,
				href: "",
			};
		default:
			return { id: card.id, name, logoUrl, href: "" };
	}
}
