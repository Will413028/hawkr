import { create } from "@bufbuild/protobuf";
import {
	type BannerAction,
	BannerAction_PopupSchema,
	BannerActionSchema,
} from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";

/**
 * Banner「點擊動作」的契約 ⇄ 表單轉換。
 *
 * 契約（ContentAdminService 的 Banner.action）是 oneof：`popup`、`url`、`product_id`、`post_id` 四選一，
 * popup 不附帶值（型別本身就是完整語意：點擊後顯示彈窗大圖）。修改時 action 在 update_mask 裡就整個換掉。
 *
 * 這一層獨立成模組是為了讓「送出的 action 一定是完整的」變成可測的不變式。
 *
 * 為什麼需要（2026-08-05 實際資料遺失）：編輯器原本用空物件起手、只有 url / product / post 三個分支會填值，
 * **「顯示彈窗大圖」完全沒有分支** → 載入一張 popup banner、什麼都不改按存檔，DB 裡的 popup 就被整個洗掉，
 * 而且全程沒有任何錯誤。同一個洞還吃掉另外三種組合：選了 url/product/post 卻把值留空。
 *
 * 因此 `actionToProto` **永遠帶 kind**。選了 url／商品／貼文卻沒填值時照樣送出空字串，由伺服器以欄位錯誤擋下
 * （`banner.action.url` 等，編輯器標在點擊行為的欄位上）；規則只在伺服器一處。
 */

/** 點擊動作的種類。 */
export const BANNER_ACTION_TYPES = ["popup", "url", "product", "post"] as const;

export type BannerActionType = (typeof BANNER_ACTION_TYPES)[number];

/** 沒有 action（或讀不懂）時的預設：顯示彈窗大圖。 */
export const DEFAULT_BANNER_ACTION_TYPE: BannerActionType = "popup";

/** 編輯器的表單狀態。空字串＝該欄未填。 */
export interface BannerActionForm {
	actionType: BannerActionType;
	actionUrl: string;
	actionId: string;
}

export function isBannerActionType(v: unknown): v is BannerActionType {
	return (
		typeof v === "string" &&
		(BANNER_ACTION_TYPES as readonly string[]).includes(v)
	);
}

/** 契約 → 表單狀態。沒有 action 時退回預設（popup ＋ 兩個空值）。 */
export function actionFromProto(
	action: BannerAction | undefined,
): BannerActionForm {
	const form: BannerActionForm = {
		actionType: DEFAULT_BANNER_ACTION_TYPE,
		actionUrl: "",
		actionId: "",
	};
	const kind = action?.kind;
	switch (kind?.case) {
		case "url":
			return { ...form, actionType: "url", actionUrl: kind.value };
		case "productId":
			return { ...form, actionType: "product", actionId: kind.value };
		case "postId":
			return { ...form, actionType: "post", actionId: kind.value };
	}
	return form;
}

/** 表單狀態 → 契約的 action。**一定帶 kind，永遠不會是空的 action。** */
export function actionToProto(form: BannerActionForm): BannerAction {
	switch (form.actionType) {
		case "popup":
			return create(BannerActionSchema, {
				kind: { case: "popup", value: create(BannerAction_PopupSchema) },
			});
		case "url":
			return create(BannerActionSchema, {
				kind: { case: "url", value: form.actionUrl.trim() },
			});
		case "product":
			return create(BannerActionSchema, {
				kind: { case: "productId", value: form.actionId.trim() },
			});
		case "post":
			return create(BannerActionSchema, {
				kind: { case: "postId", value: form.actionId.trim() },
			});
	}
}
