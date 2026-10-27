import type { StoreListItem } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCFlowStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { townByCode } from "@hawkr/twaddr/lookup";
import { sellerKYCFlowStatus } from "@hawkr/ui-text/keys";
import { staffTextOrUnknown } from "@/lib/ui-text";

// 商家清單一列的顯示判斷。契約只給兩個流程軸（kyc_flow_status、suspended）和店家的生命週期，
// 徽章與代行入口的資格都由它們推出來，不另外存一份「綜合狀態」。

export type BadgeVariant = "success" | "warning" | "danger" | "gray";

export interface RowState {
	/** 店家營運中（不是等待刪除或已關閉）：只用在按鈕與徽章的顏色（狀態的呈現），不是動作的可用性；代行能不能用看伺服器的 delegationUnavailabilities。 */
	isActive: boolean;
	/** 還在審核或綁定途中，操作者要去看詳情處理。 */
	isPending: boolean;
	/** 商家認證或 PayUni 被退回。 */
	isRejected: boolean;
}

const PENDING_FLOWS: ReadonlySet<KYCFlowStatus> = new Set([
	KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
	KYCFlowStatus.KYC_FLOW_STATUS_BINDING,
	KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED,
	KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED,
	KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW,
	KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
]);

const REJECTED_FLOWS: ReadonlySet<KYCFlowStatus> = new Set([
	KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
	KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
]);

export function rowState(item: StoreListItem): RowState {
	const flow = item.kycFlowStatus;
	return {
		isActive: item.store?.status === StoreStatus.ACTIVE,
		isPending: PENDING_FLOWS.has(flow),
		isRejected: REJECTED_FLOWS.has(flow),
	};
}

/** 認證流程狀態的徽章顏色（admin 的呈現）；沒有列的值（UNSPECIFIED、不認得的值）是灰色。 */
const FLOW_VARIANT: Readonly<Partial<Record<KYCFlowStatus, BadgeVariant>>> = {
	[KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED]: "gray",
	[KYCFlowStatus.KYC_FLOW_STATUS_DRAFT]: "gray",
	[KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED]: "warning",
	[KYCFlowStatus.KYC_FLOW_STATUS_REJECTED]: "danger",
	[KYCFlowStatus.KYC_FLOW_STATUS_BINDING]: "warning",
	[KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED]: "danger",
	[KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED]: "warning",
	[KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW]: "warning",
	[KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO]: "warning",
	[KYCFlowStatus.KYC_FLOW_STATUS_APPROVED]: "success",
	[KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED]: "danger",
};

/**
 * 認證流程狀態的徽章：文字取自共用文案目錄的後台員工受眾（沒有申請是「尚未申請」；UNSPECIFIED 與不認得的值是「未知」），
 * 顏色是 admin 的呈現。清單與商家詳情的 PayUni 卡共用。
 */
export function flowBadge(flow: KYCFlowStatus): {
	label: string;
	variant: BadgeVariant;
} {
	return {
		label: staffTextOrUnknown(sellerKYCFlowStatus(flow)),
		variant: FLOW_VARIANT[flow] ?? "gray",
	};
}

/** 一列的狀態徽章：已關閉與停權優先於認證流程。 */
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
	return flowBadge(item.kycFlowStatus);
}

/** 縣市＋鄉鎮市區；沒有地址或不認得的代碼回 `-`。 */
export function rowLocation(item: StoreListItem): string {
	const code = item.store?.address?.townCode ?? "";
	const town = code ? townByCode(code) : undefined;
	return town ? `${town.countyName}${town.name}` : "-";
}
