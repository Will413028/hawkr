import type { StoreListItem } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { townByCode } from "@hawkr/twaddr/lookup";
import { kycStatusText } from "@/lib/kyc-status";

// 商家清單一列的顯示判斷。契約只給申請狀態（kyc_status，沒有申請時不設）、suspended 和店家的生命週期，
// 徽章與代行入口的資格都由它們推出來，不另外存一份「綜合狀態」。

export type BadgeVariant = "success" | "warning" | "danger" | "gray";

export interface RowState {
	/** 店家營運中（不是等待刪除或已關閉）：只用在按鈕與徽章的顏色（狀態的呈現），不是動作的可用性；代行能不能用看伺服器的 delegationUnavailabilities。 */
	isActive: boolean;
	/** 申請審核中，操作者要去看詳情處理。 */
	isPending: boolean;
	/** 申請被退回。 */
	isRejected: boolean;
}

export function rowState(item: StoreListItem): RowState {
	return {
		isActive: item.store?.status === StoreStatus.ACTIVE,
		isPending: item.kycStatus === KYCStatus.KYC_STATUS_SUBMITTED,
		isRejected: item.kycStatus === KYCStatus.KYC_STATUS_REJECTED,
	};
}

/** 申請狀態的徽章顏色（admin 的呈現）；沒有列的值（沒有申請、UNSPECIFIED、不認得的值）是灰色。 */
const KYC_VARIANT: Readonly<Partial<Record<KYCStatus, BadgeVariant>>> = {
	[KYCStatus.KYC_STATUS_DRAFT]: "gray",
	[KYCStatus.KYC_STATUS_SUBMITTED]: "warning",
	[KYCStatus.KYC_STATUS_REJECTED]: "danger",
	[KYCStatus.KYC_STATUS_APPROVED]: "success",
	[KYCStatus.KYC_STATUS_CANCELLED]: "gray",
};

/**
 * 申請狀態的徽章：有值時文字取自共用文案目錄的後台員工受眾（UNSPECIFIED 與不認得的值是「未知」），沒有申請（欄位不帶值）
 * 是 admin 自己的「尚未申請」；顏色是 admin 的呈現。清單與商家詳情的頁首共用。
 */
export function kycBadge(status: KYCStatus | undefined): {
	label: string;
	variant: BadgeVariant;
} {
	return {
		label: kycStatusText(status),
		variant: status === undefined ? "gray" : (KYC_VARIANT[status] ?? "gray"),
	};
}

/** 一列的狀態徽章：已關閉與停權優先於申請狀態。 */
export function rowBadge(item: StoreListItem): {
	label: string;
	variant: BadgeVariant;
} {
	if (item.store?.status === StoreStatus.CLOSED) {
		return { label: "已關閉", variant: "gray" };
	}
	if (item.suspended) {
		return { label: "停權中", variant: "danger" };
	}
	if (item.store?.status === StoreStatus.PENDING_DELETION) {
		return { label: "等待刪除", variant: "warning" };
	}
	return kycBadge(item.kycStatus);
}

/** 縣市＋鄉鎮市區；沒有地址或不認得的代碼回 `-`。 */
export function rowLocation(item: StoreListItem): string {
	const code = item.store?.address?.townCode ?? "";
	const town = code ? townByCode(code) : undefined;
	return town ? `${town.countyName}${town.name}` : "-";
}
