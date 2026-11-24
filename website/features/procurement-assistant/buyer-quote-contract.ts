import {
	type AcceptSellerQuoteResponse,
	QuoteAcceptability as ProtoAcceptability,
	OrderProvisioningState as ProtoOrderState,
	type BuyerFacingSellerResponse as ProtoSellerResponse,
	TaxInclusionBasis as ProtoTaxBasis,
	SellerResponseStatus,
	SellerResponseType,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import {
	commonUnknown,
	procurementQuoteAcceptability,
} from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";
import type { ProcurementFailure } from "./errors";
import { timestampIso } from "./proto-map";

/**
 * 買家端「報價比較與接受」的畫面模型與顯示規則。
 *
 * 交易鏈上這一段的定義：賣家對一張已發布的詢價送出**不可變的報價版本**，買家比較之後
 * **接受其中一個 exact version**，伺服器的權威 ACK 即商務 award。所以每個型別都圍繞同一條不變式：
 *
 *   瀏覽器不判斷「這個版本能不能接受」，也不決定成交價格。
 *
 * 具體落實成三條：
 *   1. 可否接受只讀伺服器的 `acceptability`，不比對有效期與現在時間、不比對版本號大小、不看 `status`。
 *   2. 接受的請求只帶版本與內容 hash 的比對欄位，不帶任何金額；成交價由伺服器從被接受的那一版取。
 *   3. 不同 `unitBasis` 之間不可直接比單價，所以不提供任何「每單位價」的衍生欄位。
 */

/** 金額是新台幣元的整數，以十進位字串承載；顯示時逐字保留，不補零、不四捨五入。 */
export function formatTwdAmount(value: string): string {
	const [integer, fraction] = value.split(".");
	if (!/^\d+$/.test(integer ?? "")) return value;
	const grouped = new Intl.NumberFormat("zh-TW").format(BigInt(integer));
	return `NT$${grouped}${fraction ? `.${fraction}` : ""}`;
}

export type SellerQuoteResponseType =
	| "MANUAL_QUOTE"
	| "ACCEPT_NON_PRICE_TERMS_WITH_QUOTE";

export const SELLER_QUOTE_RESPONSE_TYPE_LABEL: Record<
	SellerQuoteResponseType,
	string
> = {
	MANUAL_QUOTE: "自行報價",
	ACCEPT_NON_PRICE_TERMS_WITH_QUOTE: "接受非價格條件並報價",
};

export type TaxInclusionBasis = "TAX_INCLUDED" | "TAX_EXCLUDED";

export const TAX_INCLUSION_BASIS_LABEL: Record<TaxInclusionBasis, string> = {
	TAX_INCLUDED: "總額已含稅",
	TAX_EXCLUDED: "總額未含稅",
};

export type QuoteAcceptability =
	| "ACCEPTABLE"
	| "EXPIRED"
	| "WITHDRAWN"
	| "SUPERSEDED"
	| "INELIGIBLE";

/** 只有恰好等於 ACCEPTABLE 才放行；不認得的值一律不可接受。 */
export function isAcceptableQuote(acceptability: string): boolean {
	return acceptability === "ACCEPTABLE";
}

/** 可接受性的顯示文字（共用文案目錄）。 */
export function quoteAcceptabilityLabel(
	ut: UiText,
	acceptability: QuoteAcceptability,
): string {
	return (
		optionalText(
			ut,
			procurementQuoteAcceptability(ProtoAcceptability[acceptability]),
		) ?? ut(commonUnknown())
	);
}

export interface SellerIdentityView {
	displayName: string;
	partyType: string;
	storeCode: string;
}

export interface SellerQuotePayload {
	responseType: SellerQuoteResponseType;
	/** 賣家對規格的補充說明；沒有時是空字串。 */
	specNote: string;
	unitBasis: { quantity: string; unit: string };
	offeredQuantity: string;
	offeredQuantityUnit: string;
	/** 計價基準與供貨單位之間的換算依據；沒有換算時是空字串。 */
	conversionBasis: string;
	currency: "TWD";
	goodsAmount: string;
	shippingAmount: string;
	taxAmount: string;
	taxInclusionBasis: TaxInclusionBasis;
	/** 由伺服器驗證等於貨款、運費與稅額的加總，前端不重算。 */
	totalAmount: string;
	/** 空字串＝賣家沒有承諾交期。 */
	leadTimeDays: string;
	/** RFC3339；空字串＝沒有承諾到貨時間。 */
	promisedDeliveryAt: string;
	/** RFC3339；空字串＝這一版沒有有效期限。 */
	validUntil: string;
	termsNote: string;
	deviationDisclosure: string;
}

export interface BuyerFacingSellerResponse {
	sellerResponseId: string;
	sellerResponseVersion: string;
	status: string;
	payloadHash: string;
	submittedAt: string;
	seller: SellerIdentityView;
	quote: SellerQuotePayload;
	/** 伺服器判定這一版落在買家希望區間之外；中性揭露，不阻擋、不自動淘汰。 */
	outsideBuyerRequestedRange: boolean;
	shortlisted: boolean;
	acceptability: QuoteAcceptability;
}

const ACCEPTABILITY_CODE: Partial<
	Record<ProtoAcceptability, QuoteAcceptability>
> = {
	[ProtoAcceptability.ACCEPTABLE]: "ACCEPTABLE",
	[ProtoAcceptability.EXPIRED]: "EXPIRED",
	[ProtoAcceptability.WITHDRAWN]: "WITHDRAWN",
	[ProtoAcceptability.SUPERSEDED]: "SUPERSEDED",
	[ProtoAcceptability.INELIGIBLE]: "INELIGIBLE",
};

/** 未知或未設定的可接受性一律當成不符合資格，不放行。 */
function acceptabilityCode(value: ProtoAcceptability): QuoteAcceptability {
	return ACCEPTABILITY_CODE[value] ?? "INELIGIBLE";
}

export function sellerResponseFromProto(
	response: ProtoSellerResponse,
	closedStoreName: string,
): BuyerFacingSellerResponse | null {
	const quote = response.quote;
	if (!quote) return null;
	const store = response.store;
	return {
		sellerResponseId: response.sellerResponseId,
		sellerResponseVersion: response.sellerResponseVersion,
		status: SellerResponseStatus[response.status] ?? "",
		payloadHash: response.payloadHash,
		submittedAt: timestampIso(response.submitTime),
		seller: {
			displayName: store?.name?.trim() || closedStoreName,
			partyType: "PLATFORM_STORE",
			storeCode: store?.storeCode ?? "",
		},
		quote: {
			responseType:
				quote.responseType ===
				SellerResponseType.ACCEPT_NON_PRICE_TERMS_WITH_QUOTE
					? "ACCEPT_NON_PRICE_TERMS_WITH_QUOTE"
					: "MANUAL_QUOTE",
			specNote: quote.specNote,
			unitBasis: {
				quantity: quote.unitBasis?.quantity ?? "",
				unit: quote.unitBasis?.unit ?? "",
			},
			offeredQuantity: quote.offeredQuantity,
			offeredQuantityUnit: quote.offeredQuantityUnit,
			conversionBasis: quote.conversionBasis,
			currency: "TWD",
			goodsAmount: String(quote.goodsAmount),
			shippingAmount: String(quote.shippingAmount),
			taxAmount: String(quote.taxAmount),
			taxInclusionBasis:
				quote.taxInclusionBasis === ProtoTaxBasis.TAX_INCLUDED
					? "TAX_INCLUDED"
					: "TAX_EXCLUDED",
			totalAmount: String(quote.totalAmount),
			leadTimeDays:
				quote.leadTimeDays === undefined ? "" : String(quote.leadTimeDays),
			promisedDeliveryAt: timestampIso(quote.promisedDeliveryTime),
			validUntil: timestampIso(quote.validUntilTime),
			termsNote: quote.termsNote,
			deviationDisclosure: quote.deviationDisclosure,
		},
		outsideBuyerRequestedRange: response.outsideBuyerRequestedRange,
		shortlisted: response.shortlisted,
		acceptability: acceptabilityCode(response.acceptability),
	};
}

/** 加入／移除比較的目標。 */
export interface QuoteShortlistTarget {
	sellerResponseId: string;
	sellerResponseVersion: string;
}

/**
 * 接受一個 exact version 的命令。`clientRequestId` 一次送出動作一把，同一次動作的重試沿用同一把；
 * 內容不同的請求用同一把會被伺服器以 INVALID_ARGUMENT 拒絕。
 */
export interface BuyerQuoteAcceptanceCommand {
	clientRequestId: string;
	sellerResponseId: string;
	expectedSellerResponseVersion: string;
	/** 買家畫面上那份詢價內容的 hash（發布結果的 `sellerFacingHash`）。 */
	expectedSellerFacingHash: string;
}

export type OrderProvisioningState =
	| "CREATING_ORDER"
	| "ORDER_CREATED"
	| "ORDER_FAILED";

/**
 * 接受成功的權威 ACK。
 *
 * ACK 在 `CREATING_ORDER` 時不帶 `orderNo`，所以這個回應不得被畫成「已成交」或「已下單」。
 */
export interface BuyerQuoteAcceptanceResponse {
	buyerAcceptanceId: string;
	sellerResponseId: string;
	sellerResponseVersion: string;
	/** 伺服器從被接受的那一版取的成交價格，前端只顯示。 */
	acceptedDealPrice: string;
	currency: "TWD";
	acceptedAt: string;
	orderProvisioningState: OrderProvisioningState;
	/** 空字串直到 ORDER_CREATED。 */
	orderNo: string;
	/** 空字串直到 ORDER_CREATED。 */
	paymentDeadline: string;
	replayed: boolean;
}

export function acceptanceFromProto(
	res: AcceptSellerQuoteResponse,
): BuyerQuoteAcceptanceResponse {
	return {
		buyerAcceptanceId: res.buyerAcceptanceId,
		sellerResponseId: res.sellerResponseId,
		sellerResponseVersion: res.sellerResponseVersion,
		acceptedDealPrice: String(res.acceptedDealPrice),
		currency: "TWD",
		acceptedAt: timestampIso(res.acceptTime),
		orderProvisioningState:
			res.orderProvisioningState === ProtoOrderState.ORDER_CREATED
				? "ORDER_CREATED"
				: res.orderProvisioningState === ProtoOrderState.ORDER_FAILED
					? "ORDER_FAILED"
					: "CREATING_ORDER",
		orderNo: res.orderNo,
		paymentDeadline: timestampIso(res.paymentDeadline),
		replayed: res.replayed,
	};
}

/**
 * 接受成功後的固定文案。`CREATING_ORDER` 是買家看得到的狀態，這句話同時承擔「價格凍結」與「尚未付款」兩個事實。
 */
export const QUOTE_ACCEPTED_HEADLINE =
	"已接受這個報價版本，成交價格已凍結，尚未付款";

export type BuyerQuoteFailure = ProcurementFailure;

export type BuyerQuoteResult<T> =
	| { ok: true; data: T }
	| { ok: false; error: BuyerQuoteFailure };

/**
 * 各家的計價基準是否一致。回 false 時比較表要把「不可直接比單價」講出來；
 * 這只回答「這些數字能不能並排看」，不碰金額。
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
