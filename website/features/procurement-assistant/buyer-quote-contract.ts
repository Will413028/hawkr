import { z } from "zod";
import {
	ProcurementOrderProvisioningState,
	ProcurementQuoteAcceptability,
	type ProcurementQuoteAcceptabilityValue,
	ProcurementSellerResponseType,
	ProcurementTaxInclusionBasis,
} from "@/lib/codemeta/enums";
import { idSchema, payloadHashSchema } from "./rfq-price-contract";

/**
 * 買家端「報價比較與接受」的 wire 契約（issue #2538 A8 前半）。
 *
 * 交易鏈上這一段的定義（doc 64 §4.2、v6.0 §V.11.5）：賣家對一張已發布的 RFQ
 * 送出**不可變的報價版本**，買家比較之後**接受其中一個 exact version**，伺服器
 * 的權威 ACK 即商務 award。所以本檔的每一個型別都圍繞同一條不變式：
 *
 *   ⛔ **瀏覽器不判斷「這個版本能不能接受」，也不決定成交價格。**
 *
 * 具體落實成三條，違反其中任何一條都是 bug 不是最佳化：
 *   1. 可否接受只讀伺服器的 `acceptability`——不比對 `validUntil` 與現在時間、
 *      不比對版本號大小、不看 `status` 自行推導（v6.0 §V.11.5 逐字要求
 *      withdrawn／expired／superseded 的 response 留在 audit 但不可接受，
 *      「UI 取得 typed reason」）。
 *   2. 接受的 body 只帶三個 CAS 欄位，**不帶任何金額**——`acceptedDealPrice`
 *      由伺服器從被接受的那一版取。前端送金額等於開一條讓瀏覽器改成交價的路。
 *   3. 不同 `unitBasis` 之間**不可直接比單價**，所以本檔不提供、也不得新增
 *      任何「每單位價」的衍生欄位。`unitBasis`／`offeredQuantityUnit`／
 *      `conversionBasis` 三欄存在就是為了讓買家看見這件事。
 */

// 金額是 canonical fixed-decimal 字串，不是 number：TWD 目前只到整數，但
// `rfq-price-contract` 的參考價已經在用四位小數，接受流程如果把金額當
// number 建模，double 的舍入會直接落在成交價上。
//
// 與 `rfq-price-contract` 的 `serverAmountSchema` 差在**允許 0**：運費與稅額
// 是可以為零的合法金額，那支 schema 專門用在必為正的價格區間端點上，不能直接
// 套過來。
const CANONICAL_AMOUNT_RE = /^(0|[1-9]\d{0,11})(?:\.(\d{1,4}))?$/;

/** 回傳以 1/10000 為單位的整數值；不是 canonical 十進位金額字串就回 null。 */
export function parseCanonicalTwdAmount(value: string): bigint | null {
	const match = CANONICAL_AMOUNT_RE.exec(value);
	if (!match) return null;
	return (
		BigInt(match[1]) * BigInt(10_000) + BigInt((match[2] ?? "").padEnd(4, "0"))
	);
}

const amountSchema = z
	.string()
	.refine(
		(value) => parseCanonicalTwdAmount(value) !== null,
		"amount must be a canonical non-negative TWD decimal string",
	);

/**
 * 金額顯示。刻意逐字保留伺服器給的小數位（`"1200.5"` → `NT$1,200.5`）：
 * 補零或四捨五入都會讓畫面上的數字與被接受的那一版不同字。
 *
 * 本 repo 目前有四份各自私有的 `formatTwd`（seller pricing／ChatCards／
 * AttachmentPicker／RfqDraftFlow），module-private 是既有慣例；統一它們是另一
 * 件事，不在本次範圍內順手做。
 */
export function formatTwdAmount(value: string): string {
	const [integer, fraction] = value.split(".");
	if (!/^\d+$/.test(integer ?? "")) return value;
	const grouped = new Intl.NumberFormat("zh-TW").format(BigInt(integer));
	return `NT$${grouped}${fraction ? `.${fraction}` : ""}`;
}

/**
 * 賣家身分投影。**只有**這三欄（doc 64 §4.2）。
 *
 * 這裡刻意用 zod 預設的「剝掉未宣告欄位」而不是 `.strict()`：剝掉是**隱私護
 * 欄**——就算 Gateway 哪天在這個物件上多帶了站外業者 email、KYC 狀態或 prospect
 * 聯絡方式，它們也進不了瀏覽器的 state，因此不可能被渲染或被 devtools 讀到。
 * 換成 `.strict()` 反而是把「多一個無害欄位」升級成整張比較表解析失敗，代價
 * 更高而擋不住更多東西。
 */
export const sellerIdentityViewSchema = z.object({
	displayName: z.string().min(1).max(128),
	/**
	 * 身分別，CodeMeta `ProcurementSellerPartyType`（`PLATFORM_STORE`／
	 * `EXTERNAL_VENDOR`）。刻意留 `z.string()`：值域由伺服器擁有，後端多送一種
	 * 身分只該讓那一列的身分標籤退化成原碼，不該讓整張比較表 fail closed。
	 */
	partyType: z.string().min(1).max(64),
	/** 站外業者沒有店鋪代碼，這一欄會是空字串——不得補值假裝有店面。 */
	storeCode: z.string().max(64),
});

export type SellerIdentityView = z.infer<typeof sellerIdentityViewSchema>;

export const sellerQuoteResponseTypeSchema = z.enum(
	ProcurementSellerResponseType,
);

export type SellerQuoteResponseType = z.infer<
	typeof sellerQuoteResponseTypeSchema
>;

/** 文案是買家端 UX，不取 CodeMeta 的 `PROCUREMENT_SELLER_RESPONSE_TYPE_LABEL`；理由同下方 acceptability。 */
export const SELLER_QUOTE_RESPONSE_TYPE_LABEL: Record<
	SellerQuoteResponseType,
	string
> = {
	MANUAL_QUOTE: "自行報價",
	ACCEPT_NON_PRICE_TERMS_WITH_QUOTE: "接受非價格條件並報價",
};

export const taxInclusionBasisSchema = z.enum(ProcurementTaxInclusionBasis);

export type TaxInclusionBasis = z.infer<typeof taxInclusionBasisSchema>;

/** 同上：CodeMeta 那份是「含稅／未稅」，這裡是比較表上的「總額已含稅／總額未含稅」。 */
export const TAX_INCLUSION_BASIS_LABEL: Record<TaxInclusionBasis, string> = {
	TAX_INCLUDED: "總額已含稅",
	TAX_EXCLUDED: "總額未含稅",
};

/**
 * 交期天數。
 *
 * ⚠️ 這支端點在 dev 上還不存在，wire 上到底是十進位字串（同
 * `recipientCount`／`supportingMarketCount` 的既有慣例）還是 JSON number 無法
 * 現在證實。它**不是** id、不是金額、也不參與任何 CAS，純粹是顯示欄位，所以
 * 這裡兩種都收並一律正規化成字串：猜錯的代價是多接受一種形狀，而把它猜死的
 * 代價是整張比較表 fail closed。金額與 id 沒有這種寬容——那些一律嚴格。
 */
const leadTimeDaysSchema = z
	.union([
		z.string().regex(/^\d{1,9}$/, "lead time must be a decimal string"),
		z.number().int().nonnegative(),
	])
	.transform((value) => String(value));

/**
 * 賣家報價 payload，doc 64 §4.2 凍結的十七欄。
 *
 * ⛔ 不要在這裡加「每單位價」「總價／數量」之類的衍生欄位。`unitBasis`
 * （報價的計價基準）、`offeredQuantityUnit`（供貨量的單位）與 `conversionBasis`
 * （兩者之間的換算依據）是三個獨立事實，各家可以不同；把它們壓成一個數字就是
 * 把「不可直接比較」抹平成「可以直接比較」，那正是這三欄存在要防的事。
 */
export const sellerQuotePayloadSchema = z.object({
	responseType: sellerQuoteResponseTypeSchema,
	offeredQuality: z.object({
		/** 實際品相等級；賣家未填時是空字串，不得補「一般」之類的預設值。 */
		grade: z.string().max(64).default(""),
		inspectionNotes: z.string().max(2000).default(""),
	}),
	/** 報價的計價基準，例如 1 KG、10 CATTY。 */
	unitBasis: z.object({
		quantity: z.string().min(1).max(64),
		unit: z.string().min(1).max(32),
	}),
	offeredQuantity: z.string().min(1).max(64),
	offeredQuantityUnit: z.string().min(1).max(32),
	/** 計價基準與供貨單位之間的換算依據；沒有換算時是空字串。 */
	conversionBasis: z.string().max(256).default(""),
	currency: z.literal("TWD"),
	goodsAmount: amountSchema,
	shippingAmount: amountSchema,
	taxAmount: amountSchema,
	taxInclusionBasis: taxInclusionBasisSchema,
	/** ⛔ 由伺服器加總。前端不重算，也不因為對不起來就自行修正。 */
	totalAmount: amountSchema,
	leadTimeDays: leadTimeDaysSchema,
	/** 空字串＝賣家沒有承諾到貨時間，整列不顯示、不補值。 */
	promisedDeliveryAt: z.string().default(""),
	/** 空字串＝這一版沒有有效期限。⛔ 不得拿它跟現在時間比對推導可否接受。 */
	validUntil: z.string().default(""),
	termsNote: z.string().max(2000).default(""),
	deviationDisclosure: z.string().max(2000).default(""),
});

export type SellerQuotePayload = z.infer<typeof sellerQuotePayloadSchema>;

/**
 * 伺服器判定的可接受性（typed reason）。
 *
 * `ACCEPTABLE` 以外的值代表這一版**不能接受但仍要留在清單裡**——它是稽核軌跡的
 * 一部分（v6.0 §V.11.5）。
 */
export const QUOTE_ACCEPTABILITY_VALUES: readonly ProcurementQuoteAcceptabilityValue[] =
	Object.values(ProcurementQuoteAcceptability);

export type QuoteAcceptability = ProcurementQuoteAcceptabilityValue;

/**
 * ⚠️ 文案刻意**不**取 CodeMeta 的 `PROCUREMENT_QUOTE_ACCEPTABILITY_LABEL`。
 * 值域是契約、文案是 UX：CodeMeta 那份是給 ADM 營運看的通用標籤（「已撤回」
 * 「該賣家已停止回應」），這裡是買家在比較表上看到的字。換過去等於改 UI 文案，
 * 要另外取得授權，不在「同步值域」的範圍內。
 */
const QUOTE_ACCEPTABILITY_LABEL: Record<QuoteAcceptability, string> = {
	ACCEPTABLE: "可接受",
	EXPIRED: "已逾有效期",
	WITHDRAWN: "賣家已撤回",
	SUPERSEDED: "已被新版本取代",
	INELIGIBLE: "不符合接受資格",
};

/**
 * 只有**恰好**等於 `ACCEPTABLE` 才放行，其餘（含伺服器日後新增、本版沒見過的
 * 值）一律不可接受。
 *
 * 這是刻意的 fail closed 方向：多一個不認得的值時，錯放行會讓買家按下一顆伺服
 * 器必然拒絕的按鈕、或更糟地接受一個不該接受的版本；錯擋下只是少一次操作，
 * 而且清單與稽核資訊都還在。
 */
export function isAcceptableQuote(acceptability: string): boolean {
	return acceptability === ProcurementQuoteAcceptability.ACCEPTABLE;
}

/**
 * 不可接受的原因文案。
 *
 * 不認得的值回一句**不宣稱原因**的中性說明——編一個原因（例如一律說「已過期」）
 * 會把稽核紀錄講錯，比不說更糟。
 */
export function quoteAcceptabilityLabel(acceptability: string): string {
	return (
		QUOTE_ACCEPTABILITY_LABEL[acceptability as QuoteAcceptability] ??
		"目前無法接受這個版本"
	);
}

/**
 * 比較清單的一列。
 *
 * `status` 刻意只留在資料裡、不轉成任何使用者可見文案：可否接受的唯一 typed
 * 來源是 `acceptability`，拿 `status` 另外講一次會變成第二套判斷，而它的值域
 * 目前沒有凍結文件可查。
 */
export const buyerFacingSellerResponseSchema = z.object({
	sellerResponseId: idSchema,
	sellerResponseVersion: idSchema,
	status: z.string().max(64).default(""),
	payloadHash: payloadHashSchema,
	submittedAt: z.string().min(1),
	seller: sellerIdentityViewSchema,
	quote: sellerQuotePayloadSchema,
	/** 伺服器判定這一版落在買家希望區間之外；中性揭露，不阻擋、不自動淘汰。 */
	outsideBuyerRequestedRange: z.boolean(),
	shortlisted: z.boolean(),
	acceptability: z.string().min(1).max(64),
});

export type BuyerFacingSellerResponse = z.infer<
	typeof buyerFacingSellerResponseSchema
>;

/**
 * 比較清單的外層形狀。
 *
 * 端點還沒上 dev，Gateway 究竟把陣列放在 `data` 本身還是包一層具名欄位無法現在
 * 證實，所以這裡接受「`data` 是陣列」與「`data` 是物件且其中恰好有一個陣列
 * 欄位」兩種，其餘 fail closed。這不是寬鬆解析：三個以上互斥的猜測才是，而
 * 「陣列在哪一層」只有這兩種可能，選錯任何一種都會讓一支已完成的畫面在接線那天
 * 才發現接不上。
 */
function unwrapResponseList(payload: unknown): unknown {
	if (Array.isArray(payload)) return payload;
	if (payload && typeof payload === "object") {
		const arrays = Object.values(payload as Record<string, unknown>).filter(
			Array.isArray,
		);
		if (arrays.length === 1) return arrays[0];
	}
	return payload;
}

export const buyerFacingSellerResponseListSchema = z.preprocess(
	unwrapResponseList,
	z.array(buyerFacingSellerResponseSchema),
);

/**
 * 加入／移除比較的目標。
 *
 * PUT／DELETE 都是冪等、零 side effect，所以**不需要冪等鍵**——這是契約明訂的，
 * 不是省略。替它加一把 key 只會讓「同一列連按兩次」變成 409。
 */
export interface QuoteShortlistTarget {
	sellerResponseId: string;
	sellerResponseVersion: string;
}

/**
 * 接受一個 exact version 的命令。
 *
 * `idempotencyKey` 走 HTTP header `Idempotency-Key`，**不放 body**；同一次送出
 * 的重試必須沿用同一把，改內容才換新的一把（與 `delegated-repos` 的既有規則
 * 一致）。
 */
export interface BuyerQuoteAcceptanceCommand {
	idempotencyKey: string;
	sellerResponseId: string;
	expectedSellerResponseVersion: string;
	/** 來自 `RfqConfirmation.payloadHash`（已確認快照本身的 hash）。 */
	expectedRfqSnapshotHash: string;
}

export interface BuyerQuoteAcceptanceBody {
	sellerResponseId: string;
	expectedSellerResponseVersion: string;
	expectedRfqSnapshotHash: string;
}

/**
 * ⛔ 恰好三個 CAS 欄位。`acceptedDealPrice` 或任何金額都**不得**出現在 body：
 * 成交價格由伺服器從被接受的那一版取，瀏覽器連提議的資格都沒有。
 * 冪等鍵也不在這裡——它是 header。
 */
export function buyerQuoteAcceptanceBody(
	command: BuyerQuoteAcceptanceCommand,
): BuyerQuoteAcceptanceBody {
	return {
		sellerResponseId: command.sellerResponseId,
		expectedSellerResponseVersion: command.expectedSellerResponseVersion,
		expectedRfqSnapshotHash: command.expectedRfqSnapshotHash,
	};
}

export const orderProvisioningStateSchema = z.enum(
	ProcurementOrderProvisioningState,
);

export type OrderProvisioningState = z.infer<
	typeof orderProvisioningStateSchema
>;

/**
 * 接受成功的權威 ACK。
 *
 * ⛔ ACK **不帶** `orderNo`（`CREATING_ORDER` 時是空字串），所以這個回應
 * **不得**被畫成「已成交」或「已下單」。買家看得到的固定文案見
 * `QUOTE_ACCEPTED_HEADLINE`。
 */
export const buyerQuoteAcceptanceResponseSchema = z
	.object({
		buyerAcceptanceId: idSchema,
		sellerResponseId: idSchema,
		sellerResponseVersion: idSchema,
		/** 伺服器從被接受的那一版取的成交價格，前端只顯示。 */
		acceptedDealPrice: amountSchema,
		currency: z.literal("TWD"),
		acceptedAt: z.string().min(1),
		orderProvisioningState: orderProvisioningStateSchema,
		/** 空字串直到 `ORDER_CREATED`。 */
		orderNo: z.string().max(64).default(""),
		/** 空字串直到 `ORDER_CREATED`。 */
		paymentDeadline: z.string().default(""),
		replayed: z.boolean(),
	})
	.superRefine((ack, context) => {
		// 兩個方向都要擋：有單號卻不是 ORDER_CREATED，會讓畫面拿著一個不該存在
		// 的單號去畫付款入口；是 ORDER_CREATED 卻沒單號，則是把「訂單已建立」
		// 講出口卻連不到那張單。任一種都是矛盾的 wire，不是可以補值的缺漏。
		if (
			ack.orderNo !== "" &&
			ack.orderProvisioningState !==
				ProcurementOrderProvisioningState.ORDER_CREATED
		) {
			context.addIssue({
				code: "custom",
				message: "order number must not appear before ORDER_CREATED",
				path: ["orderNo"],
			});
		}
		if (
			ack.orderProvisioningState ===
				ProcurementOrderProvisioningState.ORDER_CREATED &&
			ack.orderNo === ""
		) {
			context.addIssue({
				code: "custom",
				message: "ORDER_CREATED must carry an order number",
				path: ["orderNo"],
			});
		}
	});

export type BuyerQuoteAcceptanceResponse = z.infer<
	typeof buyerQuoteAcceptanceResponseSchema
>;

/**
 * 接受成功後的固定文案（v6.0 L2072、doc 64 §4.2 逐字凍結）。
 *
 * ⛔ 不要改寫、不要加「恭喜」、不要換成「已成交」。`CREATING_ORDER` 是買家看得
 * 到的狀態，這句話同時承擔「價格凍結」與「尚未付款」兩個事實。
 */
export const QUOTE_ACCEPTED_HEADLINE =
	"已接受這個報價版本，成交價格已凍結，尚未付款";

/** 業務錯誤碼（CodeMeta `ProcurementErrorReason`）＋本檔補的傳輸層碼。 */
export type BuyerQuoteFailureCode =
	| "STALE_RFQ_SNAPSHOT"
	| "STALE_PUBLICATION"
	| "STALE_SELLER_RESPONSE"
	| "RFQ_RESPONSE_EXPIRED"
	| "RFQ_RECIPIENT_CLOSED"
	| "RFQ_CHECKOUT_ALREADY_ACTIVE"
	| "RFQ_ALREADY_AWARDED"
	| "QUOTE_AMOUNT_INCONSISTENT"
	| "QUOTE_QUALITY_REQUIRED"
	| "IDEMPOTENCY_KEY_CONFLICT"
	| "RESOURCE_NOT_FOUND"
	| "UNAUTHORIZED"
	| "FORBIDDEN"
	| "RATE_LIMITED"
	| "INVALID_RESPONSE"
	| "STALE_CONTEXT"
	| "NETWORK_ERROR"
	| "SERVER_ERROR"
	| "UNKNOWN_ERROR";

export interface BuyerQuoteFailure {
	code: BuyerQuoteFailureCode;
	detail: string;
	/** 同一把冪等鍵重試會不會有意義；業務判定一律 false。 */
	retryable: boolean;
	/**
	 * 只有 `RFQ_CHECKOUT_ALREADY_ACTIVE` 可能帶：伺服器指回既有的那張待付款單。
	 *
	 * ⚠️ 契約沒有明訂它掛在錯誤 payload 的哪一層，所以這裡是「有就用、沒有就
	 * 退回訂單列表」的機會性讀取，不是必要欄位。猜錯的代價是買家多點一下，
	 * 不會導到錯的地方。
	 */
	orderNo?: string;
}

export type BuyerQuoteResult<T> =
	| { ok: true; data: T }
	| { ok: false; error: BuyerQuoteFailure };

const BUSINESS_FAILURE_DETAIL: Record<string, string> = {
	STALE_RFQ_SNAPSHOT:
		"這張詢價的內容已經變動，剛才看到的快照不再是最新的。請重新整理，確認需求之後再選要接受哪一個版本。",
	STALE_PUBLICATION: "這張詢價的發布依據已經變動。請重新整理之後再操作。",
	// ⛔ 產品硬規則：必須明確告知報價已變動，不得靜默改成接受最新版。
	STALE_SELLER_RESPONSE:
		"這家賣家已經送出新的報價版本，你剛才看到的那一版不再是最新的。系統不會自動幫你接受新版本，請重新整理、重新比較之後再決定。",
	RFQ_RESPONSE_EXPIRED: "這個報價版本已逾有效期，無法接受。",
	RFQ_RECIPIENT_CLOSED: "這家賣家已停止回應這張詢價，這個版本無法接受。",
	// 這兩個是不同的事，文案不共用：前者還有下一步，後者沒有。
	RFQ_CHECKOUT_ALREADY_ACTIVE:
		"你已經接受過這張詢價的報價，只是還沒付款。請前往付款完成這筆交易，不會再建立第二筆。",
	RFQ_ALREADY_AWARDED: "這張詢價已經成交結案，不能再接受其他報價版本。",
	QUOTE_AMOUNT_INCONSISTENT:
		"這個報價版本的金額組成不一致，伺服器拒絕接受。請等賣家更正後重新報價。",
	QUOTE_QUALITY_REQUIRED:
		"這個報價版本沒有填實際品相，伺服器拒絕接受。請等賣家補齊後重新報價。",
	IDEMPOTENCY_KEY_CONFLICT:
		"這次送出的識別碼與先前一次內容不同的請求衝突，沒有建立第二筆接受。請重新整理之後再試一次。",
	RESOURCE_NOT_FOUND: "找不到這個報價版本，它可能已經被移除。請重新整理。",
};

const TRANSPORT_FAILURE: Record<
	string,
	{ code: BuyerQuoteFailureCode; detail: string; retryable: boolean }
> = {
	UNAUTHORIZED: {
		code: "UNAUTHORIZED",
		detail: "登入狀態已失效，請重新登入之後再操作。",
		retryable: false,
	},
	FORBIDDEN: {
		code: "FORBIDDEN",
		detail: "這個帳號沒有操作這張詢價的權限。",
		retryable: false,
	},
	RATE_LIMITED: {
		code: "RATE_LIMITED",
		detail: "操作太頻繁，請稍候再試。",
		retryable: true,
	},
	SERVER_ERROR: {
		code: "SERVER_ERROR",
		detail:
			"伺服器暫時無法完成這次操作。請用原操作重試；這個畫面不會宣稱已接受。",
		retryable: true,
	},
	NETWORK_ERROR: {
		code: "NETWORK_ERROR",
		detail:
			"連線中斷，無法確認這次操作的結果。請用原操作重試；這個畫面不會宣稱已接受。",
		retryable: true,
	},
};

function isBusinessFailureCode(
	reason: string,
): reason is BuyerQuoteFailureCode {
	return Object.hasOwn(BUSINESS_FAILURE_DETAIL, reason);
}

/**
 * 把 Gateway 的 `{code, reason, message}` 收斂成畫面用的錯誤。
 *
 * `reason` 是機器碼、`message` 是內部字串——後者一律不直接端到買家面前
 * （`errors.ts` 的既有判斷同理）。認得的 `reason` 用本檔凍結的文案，認不得的
 * 才落回依 HTTP 狀態分類的傳輸層文案。
 */
export function buyerQuoteFailure(
	status: number,
	reason: string,
	orderNo?: string,
): BuyerQuoteFailure {
	if (isBusinessFailureCode(reason)) {
		return {
			code: reason,
			detail: BUSINESS_FAILURE_DETAIL[reason],
			retryable: false,
			...(reason === "RFQ_CHECKOUT_ALREADY_ACTIVE" && orderNo
				? { orderNo }
				: {}),
		};
	}
	if (status === 401) return { ...TRANSPORT_FAILURE.UNAUTHORIZED };
	if (status === 403) return { ...TRANSPORT_FAILURE.FORBIDDEN };
	if (status === 404) {
		return {
			code: "RESOURCE_NOT_FOUND",
			detail: BUSINESS_FAILURE_DETAIL.RESOURCE_NOT_FOUND,
			retryable: false,
		};
	}
	if (status === 429) return { ...TRANSPORT_FAILURE.RATE_LIMITED };
	if (status >= 500) return { ...TRANSPORT_FAILURE.SERVER_ERROR };
	if (status === 0) return { ...TRANSPORT_FAILURE.NETWORK_ERROR };
	return {
		code: "UNKNOWN_ERROR",
		detail:
			"這次操作沒有完成，原因不明。請用原操作重試；這個畫面不會宣稱已接受。",
		retryable: true,
	};
}

/**
 * 各家的計價基準是否一致。
 *
 * 回 false 時比較表要把「不可直接比單價」講出來。這**不是**在推導誰比較便宜，
 * 只是在回答「這些數字能不能並排看」——它只讀基準欄位，完全不碰金額。
 */
export function hasComparableUnitBasis(
	responses: readonly BuyerFacingSellerResponse[],
): boolean {
	const bases = new Set(
		responses.map(
			(response) =>
				`${response.quote.unitBasis.quantity} ${response.quote.unitBasis.unit} ${response.quote.offeredQuantityUnit} ${response.quote.conversionBasis}`,
		),
	);
	return bases.size <= 1;
}

/** 版本識別的短碼；完整 id 太長，列表上只用尾碼辨識。 */
export function shortVersionLabel(
	response: Pick<BuyerFacingSellerResponse, "sellerResponseVersion">,
): string {
	return `版本 ${response.sellerResponseVersion.slice(-6)}`;
}
