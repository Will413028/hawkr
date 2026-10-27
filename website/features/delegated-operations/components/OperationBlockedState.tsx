"use client";

import Button from "@/components/ui/Button";
import { delegatedSessionStateCopy } from "../labels";
import type { DelegatedSessionState } from "../state/session";

/**
 * 工作階段不可用時的阻擋畫面（mockup §5.1）。
 *
 * 顯示的是**狀態語意**，不是 gateway 的 message —— message 一律不透傳，同一個碼
 * 在兩個地方各有一份文案就會漂移。
 *
 * `expired`／`revoked`／`target_mismatch`／`invalid` 的唯一出口是返回 ADM：代行連結
 * 是單次的，在這個畫面上重試沒有任何意義。**`end_unknown` 是例外**——那個狀態下
 * credential 與工作階段刻意都還活著（伺服器沒能證明終止就不清），重試結束是真的
 * 做得到的事，文案（labels.ts）也是這樣寫的。封鎖改成活得過重新整理之後，少了這顆
 * 鈕就會把那句承諾變成一條死路。
 */
export default function OperationBlockedState({
	state,
	returnUrl,
	onRetryEnd,
	retryingEnd = false,
}: {
	state: DelegatedSessionState;
	returnUrl: string;
	onRetryEnd?: () => void;
	retryingEnd?: boolean;
}) {
	const copy = delegatedSessionStateCopy(state);
	const canRetryEnd = state === "end_unknown" && Boolean(onRetryEnd);
	return (
		<div className="adm-blocked">
			<div className="adm-blocked__card" role="alert">
				<h1 className="adm-blocked__title">{copy.title}</h1>
				<p className="adm-blocked__body">{copy.description}</p>
				{canRetryEnd && (
					<Button
						variant="bordered"
						className="self-center"
						isLoading={retryingEnd}
						loadingLabel="結束中…"
						onClick={onRetryEnd}
					>
						重試結束代行
					</Button>
				)}
				{/* `data-hkui-button`：`.adm-blocked__link` 自帶實心底＋白字（0-1-0），
				    會被 `[data-theme="dark"] body a:not([data-hkui-button])`（0-2-2）改成
				    `--color-ink-200`。這是本畫面唯一的出口鈕，不能讓它讀起來像一行內文連結。 */}
				<a data-hkui-button="" className="adm-blocked__link" href={returnUrl}>
					← 返回 ADM
				</a>
			</div>
		</div>
	);
}
