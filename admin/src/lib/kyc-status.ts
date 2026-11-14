import { KYCStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerKYCStatus } from "@hawkr/ui-text/keys";
import { staffTextOrUnknown } from "@/lib/ui-text";

/** 店家目前的 KYC 申請狀態；`kyc_status` 沒有設表示店家還沒有申請（契約是 optional），UNSPECIFIED 與不認得的值是「未知」。 */
export function kycStatusText(status: KYCStatus | undefined): string {
	if (status === undefined) return "尚未申請";
	return staffTextOrUnknown(sellerKYCStatus(status));
}

/** 申請已由平台核准（人工決定）。 */
export function isKycApproved(status: KYCStatus | undefined): boolean {
	return status === KYCStatus.KYC_STATUS_APPROVED;
}
