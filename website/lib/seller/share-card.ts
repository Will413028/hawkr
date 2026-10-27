import { defaultLocale } from "@/lib/intl/locales";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { sellerCardUrl } from "@/lib/seo/urls";

/**
 * 名片卡的輸入。兩個呼叫點的來源形狀不同，所以這裡只收「已經對映好」的欄位，
 * 呼叫端各自負責換名（見 SellerProfileV2 / SellerShareCardPreview）：
 *   - 公開賣家頁：`fetchSeller` 的前端顯示契約（`logo_url` → `logoUrl`）
 *   - 賣家後台：`GetMyStore` 的店家與表單（圖片是 `imageUri` 組好的網址）
 */
export interface SellerShareSource {
	storeId?: number | string | null;
	city?: string | null;
	/** Public seller wire → `coverKey` (objectKey SSOT, via fetchSeller). */
	coverImage?: string | null;
	/** Seller-center cover display URL. resolveCdnUrl eats both. */
	coverUrl?: string | null;
	description?: string | null;
	district?: string | null;
	location?: string | null;
	logoUrl?: string | null;
	name?: string | null;
	prize?: string | null;
	sellerName?: string | null;
	storeName?: string | null;
}

export interface SellerShareCardData {
	sellerId: string;
	/** 對外顯示碼；優先於 sellerId 決定 shareUrl/QR 與「商店 ID」欄位顯示值。 */
	displayId: string;
	name: string;
	description: string;
	location: string;
	/** 賣家自填獎項（free-text）；空字串 = 未填，不顯示。 */
	prize: string;
	avatarUrl: string;
	coverUrl: string;
	shareUrl: string;
	shareText: string;
}

function pickText(
	...values: Array<string | number | null | undefined>
): string {
	for (const value of values) {
		const text = String(value ?? "").trim();
		if (text) return text;
	}
	return "";
}

/** The card's text is zh-TW (seller-written content stays zh-TW, ADR 2026-10-17 D5), so its link is the zh-TW card page. */
export function buildSellerShareUrl(sellerId: string): string {
	return sellerCardUrl(sellerId, defaultLocale);
}

export function buildSellerShareCardData(
	sellerId: string,
	seller: SellerShareSource | null | undefined,
	displayId?: string,
): SellerShareCardData {
	const name = pickText(
		seller?.name,
		seller?.storeName,
		seller?.sellerName,
		"Hawkr 商家",
	);
	const location =
		pickText(seller?.location) ||
		[seller?.city, seller?.district]
			.map((part) => String(part ?? "").trim())
			.filter(Boolean)
			.join(" ");
	const description = pickText(
		seller?.description,
		"前往 Hawkr 查看商家介紹與商品。",
	);
	const resolvedDisplayId = pickText(displayId) || sellerId;
	const shareUrl = buildSellerShareUrl(resolvedDisplayId);

	return {
		sellerId,
		displayId: resolvedDisplayId,
		name,
		// ⚠️ 拿不到地點就是空字串，**不要**退回品牌名：名片上那一行的語意是「這家店
		// 在哪裡」，填「Hawkr」會被讀成一個地名。呼叫端負責空值不渲染。
		location,
		description,
		prize: pickText(seller?.prize),
		avatarUrl: resolveCdnUrl(pickText(seller?.logoUrl)),
		coverUrl: resolveCdnUrl(pickText(seller?.coverImage, seller?.coverUrl)),
		shareUrl,
		shareText: `${name}\n${description}\n商店 ID：${resolvedDisplayId}\n${shareUrl}`,
	};
}
