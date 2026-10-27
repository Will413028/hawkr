import SellerLandingPage from "@/features/seller/landing/SellerLandingPage";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata({
	title: "自助開店｜小農、鮮食與食品品牌",
	description:
		"Hawkr 提供小農、鮮食門市、食品品牌與加工職人自助開店工具。先建立市集內品牌賣場，再依步驟補齊商品照片、規格、價格、庫存，以及適用的金流與物流設定；完成商家與商品必要審核後開始販售，透過賣家中心管理商品、訂單、出貨、顧客、優惠、品牌動態、站內訊息與營運資料，並查看開店流程及公開合作方案。",
	path: "/seller",
});

export default SellerLandingPage;
