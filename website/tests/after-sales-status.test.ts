/**
 * lib/after-sales/status.ts — status→label/tone mapping, action gating
 * (who acts next / terminal), grouping, 4-step progress and the seller
 * order-list lookup.
 *
 * Domain rules covered (docs/planning/AFTER_SALES_UI_PLAN.md):
 *   - 26/27/28 are the return-logistics after-sale states (超商待取 /
 *     退貨已送達 / 退貨失敗).
 *   - Seller-action gate: 0/27/30 — only these statuses ask the seller
 *     to resolve / confirm receipt.
 *   - refundTaskStatus "5" is the only state where the buyer must fill in
 *     ATM refund details.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import {
	AFTER_SALE_APPEAL_LABEL,
	AFTER_SALE_APPEAL_NEXT_STEP,
	AFTER_SALE_STATUS_LABEL,
	afterSaleCanAppeal,
	afterSaleCanCancel,
	afterSaleCanConfirmReceipt,
	afterSaleCanResolve,
	afterSaleCanSupplement,
	afterSaleDisplayLabel,
	afterSaleDisplayTone,
	afterSaleGroupOf,
	afterSaleIsAppealLocked,
	afterSaleIsAppealResolved,
	afterSaleIsTerminal,
	afterSaleIsUnderAppeal,
	afterSaleNarrative,
	afterSaleNeedsSeller,
	afterSaleStatusLabel,
	afterSaleSteps,
	afterSaleTone,
	arbitrationEventLabel,
	arbitrationOutcomeLabel,
	buildAfterSaleLookup,
	refundNeedsBuyerPage,
	returnLogisticsStatusLabel,
} from "../lib/after-sales/status";
import {
	AFTER_SALE_STATUS,
	type AfterSaleCase,
	REFUND_TASK_STATUS,
} from "../lib/after-sales/types";
import { AfterSaleAppealState } from "../lib/codemeta/generated";

function makeCase(overrides: Partial<AfterSaleCase> = {}): AfterSaleCase {
	return {
		caseNo: "AS001",
		orderNo: "O001",
		buyerId: "1",
		buyerName: "買家",
		status: AFTER_SALE_STATUS.PENDING,
		reason: "5001",
		description: "",
		refundableAmount: "100",
		returnShipmentNo: "",
		returnShippingMethod: "",
		createdAt: "2026-06-01T00:00:00Z",
		...overrides,
	};
}

/* ===== labels ===== */

test("afterSaleStatusLabel maps the return-logistics codes 26/27/28", () => {
	assert.equal(afterSaleStatusLabel("26"), "超商待取");
	assert.equal(afterSaleStatusLabel("27"), "退貨已送達");
	assert.equal(afterSaleStatusLabel("28"), "退貨失敗");
});

test("returnLogisticsStatusLabel lets case status 28 override stale provider text", () => {
	assert.equal(
		returnLogisticsStatusLabel(AFTER_SALE_STATUS.RETURN_FAILED, {
			shipmentStatus: "已送達",
		}),
		"退貨失敗",
	);
	assert.equal(
		returnLogisticsStatusLabel(AFTER_SALE_STATUS.RETURN_SHIPPING, {
			shipmentStatus: "28",
		}),
		"退貨失敗",
	);
});

test("afterSaleStatusLabel covers every AFTER_SALE_STATUS code and falls back on unknown", () => {
	for (const code of Object.values(AFTER_SALE_STATUS)) {
		assert.ok(
			AFTER_SALE_STATUS_LABEL[code],
			`missing label for status ${code}`,
		);
	}
	assert.equal(afterSaleStatusLabel("999"), "售後處理中");
});

/* ===== tone ===== */

test("afterSaleTone follows the status/tone matrix and defaults to muted", () => {
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.PENDING), "warning");
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.RETURN_AT_STORE), "info");
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.RETURN_DELIVERED), "warning");
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.RETURN_FAILED), "danger");
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.CLOSED), "success");
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.REJECTED), "danger");
	assert.equal(afterSaleTone(AFTER_SALE_STATUS.CANCELLED), "muted");
	assert.equal(afterSaleTone("999"), "muted");
});

/* ===== action gating ===== */

test("afterSaleNeedsSeller gates seller action to 0/27/30 only", () => {
	const needsSeller = [
		AFTER_SALE_STATUS.PENDING, // 0 — resolve
		AFTER_SALE_STATUS.RETURN_DELIVERED, // 27 — confirm receipt
		AFTER_SALE_STATUS.RETURN_RECEIVED, // 30 — confirm refund amount
	];
	for (const status of needsSeller) {
		assert.equal(
			afterSaleNeedsSeller(makeCase({ status })),
			true,
			`status ${status} should need seller action`,
		);
	}
	const buyerOrIdle = [
		AFTER_SALE_STATUS.RETURN_PENDING, // 20 — buyer ships back
		AFTER_SALE_STATUS.RETURN_SHIPPING, // 25 — logistics
		AFTER_SALE_STATUS.RETURN_AT_STORE, // 26 — logistics pickup
		AFTER_SALE_STATUS.RETURN_FAILED, // 28 — support / buyer
		AFTER_SALE_STATUS.REFUNDING, // 40 — payment worker
		AFTER_SALE_STATUS.CLOSED,
		AFTER_SALE_STATUS.REJECTED,
		AFTER_SALE_STATUS.CANCELLED,
	];
	for (const status of buyerOrIdle) {
		assert.equal(
			afterSaleNeedsSeller(makeCase({ status })),
			false,
			`status ${status} should NOT need seller action`,
		);
	}
});

test("afterSaleNeedsSeller is false for a missing case", () => {
	assert.equal(afterSaleNeedsSeller(undefined), false);
});

test("afterSaleNeedsSeller drops a case once it is under platform appeal", () => {
	// Auto-escalate keeps status=0 but moves the case to arbitration — the
	// seller can no longer act, so it must leave the 待辦 list.
	assert.equal(
		afterSaleNeedsSeller(makeCase({ status: "0", appealState: "10" })),
		false,
	);
	assert.equal(
		afterSaleNeedsSeller(makeCase({ status: "30", appealState: "20" })),
		false,
	);
	// NONE / resolved cases still follow the raw-status gate.
	assert.equal(
		afterSaleNeedsSeller(makeCase({ status: "0", appealState: "0" })),
		true,
	);
});

/* ===== appeal-aware list/card display ===== */

test("afterSaleDisplayLabel folds 申訴仲裁中 over the raw status under appeal", () => {
	// Buyer-appeal (60) and auto-escalate (0) both read 申訴仲裁中 in lists.
	assert.equal(afterSaleDisplayLabel("60", "10"), AFTER_SALE_APPEAL_LABEL);
	assert.equal(afterSaleDisplayLabel("20", "20"), AFTER_SALE_APPEAL_LABEL);
	// NONE ("0") and undefined fall through to the plain status label.
	assert.equal(afterSaleDisplayLabel("60", "0"), afterSaleStatusLabel("60"));
	assert.equal(
		afterSaleDisplayLabel("0", undefined),
		afterSaleStatusLabel("0"),
	);
	// RESOLVED ("30") is no longer pending arbitration → raw status wins.
	assert.equal(afterSaleDisplayLabel("60", "30"), afterSaleStatusLabel("60"));
});

test("afterSaleDisplayTone uses warning under appeal, raw tone otherwise", () => {
	assert.equal(afterSaleDisplayTone("60", "10"), "warning");
	assert.equal(afterSaleDisplayTone("60", "0"), afterSaleTone("60"));
	assert.equal(afterSaleDisplayTone("40", undefined), afterSaleTone("40"));
});

test("AFTER_SALE_APPEAL_NEXT_STEP is the shared seller next-step string", () => {
	assert.equal(AFTER_SALE_APPEAL_NEXT_STEP, "等待平台裁決");
});

test("afterSaleIsTerminal is true only for 50/60/70", () => {
	assert.equal(afterSaleIsTerminal(makeCase({ status: "50" })), true);
	assert.equal(afterSaleIsTerminal(makeCase({ status: "60" })), true);
	assert.equal(afterSaleIsTerminal(makeCase({ status: "70" })), true);
	for (const status of ["0", "20", "25", "26", "27", "28", "30", "40"]) {
		assert.equal(
			afterSaleIsTerminal(makeCase({ status })),
			false,
			`status ${status} should not be terminal`,
		);
	}
});

test("afterSaleIsTerminal is false for a rejected case that is under appeal", () => {
	// 60 已拒絕 alone is terminal, but once appealed it re-enters arbitration —
	// the seller default view must not hide it as 已結案.
	assert.equal(
		afterSaleIsTerminal(makeCase({ status: "60", appealState: "10" })),
		false,
	);
	// A resolved appeal (30) follows the raw status again → terminal.
	assert.equal(
		afterSaleIsTerminal(makeCase({ status: "60", appealState: "30" })),
		true,
	);
});

/* ===== appeal lock & action predicates ===== */

const ALL_STATUSES = [
	"0",
	"20",
	"25",
	"26",
	"27",
	"28",
	"30",
	"40",
	"50",
	"60",
	"70",
];

test("afterSaleIsUnderAppeal treats undefined and empty appealState as no lock", () => {
	assert.equal(afterSaleIsUnderAppeal(undefined), false);
	assert.equal(afterSaleIsUnderAppeal(""), false);
	assert.equal(afterSaleIsUnderAppeal("10"), true);
});

test("appeal predicates read codemeta Values only — Names are not accepted", () => {
	// The wire carries the AfterSaleAppealState **Value** ("0"/"10"/"20"/"30").
	// The Name never reaches the browser: gateway 400s `appeal_state=APPEALED`
	// and aftersales-service persists Values. The old dual-track comparison
	// (`=== "10" || === "APPEALED"`) is gone, so a Name must now match nothing.
	assert.equal(afterSaleIsUnderAppeal("APPEALED"), false);
	assert.equal(afterSaleIsUnderAppeal("INVESTIGATING"), false);
	assert.equal(afterSaleIsAppealResolved("RESOLVED"), false);

	// Values keep working — this is the only accepted spelling.
	assert.equal(afterSaleIsUnderAppeal(AfterSaleAppealState.APPEALED), true);
	assert.equal(
		afterSaleIsUnderAppeal(AfterSaleAppealState.INVESTIGATING),
		true,
	);
	assert.equal(afterSaleIsUnderAppeal(AfterSaleAppealState.RESOLVED), false);
	assert.equal(afterSaleIsUnderAppeal(AfterSaleAppealState.NONE), false);
	assert.equal(afterSaleIsAppealResolved(AfterSaleAppealState.RESOLVED), true);
	assert.equal(afterSaleIsAppealResolved(AfterSaleAppealState.APPEALED), false);

	// The lock mirrors backend `assertNotUnderAppeal`, which compares against
	// the Value "0" alone — so "NONE" is an unknown value and stays fail-closed.
	assert.equal(afterSaleIsAppealLocked(AfterSaleAppealState.NONE), false);
	assert.equal(afterSaleIsAppealLocked("NONE"), true);
	assert.equal(afterSaleIsAppealLocked(AfterSaleAppealState.APPEALED), true);
	assert.equal(afterSaleIsAppealLocked(AfterSaleAppealState.RESOLVED), true);
	assert.equal(afterSaleIsAppealLocked(undefined), false);
	assert.equal(afterSaleIsAppealLocked(""), false);
});

test("appeal lock disables seller resolve and confirm-receipt (backend 500/409 guard)", () => {
	// Without the lock the seller may act on the actionable statuses…
	assert.equal(afterSaleCanResolve(AFTER_SALE_STATUS.PENDING, undefined), true);
	assert.equal(
		afterSaleCanConfirmReceipt(AFTER_SALE_STATUS.RETURN_DELIVERED, undefined),
		true,
	);
	// …but once appealState is set the gate closes regardless of status,
	// because the backend rejects such calls with 500/409.
	for (const status of ALL_STATUSES) {
		assert.equal(
			afterSaleCanResolve(status, "10"),
			false,
			`status ${status} must not allow resolve under appeal`,
		);
		assert.equal(
			afterSaleCanConfirmReceipt(status, "10"),
			false,
			`status ${status} must not allow confirm-receipt under appeal`,
		);
	}
});

test("afterSaleCanResolve allows only 0 待處理", () => {
	for (const status of ALL_STATUSES) {
		assert.equal(
			afterSaleCanResolve(status, undefined),
			status === AFTER_SALE_STATUS.PENDING,
			`status ${status}`,
		);
	}
});

test("afterSaleCanConfirmReceipt allows only delivered/received returns, not in-transit logistics", () => {
	for (const status of ALL_STATUSES) {
		assert.equal(
			afterSaleCanConfirmReceipt(status, undefined),
			status === AFTER_SALE_STATUS.RETURN_DELIVERED ||
				status === AFTER_SALE_STATUS.RETURN_RECEIVED,
			`status ${status}`,
		);
	}
	// 25/26 are logistics-owned — explicitly excluded from seller confirm.
	assert.equal(
		afterSaleCanConfirmReceipt(AFTER_SALE_STATUS.RETURN_SHIPPING, undefined),
		false,
	);
	assert.equal(
		afterSaleCanConfirmReceipt(AFTER_SALE_STATUS.RETURN_AT_STORE, undefined),
		false,
	);
});

test("buyer actions are all gated off once under appeal", () => {
	// Appeal window open, no appeal filed yet → buyer may appeal.
	assert.equal(afterSaleCanAppeal(true, undefined), true);
	// Appeal filed → the second escalation path closes…
	assert.equal(afterSaleCanAppeal(true, "10"), false);
	// …and the appeal lock also hides the other buyer actions, so
	// the case cannot be pushed back to a seller-actionable state.
	assert.equal(afterSaleIsUnderAppeal("10"), true);
	assert.equal(afterSaleCanCancel(AFTER_SALE_STATUS.PENDING, "10"), false);
	assert.equal(afterSaleCanSupplement(AFTER_SALE_STATUS.PENDING, "10"), false);
});

test("afterSaleCanAppeal requires the backend canAppeal flag", () => {
	assert.equal(afterSaleCanAppeal(undefined, undefined), false);
	assert.equal(afterSaleCanAppeal(false, undefined), false);
	// An empty appealState is not a lock.
	assert.equal(afterSaleCanAppeal(true, ""), true);
});

test("afterSaleCanCancel allows only 0 and never under appeal", () => {
	for (const status of ALL_STATUSES) {
		assert.equal(
			afterSaleCanCancel(status, undefined),
			status === AFTER_SALE_STATUS.PENDING,
			`status ${status}`,
		);
		assert.equal(afterSaleCanCancel(status, "10"), false);
	}
});

test("afterSaleCanSupplement allows 0/20 and never under appeal", () => {
	for (const status of ALL_STATUSES) {
		assert.equal(
			afterSaleCanSupplement(status, undefined),
			status === AFTER_SALE_STATUS.PENDING ||
				status === AFTER_SALE_STATUS.RETURN_PENDING,
			`status ${status}`,
		);
		assert.equal(afterSaleCanSupplement(status, "10"), false);
	}
});

test("empty status (drawer header before detail loads) enables nothing", () => {
	assert.equal(afterSaleCanResolve("", undefined), false);
	assert.equal(afterSaleCanConfirmReceipt("", undefined), false);
	assert.equal(afterSaleCanCancel("", undefined), false);
	assert.equal(afterSaleCanSupplement("", undefined), false);
});

/* ===== grouping ===== */

test("afterSaleGroupOf buckets every status into the four filter groups", () => {
	const expectations: Array<[string, string]> = [
		["0", "pending"],
		["27", "pending"],
		["30", "pending"],
		["20", "returning"],
		["25", "returning"],
		["26", "returning"],
		["28", "returning"],
		["40", "refunding"],
		["50", "closed"],
		["60", "closed"],
		["70", "closed"],
	];
	for (const [status, group] of expectations) {
		assert.equal(
			afterSaleGroupOf(makeCase({ status })),
			group,
			`status ${status} should group as ${group}`,
		);
	}
});

/* ===== narrative ===== */

test("afterSaleNarrative tells the rejected buyer about the appeal path", () => {
	const narrative = afterSaleNarrative(AFTER_SALE_STATUS.REJECTED);
	assert.ok(narrative.includes("申訴"));
	assert.ok(!narrative.includes("重啟"));
});

test("afterSaleNarrative covers 26/27/28 with distinct copy and has a fallback", () => {
	const atStore = afterSaleNarrative("26");
	const delivered = afterSaleNarrative("27");
	const failed = afterSaleNarrative("28");
	const fallback = afterSaleNarrative("999");
	assert.ok(atStore.includes("超商"));
	assert.ok(delivered.includes("確認收貨"));
	assert.ok(failed.includes("失敗"));
	assert.equal(new Set([atStore, delivered, failed]).size, 3);
	assert.equal(fallback, "售後處理中。");
	assert.ok(!atStore.includes(fallback));
});

/* ===== 4-step progress ===== */

test("afterSaleSteps marks 賣家審核 current while pending", () => {
	for (const status of [AFTER_SALE_STATUS.PENDING]) {
		const steps = afterSaleSteps(status);
		assert.equal(steps.length, 4);
		assert.deepEqual(
			steps.map((s) => s.state),
			["done", "current", "upcoming", "upcoming"],
		);
	}
});

test("afterSaleSteps puts the return-flow statuses (incl. 26/27/28) on step 3", () => {
	for (const status of ["20", "25", "26", "27", "28", "30", "40"]) {
		const steps = afterSaleSteps(status);
		assert.deepEqual(
			steps.map((s) => s.state),
			["done", "done", "current", "upcoming"],
			`status ${status}`,
		);
	}
});

test("afterSaleSteps completes all steps when closed", () => {
	assert.deepEqual(
		afterSaleSteps(AFTER_SALE_STATUS.CLOSED).map((s) => s.state),
		["done", "done", "done", "done"],
	);
});

test("afterSaleSteps fails the final step for rejected/cancelled", () => {
	for (const status of [
		AFTER_SALE_STATUS.REJECTED,
		AFTER_SALE_STATUS.CANCELLED,
	]) {
		assert.deepEqual(
			afterSaleSteps(status).map((s) => s.state),
			["done", "done", "done", "failed"],
		);
	}
});

test("afterSaleSteps treats an unknown status as freshly applied", () => {
	assert.deepEqual(
		afterSaleSteps("999").map((s) => s.state),
		["current", "upcoming", "upcoming", "upcoming"],
	);
});

/* ===== refund task / arbitration labels ===== */

test("refundNeedsBuyerPage flags only WorkerTaskStatus 5 (ATM bank details)", () => {
	assert.equal(
		refundNeedsBuyerPage(REFUND_TASK_STATUS.AWAITING_BUYER_REFUND_PAGE),
		true,
	);
	assert.equal(refundNeedsBuyerPage(REFUND_TASK_STATUS.PENDING), false);
	assert.equal(refundNeedsBuyerPage(REFUND_TASK_STATUS.PROCESSING), false);
	assert.equal(refundNeedsBuyerPage(REFUND_TASK_STATUS.DONE), false);
	assert.equal(refundNeedsBuyerPage(undefined), false);
});

test("arbitrationOutcomeLabel accepts codemeta names, ordinals and passes unknowns through", () => {
	assert.equal(arbitrationOutcomeLabel("FULL_REFUND"), "全額退款");
	assert.equal(arbitrationOutcomeLabel("2"), "部分退款");
	assert.equal(arbitrationOutcomeLabel("REJECT_APPEAL"), "駁回申訴");
	assert.equal(arbitrationOutcomeLabel("SOMETHING_NEW"), "SOMETHING_NEW");
});

test("arbitrationEventLabel accepts names and ordinals", () => {
	assert.equal(arbitrationEventLabel("APPEAL_SUBMITTED"), "申訴提交");
	assert.equal(arbitrationEventLabel("2"), "平台裁決");
	assert.equal(arbitrationEventLabel("UNKNOWN"), "UNKNOWN");
});

/* ===== seller order-list lookup ===== */

test("buildAfterSaleLookup keeps the latest case per order", () => {
	const older = makeCase({
		caseNo: "AS001",
		orderNo: "O001",
		createdAt: "2026-06-01T00:00:00Z",
	});
	const newer = makeCase({
		caseNo: "AS002",
		orderNo: "O001",
		createdAt: "2026-06-05T00:00:00Z",
	});
	// Insert newest first to prove ordering does not matter.
	const lookup = buildAfterSaleLookup([newer, older]);
	assert.equal(lookup.size, 1);
	assert.equal(lookup.get("O001")?.caseNo, "AS002");
});

test("buildAfterSaleLookup drops rejected/cancelled cases and empty orderNo", () => {
	const lookup = buildAfterSaleLookup([
		makeCase({ caseNo: "AS001", orderNo: "O001", status: "60" }),
		makeCase({ caseNo: "AS002", orderNo: "O002", status: "70" }),
		makeCase({ caseNo: "AS003", orderNo: "", status: "0" }),
		makeCase({ caseNo: "AS004", orderNo: "O004", status: "27" }),
	]);
	assert.equal(lookup.size, 1);
	assert.equal(lookup.get("O004")?.caseNo, "AS004");
	assert.equal(lookup.has("O001"), false);
	assert.equal(lookup.has("O002"), false);
});

test("buildAfterSaleLookup keeps a rejected case that is under appeal", () => {
	const lookup = buildAfterSaleLookup([
		makeCase({ caseNo: "AS001", orderNo: "O001", status: "60" }),
		makeCase({
			caseNo: "AS002",
			orderNo: "O002",
			status: "60",
			appealState: "10",
		}),
	]);
	// Plain rejected dropped; rejected-under-appeal kept (seller must see it).
	assert.equal(lookup.has("O001"), false);
	assert.equal(lookup.get("O002")?.caseNo, "AS002");
});

test("afterSaleGroupOf buckets an appealed case as active 待處理, not 已結案", () => {
	// 60 已拒絕 normally groups closed; under appeal it is active arbitration.
	assert.equal(
		afterSaleGroupOf(makeCase({ status: "60", appealState: "10" })),
		"pending",
	);
	assert.equal(afterSaleGroupOf(makeCase({ status: "60" })), "closed");
});
