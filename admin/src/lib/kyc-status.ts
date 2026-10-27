import { KYCStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { unknownText } from "@/lib/ui-text";

const KYC_STATUS_LABEL: Record<
	Exclude<KYCStatus, KYCStatus.KYC_STATUS_UNSPECIFIED>,
	string
> = {
	[KYCStatus.KYC_STATUS_DRAFT]: "填寫中",
	[KYCStatus.KYC_STATUS_SUBMITTED]: "待審核",
	[KYCStatus.KYC_STATUS_APPROVED]: "已核准",
	[KYCStatus.KYC_STATUS_REJECTED]: "已退回",
	[KYCStatus.KYC_STATUS_CANCELLED]: "已取消",
};

/** 店家目前的 KYC 申請狀態；`kyc_status` 沒有設表示店家還沒有申請（契約是 optional），UNSPECIFIED 與不認得的值是「未知」。 */
export function kycStatusText(status: KYCStatus | undefined): string {
	if (status === undefined) return "尚未申請";
	return (
		(KYC_STATUS_LABEL as Partial<Record<KYCStatus, string>>)[status] ??
		unknownText()
	);
}
