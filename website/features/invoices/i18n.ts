/**
 * 買家面發票畫面的逐字文案（58A 各畫面的「文案」表）。
 *
 * 集中一份的理由不是 i18n（本 repo 全站零 `useTranslations()`），而是同一句話
 * 出現在多處：空狀態與「不使用載具」副標講的是同一件事（發票會保留在帳戶），
 * 兩處抄兩份就會在改文案時漂移。**碼表的中文不進本檔**——類型與狀態的中文
 * 一律走 `lib/codemeta/generated.ts`（§0.4 ⑤）。
 */

export const INVOICE_PROFILE_TEXT = {
	pageTitle: "常用發票資料",
	pageSubtitle: "結帳時可直接選用；沒有設定也可以，發票會保留在你的帳戶。",
	addCta: "新增發票資料",
	setDefaultCta: "設為預設",
	settingDefault: "設定中…",
	editCta: "編輯",
	deleteCta: "刪除",
	defaultChip: "預設",
	loading: "載入中…",
	emptyHint:
		"還沒有常用發票資料。結帳時不選載具也可以，發票會保留在你的帳戶，可在「發票與收據」查看與下載。",
	listError: "發票資料載入失敗，請稍後再試。",
	retry: "重試",
	labelField: "備註（選填）",
	labelPlaceholder: "例如：公司報帳、老公的手機條碼",
	setDefaultCheckbox: "設為預設",
	typeField: "發票方式",
	cancel: "取消",
	save: "儲存",
	saving: "儲存中…",
	editValueDescription: "基於安全考量，系統不會顯示完整內容，請重新輸入。",
	deleteTitle: "刪除這筆發票資料？",
	deleteBodyPlain: "刪除後結帳時就不會出現這筆資料。",
	deleteBodyDefault: "這是你的預設資料，請先指定新的預設。",
	deleteBodyLastDefault:
		"刪除後結帳將不使用載具，發票會保留在你的帳戶。要繼續刪除嗎？",
	deleteNewDefaultLabel: "請指定新的預設資料",
	deleteNewDefaultHint: "刪除預設資料前，必須先選一筆接手。",
	deleteConfirm: "刪除",
	deleting: "刪除中…",
	retryKeepsRequest: "重試會沿用同一次請求，不會重複執行。",
} as const;

/** 端點失敗時給使用者看的話（58A §B-1.0.6 的碼 → 文案表）。 */
export const INVOICE_PROFILE_ERROR_TEXT = {
	notFound: "這筆資料已不存在，請重新整理。",
	limitExceeded: "常用發票資料最多 20 筆，請先刪除不用的資料。",
	defaultRequired: "請先指定新的預設資料。",
	targetConflict: "統一編號、載具與捐贈碼只能擇一，請重新選擇。",
	idempotencyMismatch: "操作失敗，請重新整理後再試。",
	generic: "操作失敗，請稍後再試。",
} as const;

/** B02／B08 共用的四種載具欄位格式錯誤（58A §B02 文案表逐字）。 */
export const INVOICE_CARRIER_FORMAT_TEXT = {
	mobileBarcode: "手機條碼需為斜線開頭的 8 碼，例如 /AB12+34。",
	citizenCert: "自然人憑證條碼需為 2 個英文字母加 14 位數字。",
	taxId: "統一編號需為 8 位數字。",
	donation: "捐贈碼需為 3 到 7 位數字。",
	buyerName: "請輸入公司抬頭（1–60 字）。",
} as const;

/** 四種載具的欄位 label / placeholder / 說明（58A §B02 欄位表與文案表）。 */
export const INVOICE_CARRIER_FIELD_TEXT = {
	mobileBarcode: {
		label: "手機條碼",
		placeholder: "/ABC+123",
		description: "需為財政部核發的 8 碼手機條碼。",
	},
	citizenCert: {
		label: "自然人憑證條碼",
		placeholder: "AB12345678901234",
		description: "",
	},
	taxId: {
		label: "統一編號",
		placeholder: "請輸入 8 位統一編號",
		description: "",
	},
	buyerName: {
		label: "公司抬頭",
		placeholder: "請輸入公司抬頭",
		description: "",
	},
	donation: {
		label: "捐贈碼",
		placeholder: "請輸入 3 到 7 位捐贈碼",
		description: "",
	},
} as const;

/** B04｜訂單詳情—發票與收據區（58A §B04 文案表逐字）。 */
export const INVOICE_ORDER_TEXT = {
	cardTitle: "發票與收據",
	loading: "載入中…",
	listError: "發票資料載入失敗，請稍後再試。",
	retry: "重試",
	refresh: "重新整理",
	viewDocument: "查看發票",
	downloadPdf: "下載 PDF",
	copyInvoiceNo: "複製發票號碼",
	copied: "已複製",
	fieldInvoiceNo: "發票號碼",
	fieldIssuedAt: "開立日期",
	fieldAmount: "發票金額",
	fieldCarrier: "載具",
	// backend#3084：carrierSummary 沒有載具時裝的是買受人統編，卡片要能改標籤。
	fieldRecipientTaxId: "買受人統編",
	fieldNotifiedEmail: "通知信箱",
	noCarrier: "無載具",
	/**
	 * 後端沒給 `shippingFeeNotice` 時的同一句話。有給就逐字用後端的——這句是
	 * 稅務說明，兩邊漂移會讓買家在兩個畫面讀到不同的說法。
	 */
	shippingFeeNotice:
		"物流費用已包含在本張發票的銷售額中，另列的物流明細不是另一張發票。",
	/**
	 * 三聯式限定的說明：**明細區與金額區的數字加起來對不上，那是正常的**。
	 *
	 * 台灣定價含稅，明細的單價是含稅價（後端一律 `DetailVat=1`），而三聯式的表頭
	 * 依法要把銷售額與營業稅分列 ⇒ 明細合計＝總計（含稅），銷售額卻是扣掉稅之後的數字。
	 * 例：明細 990 ＋ 65 ＝ 1055，而銷售額寫 1005、稅額 50。
	 *
	 * 🔴 少了這句話，讀的人最自然的解讀是「稅是另外加的」，於是以為要付 1055 ＋ 50
	 * ——而實際上總計 1055 就是他付的錢。二聯式與收據沒有這個問題（不分列稅額），
	 * 所以這句話只在 B2B 顯示。
	 */
	taxInclusiveNotice:
		"商品明細為含稅單價，上方的銷售額則是扣除營業稅後的金額——兩者加總方式不同是正常的。您實際支付的金額為「總計」。",
	/**
	 * 刻意是中性的「關聯文件」而不是「這張發票取代已作廢的 X」：端點 8 的
	 * `relatedDocumentId` 把兩個方向（我作廢了誰／誰取代了我）折成同一個欄位
	 * （invoice-service 的 `firstNonEmptyID(OriginalDocumentID, SupersededByID)`），
	 * 前端分不出來。寫死其中一個方向會有一半的情況說反。
	 */
	relatedWithNumber: (invoiceNo: string) => `關聯文件：${invoiceNo}`,
	relatedDocument: "查看關聯文件",
} as const;

/** B09｜買家發票中心列表（58A §B09 文案表逐字）。 */
export const INVOICE_LIST_TEXT = {
	pageTitle: "發票與收據",
	eyebrow: "INVOICE",
	subtitle: (total: number) =>
		`共 ${total} 筆發票與收據。點任一筆可查看完整內容。`,
	loading: "載入中…",
	tableCaption: "發票與收據列表",
	colIssuedAt: "開立日期",
	colInvoiceNo: "發票號碼",
	/**
	 * 🔴 label 是「開立人」不是「賣家」：方向三（平台→買家）的開立人是平台，寫
	 * 「賣家」會在收據情境下說謊。這一欄的存在本身是 58A §B09 的硬需求。
	 */
	colIssuer: "開立人",
	colKind: "類型",
	colAmount: "金額",
	colStatus: "狀態",
	viewDetail: "查看詳情",
	emptyTitle: "還沒有任何發票或收據",
	emptyBody: "完成訂單後，發票會自動開立並顯示在這裡。",
	filteredEmptyTitle: "沒有符合條件的發票或收據",
	filteredEmptyBody: "試著調整日期或類型條件。",
	clearFilters: "清除篩選",
	errorBody: "發票紀錄載入失敗，請稍後再試。",
	retry: "重試",
	filterKindLabel: "文件類型",
	/**
	 * 狀態 pill 只開得出**買家投影與後端原值 1:1 的那幾個**。要不要多開一顆，照
	 * 這兩問判：
	 *
	 *   1. 這個買家標籤恰好對應一個 `InvoiceDocumentStatus` 嗎？依據是
	 *      `lib/invoices/documentStatus.ts` 的折疊規則，不是直覺：「處理中」＝五值
	 *      合一、「已作廢」＝ `VOIDED` 與 `SUPERSEDED` 合一，**其餘四值都是 1:1**
	 *      （`ISSUED`／`CANCELLED_BEFORE_ISSUE`／`NOT_APPLICABLE`／
	 *      `RESOLVED_EXTERNALLY`）。1:1 就能誠實提供，直接開。
	 *   2. 是 fold（多對一）的那兩顆，要看端點 68 的 `status` 收不收得下集合。它
	 *      目前只收單值（proto `string status`、SQL 等號、Gateway 只取第一個），
	 *      送 CSV 是**靜默失敗**——不報錯，但篩出來的集合不是標籤講的那一集。等
	 *      後端改收集合後補上這兩顆，前端這一側不需要別的改動。
	 *
	 * ⚠️ 兩顆做不了不是把四顆一起砍的理由：能誠實提供的條件砍掉是純損失。fold
	 * 只讓「該標籤對應多個值」的那幾顆做不了，不會傳染給 1:1 的那幾顆。
	 */
	filterStatusLabel: "文件狀態",
	filterAll: "全部",
	dateFrom: "起始日期",
	dateTo: "結束日期",
	datePlaceholder: "YYYY-MM-DD",
	dateInvalid: "請輸入 YYYY-MM-DD 格式的日期",
} as const;

/** B05｜發票／收據文件詳情（58A §B05 文案表逐字）。 */
export const INVOICE_DOCUMENT_TEXT = {
	pageTitle: "發票與收據",
	eyebrow: "INVOICE",
	back: "← 回訂單詳情",
	backToOrders: "← 回訂單列表",
	loading: "載入中…",
	notFoundTitle: "找不到這份文件",
	/**
	 * 🔴 **絕不寫「你沒有權限查看」**：端點 69 對越權與不存在回同一個 9301
	 * （Tier A），前端文案跟著一致才有意義——分得出來就等於把那個旗標補回去。
	 */
	notFoundBody: "這份文件可能已被移除，或連結不正確。",
	voidedTitle: "這張發票已作廢",
	voidedBody: "作廢後平台已重新開立新的發票，你可以在下方的文件歷程查看。",
	cardDocument: "文件資訊",
	cardAmount: "金額明細",
	cardTimeline: "文件歷程",
	fieldKind: "文件類型",
	fieldInvoiceNo: "發票號碼",
	fieldIssuedAt: "開立日期",
	fieldIssuer: "開立人",
	fieldIssuerTaxId: "開立人統編",
	/** `PLATFORM_RECEIPT` 時開立人統編那一格放的是免用統一發票標示，不是統編。 */
	fieldIssuerTaxStatus: "稅籍狀態",
	fieldRecipient: "買受人名稱",
	fieldRecipientTaxId: "買受人統編",
	fieldCarrier: "載具",
	/**
	 * 銷售額那一列的標籤**依 documentKind 切換**，不能寫死。
	 *
	 * 🔴 台灣定價含稅、後端一律 `DetailVat=1`（含稅明細），而表頭只有 B2B 才拆稅：
	 *   B2B          `salesAmount` = 已扣除稅額的**未稅**銷售額
	 *   B2C／RECEIPT `salesAmount` = `totalAmount`，是**含稅**值（`taxAmount` 恆為 0，
	 *                二聯式與收據依法不分列稅額）
	 * 寫死「未稅銷售額」的話，B2C 與收據的買家會看到一個含稅價被標成未稅
	 * ——數字自洽（總計不會變多），但那句話是假的，而且會讓人以為還要再加 5%。
	 */
	fieldSalesExTax: "未稅銷售額",
	fieldSalesInclTax: "銷售額（含稅）",
	fieldTax: "稅額",
	fieldTotal: "總計",
	zeroRatedNote: "本文件適用零稅率。",
	taxExemptNote: "本文件為免稅。",
	lineSubtotal: "小計",
	downloadPdf: "下載 PDF",
	relatedDocument: "查看關聯文件",
	emptyTimeline: "目前沒有歷程紀錄。",
	/* 端點 71｜重寄通知 */
	cardResend: "沒收到通知？",
	resendBody: "可以請系統再寄一次到你的原始通知信箱。先確認一下垃圾郵件匣。",
	resendAction: "重寄通知",
	resendSending: "寄送中…",
	/** `toMasked` 為空（載具發票／收據沒有寄件對象）時退回這一句，不編一個信箱出來。 */
	resendSentFallback: "已重新寄出。",
	/** 🔴 9352 不是失敗：說「剛剛才寄過」，不要說「寄送失敗」。 */
	resendThrottled: "剛剛才寄過，10 分鐘內不會重複寄送。請稍後再試。",
	resendFailed: "寄送失敗，請稍後再試。",
} as const;

/**
 * B03｜付款完成頁的三列（58A §B03 文案表逐字）。
 *
 * 只有 label 在這裡：狀態句依 `invoiceMode` 分支，屬於折疊表而不是常數，放
 * `lib/invoices/checkoutInvoiceNotice.ts`（與 `orderInvoiceState.ts` 同一個慣例）。
 */
export const INVOICE_CHECKOUT_TEXT = {
	fieldState: "發票處理狀態",
	fieldCarrier: "發票方式",
	fieldNotifiedEmail: "通知信箱",
	/** 查詢中：三列的值都是這個字，不用 skeleton（資訊卡列高固定，不會跳動）。 */
	pendingValue: "—",
} as const;

/**
 * B07｜未收到發票／收據申訴（58A §B07 文案表逐字）。
 *
 * 🔴 **狀態的標題與內文不在這裡**：它們是折疊表（`lib/invoices/complaintStatus.ts`）
 * 的一部分，與 `orderInvoiceState.ts` 同一個慣例。歷程的 `action` 中文一律走
 * CodeMeta `InvoiceComplaintAction`（§0.4 ⑤），**不得手抄那六句**。
 */
export const INVOICE_COMPLAINT_TEXT = {
	/* 入口（B04 動作列） */
	entryCta: "沒有收到？提出申訴",
	/** `canFileComplaint === false` 且有 `complaintOpensAt` 時的灰字。 */
	notOpenYet: (opensAt: string) => `${opensAt} 之後可提出申訴`,

	/* 建案 Dialog（端點 13） */
	createTitle: "提出申訴",
	/**
	 * **不照抄既有售後的「每筆售後僅能申訴一次…」**——發票申訴結案後買家可重新
	 * 提出，上限 2 次，照抄會說謊。
	 */
	createDescription: "我們會通知賣家處理，並在有進度時通知你。",
	fieldOrderNo: "訂單編號",
	fieldSeller: "賣家",
	fieldExpectedDoc: "應收到的文件",
	fieldReason: "申訴原因",
	reasonNotReceived: "未收到發票或收據",
	descriptionLabel: "補充說明（選填）",
	descriptionPlaceholder:
		"例如：已超過三天仍未收到 Email，也在垃圾信件匣找過了。",
	slaHint: "賣家需於 3 個工作天內處理",
	submitCta: "送出申訴",
	submitting: "送出申訴中",
	cancel: "取消",

	/* 案件區塊 */
	sectionTitle: "發票申訴",
	caseNoLabel: (caseNo: string) => `案件編號 ${caseNo}`,
	copyCaseNo: "複製案件編號",
	copied: "已複製",
	reopenedChip: "已重啟",
	historyTitle: "處理歷程",
	createdAction: "已提出申訴",
	actorSeller: "賣家",
	actorAdmin: "平台",
	loadError: "申訴案件載入失敗，請稍後再試。",
	retry: "重試",
	/** `SELLER_RESPONDED` 橫幅下的小字；`buyerReviewDueAt` 為空時整句不渲染。 */
	buyerReviewDue: (deadline: string) =>
		`${deadline} 前可提出異議，逾期視為已解決`,

	/* 端點 63 */
	rejectCta: "這樣沒有解決",
	rejectTitle: "這樣沒有解決",
	rejectDescription:
		"我們會把案件轉給平台客服處理。請說明你還缺什麼，這會直接給客服看。",
	rejectFieldLabel: "請說明還缺什麼",
	rejectToast: "已轉由平台處理",

	/* 端點 64 */
	withdrawCta: "撤回申訴",
	withdrawTitle: "要撤回這件申訴嗎？",
	/**
	 * 後半句是事實不是安慰：`WITHDRAWN` 是終態、不在「每份文件只能有一件開啟中
	 * 申訴」的述詞內 ⇒ 槽位釋出。不寫這句，撤回鍵沒人敢按。
	 */
	withdrawDescription:
		"撤回後案件會關閉。如果之後仍未收到文件，你可以再次提出申訴。",
	withdrawConfirm: "撤回申訴",
	withdrawToast: "申訴已撤回",

	/* 端點 65 */
	reopenCta: "重新提出",
	reopenTitle: "重新提出這件申訴",
	reopenDescription: (remaining: number) =>
		remaining >= 2
			? "賣家會再收到一次通知，處理期限重新計算 3 個工作天。每件申訴最多可重新提出 2 次。"
			: "賣家會再收到一次通知，處理期限重新計算 3 個工作天。你還可以再重新提出 1 次。",
	reopenFieldLabel: "補充說明（選填）",
	reopenConfirm: "重新提出",
	reopenToast: "已重新提出申訴",
	/** 冷卻中：按鈕 disabled ＋ 這行小字（**不隱藏按鈕**）。 */
	reopenCooldown: (allowedAfter: string) => `${allowedAfter} 之後可重新提出`,
	reopenExhausted: "已重新提出 2 次，如仍未解決請聯絡客服",

	/* 送出結果 */
	createdToastTitle: "申訴已送出",
	createdToastBody: (caseNo: string) =>
		`案件編號 ${caseNo}，我們會通知賣家處理。`,
	/** 每個 mutation 按鈕旁的固定一句（§0.4 ②）。 */
	retryKeepsRequest: "重試會沿用同一次請求，不會重複執行。",
} as const;

/** B07 的碼 → 文案（58A §B-1.0.6）。 */
export const INVOICE_COMPLAINT_ERROR_TEXT = {
	/** 9309：Tier A，**不得寫「你沒有權限」**。 */
	notFound: "找不到這件申訴，請重新整理後再試一次。",
	/** 9311：入口靠 `canFileComplaint` 隱藏，這句只在競態時出現。 */
	notOpenYet: "這筆訂單還不能提出申訴，請稍後再試。",
	/**
	 * 9322：碼名與後端 message 都是稅務側的（「稅務資格資料已被更新」）⇒
	 * **不得直接顯示後端 message**，用申訴語境的自有文案。
	 */
	versionConflict: "這件申訴剛剛有新的進度，請重新整理後再試一次。",
	idempotencyMismatch: "操作失敗，請重新整理後再試。",
	generic: "操作失敗，請稍後再試。",
} as const;
