/**
 * Shipping onboarding repository.
 *
 * Mirrors the Flutter seller app's shipping_settings flow. Backend has
 * two carriers — 黑貓宅配（home_delivery） and 7-11 店到店（cvs） — both
 * gated by PayUni's KYC pipeline. Each carrier × temperature layer is a
 * separate onboarding row (e.g. 7-11 + 常溫 and 7-11 + 冷凍 are two
 * independent toggles).
 *
 * State machine (payuni_submit_status):
 *   ""    → 4301 NONE       initial; show "開通"
 *   4302  → REQUESTED        in-flight; UI disabled
 *   4303  → SUCCEEDED        active; address fields locked
 *   4304  → FAILED           show error + "重新嘗試"
 *
 * Address fields freeze once the row hits 4302 or 4303. To change later
 * the seller has to contact PayUni — there's no in-app edit endpoint.
 */

import { authedFetch } from "@/lib/api/client";
import { ShippingMethod } from "@/lib/codemeta/enums";
import { toLogisticsName } from "@/lib/seller/certification-options";

/* ===== Code-meta enums (mirroring Flutter codemeta) ===== */

export const SHIPPING_TYPE = {
	HOME_DELIVERY: "3652", // 黑貓宅配
	CVS: "3651", // 7-11 店到店
} as const;
export type ShippingType = (typeof SHIPPING_TYPE)[keyof typeof SHIPPING_TYPE];

export const GOODS_TYPE = {
	AMBIENT: "3201", // 常溫
	CHILLED: "3202", // 冷藏
	FROZEN: "3203", // 冷凍
} as const;
export type GoodsType = (typeof GOODS_TYPE)[keyof typeof GOODS_TYPE];

export const PAYUNI_SUBMIT_STATUS = {
	NONE: "4301",
	REQUESTED: "4302",
	SUCCEEDED: "4303",
	FAILED: "4304",
} as const;
export type PayuniSubmitStatus =
	(typeof PAYUNI_SUBMIT_STATUS)[keyof typeof PAYUNI_SUBMIT_STATUS];

export const ONBOARDING_STATUS = {
	PENDING_REVIEW: "PENDING_REVIEW",
	ACTIVE: "ACTIVE",
	REJECTED: "REJECTED",
	CLOSED: "CLOSED",
	PERMANENTLY_CLOSED: "PERMANENTLY_CLOSED",
} as const;
export type OnboardingStatus =
	(typeof ONBOARDING_STATUS)[keyof typeof ONBOARDING_STATUS];

/* ===== Wire types ===== */

export interface ShippingOnboarding {
	onboardingId?: string;
	shippingType?: string; // ShippingType but tolerate strings from older payloads
	goodsType?: string; // GoodsType
	enabled?: string; // "true" / "false"
	payuniSubmitStatus?: string; // PayuniSubmitStatus
	providerMessage?: string;
	onboardingStatus?: string;
	senderName?: string;
	senderPhone?: string;
	senderCity?: string;
	senderDistrict?: string;
	senderAddressDetail?: string;
	senderPostalCode?: string;
	returnStoreCode?: string;
	returnStoreName?: string;
	productCategory?: string;
	createdAt?: string | number;
	updatedAt?: string | number;
}

export async function fetchShippingOnboardings(): Promise<
	ShippingOnboarding[]
> {
	const env = await authedFetch<ShippingOnboarding[]>(
		"/app/shipping_onboardings",
		{ throwOnHttpError: false },
	);
	return Array.isArray(env.data) ? env.data : [];
}

/* ===== 黑貓宅配 ===== */

export interface SubmitHomeDeliveryInput {
	productCategory?: string; // default "0001"
	senderName: string;
	senderPhone: string;
	senderCity: string;
	senderDistrict: string;
	senderAddressDetail: string;
	senderPostalCode: string;
}

export async function submitHomeDeliveryOnboarding(
	body: SubmitHomeDeliveryInput,
): Promise<ShippingOnboarding> {
	const env = await authedFetch<ShippingOnboarding>(
		"/app/shipping_onboardings/home_delivery",
		{
			method: "POST",
			json: { productCategory: "0001", ...body },
		},
	);
	return env.data ?? {};
}

/* ===== 7-11 店到店 ===== */

export interface SubmitCvsInput {
	senderName: string;
	senderPhone: string;
	returnStoreCode: string;
	returnStoreName?: string;
	goodsType: GoodsType;
}

export async function submitCvsOnboarding(
	body: SubmitCvsInput,
): Promise<ShippingOnboarding> {
	const env = await authedFetch<ShippingOnboarding>(
		"/app/shipping_onboardings/cvs",
		{ method: "POST", json: body },
	);
	return env.data ?? {};
}

/**
 * Flip a previously-onboarded carrier × goodsType row on/off without
 * resubmitting to PayUni. Only valid when `onboardingId` exists.
 */
export async function toggleOnboarding(
	onboardingId: string,
	enabled: boolean,
): Promise<ShippingOnboarding> {
	const env = await authedFetch<ShippingOnboarding>(
		`/app/shipping_onboardings/${encodeURIComponent(onboardingId)}/toggle`,
		{ method: "POST", json: { enabled } },
	);
	return env.data ?? {};
}

/* ===== Address validation (TGOS) ===== */

export interface AddressValidationResult {
	valid?: boolean;
	county?: string;
	district?: string;
	[key: string]: unknown;
}

export async function validateAddress(
	address: string,
): Promise<AddressValidationResult> {
	const qs = new URLSearchParams({ address });
	const env = await authedFetch<AddressValidationResult>(
		`/app/address/validate?${qs}`,
		{ throwOnHttpError: false },
	);
	return env.data ?? {};
}

/* ===== Address autocomplete (TGOS) ===== */

export interface AddressSuggestion {
	fullAddress: string;
	county: string;
	district: string;
	detail: string;
	lat?: number;
	lng?: number;
}

/**
 * Debounced address autocomplete — mirrors the seller App's
 * `TgosAddressService.getSuggestions`. County + district are required
 * on the backend; query needs to be at least 2 characters before
 * calling. The backend returns mixed-county results when the address
 * is ambiguous, so we re-filter client-side to the (county, district)
 * the user has selected (App does the same).
 */
export async function getAddressSuggestions(params: {
	county: string;
	district: string;
	query: string;
	limit?: number;
}): Promise<AddressSuggestion[]> {
	const qs = new URLSearchParams({
		county: params.county,
		district: params.district,
		query: params.query,
		limit: String(params.limit ?? 8),
	});
	try {
		const env = await authedFetch<AddressSuggestion[]>(
			`/app/address/suggestions?${qs}`,
			{ throwOnHttpError: false },
		);
		const list = Array.isArray(env.data) ? env.data : [];
		const norm = (s: string) => toLogisticsName(s).replace(/　/g, "");
		const wantCounty = norm(params.county);
		const wantDistrict = norm(params.district);
		return list.filter(
			(s) =>
				norm(s.county ?? "") === wantCounty &&
				norm(s.district ?? "") === wantDistrict,
		);
	} catch {
		return [];
	}
}

/**
 * 物流名稱表寫「台」、內政部與 TGOS 回「臺」：比對縣市、區名稱前把整個字串的臺換成台
 * （台西鄉、霧台鄉、台東市的「台」不在開頭）。
 */
export function addressNameMatches(a: string, b: string): boolean {
	return toLogisticsName(a) === toLogisticsName(b);
}

/**
 * TGOS 驗證結果與表單選的縣市、區一致。TGOS 沒回縣市或區時，那一項不比對。
 */
export function validationMatchesSelection(
	v: AddressValidationResult | undefined,
	city: string,
	district: string,
): boolean {
	if (!v?.valid) return false;
	if (v.county && !addressNameMatches(v.county, city)) return false;
	if (v.district && !addressNameMatches(v.district, district)) return false;
	return true;
}

/* ===== Helpers for UI state machines ===== */

/**
 * 這批 onboarding 裡「本頁畫不出來」的 shippingType（去重、保留原碼）。
 *
 * 🔴 本頁只寫死渲染兩張卡：黑貓宅配（3652）與 7-11 店到店（3651）。後端若回
 * `3654` 商家自送，或未來任何新值，那筆 onboarding 在 UI
 * 上**完全不存在** —— 賣家看不到、關不掉、也不知道自己開著它。那是「靜默丟棄」，
 * 與「猜成別的 provider」是同一條禁令的兩面（：不得憑空消失）。
 *
 * 這個函式不解決渲染問題（那要 provider slot registry），
 * 它只保證那些列**會被說出來**。
 */
export function unsupportedOnboardingTypes(
	rows: ShippingOnboarding[],
): string[] {
	const known = new Set<string>(Object.values(SHIPPING_TYPE));
	const out: string[] = [];
	for (const r of rows) {
		const type = (r.shippingType ?? "").trim();
		if (!type || known.has(type) || out.includes(type)) continue;
		out.push(type);
	}
	return out;
}

export function findOnboarding(
	rows: ShippingOnboarding[],
	shippingType: ShippingType,
	goodsType?: GoodsType,
): ShippingOnboarding | undefined {
	return rows.find(
		(r) =>
			r.shippingType === shippingType &&
			(goodsType === undefined || r.goodsType === goodsType),
	);
}

export function isLocked(o: ShippingOnboarding | undefined): boolean {
	if (!o) return false;
	return (
		o.payuniSubmitStatus === PAYUNI_SUBMIT_STATUS.REQUESTED ||
		o.payuniSubmitStatus === PAYUNI_SUBMIT_STATUS.SUCCEEDED
	);
}

export function isProcessing(o: ShippingOnboarding | undefined): boolean {
	return o?.payuniSubmitStatus === PAYUNI_SUBMIT_STATUS.REQUESTED;
}

export function isFailed(o: ShippingOnboarding | undefined): boolean {
	return o?.payuniSubmitStatus === PAYUNI_SUBMIT_STATUS.FAILED;
}

export function isApproved(o: ShippingOnboarding | undefined): boolean {
	return o?.payuniSubmitStatus === PAYUNI_SUBMIT_STATUS.SUCCEEDED;
}

export function isToggleable(o: ShippingOnboarding | undefined): boolean {
	return Boolean(o?.onboardingId) && isApproved(o);
}

export function isEnabled(o: ShippingOnboarding | undefined): boolean {
	return o?.enabled === "true" || o?.enabled === ("true" as unknown);
}

export function statusLabel(o: ShippingOnboarding | undefined): string {
	if (!o?.payuniSubmitStatus) return "尚未開通";
	switch (o.payuniSubmitStatus) {
		case PAYUNI_SUBMIT_STATUS.REQUESTED:
			return "開通處理中";
		case PAYUNI_SUBMIT_STATUS.SUCCEEDED:
			return isEnabled(o) ? "已啟用" : "已停用";
		case PAYUNI_SUBMIT_STATUS.FAILED:
			return "開通失敗，請重新嘗試";
		default:
			return "尚未開通";
	}
}

export type ShippingMethodOpenState =
	| "open" // 已開通且啟用，可選
	| "disabled" // 已開通但在物流設定停用
	| "not_opened" // 尚未開通
	| "unsupported"; // 此物流商不提供此溫層（如 7-11 冷藏）

/**
 * 判斷賣家是否已在「物流設定」開通某個 物流商 × 溫層，給商品上架的可配送方案
 * picker 用：未開通的方案要顯示「尚未開通」而不是「此尺寸 / 重量不可配送」。
 *
 * - 黑貓宅急便（TAQBIN）是單一 home_delivery onboarding，審核通過即涵蓋全部溫層，
 *   物流設定頁也沒有 per-溫層 停用開關，故 approved 即視為 open。
 * - 7-11 店到店（SEVEN_ELEVEN）為 per-溫層 onboarding，且只支援常溫 / 冷凍；
 *   冷藏無法開通 → "unsupported"，其餘溫層依 approved + enabled 判斷。
 *
 * 未知物流商回 "open"，讓呼叫端維持既有（尺寸 / 重量）文案、不誤報未開通。
 */
export function shippingMethodOpenState(
	rows: ShippingOnboarding[],
	method: string,
	temperature: string,
): ShippingMethodOpenState {
	if (method === ShippingMethod.TAQBIN) {
		const row = findOnboarding(rows, SHIPPING_TYPE.HOME_DELIVERY);
		return isApproved(row) ? "open" : "not_opened";
	}
	if (method === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP) {
		if (temperature === GOODS_TYPE.CHILLED) return "unsupported";
		const row = findOnboarding(
			rows,
			SHIPPING_TYPE.CVS,
			temperature as GoodsType,
		);
		if (!isApproved(row)) return "not_opened";
		return isEnabled(row) ? "open" : "disabled";
	}
	return "open";
}

export function goodsTypeLabel(g: GoodsType | string | undefined): string {
	switch (g) {
		case GOODS_TYPE.AMBIENT:
			return "常溫";
		case GOODS_TYPE.CHILLED:
			return "冷藏";
		case GOODS_TYPE.FROZEN:
			return "冷凍";
		default:
			return String(g ?? "");
	}
}
