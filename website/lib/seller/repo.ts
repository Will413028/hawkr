/**
 * Seller-side repos.
 *
 * 店家資料走 Connect（`SellerAppService`）：GetMyStore、UpdateMyStore、CompleteMyStoreOnboarding、
 * ApplyStore、CloseMyStore；認證與 PayUni 在 `kyc.ts`、`payuni.ts`。儀表板在 `dashboard.ts`（`GetMyDashboard`），措施與申覆在 `measures.ts`。其餘端點仍是 REST（全部 authed）：
 *   GET    /app/sellers/me/customers                customers list (CRM)
 *   GET    /app/coupon_templates                    my coupon templates
 *   POST   /app/coupon_templates                    create
 *   GET    /app/coupon_templates/{id}               detail
 *   POST   /app/coupon_templates/{id}/revoke        revoke
 *   POST   /app/sellers/posts                       create post
 *   GET    /app/sellers/posts/{id}                  detail (owner read, any publish state)
 *   PUT    /app/sellers/posts/{id}                  edit
 *   DELETE /app/sellers/posts/{id}                  delete
 *   POST   /app/sellers/posts/{id}/publish          publish (4402)
 *   POST   /app/sellers/posts/{id}/unpublish        unpublish (4403)
 *
 * Product endpoints under `/app/products` are shared with seller scope —
 * see `lib/products/seller-repo.ts`.
 */

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
	type GetMyStoreOpeningResponse,
	type GetMyStoreResponse,
	type Store,
	StoreSchema,
	type UpdateMyStoreRequest,
	UpdateMyStoreRequestSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { ApiClientError, type ApiEnvelope, authedFetch } from "../api/client";
import type { DelegatedMutationMeta, RepoFetcher } from "../api/delegated";
import type { MyPostDetail, PostListItem } from "../api/openapi-types";
import { normalizePostDetail } from "../api/posts";
import {
	PostPublishStatus,
	type PostPublishStatusValue,
} from "../codemeta/enums";
import { sellerAppClient } from "../connect/seller";
import type { AgreementAcceptanceInput } from "./agreements";

/* ===== Store (Connect) ===== */

/** 店面的地址：鄉鎮市區代碼（8 碼）與明細；明細可以是空字串。 */
export interface StoreAddress {
	townCode: string;
	addressDetail: string;
}

/**
 * UpdateMyStore 的一次修改：欄位直接取自 proto 的 `Store`，`etag` 是上一次讀到（或上一次寫入回應）的店家
 * 版本，每次寫入都必填。出現的欄位才會列進 update_mask：沒出現就是不動它，`address: null` 是清掉地址。
 */
export type StoreUpdate = Pick<
	MessageInitShape<typeof StoreSchema>,
	| "storeName"
	| "description"
	| "contactPhone"
	| "contactName"
	| "contactMobile"
	| "supportEmail"
	| "prize"
	| "websiteUrl"
	| "logoStagedToken"
	| "coverStagedToken"
> & {
	etag: string;
	address?: StoreAddress | null;
};

/**
 * 店面寫入的回執。`etag` 是這一次寫入之後的店家版本，同一個畫面的下一次寫入要用它：背景 refetch
 * 回來的版本不能直接抄進表單（會把別人在這段期間的修改當成自己的版本）。代行路徑回的是代行
 * 端點的 version，賣家路徑另外帶回整個 `store`。
 */
export interface StorefrontWriteResult {
	etag?: string;
	store?: Store;
}

/**
 * 店面資料層的介面。賣家路徑讀寫 `SellerAppService`；代行的店面 DTO 不是同一個形狀（12 欄
 * allow-list），代行實作在自己的邊界把 DTO 轉成 `GetMyStoreResponse`，版本放在 `store.etag`。
 */
export interface SellerStorefrontRepo {
	fetchMyStore(): Promise<GetMyStoreResponse>;
	updateMyStore(
		update: StoreUpdate,
		meta?: DelegatedMutationMeta,
	): Promise<StorefrontWriteResult>;
	updateStoreLogo(
		etag: string,
		stagedToken: string,
		meta?: DelegatedMutationMeta,
	): Promise<StorefrontWriteResult>;
	updateStoreCover(
		etag: string,
		stagedToken: string,
		meta?: DelegatedMutationMeta,
	): Promise<StorefrontWriteResult>;
}

/** StoreUpdate 對應的請求：update_mask 由出現的欄位推導，沒有 etag 的寫入不組出請求。 */
export function buildUpdateMyStoreRequest(
	update: StoreUpdate,
): UpdateMyStoreRequest {
	const { etag, address, ...changes } = update;
	if (!etag) {
		throw new Error("UpdateMyStore 需要店家的 etag");
	}
	const fields: MessageInitShape<typeof StoreSchema> = { etag };
	const paths: string[] = [];
	for (const key of Object.keys(changes) as (keyof typeof changes)[]) {
		const value = changes[key];
		if (value === undefined) continue;
		Object.assign(fields, { [key]: value });
		paths.push(StoreSchema.field[key].name);
	}
	if (address !== undefined) {
		if (address !== null) fields.address = address;
		paths.push("address");
	}
	return create(UpdateMyStoreRequestSchema, {
		store: fields,
		updateMask: fieldMask(StoreSchema, paths),
	});
}

type SellerClient = ReturnType<typeof sellerAppClient>;

export function createSellerStorefrontRepo(
	client: () => SellerClient = sellerAppClient,
): SellerStorefrontRepo {
	const repo: SellerStorefrontRepo = {
		/**
		 * GetMyStore — 純 fetcher。去重與快取交給呼叫端的 TanStack query（`useSellerSelf`，
		 * identity-scoped key）：navbar、seller-center shell、`useCertGate`、各頁同時掛載時
		 * TanStack 自動共用一次請求。Mutation 後由呼叫端 `useInvalidateSellerSelf()` 失效該 query。
		 */
		fetchMyStore: () => client().getMyStore({}),

		async updateMyStore(update) {
			const res = await client().updateMyStore(
				buildUpdateMyStoreRequest(update),
			);
			return { etag: res.store?.etag, store: res.store };
		},

		updateStoreLogo: (etag, stagedToken) =>
			repo.updateMyStore({ etag, logoStagedToken: stagedToken }),

		updateStoreCover: (etag, stagedToken) =>
			repo.updateMyStore({ etag, coverStagedToken: stagedToken }),
	};
	return repo;
}

/** 賣家路徑的預設實例；下面的具名匯出全部經由它。 */
export const sellerStorefrontRepo = createSellerStorefrontRepo();

export const fetchMyStore: SellerStorefrontRepo["fetchMyStore"] = () =>
	sellerStorefrontRepo.fetchMyStore();

export const updateMyStore: SellerStorefrontRepo["updateMyStore"] = (
	update,
	meta,
) => sellerStorefrontRepo.updateMyStore(update, meta);

/** 送出食品業者登錄供平台核對；已有待審時由新的取代，目前核准的那一筆照常有效。 */
export async function submitFoodRegistration(
	registrationNumber: string,
	acceptedAgreements: readonly AgreementAcceptanceInput[],
): Promise<Store> {
	const res = await sellerAppClient().submitFoodRegistration({
		registrationNumber: registrationNumber.trim(),
		acceptedAgreements: acceptedAgreements.map((a) => ({
			kind: a.kind,
			version: a.version,
		})),
	});
	if (!res.store) throw new Error("SubmitFoodRegistration 的回應沒有店家");
	return res.store;
}

/** 撤回待審的登錄，保留目前核准的那一筆。 */
export async function withdrawFoodRegistration(): Promise<Store> {
	const res = await sellerAppClient().withdrawFoodRegistration({});
	if (!res.store) throw new Error("WithdrawFoodRegistration 的回應沒有店家");
	return res.store;
}

/** 清除登錄：結束核准的那一筆並撤回待審的，立即生效。 */
export async function clearFoodRegistration(): Promise<Store> {
	const res = await sellerAppClient().clearFoodRegistration({});
	if (!res.store) throw new Error("ClearFoodRegistration 的回應沒有店家");
	return res.store;
}

export async function completeOnboarding(etag: string): Promise<Store> {
	const res = await sellerAppClient().completeMyStoreOnboarding({ etag });
	if (!res.store) throw new Error("CompleteMyStoreOnboarding 的回應沒有店家");
	return res.store;
}

/**
 * ApplyStore 的輸入；地址不設表示沒有填，可以只填鄉鎮市區（明細是空字串）。
 * `acceptedAgreements` 是賣家對 SELLER_TERMS 的同意，版本取自 GetSellerAgreements；帳號已有店家時走更新，不用它。
 */
export interface ApplyStoreInput {
	storeName: string;
	acceptedAgreements: readonly AgreementAcceptanceInput[];
	description?: string;
	contactPhone?: string;
	address?: StoreAddress;
}

/** 申請開店：建立店家並授予賣家角色。已經有店家時回 already_exists。 */
export async function applyStore(input: ApplyStoreInput): Promise<Store> {
	const res = await sellerAppClient().applyStore({
		storeName: input.storeName,
		description: input.description ?? "",
		contactPhone: input.contactPhone ?? "",
		address: input.address
			? {
					townCode: input.address.townCode,
					addressDetail: input.address.addressDetail,
				}
			: undefined,
		acceptedAgreements: input.acceptedAgreements.map((a) => ({
			kind: a.kind,
			version: a.version,
		})),
	});
	if (!res.store) throw new Error("ApplyStore 的回應沒有店家");
	return res.store;
}

/** 帳號此刻能不能申請開店與依序的原因；不要求賣家角色，還沒有店家也可以呼叫。 */
export async function fetchStoreOpening(): Promise<GetMyStoreOpeningResponse> {
	return sellerAppClient().getMyStoreOpening({});
}

/** 關閉店家：帳號與買家身分留著，賣家角色移除。 */
export async function closeMyStore(): Promise<Store> {
	const res = await sellerAppClient().closeMyStore({});
	if (!res.store) throw new Error("CloseMyStore 的回應沒有店家");
	return res.store;
}

/* ===== Customers ===== */

export interface SellerCustomer {
	/** Buyer ID (TSID, kept as string) — matches the `buyerId` path param on
	 * the detail/messages routes and `buyerAccountIds` on coupon targeting. */
	id: string;
	name: string;
	avatar?: string;
	phone?: string;
	orderCount?: number;
	totalSpent?: number;
	tags?: string[];
	/** CRM classification — "normal" | "vip" | "blocklist". Defaults to
	 * "normal" when the backend omits it. */
	customerType?: SellerCustomerType;
}

/** Minimal buyer identity returned by the seller CRM lookup endpoint. */
export interface SellerCustomerLookup {
	id: string;
	name: string;
	avatar?: string;
	phone?: string;
}

/**
 * Resolve one buyer's site identity for seller-facing surfaces.
 *
 * Order snapshots can legitimately predate a profile update, and the order
 * service occasionally degrades `buyerNickname` to an empty string. The CRM
 * lookup remains the authoritative source for the buyer's current nickname;
 * it must not be replaced with the shipping recipient, who may be a different
 * person.
 */
export async function fetchSellerCustomerLookup(
	buyerId: string | number,
	options: { signal?: AbortSignal } = {},
): Promise<SellerCustomerLookup | null> {
	const qs = new URLSearchParams({ buyer_id: String(buyerId) });
	const env = await authedFetch<{
		uid?: string | number;
		buyerName?: string;
		avatarKey?: string;
		avatarUrl?: string;
		phone?: string;
	} | null>(`/app/sellers/me/customers/lookup?${qs}`, {
		signal: options.signal,
	});
	const raw = env.data;
	if (!raw) return null;
	return {
		id: String(raw.uid ?? buyerId),
		name: raw.buyerName ?? "",
		// §231: prefer bare objectKey (SSOT); <Avatar> resolves key-or-url.
		avatar: raw.avatarKey ?? raw.avatarUrl,
		phone: raw.phone,
	};
}

/** CRM customer classification (wire `customerType`)。 */
export type SellerCustomerType = "normal" | "vip" | "blocklist";

/** A CRM tag — `{ id, name }`. `id` is a TSID string, required to remove a
 * tag from a customer (`DELETE …/tags/{tagId}`); newly-typed free-text tags
 * that the backend hasn't assigned an id to yet surface with `id: ""`. */
export interface CustomerTag {
	id: string;
	name: string;
}

/** Raw row from `GET /app/sellers/me/customers` → `data.list[]`, and the same
 * shape from the detail endpoint (`/customers/{buyerId}`). Both are camelCase
 * — the list was flipped from snake_case together with the seller App's
 * parser. `id` here is the crm row PK, NOT the buyer: key off `buyerId` for
 * anything that identifies the customer (backend 2026-07-26 rename; see
 * gateway `seller_crm.go` — `buyerId` is a string, never coerce to Number). */
interface RawSellerCustomer {
	id?: number;
	/** The SELLER's store surrogate — never the customer. */
	accountId?: number;
	buyerId?: string;
	nickname?: string;
	/** §231 flat-pair: `avatarKey` (bare key, SSOT) + `avatarUrl` (Gateway URL). */
	avatarKey?: string;
	avatarUrl?: string;
	phone?: string;
	orderCount?: number;
	totalSpent?: number | string;
	customerType?: string;
	tags?: Array<{ id?: number; name?: string } | string>;
}

function normalizeSellerCustomer(raw: RawSellerCustomer): SellerCustomer {
	return {
		// The customer key the detail page, messages thread and coupon targeting
		// all use. Deliberately no fallback to `accountId` (the seller's) or
		// `id` (the crm row PK): both would silently resolve to somebody else.
		// Kept as a string — `buyerId` is a TSID and exceeds
		// Number.MAX_SAFE_INTEGER, so Number(...) would silently corrupt it.
		id: raw.buyerId ?? "",
		name: raw.nickname ?? "",
		// §231: prefer bare objectKey (SSOT); <Avatar> resolves key-or-url.
		avatar: raw.avatarKey ?? raw.avatarUrl,
		phone: raw.phone,
		orderCount: raw.orderCount,
		totalSpent: Number(raw.totalSpent ?? 0),
		tags: (raw.tags ?? [])
			.map((t) => (typeof t === "string" ? t : (t?.name ?? "")))
			.filter(Boolean),
		customerType: normalizeCustomerType(raw.customerType),
	};
}

/** Coerce the backend's loosely-typed `customerType` to our enum. */
function normalizeCustomerType(v: unknown): SellerCustomerType {
	const s = String(v ?? "").toLowerCase();
	return s === "vip" || s === "blocklist" ? s : "normal";
}

export type FetchSellerCustomersParams = {
	page?: number;
	pageSize?: number;
	/** Free-text search (matches name / phone per backend). */
	keyword?: string;
	/** Filter to customers carrying a specific CRM tag. */
	tagId?: string | number;
	/** Backend `customer_type` query enum — "" (all) | normal | vip | blocklist. */
	customerType?: SellerCustomerType;
	/** Backend sort key documented on the CRM list（query 值，維持 snake_case —
	 * 10 檔 §2：URL/Query snake、JSON body camel）。最近購買 is derived
	 * client-side from `/app/orders?view=seller&buyer_id=...`. */
	sortBy?: "total_spent" | "order_count";
	sortOrder?: "asc" | "desc";
};

export interface SellerCustomersPage {
	records: SellerCustomer[];
	total: number;
	page: number;
	pageSize: number;
}

export async function fetchSellerCustomersPage({
	page = 1,
	pageSize = 20,
	keyword,
	tagId,
	customerType,
	sortBy,
	sortOrder = "desc",
}: FetchSellerCustomersParams = {}): Promise<SellerCustomersPage> {
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
		sort_order: sortOrder,
	});
	if (keyword?.trim()) qs.set("keyword", keyword.trim());
	if (tagId !== undefined && tagId !== "") qs.set("tag_id", String(tagId));
	if (customerType) qs.set("customer_type", customerType);
	if (sortBy) qs.set("sort_by", sortBy);
	const env = await authedFetch<
		| RawSellerCustomer[]
		| {
				list?: RawSellerCustomer[];
				records?: RawSellerCustomer[];
				total?: number | string;
				page?: number | string;
				pageSize?: number | string;
				page_size?: number | string;
		  }
	>(`/app/sellers/me/customers?${qs}`);
	// Backend wraps the rows under `data.list` (per OpenAPI); keep `records`
	// and bare-array fallbacks for resilience to shape drift.
	const rows = Array.isArray(env.data)
		? env.data
		: (env.data?.list ?? env.data?.records ?? []);
	const records = rows.map(normalizeSellerCustomer);
	return {
		records,
		total: Array.isArray(env.data)
			? records.length
			: Number(env.data?.total ?? records.length),
		page: Array.isArray(env.data) ? page : Number(env.data?.page ?? page),
		// `GetCustomerList` 的 data 只有 `total` + `list`（seller_crm.go:491）
		// —— 它從來沒回過頁大小，所以這裡一律用送出去的參數。
		pageSize: Array.isArray(env.data)
			? pageSize
			: Number(env.data?.pageSize ?? pageSize),
	};
}

export async function fetchSellerCustomers(
	params: FetchSellerCustomersParams = {},
): Promise<SellerCustomer[]> {
	const page = await fetchSellerCustomersPage(params);
	return page.records;
}

/** Single buyer's CRM record — `GET /app/sellers/me/customers/{buyerId}`.
 * Superset of the list row (adds address / email / couponCount). */
export interface SellerCustomerDetail extends SellerCustomer {
	email?: string;
	address?: string;
	couponCount?: number;
}

export async function fetchSellerCustomerDetail(
	buyerId: string | number,
): Promise<SellerCustomerDetail | null> {
	const env = await authedFetch<
		| (RawSellerCustomer & {
				email?: string;
				address?: string;
				couponCount?: number;
		  })
		| null
	>(`/app/sellers/me/customers/${encodeURIComponent(String(buyerId))}`);
	const raw = env.data;
	if (!raw) return null;
	return {
		...normalizeSellerCustomer(raw),
		email: raw.email,
		address: raw.address,
		couponCount: raw.couponCount,
	};
}

/* ===== CRM tags & classification =====
 *
 * Wire shapes verified against the seller app's `CustomerRepository`
 * (Apps/hawkr_frontend) against the live backend:
 *   GET    /app/sellers/me/customers/{id}/tags        list a buyer's tags
 *   POST   /app/sellers/me/customers/{id}/tags        body { tagNames: string[] }
 *   DELETE /app/sellers/me/customers/{id}/tags/{tagId}
 *   GET    /app/sellers/me/tags                        seller's whole tag pool
 *   POST   /app/sellers/me/tags                        body { name } → tag
 *   PATCH  /app/sellers/me/tags/{tagId}                body { name }
 *   DELETE /app/sellers/me/tags/{tagId}
 *   PATCH  /app/sellers/me/customers/{id}              body { customerType }
 * Each tag row arrives as `{ id, name }` (`id` a TSID string); the list may
 * be a bare array or wrapped under data.list / data.tags. */

interface RawTag {
	id?: string;
	name?: string;
}

function normalizeTag(raw: RawTag): CustomerTag {
	return { id: raw.id ?? "", name: String(raw.name ?? "") };
}

/** Pull a tag array out of the backend's `data` (bare array | list | tags). */
function tagRows(data: unknown): RawTag[] {
	if (Array.isArray(data)) return data as RawTag[];
	const obj = (data ?? {}) as { list?: RawTag[]; tags?: RawTag[] };
	return obj.list ?? obj.tags ?? [];
}

/** GET /app/sellers/me/customers/{buyerId}/tags — tags on one buyer. */
export async function fetchCustomerTags(
	buyerId: string | number,
): Promise<CustomerTag[]> {
	const env = await authedFetch<unknown>(
		`/app/sellers/me/customers/${encodeURIComponent(String(buyerId))}/tags`,
	);
	return tagRows(env.data)
		.map(normalizeTag)
		.filter((t) => t.name);
}

/** POST /app/sellers/me/customers/{buyerId}/tags — attach one or more tags
 * (by name; the backend creates any that don't exist yet). */
export async function addCustomerTags(
	buyerId: string | number,
	tagNames: string[],
): Promise<void> {
	const names = tagNames.map((n) => n.trim()).filter(Boolean);
	if (names.length === 0) return;
	await authedFetch(
		`/app/sellers/me/customers/${encodeURIComponent(String(buyerId))}/tags`,
		{ method: "POST", json: { tagNames: names } },
	);
}

/** DELETE /app/sellers/me/customers/{buyerId}/tags/{tagId}. */
export async function removeCustomerTag(
	buyerId: string | number,
	tagId: string,
): Promise<void> {
	await authedFetch(
		`/app/sellers/me/customers/${encodeURIComponent(String(buyerId))}/tags/${tagId}`,
		{ method: "DELETE" },
	);
}

/** GET /app/sellers/me/tags — the seller's full tag pool (for suggestions
 * and tag-based segmentation). */
export async function fetchSellerTags(): Promise<CustomerTag[]> {
	const env = await authedFetch<unknown>("/app/sellers/me/tags");
	return tagRows(env.data)
		.map(normalizeTag)
		.filter((t) => t.name);
}

/** POST /app/sellers/me/tags — create a reusable seller tag. */
export async function createSellerTag(name: string): Promise<CustomerTag> {
	const env = await authedFetch<RawTag>("/app/sellers/me/tags", {
		method: "POST",
		json: { name: name.trim() },
	});
	return normalizeTag(env.data ?? { name: name.trim() });
}

/** PATCH /app/sellers/me/tags/{tagId} — rename a seller tag. */
export async function updateSellerTag(
	tagId: string,
	name: string,
): Promise<void> {
	await authedFetch(`/app/sellers/me/tags/${tagId}`, {
		method: "PATCH",
		json: { name: name.trim() },
	});
}

/** DELETE /app/sellers/me/tags/{tagId} — remove a seller tag entirely. */
export async function deleteSellerTag(tagId: string): Promise<void> {
	await authedFetch(`/app/sellers/me/tags/${tagId}`, { method: "DELETE" });
}

/** PATCH /app/sellers/me/customers/{buyerId} — set VIP / blacklist / normal. */
export async function updateCustomerType(
	buyerId: string | number,
	customerType: SellerCustomerType,
): Promise<void> {
	await authedFetch(
		`/app/sellers/me/customers/${encodeURIComponent(String(buyerId))}`,
		{ method: "PATCH", json: { customerType } },
	);
}

// NOTE: A buyer's purchase history is fetched via the proven seller orders
// pipeline — `fetchOrders({ view: "seller", buyerId })` in
// `lib/orders/repo.ts` — NOT the CRM `/customers/{buyerId}/orders`
// sub-endpoint, which returns an empty `records` array even when the
// customer aggregate reports orders (backend gap, observed 2026-05).

/* ===== Seller coupons =====
 *
 * Backend (POST /app/coupon_templates) wire format per OpenAPI:
 *
 *   {
 *     title: string,
 *     type: "1" | "2",               // 1=現金 / 2=折扣
 *     targetType: "1" | "2" | "3",   // 1=全體 / 2=賣家會員 / 3=指定買家
 *     startAt: string, endAt: string,// ISO 8601 UTC, YYYY-MM-DDTHH:mm:ssZ (REQUIRED)
 *     thresholdAmount?: string,
 *     discountAmount?: string,       // type=1 only
 *     discountPercent?: string,      // type=2 only
 *     maxDiscountAmount?: string,    // type=2 cap (avoids 100%-off on big carts)
 *     buyerAccountIds?: string[],    // targetType=3 only
 *   }
 *
 * Web kept legacy `"amount"|"percent"` strings for `type` for compat —
 * they're translated to the numeric codes inside `createSellerCoupon`.
 */

/** Semantic coupon kind used by the web form (UI-friendly). Repo maps to
 * the backend's numeric `type` enum at send time. */
export type SellerCouponKind = "amount" | "percent";

/** Semantic target audience used by the web form. Repo maps to backend's
 * numeric `targetType` enum at send time. */
export type SellerCouponTarget = "all" | "members" | "specific";

export interface SellerCoupon {
	id: string;
	templateId?: string;
	name: string;
	title?: string;
	type: SellerCouponKind | string;
	value: number;
	/** Cap for percent discount in NT$ — protects against 100%-off on huge
	 * carts. Optional for amount type. */
	maxDiscountAmount?: number;
	discountAmount?: number;
	discountPercent?: number;
	/* 以下六個 snake 欄是**前端表單的欄名**，不是 wire 名（`/app/coupon_templates`
	 * 的 body 一直是 camelCase：thresholdAmount / startAt / endAt / totalIssueLimit
	 * / issuedCount / createdAt）。normalizeSellerCoupon 負責 wire → 表單的換名。 */
	min_spend?: number;
	thresholdAmount?: number;
	starts_at?: number;
	expires_at?: number;
	startAt?: number;
	endAt?: number;
	usage_limit?: number;
	usage_count?: number;
	totalIssueLimit?: number;
	issuedCount?: number;
	scope?: string;
	status?: string;
	created_at?: number;
	createdAt?: number;
	/** Target audience — defaults to "all" if not set. */
	targetType?: SellerCouponTarget | string;
	/** Required when targetType="specific" — buyer account IDs. */
	buyerAccountIds?: string[];
	/** Detail/list payload for specific-buyer coupons — id + nickname per
	 * targeted account. Mapped into buyerAccountIds for the form. */
	targetAccounts?: {
		buyerId?: string;
		nickname?: string;
	}[];
}

/** Backend numeric `type` enum → semantic kind used by the web form. */
function kindFromCode(code: unknown): SellerCouponKind | undefined {
	const s = String(code ?? "").trim();
	if (s === "1" || s === "amount" || s === "cash") return "amount";
	if (s === "2" || s === "percent") return "percent";
	return undefined;
}

/** Backend numeric `targetType` enum → semantic target. */
function targetFromCode(code: unknown): SellerCouponTarget | undefined {
	const s = String(code ?? "").trim();
	if (s === "1" || s === "all") return "all";
	if (s === "2" || s === "members") return "members";
	if (s === "3" || s === "specific") return "specific";
	return undefined;
}

/**
 * Coerce a coupon timestamp to Unix **seconds**. The wire mixes formats:
 * create echoes back RFC3339 strings (`2026-05-01T00:00:00Z`) while older
 * rows carry epoch numbers (seconds or millis). Returns undefined when none
 * of the candidates parse, so the form leaves the date picker empty rather
 * than rendering an Invalid Date.
 */
function toEpochSeconds(...candidates: unknown[]): number | undefined {
	for (const v of candidates) {
		if (v == null || v === "") continue;
		if (typeof v === "number") {
			if (!Number.isFinite(v)) continue;
			return v > 1e12 ? Math.floor(v / 1000) : v;
		}
		const s = String(v).trim();
		if (/^\d+$/.test(s)) {
			const n = Number(s);
			return n > 1e12 ? Math.floor(n / 1000) : n;
		}
		const ms = Date.parse(s);
		if (Number.isFinite(ms)) return Math.floor(ms / 1000);
	}
	return undefined;
}

function normalizeSellerCoupon(raw: Partial<SellerCoupon>): SellerCoupon {
	const kind =
		kindFromCode(raw.type) ?? (raw.discountPercent ? "percent" : "amount");
	const valueSource =
		kind === "percent" ? raw.discountPercent : raw.discountAmount;
	// Backend uses 0 / -1 as the "unlimited" sentinel for the issue cap; keep
	// only a positive value as a real limit so the card shows ∞ instead of a
	// literal "/ -1" and couponState doesn't read 0 >= -1 as 已領完.
	const issueLimit = Number(raw.totalIssueLimit ?? 0);
	return {
		...raw,
		id: String(raw.id ?? raw.templateId ?? ""),
		name: String(raw.name ?? raw.title ?? "未命名優惠券"),
		type: kind,
		value: Number(raw.value ?? valueSource ?? 0),
		maxDiscountAmount:
			raw.maxDiscountAmount != null ? Number(raw.maxDiscountAmount) : undefined,
		min_spend: Number(raw.thresholdAmount ?? 0),
		starts_at: toEpochSeconds(undefined, raw.startAt),
		expires_at: toEpochSeconds(undefined, raw.endAt),
		usage_limit:
			Number.isFinite(issueLimit) && issueLimit > 0 ? issueLimit : undefined,
		usage_count: Number(raw.issuedCount ?? 0),
		targetType: targetFromCode(raw.targetType) ?? "all",
		buyerAccountIds:
			raw.buyerAccountIds ??
			(Array.isArray(raw.targetAccounts) && raw.targetAccounts.length
				? raw.targetAccounts.map((a) => String(a.buyerId ?? "")).filter(Boolean)
				: undefined),
	};
}

export async function fetchSellerCoupons({
	page = 1,
	pageSize = 20,
}: {
	page?: number;
	pageSize?: number;
} = {}): Promise<SellerCoupon[]> {
	const qs = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
	});
	const env = await authedFetch<SellerCoupon[] | { records?: SellerCoupon[] }>(
		`/app/coupon_templates?${qs}`,
	);
	const records = Array.isArray(env.data)
		? env.data
		: (env.data?.records ?? []);
	return records.map(normalizeSellerCoupon);
}

export async function fetchSellerCoupon(
	id: string,
): Promise<SellerCoupon | null> {
	let env: ApiEnvelope<{ template?: SellerCoupon } & SellerCoupon>;
	try {
		env = await authedFetch<{ template?: SellerCoupon } & SellerCoupon>(
			`/app/coupon_templates/${encodeURIComponent(id)}`,
		);
	} catch (e) {
		// 詳情端點對某些「列表存在」的優惠券回 7001「優惠券不存在」
		// (list / detail contract gap)。視為查無此券回 null —— 對齊本函式的
		// `| null` 契約，避免 reject 變成 unhandledRejection。其餘錯誤照常上拋
		// 給呼叫端處理(顯示錯誤、而非吞掉)。
		if (e instanceof ApiClientError && e.code === 7001) return null;
		throw e;
	}
	const data = env.data;
	if (!data) return null;
	// The detail endpoint wraps the coupon under `data.template` (with
	// usedCount / reservedCount as siblings), unlike the list endpoint which
	// returns each record flat. Unwrap so the edit form binds to the real
	// fields instead of an empty shell (everything falling back to defaults).
	return normalizeSellerCoupon(data.template ?? data);
}

/** Backend semantic → numeric `type` enum. */
const TYPE_CODE: Record<SellerCouponKind, "1" | "2"> = {
	amount: "1",
	percent: "2",
};
/** Backend semantic → numeric `targetType` enum. */
const TARGET_CODE: Record<SellerCouponTarget, "1" | "2" | "3"> = {
	all: "1",
	members: "2",
	specific: "3",
};

/** Stringify a finite number; drop undefined / NaN. OpenAPI says every
 * field on the wire is a string, including numerics. */
function strOrUndef(n: unknown): string | undefined {
	if (n === undefined || n === null || n === "") return undefined;
	const num = typeof n === "number" ? n : Number(n);
	return Number.isFinite(num) ? String(num) : undefined;
}

/**
 * Coupon `startAt`/`endAt` wire as API-standard UTC ISO 8601
 * (`YYYY-MM-DDTHH:mm:ssZ`). The form stores the picked calendar date as
 * UTC-midnight seconds (`new Date("YYYY-MM-DD")`), so recover the selected date
 * with UTC getters. Start anchors to 00:00:00; pass `endOfDay` for the 到期 date
 * so the coupon stays usable through the whole final day (23:59:59).
 */
export function toCouponDateTime(seconds: number, endOfDay = false): string {
	const d = new Date(seconds * 1000);
	const pad = (n: number) => String(n).padStart(2, "0");
	const date = `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
	return `${date}T${endOfDay ? "23:59:59" : "00:00:00"}Z`;
}

export async function createSellerCoupon(
	body: Omit<SellerCoupon, "id" | "created_at" | "usage_count">,
): Promise<{ id?: number }> {
	const kind = (kindFromCode(body.type) ?? "amount") as SellerCouponKind;
	const target = (targetFromCode(body.targetType) ??
		"all") as SellerCouponTarget;
	if (!body.starts_at || !body.expires_at) {
		// Backend marks startAt + endAt as required — fail fast instead of
		// sending a malformed request that returns a noisy 400.
		throw new Error("請設定優惠券的開始與到期時間。");
	}
	if (target === "specific" && (body.buyerAccountIds?.length ?? 0) === 0) {
		throw new Error("指定買家優惠券需要至少選擇一位買家。");
	}
	const env = await authedFetch<{ id?: number; templateId?: number }>(
		"/app/coupon_templates",
		{
			method: "POST",
			json: {
				title: body.name,
				type: TYPE_CODE[kind],
				targetType: TARGET_CODE[target],
				startAt: toCouponDateTime(body.starts_at),
				endAt: toCouponDateTime(body.expires_at, true),
				thresholdAmount: strOrUndef(body.min_spend),
				discountAmount: kind === "amount" ? strOrUndef(body.value) : undefined,
				discountPercent:
					kind === "percent" ? strOrUndef(body.value) : undefined,
				maxDiscountAmount:
					kind === "percent" ? strOrUndef(body.maxDiscountAmount) : undefined,
				buyerAccountIds:
					target === "specific" ? body.buyerAccountIds : undefined,
			},
		},
	);
	return {
		id: env.data?.id ?? env.data?.templateId,
	};
}

// No seller-side coupon update endpoint exists — coupons are immutable once
// issued (the web edit route is a read-only 檢視 view; adjust by revoke +
// recreate). Reintroduce an updateSellerCoupon() here if the backend ships one.

export async function deleteSellerCoupon(id: string): Promise<void> {
	await authedFetch(`/app/coupon_templates/${encodeURIComponent(id)}/revoke`, {
		method: "POST",
	});
}

/* ===== Seller posts ===== */

/**
 * POST /app/sellers/posts body — 可附一張圖片（stagedToken）與一個商品（productId）。
 * 圖片用 uploadImageFile() 回傳的 `stagedToken`（不是 previewUrl），後端 finalize 後存成貼文圖片。
 */
export interface CreateSellerPostInput {
	content: string;
	stagedToken?: string;
	productId?: string;
	tags?: string[];
}

/**
 * PUT /app/sellers/posts/{id} body — 圖片用 keepAssetId（保留舊圖）或
 * newStagedToken（換新圖）二擇一，與 create 的 stagedToken 契約不同。
 */
export interface UpdateSellerPostInput {
	content: string;
	keepAssetId?: string;
	newStagedToken?: string;
	productId?: string;
	tags?: string[];
}

/**
 * 賣家對自己的貼文送得出去的兩個目標狀態。
 *
 * **型別上就排除 4401**：`publish` 與 `unpublish` 是兩支各自寫死目標狀態的命令端點
 * （`gateway/internal/handler/post.go:294,313`），沒有第三支能把貼文送回未發布。
 * 值域不對稱是契約本身 —— 宣告成 `PostPublishStatusValue` 會讓「改回未發布」在
 * 編譯期看起來合法，然後在執行期變成一個沒有端點可打的分支。
 */
export type PostPublishTarget = Exclude<
	PostPublishStatusValue,
	typeof PostPublishStatus.DRAFT
>;

/**
 * 貼文資料層的介面（56 號 §4.5 L1053）。
 *
 * 與店面同住一支檔案，但**工廠刻意拆兩個**：代行注入時只換得到貼文這一段，
 * CRM／優惠券那半段（§6.1.14 #6／#7 未開放）不會被一起帶進代行工作區。
 */
export interface SellerPostRepo {
	/**
	 * 建立貼文。
	 *
	 * `version` **只有代行路徑會回**：「建立即發布」的第二支呼叫
	 * （`POST .../posts/{postId}/publish`）必填 `expectedVersion`，而新建資源的版本
	 * 只有這一次回應拿得到（缺它是 9260／400）。賣家自助端沒有第二支呼叫，不回。
	 */
	createSellerPost(
		body: CreateSellerPostInput,
		meta?: DelegatedMutationMeta,
	): Promise<{ id?: number; version?: string }>;
	updateSellerPost(
		id: number | string,
		body: UpdateSellerPostInput,
		meta?: DelegatedMutationMeta,
	): Promise<void>;
	/**
	 * 發布（4402）／下架（4403）。
	 *
	 * **沒有 `expectedVersion`**：那是代行路徑的樂觀鎖，用來擋「管理員看到的版本
	 * 已經被賣家改掉」；賣家對自己的貼文按下發布時沒有第二個寫入者要防，所以
	 * gateway 的 `updateMyPostPublishStatus` 刻意不收版本（`post.go:325` 的註解）。
	 * 帶了也沒有欄位裝，只會多一次詳情往返。
	 *
	 * 目標值域**不含 4401**：兩支端點各自寫死一個目標狀態，沒有第三支能把貼文送回
	 * 未發布。型別上就排除掉，不靠呼叫端自律。
	 */
	setSellerPostPublishStatus(
		id: number | string,
		target: PostPublishTarget,
		meta?: DelegatedMutationMeta,
	): Promise<void>;
	deleteSellerPost(id: number | string): Promise<void>;
	/**
	 * 店主讀單篇貼文，不分發布狀態與是否被平台下架（編輯頁與深連結使用；公開詳情
	 * 只回公開的貼文）。查無或不是這家店的貼文丟 `ApiClientError`（`status` 或
	 * `code` 為 404），與其他失敗用同一種方式分辨。
	 */
	fetchSellerPostDetail(id: number | string): Promise<MyPostDetail>;
	fetchSellerPosts(
		sellerId?: number | string,
		params?: { pageSize?: number },
	): Promise<PostListItem[]>;
}

export interface SellerPostRepoOptions {
	/** 寫入根（賣家 `/app/sellers/posts`）。 */
	root?: string;
	/** 自己的貼文列表（賣家 `/app/sellers/me/posts`，與寫入根不同層）。 */
	listRoot?: string;
}

export function createSellerPostRepo(
	fetcher: RepoFetcher,
	options: SellerPostRepoOptions = {},
): SellerPostRepo {
	const root = options.root ?? "/app/sellers/posts";
	const listRoot = options.listRoot ?? "/app/sellers/me/posts";

	return {
		async createSellerPost(
			body: CreateSellerPostInput,
		): Promise<{ id?: number; version?: string }> {
			const env = await fetcher<{ id?: number }>(root, {
				method: "POST",
				json: body,
			});
			return env.data ?? {};
		},

		async updateSellerPost(
			id: number | string,
			body: UpdateSellerPostInput,
		): Promise<void> {
			await fetcher(`${root}/${id}`, { method: "PUT", json: body });
		},

		/** 兩支命令端點，body 為空（賣家路徑沒有 `expectedVersion`，見介面註解）。 */
		async setSellerPostPublishStatus(
			id: number | string,
			target: PostPublishTarget,
		): Promise<void> {
			const command =
				target === PostPublishStatus.PUBLISHED ? "publish" : "unpublish";
			await fetcher(`${root}/${id}/${command}`, { method: "POST" });
		},

		async deleteSellerPost(id: number | string): Promise<void> {
			await fetcher(`${root}/${id}`, { method: "DELETE" });
		},

		async fetchSellerPostDetail(id: number | string): Promise<MyPostDetail> {
			const env = await fetcher<MyPostDetail>(
				`${root}/${encodeURIComponent(String(id))}`,
			);
			if (!env.data) {
				throw new ApiClientError("找不到這則動態", 404, 404);
			}
			return normalizePostDetail(env.data);
		},

		/**
		 * 賣家自己貼文列表。改走 authed `/app/sellers/me/posts`（ListMyPosts），
		 * 回傳含被平台下架的貼文（hiddenAt/hiddenReason）——公開 feed 會被 server 濾掉。
		 * sellerId 參數保留以相容既有呼叫點（useSellerPosts nav 徽章），現不使用。
		 */
		async fetchSellerPosts(
			_sellerId?: number | string,
			params?: { pageSize?: number },
		): Promise<PostListItem[]> {
			const env = await fetcher<{ records?: PostListItem[] } | PostListItem[]>(
				listRoot,
				{ query: { page_size: String(params?.pageSize ?? 50) } },
			);
			const data = env.data;
			if (Array.isArray(data)) return data;
			return data?.records ?? [];
		},
	};
}

/** 賣家路徑的預設實例；下面的具名匯出全部經由它。 */
export const sellerPostRepo = createSellerPostRepo(authedFetch);

export const createSellerPost: SellerPostRepo["createSellerPost"] = (
	body,
	meta,
) => sellerPostRepo.createSellerPost(body, meta);

export const updateSellerPost: SellerPostRepo["updateSellerPost"] = (
	id,
	body,
	meta,
) => sellerPostRepo.updateSellerPost(id, body, meta);

export const setSellerPostPublishStatus: SellerPostRepo["setSellerPostPublishStatus"] =
	(id, target) => sellerPostRepo.setSellerPostPublishStatus(id, target);

export const deleteSellerPost: SellerPostRepo["deleteSellerPost"] = (id) =>
	sellerPostRepo.deleteSellerPost(id);

export const fetchSellerPosts: SellerPostRepo["fetchSellerPosts"] = (
	sellerId,
	params,
) => sellerPostRepo.fetchSellerPosts(sellerId, params);
