import { create } from "@bufbuild/protobuf";
import {
	MeasureActionUnavailabilitySchema,
	type MeasureActionUnavailableReason as Reason,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";

/** 伺服器對每個原因給的文字（backend model.ActionUnavailableText）：測試只當伺服器回應的資料，client 不持有這張表。 */
export const SERVER_TEXT = {
	REVOKED: "這筆措施已撤銷",
	NOT_ACTIVE: "這筆措施不在生效中",
	KIND_HAS_NO_END_TIME: "警告沒有結束時間",
	TERMINATION_RECREATE: "終止合作要改期間，請撤銷後重建",
	FROZEN_STATEMENT:
		"賣家提出過申覆，或有待處理的解除請求，給賣家的理由不能再改",
	FROZEN_EXTEND: "賣家提出過申覆，或有待處理的解除請求，結束時間不能再延長",
	APPEAL_PENDING: "申覆待裁定，請先裁定",
} as const;

/** 伺服器回的一個原因：`message` 省略表示伺服器沒有給文字（例如舊版伺服器或這個 client 不認得的新原因）。 */
export function why(reason: Reason, message?: string) {
	return create(MeasureActionUnavailabilitySchema, {
		reason,
		message: message === undefined ? undefined : { locale: "zh-TW", message },
	});
}
