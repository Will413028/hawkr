import SellerGuidePage from "@/features/content/seller-guide/SellerGuidePage";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata({
	title: "賣家操作指南",
	description:
		"Hawkr 賣家操作指南：用文字與步驟說明開店申請、商家認證、收款與物流、商品與庫存、訂單出貨、售後、行銷、客戶與通知設定。",
	path: "/seller-guide",
});

export default function Page() {
	return <SellerGuidePage />;
}
