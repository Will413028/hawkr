"use client";

import type { SellerPayUni } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	sellerSellerPayUniFixFieldsHeading,
	sellerSellerPayUniReasonHeading,
} from "@hawkr/ui-text/keys";
import { useUiText } from "@/lib/i18n/ui-text";
import { payuniFixFieldLabels } from "@/lib/seller/payuni";

/**
 * PayUni 狀態的內容：伺服器的說明句、PayUni 的審核意見原文（第三方原文不翻譯，容器標 zh-TW）與要修正的欄位清單。
 * 標題由呼叫端依版面渲染（`payuniStateTitle`）；審核意見為空、清單為空時對應的段落不出現。
 */
export function PayuniStatusBody({ payuni }: { payuni: SellerPayUni }) {
	const ut = useUiText();
	const reason = payuni.reason.trim();
	const labels = payuniFixFieldLabels(ut, payuni);
	return (
		<>
			{payuni.explanation?.message ? <p>{payuni.explanation.message}</p> : null}
			{reason ? (
				<>
					<strong>{ut(sellerSellerPayUniReasonHeading())}</strong>
					<p lang="zh-TW">{reason}</p>
				</>
			) : null}
			{labels.length > 0 ? (
				<>
					<strong>{ut(sellerSellerPayUniFixFieldsHeading())}</strong>
					<ul className="list-disc pl-5">
						{labels.map((label) => (
							<li key={label}>{label}</li>
						))}
					</ul>
				</>
			) : null}
		</>
	);
}
