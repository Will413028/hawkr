"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useState } from "react";

/**
 * 治理台上所有「把這個值貼進工單或 log 查詢」的複製鍵。
 *
 * 只有一份的理由是行為而不是樣式：成功要在 1.5 秒內把圖示換成打勾再換回來，而
 * `navigator.clipboard` 在非安全來源、或使用者拒絕授權時會 reject —— 那時必須留在原
 * 圖示，不能給一個「已複製」的假回饋（值其實還在剪貼簿之外）。每多手寫一顆複製鍵，
 * 就多一處會漏掉這件事，而漏掉的那一顆看起來與正常的完全一樣。
 */
export default function CopyIconButton({
	value,
	label,
}: {
	value: string;
	label: string;
}) {
	const [copied, setCopied] = useState(false);
	if (!value) return null;
	return (
		<button
			type="button"
			className="adm-audit-copy-button"
			aria-label={`複製${label}`}
			onClick={() => {
				navigator.clipboard
					?.writeText(value)
					.then(() => {
						setCopied(true);
						setTimeout(() => setCopied(false), 1500);
					})
					.catch(() => setCopied(false));
			}}
		>
			<MaterialSymbol name={copied ? "check" : "content_copy"} />
		</button>
	);
}
