/**
 * 買家面電子發票的 wire 型別鏡像（58 號《電子發票服務》§6.1.1／§6.1.3）。
 *
 * 兩條規約決定了本檔的型別選擇：
 *
 *   1. **從 wire 讀進來的 enum 欄位一律是 `string`，不是 CodeMeta 的 union。**
 *      後端可以先上線一個前端還不知道的值，收窄成 union 會讓 display map 的
 *      default 分支在 TypeScript 眼中變成不可達，於是那個分支寫不出來、也測
 *      不了（§0.6 C11）。往 wire 寫出去的欄位才用 union——那是使用者從固定
 *      的四個選項裡挑的。
 *   2. **id 與金額一律字串。** `profileId` 是 64-bit TSID，`Number()` 會掉精度。
 */

import type { InvoiceCarrierTypeValue } from "@/lib/codemeta/generated";

/**
 * 端點 1–5 的 `InvoiceProfileInfo`（gateway `invoice_dto.go` 的
 * `InvoiceProfileInfo`）。
 *
 * `displayValue` 是後端決定的**顯示值**，遮不遮罩由型別決定（invoice-service
 * `preferenceDisplayValue`）：`MOBILE_BARCODE`／`CITIZEN_CERT` 給的是落庫的
 * `carrier_value_mask`（例 `/T••••+P`），`TAX_ID` 給統編、`DONATION` 給捐贈碼，
 * 後兩者本來就不是秘密。前端不得自己遮、也不得自己解（§0.4 ③）——真正的載具
 * 明碼是密文 ＋ HMAC hash，連 Gateway 都拿不到。
 */
export interface InvoiceProfile {
	profileId: string;
	/** `InvoiceCarrierType` 的值，但型別是 string——理由見檔頭第 1 點。 */
	carrierType: string;
	displayValue: string;
	buyerTaxId: string;
	buyerName: string;
	isDefault: boolean;
	label: string;
	createdAt: string;
	updatedAt: string;
}

/** 端點 1 的 `response.Page`。分頁欄位在 wire 上是數字字串。 */
export interface InvoiceProfileListResult {
	records: InvoiceProfile[];
	total: number;
	page: number;
	pageSize: number;
}

/**
 * 端點 2 的 body。四種 `carrierType` 各自只填自己的欄位，其餘留空字串——
 * 後端的 `chk_bip_shape` 就是靠「哪幾欄是空的」判形狀，多填一欄會落 9306。
 */
export interface CreateInvoiceProfileInput {
	carrierType: InvoiceCarrierTypeValue;
	/** 手機條碼／自然人憑證條碼／捐贈碼共用這一欄；`TAX_ID` 必須留空。 */
	value: string;
	buyerTaxId: string;
	buyerName: string;
	label: string;
	setDefault: boolean;
}

/**
 * 端點 3 的 body。**沒有 `carrierType`**：類型不可變更（要換類型＝刪除後新增），
 * 也**沒有 `setDefault`**（那是端點 5 的職責）。
 */
export type UpdateInvoiceProfileInput = Omit<
	CreateInvoiceProfileInput,
	"carrierType" | "setDefault"
>;

/**
 * 一份發票／收據文件在買家眼中的摘要（端點 8 的 `documents[]` 子集）。
 *
 * `status` 是 `InvoiceDocumentStatus` 的 11 值，買家看到的是它的折疊投影——
 * 折疊表在 `documentStatus.ts` 一處，B04／B05 共用。
 */
export interface InvoiceDocumentSummary {
	documentId: string;
	/** `InvoiceDocumentKind`：`B2C`／`B2B`／`RECEIPT` 三值（折讓不是 document）。 */
	documentKind: string;
	/** `InvoiceDocumentStatus`，型別是 string——理由見檔頭第 1 點。 */
	status: string;
	invoiceNumber: string;
	issuedAt: string;
	totalAmount: string;
	carrierSummary: string;
	/**
	 * `carrierSummary` 的判別欄（backend#3084）。
	 *
	 * 非空（`MOBILE_BARCODE` / `CITIZEN_CERT`）⇒ summary 裝的是**載具**；
	 * 空字串 ⇒ 裝的是**買受人統編**，或什麼都沒有。
	 *
	 * 🔴 後端在沒有載具時會把買受人統編放進 `carrierSummary`（那是刻意的 ——
	 * B2B 列表要看得到開給哪個統編）。**只看 `carrierSummary` 分不出兩者**，
	 * 標籤一律用「載具」的話，沒有載具的 B2B 就會顯示成「載具＝買受人統編」。
	 * 用 `carrierLabelOf()` 決定標籤，不要自己判斷字串長得像不像統編。
	 */
	carrierType: string;
	relatedDocumentId: string;
	fileAvailable: boolean;
	fileUnavailableReason: string;
}

/**
 * 端點 68 `GET /app/invoices/documents` 的一列（gateway `BuyerInvoiceDocumentItem`）。
 *
 * `issuerName` 平放在 `summary` 之外，因為它不是文件本身的屬性而是**開立人**：
 * 方向一（賣家→買家）是賣家名稱、方向三（平台→買家）是平台名稱。B09 的「開立人」
 * 欄讀的就是它，**不得省略、也不得用平台名稱統一填充**——付款商、平台與發票開立人
 * 是三件事（58A §B09）。
 */
export interface BuyerInvoiceDocumentItem {
	summary: InvoiceDocumentSummary;
	issuerName: string;
}

/**
 * 端點 68 的 `response.Page`。欄名由 `response.Page` 定死是 `records`／`total`，
 * **不是 `list`／`items`**。
 */
export interface BuyerInvoiceDocumentListResult {
	records: BuyerInvoiceDocumentItem[];
	total: number;
	page: number;
	pageSize: number;
}

/**
 * 端點 8 `GET /app/orders/{order_no}/invoice`（gateway `OrderInvoiceData`）。
 *
 * `state` 是**買家可見投影的 9 值**（backend #2904 起含 `ORDER_CANCELLED`），與 `documents[].status` 的 11 值是兩套刻意
 * 分離的字串（58 §6.1.3），不可互相代入。它不納管 CodeMeta（§C.4）⇒ 標題句寫在
 * `features/invoices/i18n.ts`，內文一律逐字用後端的 `stateNotice`。
 */
export interface OrderInvoice {
	orderNo: string;
	/** `InvoiceFulfillmentMode` 的值（4 值），型別是 string——理由見檔頭第 1 點。 */
	invoiceMode: string;
	/** 買家可見的 8 值投影，型別是 string——同上。 */
	state: string;
	stateNotice: string;
	/**
	 * 🔴 全部渲染，不取最後一筆：作廢重開之後常態是「舊的 SUPERSEDED ＋ 新的
	 * ISSUED」並存，只顯示其中一筆會讓買家看到已經不算數的那張。順序由後端保證
	 * （依 `issuedAt` 遞增），前端不再排序。
	 */
	documents: InvoiceDocumentSummary[];
	canRequestCorrection: boolean;
	canFileComplaint: boolean;
	complaintOpensAt: string;
	existingComplaintId: string;
	/**
	 * 這張訂單**最近一次**修改申請的 id（端點 12 的入參）；沒申請過為空字串。
	 *
	 * 🔴 它是買家端取得 correction_id 的唯一管道。2026-09-05 之前端點 8 不給它，
	 * 於是 `fetchInvoiceCorrection`（端點 12 的 client）**零呼叫端**：送出修改申請、
	 * 關掉頁面再回來，申請進度與婉拒理由三處都看不到，而 `rejectReasonCode`
	 * 決定買家該改走折讓還是申訴。
	 *
	 * ⚠️ 它**不分狀態**取最新一筆（與 existingComplaintId 只給進行中案件不同）：
	 * 被婉拒的那一筆才是最需要顯示的。
	 */
	latestCorrectionId: string;
	/** 為空代表通知還沒寄成功 ⇒ 那一列整列不渲染。 */
	notifiedEmailMasked: string;
	shippingFeeNotice: string;
	snapshotVersion: string;
}

/** 一張發票的品項行（`InvoiceLineType` 三值）。`DISCOUNT` 的 `netAmount` 是負數。 */
export interface InvoiceLine {
	/** `InvoiceLineType`，型別是 string——理由見檔頭第 1 點。 */
	lineType: string;
	name: string;
	quantity: string;
	unitPrice: string;
	netAmount: string;
	/** `InvoiceTaxCategory`（`1`／`2`／`3`）。 */
	taxCategory: string;
}

/**
 * 文件歷程的一筆（B05 的 `DocumentTimeline`）。
 *
 * 🔴 `event` 的值域是 CodeMeta **`InvoiceDocumentTimelineEvent`**（5 值），
 * **不是 `InvoiceEventType`**——後者是 MQ 事件型別，兩者交集為空，查錯表只會
 * 整欄空白而不報錯。
 */
export interface InvoiceTimelineItem {
	occurredAt: string;
	/** `InvoiceDocumentTimelineEvent`，型別是 string——理由見檔頭第 1 點。 */
	event: string;
	note: string;
}

/**
 * 端點 69 `GET /app/invoices/documents/{document_id}`（gateway
 * `InvoiceDocumentDetail`）。B05 整頁只打這一支。
 *
 * ⚠️ `invoiceFileKey` **不進本型別**：它是 media objectKey，`secure/invoice/`
 * 前綴在公開 CDN 面是 fail-closed 而 image-gateway 沒有簽章機制 ⇒ 拿去組 URL
 * 永遠是 403，而且會讓人以為那是一條可用的路徑。要 PDF 只有 `/file/content`
 * 一條路（`invoiceFileUrl`）。
 */
export interface InvoiceDocumentDetail {
	summary: InvoiceDocumentSummary;
	/** 開立人。`PLATFORM_RECEIPT` 時是平台，所以 label 是「開立人」不是「賣家」。 */
	issuerName: string;
	/** `PLATFORM_RECEIPT` 時放的是免用統一發票標示字串，不是統編。 */
	issuerTaxId: string;
	recipientName: string;
	recipientTaxId: string;
	salesAmount: string;
	taxAmount: string;
	totalAmount: string;
	/** `InvoiceTaxCategory`；不直接顯示，只在稅額列旁補一行小字。 */
	taxCategory: string;
	lines: InvoiceLine[];
	timeline: InvoiceTimelineItem[];
}

/* ===== B07｜未收到發票／收據申訴（端點 13／14／15／63／64／65） ===== */

/**
 * 申訴案件歷程的一筆（gateway `ComplaintResponseItem`）。
 *
 * 🔴 `action` 的值域是 CodeMeta **`InvoiceComplaintAction`（6 值）**，而且
 * **只裝賣家與 ADM 在畫面上點的按鈕**——買家自己的動作（提出／不接受／撤回／
 * 重新提出）一律不進這一軸，`action` 會是空字串。查 `InvoiceComplaintEventType`
 * 是查錯表：兩者是同一張表的兩個正交欄位，交集為空。
 */
export interface InvoiceComplaintResponseItem {
	responseId: string;
	/** `SELLER`／`ADMIN`。型別是 string——理由見檔頭第 1 點。 */
	actor: string;
	/** `InvoiceComplaintAction`，可能為空字串（見上）。 */
	action: string;
	note: string;
	occurredAt: string;
}

/**
 * 端點 13／15／63／64／65 共用的回應（gateway `InvoiceComplaintInfo`）。
 *
 * ⚠️ 三個數字欄位（`remainingBusinessDays`／`overdueBusinessDays`／`reopenCount`）
 * 在 wire 上是**數字字串且可為負**，直接拿去比大小是字典序比較：`"-2" >= "3"`
 * 與 `"10" >= "3"` 都是 false ⇒ 逾期與充裕會被歸成同一級而畫面看起來正常
 * （§B-1.0.9 B06t）。一律先判空字串、再 `Number()`。
 *
 * ⚠️ `remainingBusinessDays` 為空字串是**設計指定的 fallback**（行事曆鏡像沒有
 * 資料時後端不猜），不是 bug ⇒ 前端顯示「處理期限計算中」，**不得退化成日曆天
 * 自己算**（§0.6 C10）。
 */
export interface InvoiceComplaint {
	complaintId: string;
	caseNo: string;
	orderNo: string;
	storeId: string;
	/** `InvoiceComplaintStatus`（7 值），型別是 string——理由見檔頭第 1 點。 */
	status: string;
	/** `InvoiceDocumentKind`，Dialog 的「應收到的文件」。 */
	expectedDocKind: string;
	/** 不直接顯示——`remainingBusinessDays` 比絕對時間好懂（§B07）。 */
	dueAt: string;
	remainingBusinessDays: string;
	overdueBusinessDays: string;
	buyerMaskedName: string;
	description: string;
	responses: InvoiceComplaintResponseItem[];
	/** 樂觀鎖版本；63／64／65 的 `expectedVersion` **只能**來自這一欄。 */
	version: string;
	createdAt: string;
	reopenCount: string;
	/** `SELLER_RESPONDED` 時買家的異議期限；空字串時整句不渲染。 */
	buyerReviewDueAt: string;
	/** 重開冷卻結束時間；空字串代表尚不可重開。 */
	reopenAllowedAfter: string;
}

/** 端點 13 的 body。`description` 是**選填**（送出鍵不因空白而 disable）。 */
export interface CreateInvoiceComplaintInput {
	/** Phase 1 只有 `NOT_RECEIVED` 一個值。 */
	reasonCode: string;
	description: string;
}

/**
 * 買家申請修改發票資料的結果（58 §6.1 端點 11，B06）。
 *
 * 欄位逐一抄自 gateway 的 `InvoiceCorrectionData`。
 *
 * 🔴 **`handlingMode` 決定買家看到什麼**，而它是後端算出來的：同一個申請，
 * 開票前是「直接改」、開票後是「作廢重開」。前端不得自己依「有沒有發票號碼」推導
 * ——判準在後端的 §7.3.6 狀態機裡，前端複製一份必然漂移。
 */
export interface InvoiceCorrection {
	correctionId: string;
	/** `UPDATE_BEFORE_ISSUE`／`VOID_AND_REISSUE` 之類；由後端決定。 */
	handlingMode: string;
	status: string;
	/** 後端給的人話 ＋ 下一步。前端不重寫。 */
	statusNotice: string;
	createdAt: string;
	rejectReasonCode: string;
}

/** B06 送出的新發票資料。與結帳時的 `invoice` 區塊同形狀（`InvoiceInputSpec`）。 */
export interface InvoiceCorrectionInput {
	profileId?: string;
	/**
	 * `InvoiceCarrierType` 的值。手動輸入統編時**必填** `"TAX_ID"`。
	 *
	 * 🔴 後端由 `carrierType` 判買受人身分，**不從「哪個欄位有值」去推**
	 * （invoice-service `identityFromManualInput` 逐字：「由欄位有沒有值去推語意，
	 * 就是在憑證上猜買受人身分」）。少了它，端點 11 回 9305「格式不正確」，
	 * 而買家填的統編其實完全合法。結帳路徑（端點 6）一直都有送。
	 */
	carrierType?: string;
	buyerTaxId?: string;
	buyerName?: string;
	email?: string;
}
