/**
 * Seller-side repos.
 *
 * 店家資料走 Connect（`SellerAppService`）：GetMyStore、UpdateMyStore、CompleteMyStoreOnboarding、
 * ApplyStore、CloseMyStore；認證在 `kyc.ts`。儀表板在 `dashboard.ts`（`GetMyDashboard`），措施與申覆在 `measures.ts`。貼文走 Connect（`SocialAppService` 的 owner 方法，見 `createSellerPostRepo`），客戶與標籤走 Connect（`CrmAppService`，見 `crm.ts`）。其餘端點仍是 REST（全部 authed）：
 *   GET    /app/coupon_templates                    my coupon templates
 *   POST   /app/coupon_templates                    create
 *   GET    /app/coupon_templates/{id}               detail
 *   POST   /app/coupon_templates/{id}/revoke        revoke
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
import type { DelegatedMutationMeta } from "../api/delegated";
import type { MyPostDetail, PostListItem } from "../api/openapi-types";
import {
	PostPublishStatus,
	type PostPublishStatusValue,
} from "../codemeta/enums";
import { sellerAppClient } from "../connect/seller";
import { socialAppClient } from "../connect/social";
import { createOwnerPostApi, type OwnerPostClient } from "../posts/owner";
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
 * 建立貼文（`SocialAppService.CreatePost`）的輸入 — 可附一張圖片（stagedToken）與一個商品（productId）。
 * 圖片用 uploadImageFile() 回傳的 `stagedToken`（不是 previewUrl），伺服器 finalize 後存成貼文圖片。
 */
export interface CreateSellerPostInput {
	content: string;
	stagedToken?: string;
	productId?: string;
	tags?: string[];
}

/**
 * 修改貼文（`SocialAppService.UpdatePost`）的輸入 — 沒帶 `newStagedToken` 就沿用目前的圖
 * （update_mask 不含圖片）；`keepAssetId` 不進任何請求。
 */
export interface UpdateSellerPostInput {
	content: string;
	keepAssetId?: string;
	newStagedToken?: string;
	/** undefined＝不變；null＝清除商品。 */
	productId?: string | null;
	tags?: string[];
	/** 畫面載入時的貼文版本（`MyPostDetail.etag`）；自助端的樂觀鎖用，代行端取 `meta.expectedVersion`。 */
	etag?: string;
}

/**
 * 賣家對自己的貼文送得出去的兩個目標狀態。
 *
 * **型別上就排除 4401**：`PublishPost` 與 `UnpublishPost` 是兩支各自寫死目標狀態的命令
 * （`SocialAppService`），沒有第三支能把貼文送回未發布。
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
	): Promise<{ id?: string; version?: string }>;
	/** 自助端的樂觀鎖版本取自 `body.etag`（畫面載入時的值），寫入前不重讀；版本已變時由呼叫端的錯誤路徑顯示。 */
	updateSellerPost(
		id: number | string,
		body: UpdateSellerPostInput,
		meta?: DelegatedMutationMeta,
	): Promise<void>;
	/**
	 * 發布（4402）／下架（4403）。
	 *
	 * 自助端以第四個參數 `etag`（列表那一列的版本）做樂觀鎖，不在寫入前重讀；
	 * 代行端的版本在 `meta.expectedVersion`，不看 `etag`。
	 *
	 * 目標值域**不含 4401**：兩支端點各自寫死一個目標狀態，沒有第三支能把貼文送回
	 * 未發布。型別上就排除掉，不靠呼叫端自律。
	 */
	setSellerPostPublishStatus(
		id: number | string,
		target: PostPublishTarget,
		meta?: DelegatedMutationMeta,
		etag?: string,
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
	/**
	 * 動態數（導覽徽章）：伺服器以資料庫計數的各狀態篇數加總，不含已刪除的貼文；
	 * 不是列表長度（列表一頁有上限）。
	 */
	fetchPostCount(): Promise<number>;
}

/**
 * 店主自己的貼文資料層（SocialAppService 的 owner 方法，Connect）。`clientFor` 可注入是為了測試；
 * client 在每次呼叫時才建立，模組載入時不碰 transport。
 */
export function createSellerPostRepo(
	clientFor: () => OwnerPostClient = socialAppClient,
): SellerPostRepo {
	const api = () => createOwnerPostApi(clientFor());
	return {
		async createSellerPost(body: CreateSellerPostInput) {
			return api().create(body);
		},

		async updateSellerPost(id: number | string, body: UpdateSellerPostInput) {
			const { etag, ...fields } = body;
			await api().update(String(id), fields, etag);
		},

		async setSellerPostPublishStatus(
			id: number | string,
			target: PostPublishTarget,
			_meta?: DelegatedMutationMeta,
			etag?: string,
		) {
			await api().setPublished(
				String(id),
				target === PostPublishStatus.PUBLISHED,
				etag,
			);
		},

		async deleteSellerPost(id: number | string) {
			await api().remove(String(id));
		},

		async fetchSellerPostDetail(id: number | string): Promise<MyPostDetail> {
			return api().detail(String(id));
		},

		/** 店主自己的貼文列表，含草稿與被平台下架的（公開列表讀不到它們）。`sellerId` 保留以相容呼叫點，不使用。 */
		async fetchSellerPosts(
			_sellerId?: number | string,
			params?: { pageSize?: number },
		): Promise<PostListItem[]> {
			return api().list(params?.pageSize ?? 50);
		},

		async fetchPostCount(): Promise<number> {
			return api().count();
		},
	};
}

/** 賣家路徑的預設實例；下面的具名匯出全部經由它。 */
export const sellerPostRepo = createSellerPostRepo();

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
	(id, target, meta, etag) =>
		sellerPostRepo.setSellerPostPublishStatus(id, target, meta, etag);

export const deleteSellerPost: SellerPostRepo["deleteSellerPost"] = (id) =>
	sellerPostRepo.deleteSellerPost(id);

export const fetchSellerPosts: SellerPostRepo["fetchSellerPosts"] = (
	sellerId,
	params,
) => sellerPostRepo.fetchSellerPosts(sellerId, params);
