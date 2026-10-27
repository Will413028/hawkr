import JsonLd from "@/components/seo/JsonLd";
import FaqPage from "@/features/content/faq/FaqPage";
import { faqCategories } from "@/lib/content/data";
import { faqSchema } from "@/lib/seo/jsonld";
import { localeFromParam, pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata({
	title: "常見問題",
	description:
		"Hawkr 常見問題一次解答：從註冊登入、下單流程與付款方式，到配送到貨時間、退換貨與售後處理，以及賣家入駐流程、帳號設定與發票相關等常見疑問，全部整理在這裡。無論你是第一次採買的買家或想上架的賣家，都能更快上手平台操作，安心採買與販售每一天的新鮮食材。",
	path: "/faq",
});

const faqItems = faqCategories.flatMap((cat) =>
	cat.items.map((it) => ({ q: it.q, a: it.a })),
);

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const locale = localeFromParam((await params).locale);
	return (
		<>
			<JsonLd data={faqSchema(faqItems, locale)} />
			<FaqPage />
		</>
	);
}
