import JsonLd from "@/components/seo/JsonLd";
import { PUBLIC_SELLER_PRICING_FAQS } from "@/features/seller/pricing/public-data";
import SellerPricingPage from "@/features/seller/pricing/SellerPricingPage";
import { breadcrumbSchema, faqSchema } from "@/lib/seo/jsonld";
import { localeFromParam, pageMetadata } from "@/lib/seo/metadata";
import { absoluteUrl } from "@/lib/seo/urls";

export const generateMetadata = pageMetadata({
	title: {
		absolute: "Hawkr 自助開店｜小農、鮮食與食品品牌賣場",
	},
	description:
		"小農、鮮食與食品品牌可在 Hawkr 先建立賣場，完成必要審核後開始販售；查看 Free、Pro 方案、費用與平台／賣家分工，並比較月繳、年繳、成交服務費及加值代行內容，了解商品資料、接單、出貨、客服、售後、交易、結算與對帳責任，再依經營需求自助開店或留下資料洽詢；實際費用與生效規則以正式合約為準。",
	path: "/seller/pricing",
});

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const locale = localeFromParam((await params).locale);
	return (
		<>
			<JsonLd
				data={breadcrumbSchema([
					{ name: "首頁", url: absoluteUrl("/", locale) },
					{ name: "賣家招募", url: absoluteUrl("/seller", locale) },
					{ name: "合作方案", url: absoluteUrl("/seller/pricing", locale) },
				])}
			/>
			<JsonLd
				data={faqSchema(
					PUBLIC_SELLER_PRICING_FAQS.map((faq) => ({
						q: faq.question,
						a: faq.answer,
					})),
					locale,
				)}
			/>
			<SellerPricingPage />
		</>
	);
}
