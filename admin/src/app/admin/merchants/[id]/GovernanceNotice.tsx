"use client";

import { ConnectError } from "@connectrpc/connect";
import { isNoResponse } from "@hawkr/api/connect/errors";
import { submitError } from "@hawkr/api/connect/submit";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import type { Ref } from "react";
import { errorMessage } from "@/lib/error-text";
import { ApiClientError } from "@/services/api-error";

/**
 * 商家詳情頁的治理操作結果（停權／解除／退回／核准／強制下架／刪除帳號共用一份）。
 *
 * 外觀完全沿用 `DelegatedOperations.css` 既有的兩個 class，不另立第三套視覺：
 * 成功走 `.adm-authorization-notice`、失敗走 `.adm-operation-error`。
 */
export interface GovernanceNotice {
	tone: "success" | "error";
	/** 一句話；要分開排的幾句（各自完整，client 不拼接也不補標點）傳陣列，每句一行。 */
	text: string | readonly string[];
}

function NoticeText({ text }: { text: GovernanceNotice["text"] }) {
	if (typeof text === "string") return <div>{text}</div>;
	return (
		<div>
			{text.map((line) => (
				<div key={line}>{line}</div>
			))}
		</div>
	);
}

/**
 * 後端 envelope 的 `message` 是寫給操作者看的業務說明，照原樣顯示並補上業務碼與
 * traceId——客服要追一筆治理操作靠的就是這兩個值。
 *
 * `fetch` 的網路層失敗是 `TypeError`（"Failed to fetch"），那句話對操作者沒有任何
 * 意義，換成呼叫端給的那一句；`api.ts` 自己包過的逾時／401 則已是中文，照用。
 */
export function governanceErrorText(caught: unknown, fallback: string): string {
	if (!(caught instanceof ApiClientError)) {
		return caught instanceof Error && !isNoResponse(caught)
			? caught.message
			: fallback;
	}
	const code = typeof caught.code === "number" ? `（代碼 ${caught.code}）` : "";
	const trace = caught.traceId ? `（追蹤編號 ${caught.traceId}）` : "";
	return `${caught.message || fallback}${code}${trace}`;
}

export function GovernanceNoticeBanner({
	notice,
	ref,
}: {
	notice: GovernanceNotice | null;
	ref?: Ref<HTMLDivElement>;
}) {
	if (!notice) return null;
	if (notice.tone === "success") {
		return (
			<div ref={ref} className="adm-authorization-notice" role="status">
				<MaterialSymbol name="check_circle" />
				<NoticeText text={notice.text} />
			</div>
		);
	}
	return (
		<div ref={ref} className="adm-operation-error" role="alert">
			<MaterialSymbol name="error" />
			<NoticeText text={notice.text} />
		</div>
	);
}
