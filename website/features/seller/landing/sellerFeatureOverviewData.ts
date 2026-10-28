export interface SellerFeatureOverviewGroup {
	readonly id: string;
	readonly icon: SellerFeatureOverviewIconName;
	readonly title: string;
	readonly summary: string;
	readonly features: readonly string[];
}

export type SellerFeatureOverviewIconName =
	| "storefront"
	| "products"
	| "inventory"
	| "orders"
	| "fulfillment"
	| "customers"
	| "content"
	| "messages"
	| "operations";

export const SELLER_FEATURE_OVERVIEW: readonly SellerFeatureOverviewGroup[] = [
	{
		id: "seller-feature-storefront",
		icon: "storefront",
		title: "品牌店面",
		summary:
			"在 Hawkr 市集內建立買家可瀏覽的品牌店面，統一維護公開資料與分享入口；這是市集內店面，不是獨立網站或自有網域服務。",
		features: [
			"編輯商店名稱、品牌介紹、所在地與店鋪亮點",
			"上傳商店 Logo 與封面，填入品牌官網等公開連結",
			"以買家視角預覽目前公開的店面內容",
			"產生可分享的 QR 商家名片",
		],
	},
	{
		id: "seller-feature-products",
		icon: "products",
		title: "商品管理",
		summary:
			"從食品資訊、圖片到規格價格集中建檔；商品上架仍須完成對應的商家資格及資料審核。",
		features: [
			"單筆建立商品，或使用 CSV 批次匯入",
			"編輯名稱、描述、分類、產地與最多 9 張商品圖片",
			"建立多組 SKU，分別設定規格、原價與優惠價",
			"管理草稿、上下架狀態",
		],
	},
	{
		id: "seller-feature-inventory",
		icon: "inventory",
		title: "庫存管理",
		summary:
			"集中查看各商品 SKU 的實際庫存與庫存價值；安全庫存及售罄時間依近期平台訂單估算，只供補貨判斷參考。",
		features: [
			"彙整目前庫存數量與庫存價值",
			"篩選庫存不足或庫存充足的商品",
			"查看近期每週銷量、建議安全庫存與預估售罄時間",
			"匯出目前篩選結果，或回到商品頁調整庫存",
		],
	},
	{
		id: "seller-feature-orders",
		icon: "orders",
		title: "訂單管理",
		summary:
			"依訂單生命週期整理交易資料；「已結束」同時包含完成、取消與退款訂單，不等同實際成交筆數。",
		features: [
			"分開查看待出貨、配送中與已結束訂單",
			"依訂單編號或顧客搜尋，並篩選待處理項目",
			"查看商品、顧客、付款資料與狀態時間軸",
			"核對買家實付；後端有回傳時，同頁列出平台手續費與賣家入帳金額",
		],
	},
	{
		id: "seller-feature-fulfillment",
		icon: "fulfillment",
		title: "出貨與售後",
		summary:
			"商家認證完成且配送方式通過服務商開通後，才能使用對應操作；實際可用配送方式依包裹條件及服務商結果為準。",
		features: [
			"申請黑貓及 7‑11 配送",
			"依包裹尺寸與重量試算可用配送方式及費用",
			"處理黑貓預約叫件、7‑11 取號與寄件單列印",
			"查詢物流進度與配送異常，並處理退貨、退款及售後申訴",
		],
	},
	{
		id: "seller-feature-customers",
		icon: "customers",
		title: "顧客經營",
		summary:
			"整理在本店已有交易或會員關係的買家資料，協助辨識服務對象；目前不是外部名單匯入工具或獨立會員資料庫。",
		features: [
			"依姓名或電話搜尋顧客",
			"依消費金額、訂單數與最近購買時間排序",
			"分類為一般顧客、VIP 或黑名單，並使用自訂標籤篩選",
			"查看累計訂單、累計消費與實際購買紀錄",
		],
	},
	{
		id: "seller-feature-content",
		icon: "content",
		title: "內容與優惠",
		summary:
			"用品牌動態補充商品之外的產地與出貨資訊，再依實際活動條件發行本店優惠券；優惠券發行後不可直接編輯，需調整時須收回再重建。",
		features: [
			"建立與編輯文字、圖片品牌動態",
			"在動態加入 hashtag，並連結一項目前販售的商品",
			"建立現金折抵或百分比折扣優惠券",
			"設定消費門檻、有效期間、使用上限與適用買家",
		],
	},
	{
		id: "seller-feature-messages",
		icon: "messages",
		title: "訊息與通知",
		summary:
			"把顧客對話與營運提醒留在賣家中心集中處理；目前提供站內訊息與通知，不代表已串接 LINE、簡訊、Email 或其他客服通路。",
		features: [
			"查看顧客對話、最後回覆時間與未讀數量",
			"依顧客、電話、訊息內容、分類、標籤或日期搜尋",
			"在對話旁核對近期訂單與累計消費資料",
			"查看訂單、退款、聊天、互動及系統通知，並調整通知偏好",
		],
	},
	{
		id: "seller-feature-operations",
		icon: "operations",
		title: "營運管理",
		summary:
			"彙整開店進度、日常待辦、資格與市場研究資訊；統計只供營運參考，農產行情也不是本店營收、採購成本或獲利預測。",
		features: [
			"查看開店任務進度、今日待辦與常用操作",
			"核對近 7 日營收、待出貨訂單、商品瀏覽與近 7 日新追蹤者",
			"查看商家認證、銀行帳戶及 PayUni 建店狀態；網頁可核對完成訂單，但不提供提款操作",
			"查閱農業部批發市場實際行情、條件式七日預測與跨市場比較",
		],
	},
];
