/**
 * 「這個配送方式需要哪一種收件目標，而它齊備了沒」—— 購物車與結帳共用的唯一判準。
 *
 * 🔴 **未知配送方式一律回 `false`（fail-closed）。** 這條規則的直接
 * 落地：舊版購物車與結帳各自寫了一個三元
 *
 * ```ts
 * isStorePickup ? selectedPickup != null : isHomeDelivery ? selectedAddress != null : true
 * ```
 *
 * 最後那個 `: true` 的語意是「沒見過的配送方式就不用選收件地址也不用選門市」，
 * 於是 `3005` 商家自送可以一路結帳完成，後端拿到
 * 的訂單既無地址也無門市，而畫面上沒有任何異常。
 *
 * ⚠️ 「未知」是會被逐一消化掉的狀態，不是永久名單：某個方式定義了它要的收件目標，
 * 就從 `unsupported` 移到對應的分支。
 *
 * 值域取自 CodeMeta 生成檔（`lib/codemeta/enums`），本檔不手寫碼值。
 */
import { normalizeShippingMethod, ShippingMethod } from "@/lib/codemeta/enums";

/** 這個配送方式要求哪一種收件目標。 */
export type ShippingDestinationKind =
	/** 宅配到府：要一筆收件地址。 */
	| "address"
	/** 超商店到店：要一間取貨門市。 */
	| "store-pickup"
	/** 🔴 前端還不知道這個 provider 要什麼 —— 一律擋住，不猜。 */
	| "unsupported";

export function shippingDestinationKind(
	method: string | undefined,
): ShippingDestinationKind {
	switch (normalizeShippingMethod(method)) {
		case ShippingMethod.TAQBIN:
			return "address";
		case ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP:
			return "store-pickup";
		default:
			// 3003 Uber（已刪除的功能）、3005 商家自送與後端未來新增的
			// 任何 provider 都走這裡。
			// 它們各自需要什麼收件資料尚未定義，現在猜一個等於放行。
			return "unsupported";
	}
}

/**
 * 收件目標齊備了沒。`targets` 由呼叫端提供 —— 購物車給的是「有沒有選地址／門市」，
 * 結帳給的是「草稿裡的收件人／取貨資料填齊了沒」，判準共用同一個 switch。
 */
export function isShippingDestinationReady(
	method: string | undefined,
	targets: { hasAddress: boolean; hasStorePickup: boolean },
): boolean {
	switch (shippingDestinationKind(method)) {
		case "address":
			return targets.hasAddress;
		case "store-pickup":
			return targets.hasStorePickup;
		default:
			return false;
	}
}
