import type { AppIconName } from "@/components/shared/AppIcon";

export type SellerPlanId = "starter" | "growth" | "pro";

export type CollaborationMode = "guided" | "self_serve" | "project";

export interface SellerPlan {
	id: SellerPlanId;
	/** 對應計費系統的 PlanDefinition.planCode。 */
	code: "STARTER" | "GROWTH" | "PRO";
	name: string;
	subtitle: string;
	/** 方案月費（含稅，新臺幣元）。 */
	monthlyFee: number;
	/** 成交服務費率，0.06 / 0.03 / 0。 */
	transactionFeeRate: number;
	/** 適合的經營階段。 */
	stage: string;
	/** 當月沒有有效交易時的收費結果。 */
	zeroSalesNote: string;
	/** 方案月費與成交服務費以外，一律另計的費用。 */
	excluded: string;
	cta: string;
	icon: AppIconName;
	operatingModel: {
		mode: CollaborationMode;
		label: string;
	};
	/** 依成交規模給的中性提示，不使用沒有數據支持的「最多人選」。 */
	badge?: string;
}

export type ResponsibilityActor = "platform" | "shared" | "seller";

export interface ResponsibilityCell {
	actor: ResponsibilityActor;
	label: string;
}

export interface ResponsibilityRow {
	name: string;
	plans: Record<SellerPlanId, ResponsibilityCell>;
}

export type ServiceInterestId = "none" | ServiceBundleId;

const platform = (label: string): ResponsibilityCell => ({
	actor: "platform",
	label,
});

const shared = (label: string): ResponsibilityCell => ({
	actor: "shared",
	label,
});

const seller = (label: string): ResponsibilityCell => ({
	actor: "seller",
	label,
});

export const SELLER_PLANS: SellerPlan[] = [
	{
		id: "starter",
		code: "STARTER",
		name: "Starter 起步",
		subtitle: "平台陪你一起經營",
		monthlyFee: 0,
		transactionFeeRate: 0.06,
		stage: "尚未有穩定線上成交，想先用低固定風險驗證商品。",
		zeroSalesNote: "當月沒有有效交易，不收方案月費，也不收成交服務費。",
		excluded: "金流手續費、物流費、廣告費與人工服務費另計。",
		cta: "選 Starter 起步",
		icon: "leaf",
		operatingModel: {
			mode: "guided",
			label: "平台共同協作",
		},
	},
	{
		id: "growth",
		code: "GROWTH",
		name: "Growth 成長",
		subtitle: "平台協助，你自主營運",
		monthlyFee: 990,
		transactionFeeRate: 0.03,
		stage: "已有固定品項、出貨流程與穩定訂單，想降低每筆成交成本。",
		zeroSalesNote: "當月沒有有效交易，仍收方案月費 NT$990。",
		excluded: "金流手續費、物流費、廣告費與人工服務費另計。",
		cta: "選 Growth 成長",
		icon: "goods",
		operatingModel: {
			mode: "self_serve",
			label: "平台協助營運",
		},
		badge: "成長階段建議",
	},
	{
		id: "pro",
		code: "PRO",
		name: "Pro 專業",
		subtitle: "完整掌握自己的店",
		monthlyFee: 2990,
		transactionFeeRate: 0,
		stage: "有專責人員與穩定供貨，重視平台成本可預測。",
		zeroSalesNote: "當月沒有有效交易，仍收方案月費 NT$2,990。",
		excluded:
			"0% 只指成交服務費；方案月費、金流、物流、廣告與人工服務費仍另計。",
		cta: "選 Pro 專業",
		icon: "work",
		operatingModel: {
			mode: "project",
			label: "團隊自主營運",
		},
	},
];

// 公開頁一律使用「成交服務費」；契約正文使用「平台交易服務費，以下簡稱成交服務費」。
export const TAX_NOTE =
	"上列方案月費與百分比均為含 5% 營業稅的價格，結帳後不再另加。";

/** Starter 與 Growth 的費用交叉點：990 ÷（6% － 3%）。 */
export const CROSSOVER_STARTER_GROWTH = 33_000;

/** Growth 與 Pro 的費用交叉點：（2,990 － 990）÷ 3%。 */
export const CROSSOVER_GROWTH_PRO = 66_667;

export const CALCULATOR_MAX_AMOUNT = 200_000;

export const CALCULATOR_STEP = 1_000;

export const CALCULATOR_DEFAULT_AMOUNT = 30_000;

export const CALCULATOR_NOTE =
	"試算不含金流、物流、廣告、人工服務與其他另約費用；實際以有效交易、退款及當期方案為準。";

export const CALCULATOR_ROUNDING_NOTE =
	"試算以整月估計金額一次計算；實際帳單的成交服務費是逐筆訂單四捨五入後加總，因此在交叉點附近可能有小額差異。";

export const CALCULATOR_CROSSOVER_NOTE =
	"月商品實際成交金額低於 NT$33,000 時 Starter 最省；約 NT$33,000 至 NT$66,667 之間 Growth 最省；約 NT$66,667 之後 Pro 最省。交叉點附近三方案費用非常接近甚至同價，請以上方試算結果為準；試算只提供建議，平台不會未經你同意變更方案。";

export interface PlanFeeBreakdown {
	plan: SellerPlan;
	/** 該期方案月費（含稅）。 */
	monthlyFee: number;
	/** 依整月估計金額一次算出並四捨五入的成交服務費（試算用）。 */
	transactionFee: number;
	/** 方案月費＋成交服務費；不含金流、物流、廣告與人工服務。 */
	total: number;
}

/**
 * 以「預估的整月商品實際成交金額」算出三方案的平台費用，只涵蓋方案月費與成交服務費。
 *
 * 實際帳單依契約附件一是逐筆訂單四捨五入後加總，這裡拿不到逐筆資料，因此以整月金額
 * 一次四捨五入；兩者在交叉點附近可能有數元差異，公開頁以 CALCULATOR_ROUNDING_NOTE 揭露。
 */
export function calculatePlanFees(amount: number): PlanFeeBreakdown[] {
	const basis = Number.isFinite(amount) ? Math.max(0, amount) : 0;
	return SELLER_PLANS.map((plan) => {
		const transactionFee = Math.round(plan.transactionFeeRate * basis);
		return {
			plan,
			monthlyFee: plan.monthlyFee,
			transactionFee,
			total: plan.monthlyFee + transactionFee,
		};
	});
}

/** 回傳當期費用最低的方案；同價時一併回傳。 */
export function lowestCostPlans(
	breakdowns: PlanFeeBreakdown[],
): PlanFeeBreakdown[] {
	const lowestTotal = Math.min(...breakdowns.map((entry) => entry.total));
	return breakdowns.filter((entry) => entry.total === lowestTotal);
}

export const HERO_TRUST_POINTS = [
	"Starter 月費 0 元",
	"三方案費率公開",
	"金流、物流、人工費分開列示",
	"可先試算，不自動升級",
];

export const COLLABORATION_ROWS: ResponsibilityRow[] = [
	{
		name: "商品資料與食品事實",
		plans: {
			starter: shared("平台提示缺漏，賣家確認"),
			growth: seller("賣家維護，系統提示異常"),
			pro: seller("團隊完整維護"),
		},
	},
	{
		name: "商品上架與頁面內容",
		plans: {
			starter: shared("標準入駐指引與一次上線前缺漏檢查"),
			growth: seller("賣家依標準指引自主完成"),
			pro: seller("團隊自行上架與維護"),
		},
	},
	{
		name: "價格、庫存與促銷",
		plans: {
			starter: seller("賣家決定並維護"),
			growth: seller("賣家決定並維護"),
			pro: seller("團隊決定並維護"),
		},
	},
	{
		name: "訂單、客服、售後與出貨",
		plans: {
			starter: seller("賣家負責"),
			growth: seller("賣家負責"),
			pro: seller("團隊負責"),
		},
	},
	{
		name: "平台交易、結算與對帳",
		plans: {
			starter: platform("平台負責"),
			growth: platform("平台負責"),
			pro: platform("平台負責"),
		},
	},
	{
		name: "行銷與廣告",
		plans: {
			starter: seller("賣家自行執行或加購"),
			growth: seller("賣家自行執行或加購"),
			pro: seller("團隊執行或加購"),
		},
	},
];

export const COLLABORATION_NOTE =
	"方案中的「平台協助」指標準入駐、系統提示與平台問題受理，不含客製化人工製作。需要團隊實際代做時，再選購人工服務 Bundle。";

export const COLLABORATION_CHECK_NOTE =
	"「一次上線前缺漏檢查」只檢查欄位是否完整、格式是否符合平台規則，不是法規驗證、文案撰寫、圖片製作或商品上架代工。";

export type ServiceBundleId = "launch" | "content" | "full";

export interface ServiceBundle {
	id: ServiceBundleId;
	name: string;
	/** 含稅單次售價（新臺幣元）。 */
	price: number;
	icon: AppIconName;
	/** 公開頁一行說明。 */
	tagline: string;
	/** 適合情況。 */
	situation: string;
	/** 賣家要提供什麼。 */
	sellerProvides: string;
	/** 標準包含的工作。 */
	includes: string[];
	/** 標準不包含的項目。 */
	excluded: string;
	/** 只有部分 Bundle 需要的額外前提，例如寄樣與拍攝範圍。 */
	notes?: string[];
	cta: string;
}

export const SERVICE_BUNDLES: ServiceBundle[] = [
	{
		id: "launch",
		name: "開店上線包",
		price: 2_990,
		icon: "work",
		tagline: "帳號你註冊，開店後台我們陪你完成。",
		situation: "還沒完成店舖上線，可以自行註冊但不熟悉後台與上架。",
		sellerProvides:
			"自行完成帳號註冊、身分與商家驗證、契約及收款資料，並提供品牌資料、商品資訊、價格、庫存與可使用的圖片。",
		includes: [
			"後台基本設定與開店資料整理",
			"與你一起調整店舖門面、分類與品牌介紹",
			"整理既有圖文及首波上線內容",
			"補齊基礎行銷標題與商品賣點",
			"後台代行上架、發布前檢查與一次彙整修改",
		],
		excluded: "商品攝影、重新製作素材、實際廣告投放與媒體費。",
		notes: [
			"Hawkr 透過平台正式的後台代行授權操作，不要求也不共用你的密碼；系統保存操作者、時間與變更前後內容。",
		],
		cta: "預約開店上線",
	},
	{
		id: "content",
		name: "成交內容包",
		price: 5_990,
		icon: "document",
		tagline: "你提供素材，我們把商家頁面與產品文案整理好。",
		situation:
			"已有可用照片與完整商品資料，需要把頁面與文案整理成可成交的樣子。",
		sellerProvides:
			"完整品牌資料、商品事實，以及具有使用權的現有素材；聚焦一個店舖、一套商品線與一個主力銷售情境。",
		includes: [
			"整理商家頁面架構與品牌介紹",
			"統整既有圖片的使用順序與頁面配置",
			"商品標題、核心賣點、購買理由、商品說明與行動引導",
			"統一商家頁面與商品頁的語氣及資訊層級",
			"一套可延伸至社群或廣告的核心文案",
			"一次彙整修改與正式交付",
		],
		excluded: "重新拍攝、進階修圖、實際廣告投放與媒體費。",
		notes: [
			"需要你已有可使用的店舖帳號；尚未完成開店設定者可另搭配開店上線包。",
		],
		cta: "提交素材，開始整理",
	},
	{
		id: "full",
		name: "完整行銷包",
		price: 7_990,
		icon: "camera",
		tagline: "寄來樣品，基礎商拍、修圖與產品文案一次完成。",
		situation: "缺少可用照片，需要從商拍開始製作一整套可上線的內容。",
		sellerProvides:
			"品牌方向、完整商品資料，以及在排程前寄達、符合拍攝狀態的實體樣品。",
		includes: [
			"拍攝前確認產品特色、目標客群與單一主打方向",
			"使用既有桌拍設備與固定棚景完成標準化輕商拍",
			"照片挑選、基礎調色、去背、裁切與頁面用精修",
			"商品名稱、核心賣點、產品介紹、使用情境與行動文案",
			"商家頁面與商品頁的圖片順序、資訊層級與基本視覺編排",
			"於 Hawkr 後台完成約定內容上架",
			"一次彙整修改與正式交付",
		],
		excluded:
			"模特兒、外景、棚租、專業食物造型、複雜合成、影片、RAW 原始檔、實際廣告投放與媒體費。",
		notes: [
			"限標準化桌面商拍與一個主打銷售主題；公開頁不以照片張數分級，服務確認單的標準基準是一組 4～6 張精選成品。",
			"商品資料、樣品與拍攝方向全部確認後，約十個工作日交付第一版；含一次彙整修改。",
			"寄送、冷鏈與退回費由你負擔。拍攝時拆封、切割、加熱或擺盤的食品視為耗材，原則上不退回。",
			"會把約定成品上架到既有店舖，但不含帳號註冊與完整開店設定；尚未開店者可另搭配開店上線包。",
		],
		cta: "預約完整行銷製作",
	},
];

export const SERVICE_BUNDLE_INTRO =
	"你提供真實的品牌與商品資料，我們協助完成後台、頁面與行銷內容。人工服務採單次專案計價，與方案月費及市集成交服務費分開計算。";

/** §13.6 三題選擇器：依賣家已經準備到哪一步，指向對應的 Bundle。 */
export const BUNDLE_CHOOSER: Array<{
	situation: string;
	bundleId: ServiceBundleId;
}> = [
	{
		situation: "可以自行註冊，但不熟悉後台與上架",
		bundleId: "launch",
	},
	{
		situation: "已有可用照片與完整商品資料",
		bundleId: "content",
	},
	{
		situation: "缺少可用照片，需要從商拍開始",
		bundleId: "full",
	},
];

export const SERVICE_BOUNDARY_NOTE =
	"平台方案與人工服務分開算、分開選。本次加購不會改變你目前的平台方案，也不會改變 6%／3%／0% 的成交服務費。完整行銷包只包含標準化輕商拍；影片、模特兒、外景、廣告媒體費、實際廣告投放、客服、出貨與法規認證不包含。";

export const SERVICE_PRINCIPLE = "代做但不代決";

export const SERVICE_PLATFORM_SCOPE = [
	"整理你提供的資料、內容與素材",
	"依已核准內容設定店舖、更新商品資料、頁面與上架文案",
	"僅在完整行銷包的確認範圍內拍攝與修飾產品照片",
	"提醒缺漏、異常或待決事項",
	"提供交付成果與發布前檢查",
];

export const SERVICE_SELLER_SCOPE = [
	"食品事實、合法性與公開宣稱",
	"售價、庫存、促銷、廣告預算與發布核准",
	"接單、備貨、包裝、出貨與供應",
	"商品客服、退款、召回、食安事件與消費爭議",
	"商標、圖片、帳戶及素材的合法權利",
];

export interface PricingFaq {
	question: string;
	answer: string;
}

export const SELLER_PRICING_FAQS: PricingFaq[] = [
	{
		question: "你們會分辨客人是我帶來的還是平台帶來的嗎？",
		answer:
			"不會。只要在 Hawkr 市集完成有效交易，就依訂單適用方案計費。買家從哪裡進入、由誰分享連結、是否看過廣告，都不影響費率。",
	},
	{
		question: "Growth 為什麼既收月費又收 3%？",
		answer:
			"NT$990 是降低成交服務費的方案月費，把 6% 換成 3%。頁面的試算器會直接顯示兩者合計，你可以先確認自己的成交規模適不適合。",
	},
	{
		question: "Pro 的 0% 是不是所有費用都免了？",
		answer:
			"不是。0% 只指成交服務費；方案月費 NT$2,990、金流手續費、物流費、廣告費與人工服務費仍然另計。",
	},
	{
		question: "當月沒有成交還要付費嗎？",
		answer:
			"Starter 不需要，沒有有效交易就沒有任何平台費用。Growth 與 Pro 仍收該期的方案月費。",
	},
	{
		question: "退款後成交服務費怎麼處理？",
		answer:
			"依原訂單費率與退款後保留的商品實際成交金額重新計算，多收的部分回沖。全額退款時該筆成交服務費歸零。",
	},
	{
		question: "可以隨時換方案嗎？",
		answer:
			"可以預約下一個計費週期生效；升級另可選擇立即生效並補收當期月費差額。方案變更只影響生效後鎖定的新訂單，不回溯已成立的舊訂單。",
	},
	{
		question: "人工上架包含在方案內嗎？",
		answer:
			"平台方案不包含客製化人工上架。你自行完成帳號註冊與驗證後，可加購開店上線包，由團隊透過正式後台權限協作設定、美化與上架。",
	},
	{
		question: "平台會保證流量或成交嗎？",
		answer:
			"不會。平台提供食品市集與交易基礎，不保證曝光、排名、轉換率或業績。",
	},
	{
		question: "食品標示由誰負責？",
		answer:
			"由賣家提供並確認商品事實與合法性。Hawkr 可以整理、改寫並指出缺漏與高風險用語，但不替賣家證明內容真實、完成法規審查或承擔食品責任。",
	},
	{
		question: "付了平台月費或成交服務費，為什麼人工服務還要加購？",
		answer:
			"兩筆費用購買不同服務。平台費支付市集、交易、結算與系統；人工服務是請團隊實際設定後台、整理頁面、撰寫或拍攝。你自行處理，就不會產生人工服務費。",
	},
	{
		question: "完整行銷包的「完整」包含所有行銷工作嗎？",
		answer:
			"不是。「完整」是指完成一套可上線的基礎商拍、照片精修、商品頁與產品文案；不包含長期代營運、實際廣告投放、媒體費、影片或大型商業製作。",
	},
	{
		question: "完整行銷包怎麼寄樣？樣品會退回嗎？",
		answer:
			"開拍前會確認到貨日期、溫層與樣品處理方式。寄送、冷鏈及退回費由賣家負擔；食品可能因拆封、切割、加熱或擺盤而耗用，原則上不退回。",
	},
	{
		question: "完整行銷包也會處理開店後台嗎？",
		answer:
			"完整行銷包會把約定成品上架到既有店舖，但不包含帳號註冊與完整開店設定；尚未完成開店者可另搭配開店上線包。",
	},
];

export const SOURCE_NEUTRALITY_NOTE =
	"買家只要在 Hawkr 市集內完成一筆符合有效交易定義的訂單，就依該筆訂單鎖定的賣家方案計算成交服務費。";
