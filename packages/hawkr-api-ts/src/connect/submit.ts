import type { DescMethod } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	classifyError,
	type DefaultErrorText,
	factsFromError,
} from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";

/** 後台表單送出失敗時要做的事（錯誤模型 ADR 2026-09-25-api-error-model D7）。 */
export type SubmitError =
	/**
	 * 版本衝突：保留使用者的輸入，顯示最新版本讓使用者對照後再送出。metadata 是該錯誤的 ErrorInfo.metadata（沒有時是空物件），
	 * 例如治理 API 的 `conflictTarget` 說是哪個物件的 etag 過時。
	 */
	| { kind: "conflict"; metadata: Readonly<Record<string, string>> }
	/** 欄位錯誤：key 是請求訊息的 proto 路徑（例如 `banner.title`）。 */
	| { kind: "fields"; fields: Record<string, string>; message?: string }
	| { kind: "message"; message: string };

/**
 * 把送出失敗的 [err] 分類成表單要做的事。沒有伺服器文字時的預設文字由 [defaults] 給（目錄在
 * `@hawkr/ui-text`，不能由本套件依賴）；欄位沒有文字時用 InvalidArgument 的預設文字。
 */
export function submitError(
	err: unknown,
	defaults: DefaultErrorText,
	method?: DescMethod,
): SubmitError {
	const e = ConnectError.from(err);
	const d = classifyError(factsFromError(e, method));
	if (d.action === "reason" && d.reason === ErrorReason.VERSION_CONFLICT) {
		return { kind: "conflict", metadata: d.metadata ?? {} };
	}
	if (d.action === "fields" && d.fields) {
		return {
			kind: "fields",
			fields: Object.fromEntries(
				d.fields.map((f) => [
					f.field,
					f.message ?? defaults(Code.InvalidArgument),
				]),
			),
			message: d.message,
		};
	}
	return {
		kind: "message",
		message:
			d.message ?? defaults(e.code === Code.Canceled ? Code.Internal : e.code),
	};
}

/**
 * 某個畫面欄位的錯誤文字：第一個符合的路徑，或以它開頭的子路徑（`banner.action` 也涵蓋 `banner.action.url`）。
 */
export function fieldMessage(
	fields: Record<string, string>,
	...paths: string[]
): string | undefined {
	for (const path of paths) {
		for (const [field, message] of Object.entries(fields)) {
			if (
				field === path ||
				field.startsWith(`${path}.`) ||
				field.startsWith(`${path}[`)
			) {
				return message;
			}
		}
	}
	return undefined;
}
