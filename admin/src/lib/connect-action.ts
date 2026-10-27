import { Code, ConnectError } from "@connectrpc/connect";
import {
	classifyError,
	factsFromError,
	isNoResponse,
	userMessage,
} from "@hawkr/api/connect/errors";
import { submitError } from "@hawkr/api/connect/submit";
import type { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";

/** 版本衝突的說明：固定一句，或依錯誤的 ErrorInfo.metadata 挑（例如通報與案件各一句，看 `conflictTarget`）。 */
export type ConflictText =
	| string
	| ((metadata: Readonly<Record<string, string>>) => string);

/**
 * 送出的結果。失敗一律帶 `noResponse`（沒收到回應）與 `code`（Connect 的錯誤碼；沒收到回應時沒有）。`message` 失敗帶伺服器認得的
 * reason 時一併回 `reason` 與 `metadata`，呼叫端依 reason 做自己的處理（例如比對到既有資料）；沒收到回應或沒有認得的 reason 時兩者都沒有。
 */
export type ActionResult<T = unknown> =
	| { ok: true; value: T }
	| (ActionFailureBase & {
			kind: "message";
			reason?: ErrorReason;
			metadata?: Readonly<Record<string, string>>;
	  })
	| (ActionFailureBase & { kind: "conflict" });

interface ActionFailureBase {
	ok: false;
	text: string;
	code?: Code;
	noResponse: boolean;
}

/**
 * 送出失敗時給操作者的文字：伺服器的訊息（欄位錯誤合併成一句），網路層失敗或沒有訊息時用 fallback，
 * 版本衝突用 conflictText。
 */
export function actionErrorText(
	caught: unknown,
	fallback: string,
	conflictText: ConflictText,
): string {
	// 網路層失敗（沒有收到回應）的訊息對操作者沒有意義，用 fallback。
	if (isNoResponse(caught)) return fallback;
	const failure = submitError(caught, defaultErrorTextZhTW);
	if (failure.kind === "conflict") {
		return typeof conflictText === "function"
			? conflictText(failure.metadata)
			: conflictText;
	}
	const text =
		failure.kind === "fields"
			? (failure.message ?? Object.values(failure.fields)[0] ?? fallback)
			: failure.message;
	return text || fallback;
}

export interface RunActionOptions {
	/** 版本衝突後重讀畫面的資料；沒有就不重讀（沒有版本比對的呼叫，例如建立）。 */
	reload?: () => Promise<unknown>;
	fallback: string;
	conflictText: ConflictText;
}

/**
 * 送出一個後台操作，成功時回傳寫入的回應。VERSION_CONFLICT（別人先改過）時有 reload 就先重讀再回傳 conflict；
 * conflictText 的說法（「頁面已重新載入」）只在重讀成功時才成立：沒有 reload 或重讀失敗時改回伺服器自己的句子
 * （只要求重新讀取，沒有說已經重新載入）。其他失敗回 actionErrorText。不丟例外：呼叫端依結果決定要不要關對話框或清草稿。
 */
export async function runAction<T>(
	call: () => Promise<T>,
	{ reload, fallback, conflictText }: RunActionOptions,
): Promise<ActionResult<T>> {
	try {
		return { ok: true, value: await call() };
	} catch (caught) {
		const noResponse = isNoResponse(caught);
		const code = noResponse ? undefined : ConnectError.from(caught).code;
		if (
			!noResponse &&
			submitError(caught, defaultErrorTextZhTW).kind === "conflict"
		) {
			const reloaded = reload
				? await reload().then(
						() => true,
						() => false,
					)
				: false;
			const text = reloaded
				? actionErrorText(caught, fallback, conflictText)
				: (userMessage(caught, defaultErrorTextZhTW) ??
					defaultErrorTextZhTW(Code.Aborted));
			return { ok: false, kind: "conflict", text, code, noResponse };
		}
		const text = actionErrorText(caught, fallback, conflictText);
		if (noResponse) return { ok: false, kind: "message", text, noResponse };
		const decision = classifyError(factsFromError(ConnectError.from(caught)));
		if (decision.action === "reason") {
			return {
				ok: false,
				kind: "message",
				text,
				code,
				noResponse,
				reason: decision.reason,
				metadata: decision.metadata,
			};
		}
		return { ok: false, kind: "message", text, code, noResponse };
	}
}
