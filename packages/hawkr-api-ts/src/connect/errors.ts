import { type DescMethod, isFieldSet, type Message } from "@bufbuild/protobuf";
import { MethodOptions_IdempotencyLevel } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
	RetryInfoSchema,
} from "../gen/google/rpc/error_details_pb";
import { ErrorReason } from "../gen/hawkr/common/v1alpha1/error_reason_pb";

// Connect 錯誤的分派（ADR backend/docs/adr/2026-09-25-api-error-model.md 的 D7）。classifyError 是純函式，
// 以 api/testdata/error-dispatch.json 的情境表測試，四個 client 的結果必須相同（website 與 admin 共用這一份，Dart 在
// packages/hawkr_api/lib/errors.dart）。factsFromError 是 connect-es 的轉接層。

/** ErrorInfo 的 domain：只有這個 domain 的 reason 是我們的 ErrorReason。 */
export const ERROR_DOMAIN = "hawkr";

export type ErrorAction =
	| "ignore"
	| "retry"
	| "reauthenticate"
	| "reason"
	| "fields"
	| "message";

/** 一個欄位的錯誤。field 是請求訊息的 proto 路徑，例如 `banner.title`。 */
export interface FieldError {
	field: string;
	/** 伺服器給使用者的欄位文字（LocalizedMessage）；undefined 表示沒有，由呼叫端補預設文字。 */
	message: string | undefined;
}

/** 分類器的輸入。 */
export interface ErrorFacts {
	code: Code;
	/** 沒有收到伺服器的回應（連線失敗、斷線）。 */
	noResponse: boolean;
	idempotency: MethodOptions_IdempotencyLevel;
	hasRequestId: boolean;
	reason?: string;
	reasonDomain?: string;
	/** ErrorInfo.metadata（API 設計規範 §八：key 是 lowerCamelCase，值是字串）；只在 ErrorInfo 的 domain 是我們的時候才有值。 */
	metadata: Readonly<Record<string, string>>;
	localizedMessage?: string;
	fieldErrors: FieldError[];
	retryDelayMs?: number;
}

export interface ErrorDecision {
	action: ErrorAction;
	/** action 是 retry 時：伺服器指定的等待時間；undefined 表示用退避。 */
	retryDelayMs?: number;
	reason?: ErrorReason;
	/** action 是 reason 時：該 reason 的 ErrorInfo.metadata（清單值用 metadataList 拆開）。 */
	metadata?: Readonly<Record<string, string>>;
	fields?: FieldError[];
	/** 伺服器給使用者的文字（LocalizedMessage）；undefined 表示顯示該 code 的預設文字。 */
	message?: string;
}

/** 依 D7 的順序分派：取消 → 可以安全重送而且可以重試 → 重新取得身分 → 認得的 reason → 欄位錯誤 → 文字。 */
export function classifyError(f: ErrorFacts): ErrorDecision {
	if (f.code === Code.Canceled) return { action: "ignore" };
	const safe =
		f.idempotency !== MethodOptions_IdempotencyLevel.IDEMPOTENCY_UNKNOWN ||
		f.hasRequestId;
	const retryable =
		f.code === Code.Unavailable ||
		f.noResponse ||
		(f.code === Code.ResourceExhausted && f.retryDelayMs !== undefined);
	if (safe && retryable) {
		return { action: "retry", retryDelayMs: f.retryDelayMs };
	}
	if (f.code === Code.Unauthenticated) return { action: "reauthenticate" };
	const reason =
		f.reasonDomain === ERROR_DOMAIN ? knownReason(f.reason) : undefined;
	if (reason !== undefined) {
		return {
			action: "reason",
			reason,
			metadata: f.metadata,
			message: f.localizedMessage,
		};
	}
	if (f.fieldErrors.length > 0) {
		return {
			action: "fields",
			fields: f.fieldErrors,
			message: f.localizedMessage,
		};
	}
	return { action: "message", message: f.localizedMessage };
}

/**
 * 沒有 LocalizedMessage 時顯示的該 code 的預設文字（D3，client 的呈現層）。文字在共用文案目錄
 * （`@hawkr/ui-text`，依賴本套件，所以由呼叫端注入）；Canceled 不顯示，不會傳進來。
 */
export type DefaultErrorText = (code: Exclude<Code, Code.Canceled>) => string;

/** 顯示給使用者的文字：伺服器的 LocalizedMessage，沒有時用 [defaults] 給的該 code 的預設文字；取消時是 undefined。 */
export function userMessage(
	err: unknown,
	defaults: DefaultErrorText,
): string | undefined {
	const e = ConnectError.from(err);
	if (e.code === Code.Canceled) return undefined;
	return e.findDetails(LocalizedMessageSchema)[0]?.message ?? defaults(e.code);
}

/**
 * 一定有字的 [userMessage]：伺服器的 LocalizedMessage，沒有時是該 code 的預設文字；取消（目錄沒有 canceled 的文字）當成
 * [Code.Internal]，與 `submitError` 和 Dart 的 `errorText` 同一條規則。呼叫點不各自補一句寫死的文字。
 */
export function errorText(err: unknown, defaults: DefaultErrorText): string {
	return userMessage(err, defaults) ?? defaults(Code.Internal);
}

/** 從 ConnectError 與呼叫的方法取出分類需要的事實。 */
export function factsFromError(
	err: ConnectError,
	method?: DescMethod,
	request?: Message,
): ErrorFacts {
	const info = err.findDetails(ErrorInfoSchema)[0];
	const retry = err.findDetails(RetryInfoSchema)[0]?.retryDelay;
	return {
		code: err.code,
		noResponse: isNoResponse(err),
		idempotency:
			method?.idempotency ?? MethodOptions_IdempotencyLevel.IDEMPOTENCY_UNKNOWN,
		hasRequestId: carriesRequestId(method, request),
		reason: info?.reason,
		reasonDomain: info?.domain,
		metadata: info?.domain === ERROR_DOMAIN ? { ...info.metadata } : {},
		localizedMessage: err.findDetails(LocalizedMessageSchema)[0]?.message,
		fieldErrors: err.findDetails(BadRequestSchema).flatMap((b) =>
			b.fieldViolations.map((v) => ({
				field: v.field,
				message: v.localizedMessage?.message,
			})),
		),
		retryDelayMs:
			retry === undefined
				? undefined
				: Number(retry.seconds) * 1000 + Math.floor(retry.nanos / 1e6),
	};
}

/**
 * 規範的清單值：以逗號連接。缺少或空字串是空清單；不排序、不轉型（順序由伺服器保證，值的型別由呼叫方解析）。
 */
export function metadataList(value: string | undefined): string[] {
	return value === undefined || value === "" ? [] : value.split(",");
}

/** 請求訊息有沒有帶 AIP-155 的 `request_id`。 */
export function carriesRequestId(
	method: DescMethod | undefined,
	request: Message | undefined,
): boolean {
	const field = method?.input.fields.find((f) => f.name === "request_id");
	return field !== undefined && request !== undefined
		? isFieldSet(request, field)
		: false;
}

function knownReason(name: string | undefined): ErrorReason | undefined {
	if (name === undefined) return undefined;
	const value = (ErrorReason as unknown as Record<string, number>)[name];
	return typeof value === "number" &&
		value !== ErrorReason.ERROR_REASON_UNSPECIFIED
		? value
		: undefined;
}

/**
 * 沒有收到回應的傳輸失敗（網路中斷、DNS、CORS、被擋掉）：fetch 失敗時丟 TypeError，connect-es 以 ConnectError.from 包成
 * Unknown、cause 是那個 TypeError；還沒被包過的 TypeError 也算。伺服器回的錯誤沒有這樣的 cause，解碼失敗的 cause 是
 * SyntaxError 或 Error，都不算。classifyError 的 noResponse 事實與各 client 對「網路失敗」的判斷共用這一支，不各自再寫一份。
 */
export function isNoResponse(err: unknown): boolean {
	if (err instanceof TypeError) return true;
	return err instanceof ConnectError && err.cause instanceof TypeError;
}
