"use client";

import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type KYCApplication,
	KYCStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	commonUnknown,
	sellerKYCApplicationDecideTime,
	sellerKYCApplicationSubmitTime,
	sellerKYCStatus,
} from "@hawkr/ui-text/keys";
import Button from "@/components/ui/Button";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { optionalText, useUiText } from "@/lib/i18n/ui-text";
import { kycCancellable, kycStatusTone } from "@/lib/seller/kyc";

type Stamp = { kind: "decide" | "submit"; at: Timestamp };

/** 已通過看核准時間、審核中看送出時間；其他狀態沒有時間。 */
function timeOf(
	application: KYCApplication | null,
	status: KYCStatus | undefined,
): Stamp | undefined {
	if (status === KYCStatus.KYC_STATUS_APPROVED && application?.decideTime) {
		return { kind: "decide", at: application.decideTime };
	}
	if (status === KYCStatus.KYC_STATUS_SUBMITTED && application?.submitTime) {
		return { kind: "submit", at: application.submitTime };
	}
	return undefined;
}

/**
 * 「平台審核」區（D15）：讀 `KYCApplication.status`，徽章文字取自共用文案目錄。審核中可以取消申請；
 * 已通過顯示核准時間。被退回與草稿在表單頁顯示，不會走到這裡。
 */
export function PlatformReviewSection({
	application,
	status,
	submitted,
	cancelling,
	onCancel,
}: {
	application: KYCApplication | null;
	/** 畫面上顯示的狀態：店家已認證時一律是已通過。 */
	status: KYCStatus | undefined;
	/** 剛送出（這個畫面送出的）：多一句通知說明。 */
	submitted: boolean;
	cancelling: boolean;
	onCancel: () => void;
}) {
	const ut = useUiText();
	const known = status !== undefined;
	const badge = known
		? (optionalText(ut, sellerKYCStatus(status)) ?? ut(commonUnknown()))
		: "未送出";
	const tone = known ? (kycStatusTone(status) ?? "neutral") : "neutral";
	const inReview = kycCancellable(status);
	const stamp = timeOf(application, status);
	return (
		<section className="seller-card" aria-label="平台審核">
			<header className="seller-card__head">
				<h2>平台審核</h2>
			</header>
			<div className={`seller-cert__status is-${tone}`}>
				<strong>{badge}</strong>
				{stamp?.kind === "decide" && (
					<p>
						{ut(
							sellerKYCApplicationDecideTime({
								time: formatTaipeiDateTime(timestampDate(stamp.at)),
							}),
						)}
					</p>
				)}
				{stamp?.kind === "submit" && (
					<p>
						{ut(
							sellerKYCApplicationSubmitTime({
								time: formatTaipeiDateTime(timestampDate(stamp.at)),
							}),
						)}
					</p>
				)}
				{submitted ? <p>審核完成後會以通知告知你結果。</p> : null}
			</div>
			{inReview && (
				<Button
					variant="bordered"
					size="sm"
					onClick={onCancel}
					isLoading={cancelling}
					disabled={cancelling}
					loadingLabel="取消申請中…"
				>
					取消申請
				</Button>
			)}
		</section>
	);
}
