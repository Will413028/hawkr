"use client";

import type { KYCFlow } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { useUiText } from "@/lib/i18n/ui-text";
import { CERT_PAGE_HREF } from "@/lib/seller/cert-gate";
import { kycFlowGate } from "@/lib/seller/kyc-flow";

interface Props {
	/** 申請的流程（`useCertGate().flow`）；決定提示的文字與按鈕，有 PayUni 狀態時用 PayUni 的標題與說明。 */
	flow?: KYCFlow | null;
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
 * the pre-certification chrome stays identical.
 */
export function SellerCertGate({
	flow,
	subject = "繼續",
	title,
	className,
	detail,
}: Props) {
	const ut = useUiText();
	const { copy, action } = kycFlowGate(ut, flow);
	const message = copy.message(subject);
	const heading = title ?? copy.heading;
	const actionLabel = action;

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
				<strong>{heading}</strong>
				<p>{message}</p>
				{detail ? <p className="seller-cert-gate__detail">{detail}</p> : null}
			</div>
			<Button href={CERT_PAGE_HREF} className="seller-cert-gate__action">
				{actionLabel}
			</Button>
		</div>
	);
}
