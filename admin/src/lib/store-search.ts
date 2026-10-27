import type { MessageInitShape } from "@bufbuild/protobuf";
import type { ListStoresRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { Role } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import {
	KYCStatus,
	PayUniBindStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

// 店家清單與搜尋（SellerAdminService.ListStores）的請求組法。畫面只管狀態，哪個篩選分頁對應哪些請求欄位、
// 搜尋字串怎麼分流都在這裡，才能不靠畫面測試。

/** 商家清單的篩選分頁（決策簡報 D4 第 1 點）。 */
export const FILTER_TABS = [
	"all",
	"approved",
	"pending",
	"rejected",
	"rebind",
	"suspended",
	"closed",
] as const;

export type FilterTab = (typeof FILTER_TABS)[number];

export const FILTER_TAB_LABEL: Record<FilterTab, string> = {
	all: "全部",
	approved: "已核准",
	pending: "待審核",
	rejected: "已退回",
	rebind: "待重新綁定",
	suspended: "停權中",
	closed: "已關閉",
};

/** 網址的 `?filter=`：不認得的值（含舊版的 active、forwarded）回到「全部」。 */
export function parseFilterTab(raw: string | null | undefined): FilterTab {
	return (FILTER_TABS as readonly string[]).includes(raw ?? "")
		? (raw as FilterTab)
		: "all";
}

type ListStoresInit = MessageInitShape<typeof ListStoresRequestSchema>;

/**
 * 篩選分頁對應的請求欄位。「全部」什麼都不送（不是送 UNSPECIFIED 以外的假值）：不帶 status 就是目前的店家，
 * 已關閉的店要明列 CLOSED。
 */
export function filterFields(
	tab: FilterTab,
): Pick<
	ListStoresInit,
	| "status"
	| "kycStatus"
	| "kycAwaitingReview"
	| "payuniBindStatus"
	| "suspended"
> {
	switch (tab) {
		case "all":
			return {};
		case "approved":
			return { kycStatus: KYCStatus.KYC_STATUS_APPROVED };
		case "pending":
			// 「待審核」是後台待辦的成員（審核中、店家營運中、帳號沒有停權），不是申請的儲存狀態：伺服器的 kycAwaitingReview。
			return { kycAwaitingReview: true };
		case "rejected":
			return { kycStatus: KYCStatus.KYC_STATUS_REJECTED };
		case "rebind":
			return { payuniBindStatus: PayUniBindStatus.FAILED };
		case "suspended":
			return { suspended: true };
		case "closed":
			return { status: StoreStatus.CLOSED };
	}
}

export interface ListStoresInput {
	tab: FilterTab;
	/** 店名、聯絡電話或 store_code；空字串不篩選。 */
	query?: string;
	/** 只列這個帳號的店家（email 搜尋的第二段）。 */
	accountId?: string;
	pageToken?: string;
	pageSize: number;
}

/** 沒有值的欄位不放進請求（空字串、0 與 UNSPECIFIED 都是「不篩選」，不是要送出的值）。 */
export function listStoresRequest(input: ListStoresInput): ListStoresInit {
	const request: ListStoresInit = {
		pageSize: input.pageSize,
		...filterFields(input.tab),
	};
	if (input.query) request.query = input.query;
	if (input.accountId) request.accountId = input.accountId;
	if (input.pageToken) request.pageToken = input.pageToken;
	return request;
}

/** 搜尋框的字串怎麼查：含 `@` 的是登入信箱（兩段查詢，K5），其他交給 ListStores 的 query。 */
export type StoreSearch =
	| { kind: "none" }
	| { kind: "text"; query: string }
	| { kind: "email"; email: string };

export function planStoreSearch(keyword: string): StoreSearch {
	const text = keyword.trim();
	if (!text) return { kind: "none" };
	if (text.includes("@")) return { kind: "email", email: text };
	return { kind: "text", query: text };
}

/** 信箱查帳號每頁的筆數；email 在 identity 不是唯一的（帳號可能重複），所以要走完所有頁。 */
export const ACCOUNT_LOOKUP_PAGE_SIZE = 20;
/** 走頁數的上限：完全相符的賣家帳號只會有幾個，超過就是異常，報錯而不是靜默截斷。 */
const ACCOUNT_LOOKUP_MAX_PAGES = 10;

/** 信箱找帳號的請求：完全相符的 email（不分大小寫），而且是賣家角色（買家帳號沒有店家，不能擋掉賣家的帳號）。 */
export function accountLookupRequest(
	email: string,
	pageToken: string,
): {
	email: string;
	role: Role;
	pageSize: number;
	pageToken?: string;
} {
	const request = {
		email,
		role: Role.SELLER,
		pageSize: ACCOUNT_LOOKUP_PAGE_SIZE,
	};
	return pageToken ? { ...request, pageToken } : request;
}

/** 依 next_page_token 走完所有頁，回傳符合的帳號 ID（依回傳順序，去重複）。 */
export async function collectAccountIds(
	email: string,
	fetchPage: (
		request: ReturnType<typeof accountLookupRequest>,
	) => Promise<{ accounts: readonly { id: string }[]; nextPageToken: string }>,
): Promise<string[]> {
	const ids: string[] = [];
	let pageToken = "";
	for (let page = 0; page < ACCOUNT_LOOKUP_MAX_PAGES; page++) {
		const res = await fetchPage(accountLookupRequest(email, pageToken));
		for (const account of res.accounts) {
			if (!ids.includes(account.id)) ids.push(account.id);
		}
		if (!res.nextPageToken) return ids;
		pageToken = res.nextPageToken;
	}
	throw new Error(
		`信箱 ${email} 對到超過 ${ACCOUNT_LOOKUP_MAX_PAGES} 頁的帳號`,
	);
}

/** 合併多個帳號的店家：保持各清單的順序與清單間的先後，同一家店（store.id）只留第一次出現。 */
export function mergeStoreLists<T extends { store?: { id: string } }>(
	lists: readonly (readonly T[])[],
): T[] {
	const seen = new Set<string>();
	const out: T[] = [];
	for (const item of lists.flat()) {
		const id = item.store?.id ?? "";
		if (id && seen.has(id)) continue;
		if (id) seen.add(id);
		out.push(item);
	}
	return out;
}

/** 信箱搜尋時，每個帳號查店家的每頁筆數：一個帳號最多一家目前的店，加上已關閉的也遠小於這個數。 */
export const STORES_PER_ACCOUNT = 100;

/** 文字搜尋與沒有關鍵字的 ListStores 請求（信箱搜尋走 collectAccountIds + mergeStoreLists）。 */
export function textSearchRequest(
	search: Exclude<StoreSearch, { kind: "email" }>,
	input: Omit<ListStoresInput, "query" | "accountId">,
): ReturnType<typeof listStoresRequest> {
	return listStoresRequest({
		...input,
		query: search.kind === "text" ? search.query : "",
	});
}
