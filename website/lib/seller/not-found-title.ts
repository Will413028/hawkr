import { createTranslator } from "next-intl";
import { messagesFor } from "@/lib/intl/config";
import { localeFromParam } from "@/lib/seo/metadata";

/** 找不到店家時的頁面標題；RSC 不用 `getTranslations()`，語系取自 `params.locale`。 */
export function sellerNotFoundTitle(locale: string): string {
	const pageLocale = localeFromParam(locale);
	return createTranslator({
		locale: pageLocale,
		messages: messagesFor(pageLocale),
		namespace: "seller",
	})("notFound");
}
