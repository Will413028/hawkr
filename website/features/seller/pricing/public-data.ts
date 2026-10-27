import type { AppIconName } from "@/components/shared/AppIcon";

export type PublicSellerPlanId = "starter" | "pro";
export type PublicBillingCycle = "monthly" | "annual";

export interface PublicSellerPlan {
	id: PublicSellerPlanId;
	code: "STARTER" | "PRO";
	name: string;
	contractName: string;
	subtitle: string;
	monthlyFee: number;
	annualFee: number;
	annualSavings: number;
	transactionFeeRate: number;
	stage: string;
	zeroSalesNote: string;
	excluded: string;
	cta: string;
	icon: AppIconName;
}

/**
 * 公開頁只輸出這兩筆資料。Free 沿用既有 STARTER 帳務代碼，避免改名
 * 造成既有訂單、申請與合約附件失聯；舊版方案只留在伺服器相容層。
 */
export const PUBLIC_SELLER_PLANS: PublicSellerPlan[] = [
	{
		id: "starter",
		code: "STARTER",
		name: "Free 免費",
		contractName: "正式合約與帳務沿用 Starter 名稱",
		subtitle: "先成交，再依成交金額付費",
		monthlyFee: 0,
		annualFee: 0,
		annualSavings: 0,
		transactionFeeRate: 0.06,
		stage: "適合剛開始經營線上通路，想用最低固定成本驗證商品。",
		zeroSalesNote: "當月沒有有效交易，不收方案月費，也不收成交服務費。",
		excluded: "金流手續費、物流費、廣告費與人工服務費另計。",
		cta: "選 Free",
		icon: "leaf",
	},
	{
		id: "pro",
		code: "PRO",
		name: "Pro 專業",
		contractName: "正式合約與帳務名稱同為 Pro",
		subtitle: "固定方案費，成交服務費 0%",
		monthlyFee: 2_990,
		annualFee: 29_900,
		annualSavings: 5_980,
		transactionFeeRate: 0,
		stage: "適合已有穩定供貨與專責人員，重視每月平台成本可預測。",
		zeroSalesNote: "當月沒有有效交易，仍收方案月費 NT$2,990。",
		excluded: "0% 只指成交服務費；金流、物流、廣告與人工服務費仍另計。",
		cta: "選 Pro",
		icon: "work",
	},
];

export const PUBLIC_MANAGED_SERVICE_ADD_ON = {
	id: "managed",
	name: "加值代行包",
	feeRate: 0.06,
	feeLabel: "每筆有效交易額外 6%",
	tagline: "把商品上架與電商圖整理交給 Hawkr 上架支援團隊。",
	includes: [
		"上架支援團隊依賣家確認的商品資料協助上架",
		"精修賣家提供且有權使用的電商圖片",
	],
	sellerKeeps: "賣家仍負責接單確認、備貨、出貨、消費者客服與售後。",
	boundary:
		"代行服務費與方案成交服務費分開計算；Free 加購後合計 12%，Pro 加購後合計 6%，金流與物流費仍另計。",
	cta: "加購代行包",
} as const;

export interface PublicPricingFaq {
	category: PublicPricingFaqCategoryId;
	question: string;
	answer: string;
}

export type PublicPricingFaqCategoryId = "plans" | "service" | "changes";

export interface PublicPricingFaqCategory {
	id: PublicPricingFaqCategoryId;
	label: string;
	description: string;
	icon: AppIconName;
}

export const PUBLIC_SELLER_PRICING_FAQ_CATEGORIES: PublicPricingFaqCategory[] =
	[
		{
			id: "plans",
			label: "方案與費用",
			description: "Free、Pro 與年繳計算",
			icon: "wallet",
		},
		{
			id: "service",
			label: "代行與責任",
			description: "加值服務與合作分工",
			icon: "work",
		},
		{
			id: "changes",
			label: "退款與變更",
			description: "退款計費與方案調整",
			icon: "reorder",
		},
	];

export const PUBLIC_SELLER_PRICING_FAQS: PublicPricingFaq[] = [
	{
		category: "plans",
		question: "Free 包含哪些費用？",
		answer:
			"Free 不收方案月費；在 Hawkr 市集完成有效交易時，收取商品實際成交金額 6% 的成交服務費。金流、物流、廣告與人工服務另計。正式合約與帳務仍沿用 Starter 名稱。",
	},
	{
		category: "plans",
		question: "Pro 的 0% 是不是所有費用都免了？",
		answer:
			"不是。0% 只指成交服務費；方案月費 NT$2,990、金流手續費、物流費、廣告費與人工服務費仍然另計。",
	},
	{
		category: "plans",
		question: "Pro 年繳怎麼計算？",
		answer:
			"Pro 年繳優惠價為 NT$29,900，含 12 個月方案使用期；相較每月 NT$2,990 連續支付 12 個月，共省 NT$5,980。Free 的固定方案費仍為 0 元。",
	},
	{
		category: "service",
		question: "加值代行包包含什麼？",
		answer:
			"Hawkr 上架支援團隊會依賣家確認的商品資料協助上架，並精修賣家提供且有權使用的電商圖片。每筆有效交易額外收取 6% 代行服務費；賣家仍負責接單確認、備貨、出貨、消費者客服與售後。",
	},
	{
		category: "plans",
		question: "當月沒有成交還要付費嗎？",
		answer:
			"Free 沒有有效交易時，不收方案月費，也不收成交服務費。Pro 月繳每月收取 NT$2,990；年繳以 NT$29,900 提供 12 個月方案使用期。沒有有效交易時不另收成交服務費，但固定方案費不因此變更。",
	},
	{
		category: "changes",
		question: "退款後成交服務費怎麼處理？",
		answer:
			"依原訂單費率與退款後保留的商品實際成交金額重新計算，多收的部分回沖。全額退款時該筆成交服務費歸零。",
	},
	{
		category: "changes",
		question: "可以變更方案嗎？",
		answer:
			"可以提出變更申請。實際生效日、差額、年繳續約與取消規則會依正式合約確認；方案變更不回溯已成立的舊訂單。",
	},
	{
		category: "service",
		question: "平台和賣家各自負責什麼？",
		answer:
			"平台提供食品市集、交易、結算與對帳基礎。賣家負責食品事實與合法性、售價、庫存、接單、出貨、客服及售後；需要人工上架、內容或拍攝時另行洽談。",
	},
];

export const PUBLIC_TAX_NOTE =
	"上列方案費與百分比均為含 5% 營業稅的價格，結帳後不再另加。";
