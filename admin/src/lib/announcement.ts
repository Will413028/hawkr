/**
 * 系統公告的畫面邏輯：受眾的顯示文字與選項、表單驗證、建立請求。
 * 公告一律只寫英文（單一 title／body，不分語系，所有使用者看到同一份）。
 */

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateAnnouncementRequestSchema } from "@hawkr/api/gen/hawkr/admin/notification/v1alpha1/notification_pb";
import { NotificationAudience } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { trimGoSpace } from "@hawkr/api/text/server-text";
import { unknownText } from "./ui-text";

/** 受眾不在共用文案目錄（只有後台這一端顯示）：UNSPECIFIED 與不認得的值顯示「未知」。 */
const AUDIENCE_TEXT: Partial<Record<NotificationAudience, string>> = {
	[NotificationAudience.BUYER]: "買家",
	[NotificationAudience.STORE]: "賣家",
};

export function audienceText(audience: NotificationAudience): string {
	return AUDIENCE_TEXT[audience] ?? unknownText();
}

export const AUDIENCE_OPTIONS: readonly NotificationAudience[] = [
	NotificationAudience.BUYER,
	NotificationAudience.STORE,
];

export interface AnnouncementForm {
	title: string;
	body: string;
	audiences: readonly NotificationAudience[];
}

export function emptyAnnouncementForm(): AnnouncementForm {
	return { title: "", body: "", audiences: AUDIENCE_OPTIONS };
}

/** 送出前的欄位檢查；通過回 null。長度上限由伺服器訂，超過時顯示伺服器的訊息。 */
export function announcementFormError(form: AnnouncementForm): string | null {
	if (trimGoSpace(form.title) === "" || trimGoSpace(form.body) === "") {
		return "請填入標題和內容";
	}
	if (form.audiences.length === 0) return "請至少選一個發送對象";
	return null;
}

export function toggleAudience(
	audiences: readonly NotificationAudience[],
	audience: NotificationAudience,
): NotificationAudience[] {
	return audiences.includes(audience)
		? audiences.filter((a) => a !== audience)
		: AUDIENCE_OPTIONS.filter((a) => a === audience || audiences.includes(a));
}

/** 建立請求：`requestId` 在同一次對話框裡固定，失敗重送時伺服器以它去重。 */
export function createAnnouncementRequest(
	form: AnnouncementForm,
	requestId: string,
): MessageInitShape<typeof CreateAnnouncementRequestSchema> {
	return {
		audiences: [...form.audiences],
		title: trimGoSpace(form.title),
		body: trimGoSpace(form.body),
		requestId,
	};
}
