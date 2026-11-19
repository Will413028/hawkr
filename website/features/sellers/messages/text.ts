import { createTranslator } from "next-intl";
import { messagesFor } from "@/lib/intl/config";
import { localeFromParam } from "@/lib/seo/metadata";

/** 店家留言頁與討論串頁的標題文字；RSC 不用 `getTranslations()`，語系取自 `params.locale`。 */
export function sellerMessagesText(locale: string) {
	const t = createTranslator({
		locale: localeFromParam(locale),
		messages: messagesFor(localeFromParam(locale)),
		namespace: "seller",
	});
	return {
		notFound: () => t("notFound"),
		title: (name: string) => t("messagesTitle", { name }),
	};
}
