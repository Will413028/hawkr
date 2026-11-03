"use client";

import {
	type KYCApplication,
	KYCStatus,
	type PaymentAccount,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerKYCStatus } from "@hawkr/ui-text/keys";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { optionalText, useUiText } from "@/lib/i18n/ui-text";
import { CERT_PAGE_HREF } from "@/lib/seller/cert-gate";
import { paymentAccountCopy } from "@/lib/seller/payment-account";

interface Props {
	/** 申請（`useCertGate().application`）；沒有申請是 null。 */
	application?: KYCApplication | null;
	/** 收款帳戶（`useCertGate().paymentAccount`）；平台審核核准之前沒有。 */
	paymentAccount?: PaymentAccount | null;
	/** Subject the seller is being blocked from doing — used to template
	 * the default message ("通過後就能{subject}"). Default: "繼續". */
	subject?: string;
	/** Optional override when a page needs a more specific heading. */
	title?: string;
	/** Optional extra class for page-specific spacing hooks. */
	className?: string;
	detail?: string;
}

/**
 * Banner shown in place of a workspace surface when the seller hasn't
 * completed business certification. Used by the wallet and shipping gates so
 * the pre-certification chrome stays identical. 狀態與說明來自共用文案目錄：
 * 平台審核核准後是收款帳戶的說明與動作，之前是申請狀態的名詞。
 */
export function SellerCertGate({
	application,
	paymentAccount,
	subject = "繼續",
	title,
	className,
	detail,
}: Props) {
	const ut = useUiText();
	const approved = application?.status === KYCStatus.KYC_STATUS_APPROVED;
	const payment =
		approved && paymentAccount ? paymentAccountCopy(ut, paymentAccount) : null;
	const reviewStatus = application
		? optionalText(ut, sellerKYCStatus(application.status))
		: undefined;
	const message = payment?.description
		? payment.description
		: `通過平台審核並啟用收款帳戶後，就能${subject}。`;
	const status = payment?.badge ?? reviewStatus ?? "未送出";
	const actionLabel = payment?.action ?? "前往商家認證";

	return (
		<div
			className={["seller-cert-gate", className].filter(Boolean).join(" ")}
			role="status"
		>
			<div className="seller-cert-gate__icon" aria-hidden="true">
				<AppIcon name="shield" size={40} />
			</div>
			<div className="seller-cert-gate__body">
				<span className="seller-cert-gate__eyebrow">商家認證</span>
				<strong>{title ?? "尚未通過商家認證"}</strong>
				<p>{message}</p>
				<p className="seller-cert-gate__detail">目前狀態：{status}</p>
				{detail ? <p className="seller-cert-gate__detail">{detail}</p> : null}
			</div>
			<Button href={CERT_PAGE_HREF} className="seller-cert-gate__action">
				{actionLabel}
			</Button>
		</div>
	);
}
