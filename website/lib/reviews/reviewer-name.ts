import { reviewReviewAuthorAnonymousName } from "@hawkr/ui-text/keys";
import type { UiText } from "@/lib/i18n/ui-text";

/** 評價者沒有名字時顯示目錄的匿名名稱；所有評價畫面共用這一處，資料層一律傳 undefined。 */
export function reviewerName(
	ut: UiText,
	name: string | null | undefined,
): string {
	return name?.trim() || ut(reviewReviewAuthorAnonymousName());
}
