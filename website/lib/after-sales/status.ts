/**
 * After-sale status helpers — labels, tone, grouping, progress steps and the
 * "who acts next" narrative. Shared by buyer + seller UI so a status only ever
 * has one source of truth.
 *
 * Status / tone matrix (docs/planning/AFTER_SALES_UI_PLAN.md §狀態與 Tone 對照):
 *   0  待處理      pending   warning  賣家處理
 *   20 待退貨      returning info     買家寄回
 *   25 退貨運送中  returning info     等物流
 *   30 已收退貨    pending   warning  賣家確認收貨
 *   40 退款處理中  refunding warning  等退款 / ATM 填寫
 *   50 已結案      closed    success  查看詳情
 *   60 已拒絕      closed    danger   買家申訴
 *   70 已取消      closed    muted    查看詳情
 */

import { AfterSaleAppealState } from "@/lib/codemeta/generated";
import {
	AFTER_SALE_STATUS,
	type AfterSaleCase,
	type AfterSaleGroup,
	type AfterSaleReturnLogistics,
	type AfterSaleTone,
	REFUND_TASK_STATUS,
} from "./types";

/** codemeta `AfterSaleCaseStatus` display labels. */
export const AFTER_SALE_STATUS_LABEL: Record<string, string> = {
	"0": "待處理",
	"20": "待退貨",
	"25": "退貨運送中",
	"26": "超商待取",
	"27": "退貨已送達",
	"28": "退貨失敗",
	"30": "已收退貨",
	"33": "待寄回逾時自動關案",
	"40": "退款處理中",
	"50": "已結案",
	"60": "已拒絕",
	"70": "已取消",
};

/** PayUni WorkerTaskStatus (`refundTaskStatus`) display labels. */
export const REFUND_TASK_STATUS_LABEL: Record<string, string> = {
	"0": "待處理",
	"5": "等待買家填寫退款資訊",
	"10": "退款處理中",
	"15": "等待金流確認",
	"20": "退款完成",
	"30": "退款失敗",
	"40": "需人工處理",
};

/** True when the buyer must submit ATM bank details for the refund to run. */
export function refundNeedsBuyerPage(refundTaskStatus?: string): boolean {
	return refundTaskStatus === REFUND_TASK_STATUS.AWAITING_BUYER_REFUND_PAGE;
}

/** Platform arbitration outcome label. Accepts the codemeta name
 * (FULL_REFUND…) or the numeric ordinal ("1"/"2"/"3") the wire may send. */
export function arbitrationOutcomeLabel(outcome: string): string {
	const byNumber: Record<string, string> = {
		"1": "全額退款",
		"2": "部分退款",
		"3": "駁回申訴",
	};
	const byName: Record<string, string> = {
		FULL_REFUND: "全額退款",
		PARTIAL_REFUND: "部分退款",
		REJECT_APPEAL: "駁回申訴",
	};
	return byName[outcome] ?? byNumber[outcome] ?? outcome;
}

/** Arbitration timeline event label fallback. */
export function arbitrationEventLabel(eventType: string): string {
	const byNumber: Record<string, string> = {
		"1": "申訴提交",
		"2": "平台裁決",
	};
	const byName: Record<string, string> = {
		APPEAL_SUBMITTED: "申訴提交",
		DECISION_MADE: "平台裁決",
	};
	return byName[eventType] ?? byNumber[eventType] ?? eventType;
}

export function afterSaleStatusLabel(status: string): string {
	return AFTER_SALE_STATUS_LABEL[status] ?? "售後處理中";
}

const RETURN_LOGISTICS_STATUS_LABEL: Record<string, string> = {
	"20": "待寄回",
	"25": "退貨運送中",
	"26": "超商待取",
	"27": "退貨已送達",
	"28": "退貨失敗",
	"30": "已收退貨",
	RETURN_PENDING: "待寄回",
	RETURN_SHIPPING: "退貨運送中",
	RETURN_AT_STORE: "超商待取",
	RETURN_DELIVERED: "退貨已送達",
	RETURN_FAILED: "退貨失敗",
	RETURN_RECEIVED: "已收退貨",
};

/** Display label for return logistics. The case status is authoritative:
 * when the case is status 28, provider text like "已送達" must not override
 * the buyer/seller-facing "退貨失敗" state. */
export function returnLogisticsStatusLabel(
	caseStatus: string,
	logistics?: Pick<AfterSaleReturnLogistics, "shipmentStatus"> | null,
): string | undefined {
	if (caseStatus === AFTER_SALE_STATUS.RETURN_FAILED) return "退貨失敗";
	const raw = logistics?.shipmentStatus?.trim();
	if (!raw) return undefined;
	return RETURN_LOGISTICS_STATUS_LABEL[raw] ?? raw;
}

const STATUS_TONE: Record<string, AfterSaleTone> = {
	"0": "warning",
	"20": "info",
	"25": "info",
	"26": "info",
	"27": "warning",
	"28": "danger",
	"30": "warning",
	"33": "muted",
	"40": "warning",
	"50": "success",
	"60": "danger",
	"70": "muted",
};

export function afterSaleTone(status: string): AfterSaleTone {
	return STATUS_TONE[status] ?? "muted";
}

/** Statuses where the SELLER is expected to act next (mirror APP
 * `_needsSellerAction`): 0 待處理, 30 已收退貨. */
const SELLER_ACTION_STATUSES = new Set([
	AFTER_SALE_STATUS.PENDING,
	AFTER_SALE_STATUS.RETURN_DELIVERED,
	AFTER_SALE_STATUS.RETURN_RECEIVED,
]);

export function afterSaleNeedsSeller(c: AfterSaleCase | undefined): boolean {
	if (!c) return false;
	// 案件進入平台仲裁後由平台裁決，賣家無法自行處理（後端會擋寫入），
	// 因此不再算作賣家的待辦——否則 dashboard / 訂單卡會誤導賣家去處理。
	if (afterSaleIsUnderAppeal(c.appealState)) return false;
	return SELLER_ACTION_STATUSES.has(c.status as never);
}

/** Terminal cases — closed / rejected / cancelled / ship-back timeout
 * (50/60/70/33). */
const TERMINAL_STATUSES = new Set([
	AFTER_SALE_STATUS.CLOSED,
	AFTER_SALE_STATUS.REJECTED,
	AFTER_SALE_STATUS.CANCELLED,
	AFTER_SALE_STATUS.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT,
]);

export function afterSaleIsTerminal(c: AfterSaleCase): boolean {
	// 仲裁中的案件即使 raw status 為 60 已拒絕，也已重新進入平台審理，不算終態 ——
	// 否則賣家預設視圖（排除終態）會把它藏進「已結案」，看不到「申訴仲裁中」。
	if (afterSaleIsUnderAppeal(c.appealState)) return false;
	return TERMINAL_STATUSES.has(c.status as never);
}

/* ---- Appeal lock & action predicates ------------------------------------ */

/**
 * codemeta `AfterSaleAppealState` (§10.2.1): NONE=0 / APPEALED=10 /
 * INVESTIGATING=20 (Phase 2, unused) / RESOLVED=30. The DB column is
 * `NOT NULL DEFAULT '0'`, so EVERY case carries `appealState: "0"` on the
 * wire — "non-empty" must never be read as "appealed".
 *
 * The wire carries the codemeta **Value** only ("10"/"20"/"30"); the Name
 * ("APPEALED"…) is never sent — gateway rejects it with 400 and the service
 * persists Values. Comparing against Names would silently match nothing, so
 * these predicates read the generated codemeta table and nothing else.
 */
const APPEAL_STATE_PENDING: ReadonlySet<string> = new Set([
	AfterSaleAppealState.APPEALED,
	AfterSaleAppealState.INVESTIGATING,
]);

/** True while the case awaits platform arbitration (APPEALED/INVESTIGATING) —
 * drives the「申訴中，等待平台裁決」banner. */
export function afterSaleIsUnderAppeal(
	appealState: string | undefined,
): boolean {
	return appealState !== undefined && APPEAL_STATE_PENDING.has(appealState);
}

/** True once the platform has decided the appeal (RESOLVED). */
export function afterSaleIsAppealResolved(
	appealState: string | undefined,
): boolean {
	return appealState === AfterSaleAppealState.RESOLVED;
}

/**
 * Appeal lock — mirrors backend `assertNotUnderAppeal` (§10.12), which blocks
 * ALL write RPCs whenever `appeal_state != '0'`, RESOLVED included (bypassing
 * calls get 409/500). Buyer appeal/cancel/supplement and seller
 * resolve/confirm-receipt must all be gated on this.
 */
export function afterSaleIsAppealLocked(
	appealState: string | undefined,
): boolean {
	return Boolean(appealState) && appealState !== AfterSaleAppealState.NONE;
}

/** Buyer 取消申請 — only while awaiting the seller's decision (0 待處理),
 * never while the appeal lock is on. */
export function afterSaleCanCancel(
	status: string,
	appealState: string | undefined,
): boolean {
	return (
		!afterSaleIsAppealLocked(appealState) &&
		status === AFTER_SALE_STATUS.PENDING
	);
}

/** Buyer 提出申訴 — backend `canAppeal` flag minus the appeal lock. Once
 * under appeal the case must not be pushed back to a seller-actionable
 * state. */
export function afterSaleCanAppeal(
	canAppeal: boolean | undefined,
	appealState: string | undefined,
): boolean {
	return Boolean(canAppeal) && !afterSaleIsAppealLocked(appealState);
}

/** Buyer 補充說明 — while the seller reviews (0 待處理) or the return has not
 * shipped yet (20 待退貨), never while the appeal lock is on. */
export function afterSaleCanSupplement(
	status: string,
	appealState: string | undefined,
): boolean {
	return (
		!afterSaleIsAppealLocked(appealState) &&
		(status === AFTER_SALE_STATUS.PENDING ||
			status === AFTER_SALE_STATUS.RETURN_PENDING)
	);
}

/** Seller 送出處理 (resolve) — only 0 待處理, and never while the appeal
 * lock is on (the backend answers such calls with 500/409). */
export function afterSaleCanResolve(
	status: string,
	appealState: string | undefined,
): boolean {
	return (
		!afterSaleIsAppealLocked(appealState) &&
		status === AFTER_SALE_STATUS.PENDING
	);
}

/** Seller 確認收貨 — only after return delivery is confirmed: 27 退貨已送達
 * or 30 已收退貨. 25 退貨運送中 and 26 超商待取 are logistics-owned and must
 * remain read-only for the seller, otherwise the backend rejects with 9004. */
export function afterSaleCanConfirmReceipt(
	status: string,
	appealState: string | undefined,
): boolean {
	return (
		!afterSaleIsAppealLocked(appealState) &&
		(status === AFTER_SALE_STATUS.RETURN_RECEIVED ||
			status === AFTER_SALE_STATUS.RETURN_DELIVERED)
	);
}

/* ---- Appeal-aware list/card display ------------------------------------ */

/**
 * Shared appeal display strings — list/card/summary rows fold these over the
 * raw status so an appealed case reads「申訴仲裁中」/「等待平台裁決」instead of
 * the stale 已拒絕 / 待你處理. Mirrors the detail drawer's arbitration banner
 * (`此案件已進入平台申訴`), giving list rows the same signal without a banner.
 */
export const AFTER_SALE_APPEAL_LABEL = "申訴仲裁中";
export const AFTER_SALE_APPEAL_NEXT_STEP = "等待平台裁決";

/** Status label folded with appeal state — under appeal overrides the raw
 * status label so the row no longer shows 已拒絕 / 待處理. */
export function afterSaleDisplayLabel(
	status: string,
	appealState: string | undefined,
): string {
	if (afterSaleIsUnderAppeal(appealState)) return AFTER_SALE_APPEAL_LABEL;
	return afterSaleStatusLabel(status);
}

/** Tone folded with appeal state — arbitration cases use `warning` (pending
 * external decision, not a terminal danger/success). */
export function afterSaleDisplayTone(
	status: string,
	appealState: string | undefined,
): AfterSaleTone {
	if (afterSaleIsUnderAppeal(appealState)) return "warning";
	return afterSaleTone(status);
}

export function afterSaleGroupOf(
	c: AfterSaleCase,
): Exclude<AfterSaleGroup, "all"> {
	// 仲裁中的案件是活躍待裁決，不論 raw status（含 60 已拒絕）都歸「待處理」群組，
	// 才不會被當作已結案而藏起來。
	if (afterSaleIsUnderAppeal(c.appealState)) return "pending";
	if (SELLER_ACTION_STATUSES.has(c.status as never)) return "pending"; // 0/30
	if (
		c.status === AFTER_SALE_STATUS.RETURN_PENDING ||
		c.status === AFTER_SALE_STATUS.RETURN_SHIPPING ||
		c.status === AFTER_SALE_STATUS.RETURN_AT_STORE ||
		c.status === AFTER_SALE_STATUS.RETURN_FAILED
	) {
		return "returning";
	}
	if (c.status === AFTER_SALE_STATUS.REFUNDING) return "refunding";
	return "closed"; // 50/60/70
}

/** One-line "現在輪到誰做什麼" narrative — drives status banners. */
export function afterSaleNarrative(status: string): string {
	switch (status) {
		case AFTER_SALE_STATUS.PENDING:
			return "等待賣家審核售後申請。";
		case AFTER_SALE_STATUS.RETURN_PENDING:
			return "賣家已同意退貨，請依退貨指示將商品寄回。";
		case AFTER_SALE_STATUS.RETURN_SHIPPING:
			return "退貨包裹運送中，等待賣家收貨驗收。";
		case AFTER_SALE_STATUS.RETURN_AT_STORE:
			return "退貨已送達超商，等待物流取件（請於期限內完成寄件）。";
		case AFTER_SALE_STATUS.RETURN_DELIVERED:
			return "退貨已送達賣家，等待賣家確認收貨（逾時將自動確認）。";
		case AFTER_SALE_STATUS.RETURN_FAILED:
			return "退貨物流失敗，請查看失敗原因並聯絡客服或重新安排退貨。";
		case AFTER_SALE_STATUS.RETURN_RECEIVED:
			return "賣家已收到退貨，正在驗貨確認退款金額。";
		case AFTER_SALE_STATUS.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT:
			return "逾時未寄回退貨商品，退款資格已解除；若有特殊情況可於申訴期內提出申訴。";
		case AFTER_SALE_STATUS.REFUNDING:
			return "退款處理中，款項將原路退回或依退款資訊撥款。";
		case AFTER_SALE_STATUS.CLOSED:
			return "售後案件已結案。";
		case AFTER_SALE_STATUS.REJECTED:
			return "賣家已拒絕本次售後，你可以提出申訴。";
		case AFTER_SALE_STATUS.CANCELLED:
			return "售後申請已取消。";
		default:
			return "售後處理中。";
	}
}

/* ---- 4-step progress --------------------------------------------------- */

export type AfterSaleStepState = "done" | "current" | "upcoming" | "failed";

export interface AfterSaleStep {
	key: string;
	label: string;
	state: AfterSaleStepState;
}

const STEP_LABELS = ["售後申請", "賣家審核", "退貨 / 退款", "完成"] as const;

/**
 * Map a status to a 4-step progress bar. Rejected/cancelled mark the final
 * step `failed` so the buyer sees the flow stopped rather than completed.
 */
export function afterSaleSteps(status: string): AfterSaleStep[] {
	// index of the "current" step; -1 means terminal handled below
	const order = ["申請", "審核", "退貨退款", "完成"];
	let activeIdx: number;
	let terminalFailed = false;
	switch (status) {
		case AFTER_SALE_STATUS.PENDING:
			activeIdx = 1; // 賣家審核
			break;
		case AFTER_SALE_STATUS.RETURN_PENDING:
		case AFTER_SALE_STATUS.RETURN_SHIPPING:
		case AFTER_SALE_STATUS.RETURN_AT_STORE:
		case AFTER_SALE_STATUS.RETURN_DELIVERED:
		case AFTER_SALE_STATUS.RETURN_FAILED:
		case AFTER_SALE_STATUS.RETURN_RECEIVED:
		case AFTER_SALE_STATUS.REFUNDING:
			activeIdx = 2; // 退貨/退款
			break;
		case AFTER_SALE_STATUS.CLOSED:
			activeIdx = 3; // 完成
			break;
		case AFTER_SALE_STATUS.REJECTED:
		case AFTER_SALE_STATUS.CANCELLED:
			activeIdx = 3;
			terminalFailed = true;
			break;
		default:
			activeIdx = 0;
	}
	return order.map((_, i) => {
		let state: AfterSaleStepState;
		if (terminalFailed && i === activeIdx) state = "failed";
		else if (i < activeIdx) state = "done";
		else if (i === activeIdx)
			state = status === AFTER_SALE_STATUS.CLOSED ? "done" : "current";
		else state = "upcoming";
		return { key: order[i], label: STEP_LABELS[i], state };
	});
}

/* ---- Lookup ------------------------------------------------------------ */

/** Statuses excluded from the seller order-list lookup (REJECTED/CANCELLED
 * never entered the refund flow — surfacing them misleads the seller). */
const EXCLUDED_FROM_LOOKUP = new Set([
	AFTER_SALE_STATUS.REJECTED,
	AFTER_SALE_STATUS.CANCELLED,
]);

/** Build the `orderNo → AfterSaleCase` lookup. Keeps the latest case per
 * order; drops excluded statuses — except cases under platform appeal, which
 * have re-entered arbitration and must stay visible to the seller. */
export function buildAfterSaleLookup(
	cases: ReadonlyArray<AfterSaleCase>,
): Map<string, AfterSaleCase> {
	const map = new Map<string, AfterSaleCase>();
	for (const c of cases) {
		if (!c.orderNo) continue;
		if (
			EXCLUDED_FROM_LOOKUP.has(c.status as never) &&
			!afterSaleIsUnderAppeal(c.appealState)
		) {
			continue;
		}
		const existing = map.get(c.orderNo);
		if (!existing || (c.createdAt ?? "") > (existing.createdAt ?? "")) {
			map.set(c.orderNo, c);
		}
	}
	return map;
}
