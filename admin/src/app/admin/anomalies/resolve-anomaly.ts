import type {
	Payment,
	PaymentRefund,
	PaymentTransfer,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentAnomalyKind,
	PaymentAnomalyOutcome,
	PaymentRefundStatus,
	PaymentReversalStatus,
	PaymentStatus,
	PaymentTopupStatus,
	PaymentTransferStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";

// 異常的人工結案（PaymentAdminService.ResolvePaymentAnomaly）：哪些物件可以結案、表單的驗證與請求。
// 契約：id 是付款、轉帳、退款的 ID；收回是撤回列 ID；補差額是退款 ID。成功結案要附金流商物件 ID，失敗結案可省略。

/** 列上可結案的位置；稱呼用 `TARGET_NOUN[slot]`，不從 enum 成員選文字（這個 enum 的文字不在共用目錄）。 */
export type ResolveSlot =
	| "payment"
	| "transfer"
	| "refund"
	| "reversal"
	| "topup";

export const TARGET_NOUN: Readonly<Record<ResolveSlot, string>> = {
	payment: "付款",
	transfer: "轉帳",
	refund: "退款",
	reversal: "收回",
	topup: "補差額",
};

const KIND_OF_SLOT: Readonly<Record<ResolveSlot, PaymentAnomalyKind>> = {
	payment: PaymentAnomalyKind.PAYMENT,
	transfer: PaymentAnomalyKind.TRANSFER,
	refund: PaymentAnomalyKind.REFUND,
	reversal: PaymentAnomalyKind.REVERSAL,
	topup: PaymentAnomalyKind.TOPUP,
};

export interface ResolveTarget {
	slot: ResolveSlot;
	/** 物件 ID：付款、轉帳、退款的 ID；收回是撤回列 ID；補差額是退款 ID。 */
	id: string;
}

/** 核對的結果；空字串是還沒選。 */
export type ResolveOutcomeKey = "" | "succeeded" | "failed";

export interface ResolveDraft {
	outcome: ResolveOutcomeKey;
	evidence: string;
}

export const EMPTY_RESOLVE_DRAFT: ResolveDraft = { outcome: "", evidence: "" };

export type ResolveInvalid = "outcome" | "evidence";

export const RESOLVE_INVALID_TEXT: Readonly<Record<ResolveInvalid, string>> = {
	outcome: "請選擇核對的結果。",
	evidence: "結案成功要填金流商的物件 ID。",
};

export function paymentResolveTargets(p: Payment): ResolveTarget[] {
	return p.status === PaymentStatus.ANOMALY
		? [{ slot: "payment", id: p.id }]
		: [];
}

export function transferResolveTargets(t: PaymentTransfer): ResolveTarget[] {
	return t.status === PaymentTransferStatus.ANOMALY
		? [{ slot: "transfer", id: t.id }]
		: [];
}

/** 一筆退款可以同時有退款、收回（每個撤回列各一個）、補差額三種異常。 */
export function refundResolveTargets(r: PaymentRefund): ResolveTarget[] {
	const targets: ResolveTarget[] = [];
	if (r.status === PaymentRefundStatus.ANOMALY) {
		targets.push({ slot: "refund", id: r.id });
	}
	for (const reversal of r.reversals) {
		if (reversal.status === PaymentReversalStatus.ANOMALY) {
			targets.push({ slot: "reversal", id: reversal.id });
		}
	}
	if (r.topupStatus === PaymentTopupStatus.ANOMALY) {
		targets.push({ slot: "topup", id: r.id });
	}
	return targets;
}

/** 回傳第一個不合的欄位；合法回 null。證據只在成功結案時必填。 */
export function validateResolveDraft(
	draft: ResolveDraft,
): ResolveInvalid | null {
	if (draft.outcome === "") return "outcome";
	if (draft.outcome === "succeeded" && draft.evidence.trim() === "") {
		return "evidence";
	}
	return null;
}

const OUTCOME_OF_KEY: Readonly<
	Record<"succeeded" | "failed", PaymentAnomalyOutcome>
> = {
	succeeded: PaymentAnomalyOutcome.SUCCEEDED,
	failed: PaymentAnomalyOutcome.FAILED,
};

/** 失敗結案不帶證據：選過成功、填了 ID 又改選失敗時，輸入框的內容不能漏進請求。 */
export function resolveAnomalyRequest(
	target: ResolveTarget,
	draft: ResolveDraft,
) {
	if (draft.outcome === "") throw new Error("結案前要先選結果");
	return {
		kind: KIND_OF_SLOT[target.slot],
		id: target.id,
		outcome: OUTCOME_OF_KEY[draft.outcome],
		evidence: draft.outcome === "succeeded" ? draft.evidence.trim() : "",
	};
}
