/**
 * 店家的客戶與標籤（`CrmAppService`）。每個方法帶 `storeId`（呼叫者此刻的店，取自 `GetMyStore`）。
 * 沒有總數、關鍵字搜尋、電話、Email；分頁用 `page_token`，結束只看 `nextPageToken`。
 * 客戶的訂單不在這裡：用店家訂單列表加買家篩選（`lib/orders/repo.ts` 的 `fetchOrders({ view: "seller", buyerId })`）。
 */

import { timestampMs } from "@bufbuild/protobuf/wkt";
import {
	type Customer,
	CustomerSchema,
	CustomerSort,
	CustomerType,
	type Tag,
	TagSchema,
} from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import {
	commonUnknown,
	crmCustomerSort,
	crmCustomerType,
} from "@hawkr/ui-text/keys";
import { crmAppClient } from "../connect/crm";
import { optionalText, type UiText } from "../i18n/ui-text";
import { imageUri } from "../media/image";

type CrmClient = ReturnType<typeof crmAppClient>;

/** 客戶列表一頁的筆數（契約預設 20、上限 100）。 */
export const CUSTOMERS_PAGE_SIZE = 50;

export interface CustomerTag {
	id: string;
	name: string;
}

export interface SellerCustomer {
	/** 買家 ID（不透明字串，不解析）。 */
	id: string;
	/** 暱稱；沒有設定或買家已刪除時是空字串。 */
	name: string;
	avatar?: string;
	/** 買家帳號已刪除。 */
	deleted: boolean;
	customerType: CustomerType;
	/** 完成訂單的累計實付淨額（新台幣元）。 */
	totalSpent: number;
	orderCount: number;
	/** 首筆完成訂單的時間（epoch ms）；伺服器沒帶時不設。 */
	firstOrderAt?: number;
	tags: CustomerTag[];
}

export function customerTagFrom(tag: Tag): CustomerTag {
	return { id: tag.id, name: tag.name };
}

export function sellerCustomerFrom(customer: Customer): SellerCustomer {
	return {
		id: customer.buyerId,
		name: customer.display?.nickname ?? "",
		avatar: imageUri(customer.display?.avatar, "avatar"),
		deleted: customer.display?.deleted ?? false,
		customerType: customer.type,
		totalSpent: Number(customer.totalSpent),
		orderCount: customer.orderCount,
		firstOrderAt: customer.firstOrderTime
			? timestampMs(customer.firstOrderTime)
			: undefined,
		tags: customer.tags.map(customerTagFrom),
	};
}

export interface ListCustomersParams {
	/** 只列這個分類；不設是全部（不能送 UNSPECIFIED）。 */
	type?: CustomerType;
	/** 只列貼了這個標籤的客戶（單一標籤）。 */
	tagId?: string;
	sort?: CustomerSort;
	/** 為真時大的在前。 */
	descending?: boolean;
	pageSize?: number;
	pageToken?: string;
}

export interface SellerCustomersPage {
	customers: SellerCustomer[];
	/** 空字串表示沒有下一頁。 */
	nextPageToken: string;
}

export async function listCustomers(
	storeId: string,
	params: ListCustomersParams = {},
	client: () => CrmClient = crmAppClient,
): Promise<SellerCustomersPage> {
	const res = await client().listCustomers({
		storeId,
		type:
			params.type === undefined || params.type === CustomerType.UNSPECIFIED
				? undefined
				: params.type,
		tagId: params.tagId || undefined,
		sort: params.sort ?? CustomerSort.UNSPECIFIED,
		descending: params.descending ?? false,
		pageSize: params.pageSize ?? CUSTOMERS_PAGE_SIZE,
		pageToken: params.pageToken ?? "",
	});
	return {
		customers: res.customers.map(sellerCustomerFrom),
		nextPageToken: res.nextPageToken,
	};
}

/** 依 `nextPageToken` 一頁一頁讀完（給「全選此分群」）；任何一頁失敗就整個失敗，不回部分結果。 */
export async function listAllCustomers(
	storeId: string,
	params: Omit<ListCustomersParams, "pageToken"> = {},
	client: () => CrmClient = crmAppClient,
): Promise<SellerCustomer[]> {
	const all: SellerCustomer[] = [];
	let pageToken = "";
	do {
		const page = await listCustomers(
			storeId,
			{ ...params, pageSize: 100, pageToken },
			client,
		);
		all.push(...page.customers);
		pageToken = page.nextPageToken;
	} while (pageToken);
	return all;
}

function required(customer: Customer | undefined, rpc: string): Customer {
	if (!customer) throw new Error(`${rpc} 的回應沒有客戶`);
	return customer;
}

export async function getCustomer(
	storeId: string,
	buyerId: string,
	client: () => CrmClient = crmAppClient,
): Promise<SellerCustomer> {
	const res = await client().getCustomer({ storeId, buyerId });
	return sellerCustomerFrom(required(res.customer, "GetCustomer"));
}

/** 改客戶分類（update_mask 只有 `type`；只能設一般或 VIP）。 */
export async function updateCustomerType(
	storeId: string,
	buyerId: string,
	type: CustomerType,
	client: () => CrmClient = crmAppClient,
): Promise<SellerCustomer> {
	const res = await client().updateCustomer({
		storeId,
		customer: { buyerId, type },
		updateMask: fieldMask(CustomerSchema, ["type"]),
	});
	return sellerCustomerFrom(required(res.customer, "UpdateCustomer"));
}

export async function listTags(
	storeId: string,
	client: () => CrmClient = crmAppClient,
): Promise<CustomerTag[]> {
	const res = await client().listTags({ storeId });
	return res.tags.map(customerTagFrom);
}

export async function createTag(
	storeId: string,
	name: string,
	client: () => CrmClient = crmAppClient,
): Promise<CustomerTag> {
	const res = await client().createTag({ storeId, name });
	if (!res.tag) throw new Error("CreateTag 的回應沒有標籤");
	return customerTagFrom(res.tag);
}

export async function renameTag(
	storeId: string,
	tagId: string,
	name: string,
	client: () => CrmClient = crmAppClient,
): Promise<CustomerTag> {
	const res = await client().updateTag({
		storeId,
		tag: { id: tagId, name },
		updateMask: fieldMask(TagSchema, ["name"]),
	});
	if (!res.tag) throw new Error("UpdateTag 的回應沒有標籤");
	return customerTagFrom(res.tag);
}

export async function deleteTag(
	storeId: string,
	tagId: string,
	client: () => CrmClient = crmAppClient,
): Promise<void> {
	await client().deleteTag({ storeId, tagId });
}

/** 替客戶貼上標籤（名稱不存在的先建立）；回貼完之後的客戶。 */
export async function addCustomerTags(
	storeId: string,
	buyerId: string,
	tagNames: string[],
	client: () => CrmClient = crmAppClient,
): Promise<SellerCustomer> {
	const res = await client().addCustomerTags({ storeId, buyerId, tagNames });
	return sellerCustomerFrom(required(res.customer, "AddCustomerTags"));
}

/** 移除客戶身上的一個標籤；回移除之後的客戶。 */
export async function removeCustomerTag(
	storeId: string,
	buyerId: string,
	tagId: string,
	client: () => CrmClient = crmAppClient,
): Promise<SellerCustomer> {
	const res = await client().removeCustomerTag({ storeId, buyerId, tagId });
	return sellerCustomerFrom(required(res.customer, "RemoveCustomerTag"));
}

/** 客戶分類的顯示文字（共用文案目錄）；UNSPECIFIED 與不認得的值顯示「未知」。 */
export function customerTypeText(ut: UiText, type: CustomerType): string {
	return optionalText(ut, crmCustomerType(type)) ?? ut(commonUnknown());
}

/** 排序鍵的正規化：UNSPECIFIED 是伺服器預設的首購時間。 */
export function normalizeCustomerSort(sort: CustomerSort): CustomerSort {
	return sort === CustomerSort.UNSPECIFIED
		? CustomerSort.FIRST_ORDER_TIME
		: sort;
}

/** 排序鍵的顯示文字（共用文案目錄），先經 {@link normalizeCustomerSort}。 */
export function customerSortText(ut: UiText, sort: CustomerSort): string {
	return (
		optionalText(ut, crmCustomerSort(normalizeCustomerSort(sort))) ??
		ut(commonUnknown())
	);
}

/** 客戶分類的 CSS 修飾 class（`is-vip` / `is-normal`）。 */
export function customerTypeClass(type: CustomerType): string {
	return type === CustomerType.VIP ? "is-vip" : "is-normal";
}
