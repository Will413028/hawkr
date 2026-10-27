/** BuyerAppService address methods mapped to the account address view. */

import {
	AddressSchema,
	type Address as BuyerAddress,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { formatAddress } from "@hawkr/twaddr/lookup";
import { buyerAppClient } from "../connect/buyer";

export interface Address {
	id: string;
	etag: string;
	receiverName: string;
	receiverPhone: string;
	/** 內政部 8 碼鄉鎮市區代碼；縣市是前 5 碼，名稱與郵遞區號由 twaddr.gen 依代碼取得。 */
	townCode: string;
	/** 鄉鎮市區之後的明細，最多 255 字。 */
	addressDetail: string;
	isDefault: boolean;
}

/** 組出完整地址字串（縣市＋鄉鎮市區＋明細，內政部寫法）。 */
export function addressText(
	a: Pick<Address, "townCode" | "addressDetail">,
): string {
	return formatAddress(a.townCode, a.addressDetail);
}

export type AddressInput = Omit<Address, "id" | "etag">;

function fromConnect(a: BuyerAddress): Address {
	return {
		id: a.id,
		etag: a.etag,
		receiverName: a.receiverName,
		receiverPhone: a.receiverPhone,
		townCode: a.postalAddress?.townCode ?? "",
		addressDetail: a.postalAddress?.addressDetail ?? "",
		isDefault: a.defaultAddress,
	};
}

// 每位買家的地址有上限（buyer 模組在新增時驗證，目前 20 筆），通常一頁就是完整清單。
// 伺服器調整上限時仍會回 next_page_token，照著讀完，不無聲截斷。
const ADDRESS_PAGE_SIZE = 20;

export async function fetchAddresses(): Promise<Address[]> {
	const client = buyerAppClient();
	const addresses: Address[] = [];
	let pageToken = "";
	do {
		const page = await client.listAddresses(
			pageToken
				? { pageSize: ADDRESS_PAGE_SIZE, pageToken }
				: { pageSize: ADDRESS_PAGE_SIZE },
		);
		addresses.push(...page.addresses.map(fromConnect));
		pageToken = page.nextPageToken;
	} while (pageToken);
	return addresses;
}

export async function fetchDefaultAddress(): Promise<Address | null> {
	const addresses = await fetchAddresses();
	return addresses.find((a) => a.isDefault) ?? addresses[0] ?? null;
}

export async function createAddress(
	body: AddressInput,
): Promise<Address | null> {
	const result = await buyerAppClient().createAddress({
		address: {
			receiverName: body.receiverName,
			receiverPhone: body.receiverPhone,
			postalAddress: {
				townCode: body.townCode,
				addressDetail: body.addressDetail,
			},
			defaultAddress: body.isDefault,
		},
	});
	return result.address ? fromConnect(result.address) : null;
}

export async function updateAddress(
	id: string | number,
	etag: string,
	body: AddressInput,
): Promise<void> {
	await buyerAppClient().updateAddress({
		address: {
			id: String(id),
			etag,
			receiverName: body.receiverName,
			receiverPhone: body.receiverPhone,
			postalAddress: {
				townCode: body.townCode,
				addressDetail: body.addressDetail,
			},
			defaultAddress: body.isDefault,
		},
		updateMask: fieldMask(AddressSchema, [
			"receiver_name",
			"receiver_phone",
			"postal_address",
			"default_address",
		]),
	});
}

export async function deleteAddress(id: string | number): Promise<void> {
	await buyerAppClient().deleteAddress({ id: String(id) });
}

export async function setDefaultAddress(
	id: string | number,
	etag: string,
): Promise<void> {
	await buyerAppClient().updateAddress({
		address: { id: String(id), etag, defaultAddress: true },
		updateMask: fieldMask(AddressSchema, ["default_address"]),
	});
}
