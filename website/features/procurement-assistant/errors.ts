import { Code, ConnectError } from "@connectrpc/connect";
import {
	classifyError,
	type DefaultErrorText,
	errorText,
	factsFromError,
} from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";

export interface ProcurementError {
	code: string;
	detail: string;
}

/**
 * 這一輪打在一個已經不存在的 request 上——對話被刪掉（後端 2026-08-11：
 * 軟刪的對話不再接受新 turn）、或深連結指向別人的／不存在的需求。後端一律
 * 回 404 `"request not found"`，那是內部字串，不能直接端到使用者面前。
 */
export const CONVERSATION_GONE: ProcurementError = {
	code: "CONVERSATION_GONE",
	detail: "這個對話已經不在了，請開一個新對話重新開始。",
};

/** 後端把「找不到 request」表達成 404＋這句英文。 */
export function isRequestNotFound(message: string): boolean {
	return message.toLowerCase().includes("request not found");
}

/**
 * 把一個 ApiClientError 收斂成畫面用的錯誤。turns／answers／rewinds 三個端點
 * 都是對同一個 requestId 下命令，所以「request 不在了」對三者是同一件事，
 * 統一在這裡翻譯，不在每個呼叫點各寫一次。
 */
export function procurementErrorFromStatus(
	status: number,
	reason: string,
	message: string,
): ProcurementError {
	if (status === 404 || isRequestNotFound(message)) return CONVERSATION_GONE;
	return parseProcurementError(reason, message);
}

/**
 * procurement 端點的業務錯誤現在是 {code, reason, message} 三個獨立欄位
 * （backend-payuni AIProcurementService 遷移，2026-08-16）——reason 是機器碼，
 * message 是純人看文字，不再需要切字串還原。
 */
export function parseProcurementError(
	reason: string,
	message: string,
): ProcurementError {
	if (!reason) {
		return { code: "UNKNOWN", detail: message };
	}
	return { code: reason, detail: message };
}

/** 買家採購操作的失敗：`code` 給畫面分支用，`detail` 是伺服器的 LocalizedMessage 或該 Connect code 的預設文字。 */
export interface ProcurementFailure {
	code: ProcurementFailureCode;
	detail: string;
	/** 以同一個 client_request_id 原操作重送有意義（沒有收到回應、unavailable、帶 RetryInfo 的 resource_exhausted、逾時）。 */
	retryable: boolean;
	/** RFQ_CHECKOUT_ALREADY_ACTIVE 的既有訂單編號（ErrorInfo.metadata 的 orderNo）。 */
	orderNo?: string;
}

export type ProcurementFailureCode =
	| "STALE_REVISION"
	| "STALE_CANDIDATE_SET"
	| "STALE_APPROVAL"
	| "STALE_PUBLICATION"
	| "STALE_SELLER_RESPONSE"
	| "VERSION_CONFLICT"
	| "REQUIREMENT_NOT_READY"
	| "PROCUREMENT_RUN_IN_PROGRESS"
	| "APPROVAL_PENDING"
	| "APPROVAL_ALREADY_RESOLVED"
	| "RUN_NOT_ACTIVE"
	| "RFQ_RESPONSE_EXPIRED"
	| "RFQ_RECIPIENT_CLOSED"
	| "RFQ_CHECKOUT_ALREADY_ACTIVE"
	| "AGENT_MONTHLY_QUOTA_EXCEEDED"
	| "REPLAY_CURSOR_EXPIRED"
	| "REPLAY_CURSOR_AHEAD"
	| "INVALID_ARGUMENT"
	| "CONVERSATION_GONE"
	| "UNAUTHENTICATED"
	| "FORBIDDEN"
	| "RATE_LIMITED"
	| "NETWORK_ERROR"
	| "SERVER_ERROR"
	| "INVALID_RESPONSE"
	| "STALE_CONTEXT"
	| "UNKNOWN_ERROR";

const CONFLICT_TARGET_CODE: Record<string, ProcurementFailureCode> = {
	request_revision: "STALE_REVISION",
	candidate_set: "STALE_CANDIDATE_SET",
	publication_approval: "STALE_APPROVAL",
	publication: "STALE_PUBLICATION",
	seller_response: "STALE_SELLER_RESPONSE",
};

const REASON_CODE: Partial<Record<ErrorReason, ProcurementFailureCode>> = {
	[ErrorReason.REQUIREMENT_NOT_READY]: "REQUIREMENT_NOT_READY",
	[ErrorReason.PROCUREMENT_RUN_IN_PROGRESS]: "PROCUREMENT_RUN_IN_PROGRESS",
	[ErrorReason.APPROVAL_PENDING]: "APPROVAL_PENDING",
	[ErrorReason.APPROVAL_ALREADY_RESOLVED]: "APPROVAL_ALREADY_RESOLVED",
	[ErrorReason.RUN_NOT_ACTIVE]: "RUN_NOT_ACTIVE",
	[ErrorReason.RFQ_RESPONSE_EXPIRED]: "RFQ_RESPONSE_EXPIRED",
	[ErrorReason.RFQ_RECIPIENT_CLOSED]: "RFQ_RECIPIENT_CLOSED",
	[ErrorReason.RFQ_CHECKOUT_ALREADY_ACTIVE]: "RFQ_CHECKOUT_ALREADY_ACTIVE",
	[ErrorReason.AGENT_MONTHLY_QUOTA_EXCEEDED]: "AGENT_MONTHLY_QUOTA_EXCEEDED",
	[ErrorReason.REPLAY_CURSOR_EXPIRED]: "REPLAY_CURSOR_EXPIRED",
	[ErrorReason.REPLAY_CURSOR_AHEAD]: "REPLAY_CURSOR_AHEAD",
};

export function failureCode(err: ConnectError): ProcurementFailureCode {
	const facts = factsFromError(err);
	if (facts.noResponse) return "NETWORK_ERROR";
	const decision = classifyError(facts);
	if (decision.action === "reason" && decision.reason !== undefined) {
		if (decision.reason === ErrorReason.VERSION_CONFLICT) {
			return (
				CONFLICT_TARGET_CODE[decision.metadata?.conflictTarget ?? ""] ??
				"VERSION_CONFLICT"
			);
		}
		const byReason = REASON_CODE[decision.reason];
		if (byReason) return byReason;
	}
	switch (err.code) {
		case Code.NotFound:
			return "CONVERSATION_GONE";
		case Code.InvalidArgument:
			return "INVALID_ARGUMENT";
		case Code.Unauthenticated:
			return "UNAUTHENTICATED";
		case Code.PermissionDenied:
			return "FORBIDDEN";
		case Code.ResourceExhausted:
			return "RATE_LIMITED";
		case Code.Unavailable:
		case Code.DeadlineExceeded:
		case Code.Internal:
		case Code.DataLoss:
			return "SERVER_ERROR";
		default:
			return "UNKNOWN_ERROR";
	}
}

export function failureRetryable(err: ConnectError): boolean {
	const facts = factsFromError(err);
	return (
		facts.noResponse ||
		err.code === Code.Unavailable ||
		err.code === Code.DeadlineExceeded ||
		(err.code === Code.ResourceExhausted && facts.retryDelayMs !== undefined)
	);
}

/** Connect 呼叫的任何丟出值收斂成畫面用的失敗；文字不在這裡寫死。 */
export function procurementFailureFrom(
	err: unknown,
	defaults: DefaultErrorText,
): ProcurementFailure {
	const connectErr = ConnectError.from(err);
	const failure: ProcurementFailure = {
		code: failureCode(connectErr),
		detail: errorText(connectErr, defaults),
		retryable: failureRetryable(connectErr),
	};
	const orderNo = factsFromError(connectErr).metadata.orderNo;
	if (failure.code === "RFQ_CHECKOUT_ALREADY_ACTIVE" && orderNo) {
		failure.orderNo = orderNo;
	}
	return failure;
}

/** 是不是「需求版本、候選集或核准已過時」這一類要重讀狀態再操作的衝突。 */
export function isStaleConflict(failure: ProcurementFailure): boolean {
	return (
		failure.code === "STALE_REVISION" ||
		failure.code === "STALE_CANDIDATE_SET" ||
		failure.code === "STALE_APPROVAL" ||
		failure.code === "STALE_PUBLICATION" ||
		failure.code === "STALE_SELLER_RESPONSE" ||
		failure.code === "VERSION_CONFLICT"
	);
}

/** 回應缺了必要欄位：不猜、不補值，顯示該 Connect code 的預設文字。 */
export function invalidResponseFailure(
	defaults: DefaultErrorText,
): ProcurementFailure {
	return {
		code: "INVALID_RESPONSE",
		detail: defaults(Code.Internal),
		retryable: false,
	};
}
