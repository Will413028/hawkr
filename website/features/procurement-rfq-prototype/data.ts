import type {
	ExternalVendorAccessStep,
	ExternalVendorBuyerOffer,
	ExternalVendorStep,
	ExternalVendorStepDefinition,
	ExternalVendorTransactionStep,
	RfqPrototypeQuote,
	RfqPrototypeScreenDefinition,
	RfqPrototypeScreenSlug,
	RfqPrototypeSupplier,
} from "./types";

export const EXTERNAL_VENDOR_ACCESS_STEPS: ReadonlyArray<
	ExternalVendorStepDefinition & { slug: ExternalVendorAccessStep }
> = [
	{
		slug: "invitation",
		label: "查看邀請",
		shortLabel: "邀請",
		description: "確認邀請來源、有效期限與本次詢價範圍。",
	},
	{
		slug: "exchange",
		label: "確認邀請連結",
		shortLabel: "確認連結",
		description: "以一次性憑證換取受限的安全工作階段。",
	},
	{
		slug: "sign-in",
		label: "驗證受邀信箱",
		shortLabel: "登入",
		description: "使用受邀 Google 帳號或 Email 驗證碼繼續。",
	},
	{
		slug: "verify-code",
		label: "輸入驗證碼",
		shortLabel: "驗證碼",
		description: "輸入寄到受邀信箱的 6 位數一次性驗證碼。",
	},
	{
		slug: "session-expired",
		label: "工作階段已過期",
		shortLabel: "重新登入",
		description: "工作階段逾時後重新驗證，不必尋找原始邀請連結。",
	},
	{
		slug: "invite-unavailable",
		label: "邀請無法使用",
		shortLabel: "連結狀態",
		description: "處理邀請已使用、過期或撤回的恢復路徑。",
	},
	{
		slug: "portal",
		label: "供應商工作台",
		shortLabel: "工作台",
		description: "從固定入口查看詢價、報價、履約與結算。",
	},
];

export const EXTERNAL_VENDOR_TRANSACTION_STEPS: ReadonlyArray<
	ExternalVendorStepDefinition & { slug: ExternalVendorTransactionStep }
> = [
	{
		slug: "business-verification",
		label: "企業驗證",
		shortLabel: "驗證",
		description: "提供公司、負責人與收款資格資料。",
	},
	{
		slug: "rfq",
		label: "確認詢價單",
		shortLabel: "詢價",
		description: "閱讀買家已確認的品項、交貨條件與採購範圍。",
	},
	{
		slug: "quote",
		label: "確認報價條件",
		shortLabel: "條件",
		description: "查看買家以 Pricebook 試算並確認的價格與交易條件。",
	},
	{
		slug: "review",
		label: "回覆詢價單",
		shortLabel: "回覆",
		description: "接受買家條件，或選擇提出異議與婉拒。",
	},
	{
		slug: "transaction",
		label: "交易追蹤",
		shortLabel: "追蹤",
		description: "查看議價、成交、履約、發票與結算狀態。",
	},
];

export const EXTERNAL_VENDOR_STEPS: ReadonlyArray<ExternalVendorStepDefinition> =
	[...EXTERNAL_VENDOR_ACCESS_STEPS, ...EXTERNAL_VENDOR_TRANSACTION_STEPS];

export const EXTERNAL_VENDOR_STEP_SLUGS = EXTERNAL_VENDOR_STEPS.map(
	(step) => step.slug,
);

export function isExternalVendorStep(
	value: string,
): value is ExternalVendorStep {
	return EXTERNAL_VENDOR_STEP_SLUGS.includes(value as ExternalVendorStep);
}

export function externalVendorStepPath(step: ExternalVendorStep): string {
	return step === "invitation"
		? "/hawkr-agent-rfq-preview/external-vendor"
		: `/hawkr-agent-rfq-preview/external-vendor/${step}`;
}

export const RFQ_PROTOTYPE_SCREENS: ReadonlyArray<RfqPrototypeScreenDefinition> =
	[
		{
			slug: "candidate-handoff",
			label: "候選交接",
			title: "把已選候選轉成詢價名單",
			description: "先確認收件對象，再建立版本化 RFQ；候選不代表供貨承諾。",
			role: "buyer",
			wave: "wave-1",
		},
		{
			slug: "rfq-compose",
			label: "設定詢價",
			title: "確認價格與品質，再送出正式詢價",
			description:
				"把需求、Pricebook 價格區間、理想價、品質期待與收件名單固定成版本。",
			role: "buyer",
			wave: "wave-1",
		},
		{
			slug: "seller-inbox",
			label: "賣家收件匣",
			title: "處理買家已定稿的採購條件",
			description:
				"賣家先看理想價、可接受區間與品質期待，再決定接受、提出異議或婉拒。",
			role: "seller",
			wave: "wave-1",
		},
		{
			slug: "seller-response",
			label: "供應商回覆",
			title: "接受條件，或提出可追溯的異議",
			description:
				"買家詢價單保持唯讀；供應商回覆以獨立版本紀錄，不覆寫原始條件。",
			role: "seller",
			wave: "wave-1",
		},
		{
			slug: "quote-compare",
			label: "比較回覆",
			title: "逐項比較供應商接受與異議",
			description:
				"保留接受狀態、調整價格與例外條件，由買家自行加入 shortlist。",
			role: "buyer",
			wave: "wave-1",
			targetId: "B-02",
		},
		{
			slug: "checkout",
			label: "RFQ 結帳",
			title: "確認唯一一筆待付款報價",
			description: "桌機與手機共用同一契約；付款成功才建立正式訂單。",
			role: "buyer",
			wave: "later",
			targetId: "B-04/B-05",
		},
		{
			slug: "atm-payment",
			label: "ATM 鎖定",
			title: "取得虛擬帳號並鎖定付款期限",
			description: "顯示剩餘時間、晚到款退款與訂單尚未成立的明確界線。",
			role: "buyer",
			wave: "later",
			targetId: "B-06",
		},
		{
			slug: "external-vendor",
			label: "外部 Portal",
			title: "完成資格驗證後才能進入交易",
			description:
				"外部業者可查看買家定稿條件並接受、提出異議或婉拒；未完成 KYC 前不可進入收款。",
			role: "vendor",
			wave: "later",
			targetId: "E-03",
		},
		{
			slug: "seller-order",
			label: "履約結算",
			title: "由賣家管理出貨、發票與可結算金額",
			description: "Agent 與平台管理員不取得賣家出貨能力。",
			role: "seller",
			wave: "later",
			targetId: "S-01",
		},
		{
			slug: "aftersales",
			label: "售後凍結",
			title: "回報問題並保留款項處理軌跡",
			description: "買家上傳證據、查看 freeze 狀態與後續處理節點。",
			role: "buyer",
			wave: "later",
			targetId: "B-08",
		},
		{
			slug: "finance",
			label: "財務雙審",
			title: "初審與覆審分離的匯款佇列",
			description: "每個決策保留操作者、時間、原因與不可覆寫的稽核紀錄。",
			role: "finance",
			wave: "later",
			targetId: "F-01",
		},
	];

export const RFQ_PROTOTYPE_SCREEN_SLUGS = RFQ_PROTOTYPE_SCREENS.map(
	(screen) => screen.slug,
);

export function isRfqPrototypeScreenSlug(
	value: string,
): value is RfqPrototypeScreenSlug {
	return RFQ_PROTOTYPE_SCREEN_SLUGS.includes(value as RfqPrototypeScreenSlug);
}

export function rfqPrototypeScreenPath(slug: RfqPrototypeScreenSlug): string {
	return `/hawkr-agent-rfq-preview/${slug}`;
}

export const RFQ_PROTOTYPE_SUPPLIERS: ReadonlyArray<RfqPrototypeSupplier> = [
	{
		id: "supplier-mountain",
		name: "山腳蔬果共同運銷社",
		lane: "平台賣家",
		selected: true,
		matchScore: 93,
		status: "資格與冷藏配送已驗證",
		facts: ["可供 80 份", "週六上午配送", "可開立電子發票"],
	},
	{
		id: "supplier-green",
		name: "青禾食材行",
		lane: "平台賣家",
		selected: true,
		matchScore: 88,
		status: "供貨量待正式報價確認",
		facts: ["北部配送", "葉菜與根莖組合", "含分裝服務"],
	},
	{
		id: "supplier-field",
		name: "豐田產地直送",
		lane: "外部業者",
		selected: false,
		matchScore: 81,
		status: "公開資料已比對，平台資格待確認",
		facts: ["產地直送", "可提供履歷蔬菜", "尚未完成平台 KYC"],
	},
];

export const RFQ_PROTOTYPE_QUOTES: ReadonlyArray<RfqPrototypeQuote> = [
	{
		id: "quote-mountain-v1",
		supplier: "山腳蔬果共同運銷社",
		total: 22000,
		unitPrice: 265,
		availableQuantity: 80,
		leadTime: "8/15（六）09:00 前",
		freight: "已含冷藏運費",
		tax: "含稅",
		validUntil: "8/12（三）18:00",
		submittedAt: "8/10（一）14:32",
		version: "v1",
		notes: ["接受買家理想成交價", "80 份一次交貨", "價格、品質與交期皆可履行"],
		shortlisted: true,
	},
	{
		id: "quote-green-v1",
		supplier: "青禾食材行",
		total: 22800,
		unitPrice: 275,
		availableQuantity: 80,
		leadTime: "8/15（六）11:00 前",
		freight: "含冷藏運費 800 元",
		tax: "含稅",
		validUntil: "8/11（二）17:00",
		submittedAt: "8/10（一）16:08",
		version: "v1",
		notes: [
			"提出單價調整至 275 元",
			"仍在買家接受上限內",
			"品質與交期可完全符合",
		],
		shortlisted: false,
	},
	{
		id: "quote-field-v2",
		supplier: "豐田產地直送",
		total: 21320,
		unitPrice: 285,
		availableQuantity: 72,
		leadTime: "8/15（六）08:30 前",
		freight: "已含運",
		tax: "含稅",
		validUntil: "8/12（三）12:00",
		submittedAt: "8/10（一）17:20",
		version: "v2",
		notes: [
			"超出買家單價上限",
			"最多承接 72 份",
			"外部業者資格待確認，此回覆尚不可接受",
		],
		shortlisted: false,
	},
];

export const RFQ_DEMO_SUMMARY = {
	rfqNo: "RFQ-2026-0810-014",
	title: "夏季員工午餐｜小農鮮蔬食材組",
	quantity: "80 份",
	budget: "總預算 24,000 元內",
	delivery: "2026/08/15 09:00 前｜台北市內湖區",
	deadline: "2026/08/12 18:00",
	requestVersion: "需求 v4",
} as const;

export const EXTERNAL_VENDOR_BUYER_OFFER: ExternalVendorBuyerOffer = {
	id: "buyer-offer-rfq-2026-0810-014-v4",
	status: "buyer-confirmed",
	version: "v4",
	buyerConfirmedAt: "2026/08/10 15:20",
	pricebookAsOf: "2026/08/10",
	pricebookVersion: "tw-produce-2026.08-v1",
	pricingRuleVersion: "retail-x2-v1",
	pricebookFloor: 240,
	pricebookCeiling: 290,
	buyerMinimumUnitPrice: 250,
	buyerMaximumUnitPrice: 280,
	quantity: 80,
	unit: "箱",
	transactionUnitPrice: 265,
	qualityTarget: "均衡品質",
	retailMultiplier: 2,
	retailUnitPrice: 530,
	freight: 800,
	subtotal: 21200,
	total: 22000,
	tax: "含稅",
	deliveryAt: "2026/08/15 09:30 前",
	validUntil: "2026/08/12 18:00",
};
