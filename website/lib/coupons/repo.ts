/**
 * Coupons HTTP repository (buyer-facing).
 *
 *   GET    /app/coupons/available       — claimable coupons
 *   GET    /app/coupons                 — buyer's claimed coupons (status filter)
 *   POST   /app/coupons/{id}/claim      — claim
 */

import { authedFetch } from "../api/client";

/**
 * OpenAPI coupon types (handler.CouponInstanceResponse / CouponTemplateResponse):
 *   type:   "1" 現金券 (cash, fixed amount off)
 *           "2" 折扣券 (percent off, capped by maxDiscountAmount)
 *   status (instance only): "0" unused, "1" locked, "2" used, "3" expired,
 *           "4" revoked, "5" void
 */
export type CouponKind = "cash" | "percent";

export interface CouponItem {
	id: number; // legacy numeric id (falls back to templateId hash)
	instanceId?: string;
	templateId?: string;
	code?: string;
	name: string;
	title?: string;
	description?: string;
	/** Legacy alias kept for `app/account/coupons` UI: "amount" | "percent". */
	type?: "amount" | "percent" | string;
	/** Normalized kind derived from OpenAPI type code. */
	kind?: CouponKind;
	value?: number; // amount (cash) or percent (% off)
	discountAmount?: number;
	discountPercent?: number;
	maxDiscountAmount?: number;
	min_spend?: number;
	thresholdAmount?: number;
	expires_at?: number;
	starts_at?: number;
	endAt?: number | string;
	startAt?: number | string;
	usage_limit?: number;
	usage_count?: number;
	seller_id?: number;
	seller_name?: string;
	sellerName?: string;
	sellerAvatarUrl?: string;
	/** 適用賣家的 store surrogate（空 = 全平台券）。wire `applicableStoreId`。 */
	applicableStoreId?: string;
	/**
	 * 適用賣家店面 canonical 路徑短碼（`sellers.store_code`）。wire `storeCode`。
	 *
	 * `applicableStoreId` 是內部代理鍵，賣家頁 canonical 網址走 store_code。
	 * coupon-service 為了 sellerName 本來就會呼叫 batchGetSellerMeta，值同源；
	 * 全平台券或聚合失敗時為空（→ undefined，不連結）。
	 */
	storeCode?: string;
	/** codemeta CouponTargetType: 1 all, 2 member, 3 designated, 4 compensation. */
	targetType?: string;
	/** codemeta IssuerType: 1 platform/admin, 2 seller. */
	issuerType?: string;
	claimRequired?: boolean;
	claimed?: boolean;
	scope?: "all" | "seller" | "category";
}

export interface MyCouponItem extends CouponItem {
	status: "unused" | "used" | "expired" | string;
	claimed_at?: number;
	used_at?: number;
}

/** Map OpenAPI numeric type → semantic kind. */
function toKind(raw: unknown): CouponKind | undefined {
	const s = String(raw ?? "").trim();
	if (s === "1" || s === "amount" || s === "cash") return "cash";
	if (s === "2" || s === "percent") return "percent";
	return undefined;
}

/** Map OpenAPI numeric instance status → legacy string status. */
function toStatusString(raw: unknown): string {
	const s = String(raw ?? "").trim();
	switch (s) {
		case "0":
			return "unused";
		case "1":
			return "locked";
		case "2":
			return "used";
		case "3":
			return "expired";
		case "4":
			return "revoked";
		case "5":
			return "void";
		default:
			return s || "unused";
	}
}

function toEpoch(raw: unknown): number | undefined {
	if (raw == null || raw === "") return undefined;
	if (typeof raw === "number") {
		return raw < 1e12 ? raw * 1000 : raw;
	}
	const s = String(raw);
	// Numeric string (seconds or ms)
	if (/^\d+$/.test(s)) {
		const n = Number(s);
		return n < 1e12 ? n * 1000 : n;
	}
	const t = Date.parse(s);
	return Number.isNaN(t) ? undefined : t;
}

function toBoolean(raw: unknown): boolean | undefined {
	if (raw == null || raw === "") return undefined;
	if (typeof raw === "boolean") return raw;
	if (typeof raw === "number") return raw !== 0;
	const s = String(raw).trim().toLowerCase();
	if (["1", "true", "yes", "y"].includes(s)) return true;
	if (["0", "false", "no", "n"].includes(s)) return false;
	return undefined;
}

export function normalizeCoupon(input: Partial<CouponItem>): CouponItem {
	const raw = input as Partial<CouponItem> & Record<string, unknown>;
	const kind =
		toKind(raw.kind) ??
		toKind(raw.type) ??
		(raw.discountPercent ? "percent" : "cash");
	const legacyType = kind === "percent" ? "percent" : "amount";
	const id =
		Number(raw.id ?? raw.instanceId ?? raw.templateId ?? 0) ||
		// Stringy ids — hash to a stable positive int so React keys stay unique.
		Math.abs(
			(String(raw.instanceId ?? raw.templateId ?? "")
				.split("")
				.reduce((h, c) => (h * 31 + c.charCodeAt(0)) | 0, 0) || 0) >>> 0,
		);
	const numericValue = Number(
		raw.value ??
			(kind === "percent" ? raw.discountPercent : raw.discountAmount) ??
			0,
	);
	return {
		...raw,
		id,
		instanceId: raw.instanceId != null ? String(raw.instanceId) : undefined,
		templateId: raw.templateId != null ? String(raw.templateId) : undefined,
		name: String(raw.name ?? raw.title ?? "Hawkr 優惠券"),
		title: raw.title != null ? String(raw.title) : undefined,
		type: legacyType,
		kind,
		value: Number.isFinite(numericValue) ? numericValue : 0,
		discountAmount:
			raw.discountAmount != null ? Number(raw.discountAmount) : undefined,
		discountPercent:
			raw.discountPercent != null ? Number(raw.discountPercent) : undefined,
		maxDiscountAmount:
			raw.maxDiscountAmount != null ? Number(raw.maxDiscountAmount) : undefined,
		min_spend: Number(raw.min_spend ?? raw.thresholdAmount ?? 0),
		thresholdAmount:
			raw.thresholdAmount != null ? Number(raw.thresholdAmount) : undefined,
		starts_at: toEpoch(raw.starts_at ?? raw.startAt),
		expires_at: toEpoch(raw.expires_at ?? raw.endAt),
		sellerName:
			raw.sellerName != null ? String(raw.sellerName) : raw.seller_name,
		applicableStoreId:
			raw.applicableStoreId != null ? String(raw.applicableStoreId) : undefined,
		storeCode:
			typeof raw.storeCode === "string"
				? raw.storeCode.trim() || undefined
				: undefined,
		targetType:
			raw.targetType != null
				? String(raw.targetType)
				: raw.target_type != null
					? String(raw.target_type)
					: undefined,
		issuerType:
			raw.issuerType != null
				? String(raw.issuerType)
				: raw.issuer_type != null
					? String(raw.issuer_type)
					: undefined,
		claimRequired: toBoolean(raw.claimRequired ?? raw.claim_required),
		claimed: toBoolean(raw.claimed ?? raw.is_claimed),
	};
}

export async function fetchAvailableCoupons({
	page = 1,
	pageSize = 20,
	storeId,
	orderAmount,
}: {
	page?: number;
	pageSize?: number;
	/** Filter to coupons applicable to this single seller (or empty for platform). */
	storeId?: string;
	/** Current order total — backend computes `estimatedDiscount` against this. */
	orderAmount?: number;
} = {}): Promise<{
	records: CartAvailableCoupon[];
	total: number;
}> {
	// Backend currently requires BOTH `store_id` AND `order_amount` and
	// returns 400 "store_id and order_amount are required" when either is
	// missing. OpenAPI marks them optional ("空 = 全部") but the runtime
	// disagrees — so this endpoint only works in the cart-context per-seller
	// picker. Until backend exposes a real platform-wide listing, the
	// account-level 全站可領 hub short-circuits to empty instead of
	// triggering a console 400.
	if (!storeId) {
		return { records: [], total: 0 };
	}
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
		store_id: storeId,
	});
	if (orderAmount != null && orderAmount > 0) {
		qs.set("order_amount", String(Math.round(orderAmount)));
	}
	const env = await authedFetch<unknown>(`/app/coupons/available?${qs}`);
	const data = env.data;
	const rawRecords: unknown[] = Array.isArray(data)
		? data
		: ((data as { records?: unknown[]; rows?: unknown[]; list?: unknown[] })
				?.records ??
			(data as { rows?: unknown[] })?.rows ??
			(data as { list?: unknown[] })?.list ??
			[]);
	const total = Array.isArray(data)
		? rawRecords.length
		: Number((data as { total?: number })?.total ?? rawRecords.length);
	const records: CartAvailableCoupon[] = rawRecords.map((raw) => {
		// Cart-context shape: { template: {...}, estimatedDiscount: "..." }
		// Fallback: the record itself is the template (older API revisions).
		const r = raw as Record<string, unknown>;
		const template = (r.template ?? r) as Partial<CouponItem>;
		const normalized = normalizeCoupon(template);
		const estimated = Number(
			r.estimatedDiscount ?? r.estimated_discount ?? r.discount ?? 0,
		);
		return {
			...normalized,
			estimatedDiscount: Number.isFinite(estimated) ? estimated : 0,
		};
	});
	return { records, total };
}

/** Coupon with the server-computed discount for the current cart subtotal. */
export interface CartAvailableCoupon extends CouponItem {
	/** NT$ discount the backend pre-computes against `order_amount`. */
	estimatedDiscount: number;
}

/** Map legacy status filter names to the OpenAPI numeric codes the backend expects. */
function statusFilterToCode(s?: string): string | undefined {
	if (!s) return undefined;
	switch (s) {
		case "unused":
			return "0";
		case "locked":
			return "1";
		case "used":
			return "2";
		case "expired":
			return "3";
		case "revoked":
			return "4";
		case "void":
			return "5";
		default:
			// Already a numeric code, or unknown — pass through.
			return s;
	}
}

interface MyCouponsWire {
	records?: MyCouponItem[];
	rows?: MyCouponItem[];
	list?: MyCouponItem[];
	usedRecords?: MyCouponItem[];
	used_records?: MyCouponItem[];
}

function pickMyCouponRecords(
	data: MyCouponItem[] | MyCouponsWire | undefined,
	statusCode?: string,
): MyCouponItem[] {
	if (Array.isArray(data)) return data;
	if (!data) return [];
	if (statusCode === "2") {
		return (
			data.usedRecords ??
			data.used_records ??
			data.records ??
			data.rows ??
			data.list ??
			[]
		);
	}
	return data.records ?? data.rows ?? data.list ?? [];
}

export async function fetchMyCoupons({
	status,
	page = 1,
	pageSize = 20,
}: {
	status?: string;
	page?: number;
	pageSize?: number;
} = {}): Promise<MyCouponItem[]> {
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
	});
	const code = statusFilterToCode(status);
	if (code) qs.set("status", code);
	const env = await authedFetch<MyCouponItem[] | MyCouponsWire>(
		`/app/coupons?${qs}`,
	);
	const records = pickMyCouponRecords(env.data, code);
	return records.map((item) => {
		const normalized = normalizeCoupon(item) as MyCouponItem;
		normalized.status = toStatusString(
			(item as unknown as Record<string, unknown>).status ?? code,
		);
		return normalized;
	});
}

export async function claimCoupon(id: number | string): Promise<boolean> {
	const env = await authedFetch<{ success?: boolean }>(
		`/app/coupons/${encodeURIComponent(String(id))}/claim`,
		{ method: "POST" },
	);
	return env.data?.success !== false && env.code === 0;
}
