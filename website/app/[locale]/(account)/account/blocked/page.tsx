import { createTranslator } from "next-intl";
import BlockedListClient from "@/features/account/screens/blocked/BlockedListClient";
import { messagesFor } from "@/lib/intl/config";
import { localeFromParam } from "@/lib/seo/metadata";

export async function generateMetadata({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const locale = localeFromParam((await params).locale);
	const t = createTranslator({
		locale,
		messages: messagesFor(locale),
		namespace: "account.blocked",
	});
	return { title: t("title") };
}

export default function Page() {
	return <BlockedListClient />;
}
