import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { AdminRevokeReason } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	EvidenceSchema,
	MeasureRevisionSchema,
	MeasureSchema,
	MeasureActionUnavailableReason as Reason,
	ViolationSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureBasis,
	MeasureRequestState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	actionBar,
	buildCreateNote,
	buildDecideAppeal,
	buildDecideRemediation,
	buildRevoke,
	buildScheduleReview,
	buildUpdateMeasure,
	canSubmitDecision,
	decideAcceptLabel,
	decideTitle,
	editedFields,
	endTimeField,
	evidenceLinks,
	initialUpdateForm,
	REVOKE_REASON_OPTIONS,
	rebaseForm,
	revisionComparison,
	statementLockReasons,
	submittedVersionText,
	updatePaths,
	validateDecision,
	validateSchedule,
	validateUpdate,
} from "../src/app/admin/measures/measure-actions";
import {
	adminRevokeReasonLabel,
	toLocalInput,
} from "../src/app/admin/merchants/[id]/measure";
import {
	GENERIC_UNAVAILABLE_LABEL,
	unavailableLabels,
} from "../src/lib/unavailability";
import { SERVER_TEXT, why } from "./fixtures/measure-unavailability";

/**
 * 措施詳情的純邏輯：操作列顯示與停用、伺服器原因的文字、修改措施的欄位限制與 update_mask、裁定與撤銷等請求。
 * 能不能做、為什麼不能，都由伺服器的 can_* 與 *_unavailabilities（原因與伺服器的文字）決定；這裡只驗證呈現。時間用注入的 now。
 */

const NOW = new Date("2026-10-16T02:00:00Z");
const HOUR = 60 * 60 * 1000;
const END = new Date(NOW.getTime() + 48 * HOUR);

function measure(
	init: Parameters<typeof create<typeof MeasureSchema>>[1] = {},
) {
	return create(MeasureSchema, {
		id: "m1",
		etag: "etag-1",
		statement: "原本的理由",
		endTime: timestampFromDate(END),
		...init,
	});
}

// ---------------------------------------------------------------------------
// 原因文字

test("原因的文字直接取自伺服器的 message，不查原因對照表", () => {
	assert.deepEqual(
		unavailableLabels([
			why(Reason.REVOKED, "伺服器說的第一句"),
			why(Reason.NOT_ACTIVE, "伺服器說的第二句"),
		]),
		["伺服器說的第一句", "伺服器說的第二句"],
	);
	assert.deepEqual(unavailableLabels([why(99 as Reason, "新原因的文字")]), [
		"新原因的文字",
	]);
});

test("原因不只一個時全部列出：保持伺服器的優先序、相同的文字只列一次；可以合併多個動作的清單", () => {
	assert.deepEqual(
		unavailableLabels([
			why(Reason.REVOKED, SERVER_TEXT.REVOKED),
			why(Reason.NOT_ACTIVE, SERVER_TEXT.NOT_ACTIVE),
			why(Reason.REVOKED, SERVER_TEXT.REVOKED),
		]),
		[SERVER_TEXT.REVOKED, SERVER_TEXT.NOT_ACTIVE],
	);
	assert.deepEqual(
		unavailableLabels(
			[why(Reason.NOT_ACTIVE, SERVER_TEXT.NOT_ACTIVE)],
			[
				why(Reason.NOT_ACTIVE, SERVER_TEXT.NOT_ACTIVE),
				why(Reason.TERMINATION_RECREATE, SERVER_TEXT.TERMINATION_RECREATE),
			],
		),
		[SERVER_TEXT.NOT_ACTIVE, SERVER_TEXT.TERMINATION_RECREATE],
	);
});

test("沒有 message 的原因（含 UNSPECIFIED）顯示通用文字，只列一次；沒有原因但伺服器說不行也是通用文字", () => {
	assert.deepEqual(unavailableLabels([why(99 as Reason)]), [
		GENERIC_UNAVAILABLE_LABEL,
	]);
	assert.deepEqual(
		unavailableLabels([
			why(Reason.UNSPECIFIED),
			why(99 as Reason),
			why(Reason.NOT_ACTIVE, SERVER_TEXT.NOT_ACTIVE),
		]),
		[GENERIC_UNAVAILABLE_LABEL, SERVER_TEXT.NOT_ACTIVE],
	);
	assert.deepEqual(
		unavailableLabels([why(Reason.REVOKED, "")]),
		[GENERIC_UNAVAILABLE_LABEL],
		"空字串的 message 等於沒有",
	);
	assert.deepEqual(unavailableLabels([]), [GENERIC_UNAVAILABLE_LABEL]);
});

// ---------------------------------------------------------------------------
// 操作列

test("操作列：全部 can_* 都是 true 時所有動作可用、沒有原因文字", () => {
	const bar = actionBar(
		measure({
			canUpdateStatement: true,
			canExtend: true,
			canShorten: true,
			canEnd: true,
			canRevoke: true,
			canScheduleReview: true,
			canDecideAppeal: true,
			canDecideRemediation: true,
		}),
	);
	for (const [name, state] of Object.entries(bar)) {
		assert.deepEqual(
			state,
			{ visible: true, enabled: true, reasons: [] },
			name,
		);
	}
});

test("操作列：修改與撤銷不能做時停用並列出伺服器的原因（不隱藏）", () => {
	const bar = actionBar(
		measure({
			canUpdateStatement: false,
			canExtend: false,
			canShorten: false,
			canRevoke: false,
			updateStatementUnavailabilities: [
				why(Reason.REVOKED, SERVER_TEXT.REVOKED),
			],
			extendUnavailabilities: [why(Reason.REVOKED, SERVER_TEXT.REVOKED)],
			shortenUnavailabilities: [why(Reason.REVOKED, SERVER_TEXT.REVOKED)],
			revokeUnavailabilities: [why(Reason.REVOKED, SERVER_TEXT.REVOKED)],
		}),
	);
	assert.deepEqual(bar.edit, {
		visible: true,
		enabled: false,
		reasons: [SERVER_TEXT.REVOKED],
	});
	assert.deepEqual(bar.revoke, {
		visible: true,
		enabled: false,
		reasons: [SERVER_TEXT.REVOKED],
	});
});

test("操作列：結束、延後複審與兩種裁定不能做時隱藏，不顯示原因", () => {
	const bar = actionBar(measure({}));
	for (const state of [
		bar.end,
		bar.scheduleReview,
		bar.decideAppeal,
		bar.decideRemediation,
	]) {
		assert.deepEqual(state, { visible: false, enabled: false, reasons: [] });
	}
});

test("操作列：只有一種 can_* 為 true 就是這個動作 true（只有裁定解除請求時只有它出現）", () => {
	const bar = actionBar(measure({ canDecideRemediation: true }));
	assert.equal(bar.decideRemediation.visible, true);
	assert.equal(bar.decideAppeal.visible, false);
	assert.equal(bar.end.visible, false);
});

test("操作列：修改只要改理由、延長、縮短其中之一可以就可開啟（凍結時只能縮短）", () => {
	const bar = actionBar(
		measure({
			canUpdateStatement: false,
			canExtend: false,
			canShorten: true,
			updateStatementUnavailabilities: [
				why(Reason.REQUEST_PENDING_FROZEN, SERVER_TEXT.FROZEN_STATEMENT),
			],
			extendUnavailabilities: [
				why(Reason.REQUEST_PENDING_FROZEN, SERVER_TEXT.FROZEN_EXTEND),
			],
		}),
	);
	assert.deepEqual(bar.edit, { visible: true, enabled: true, reasons: [] });
});

test("操作列：修改的三組原因合併成一份、依序去重；全空但伺服器說不行是通用文字", () => {
	const merged = actionBar(
		measure({
			updateStatementUnavailabilities: [
				why(Reason.NOT_ACTIVE, SERVER_TEXT.NOT_ACTIVE),
			],
			extendUnavailabilities: [
				why(Reason.NOT_ACTIVE, SERVER_TEXT.NOT_ACTIVE),
				why(Reason.TERMINATION_RECREATE, SERVER_TEXT.TERMINATION_RECREATE),
			],
			shortenUnavailabilities: [
				why(Reason.TERMINATION_RECREATE, SERVER_TEXT.TERMINATION_RECREATE),
			],
		}),
	);
	assert.deepEqual(merged.edit.reasons, [
		SERVER_TEXT.NOT_ACTIVE,
		SERVER_TEXT.TERMINATION_RECREATE,
	]);
	const empty = actionBar(measure({}));
	assert.deepEqual(empty.edit.reasons, [GENERIC_UNAVAILABLE_LABEL]);
	assert.deepEqual(empty.revoke.reasons, [GENERIC_UNAVAILABLE_LABEL]);
});

test("操作列：撤銷停用時列出全部原因，包含這個版本不認得的值（仍相信伺服器的 false）", () => {
	const bar = actionBar(
		measure({
			canRevoke: false,
			revokeUnavailabilities: [
				why(Reason.APPEAL_PENDING, SERVER_TEXT.APPEAL_PENDING),
				why(77 as Reason),
			],
		}),
	);
	assert.deepEqual(bar.revoke, {
		visible: true,
		enabled: false,
		reasons: [SERVER_TEXT.APPEAL_PENDING, GENERIC_UNAVAILABLE_LABEL],
	});
});

test("操作列：can_revoke 是 true 時不看原因欄（伺服器說可以就可以）", () => {
	const bar = actionBar(
		measure({
			canRevoke: true,
			revokeUnavailabilities: [why(Reason.REVOKED, SERVER_TEXT.REVOKED)],
		}),
	);
	assert.deepEqual(bar.revoke, { visible: true, enabled: true, reasons: [] });
});

// ---------------------------------------------------------------------------
// 修改措施

test("理由欄：can_update_statement 為 false 時不能編輯，旁邊列出伺服器的原因", () => {
	assert.deepEqual(
		statementLockReasons(measure({ canUpdateStatement: true })),
		[],
	);
	assert.deepEqual(
		statementLockReasons(
			measure({
				canUpdateStatement: false,
				updateStatementUnavailabilities: [
					why(Reason.REQUEST_PENDING_FROZEN, SERVER_TEXT.FROZEN_STATEMENT),
				],
			}),
		),
		[SERVER_TEXT.FROZEN_STATEMENT],
	);
});

test("結束時間欄：可延長可縮短不設上限、下限是現在", () => {
	const field = endTimeField(
		measure({ canExtend: true, canShorten: true }),
		NOW,
	);
	assert.deepEqual(field, {
		disabled: false,
		min: toLocalInput(NOW),
		reasons: [],
	});
});

test("結束時間欄：只能縮短時上限是目前的結束時間、下限是現在，並說明不能延長的原因", () => {
	const field = endTimeField(
		measure({
			canExtend: false,
			canShorten: true,
			extendUnavailabilities: [
				why(Reason.REQUEST_PENDING_FROZEN, SERVER_TEXT.FROZEN_EXTEND),
			],
		}),
		NOW,
	);
	assert.deepEqual(field, {
		disabled: false,
		min: toLocalInput(NOW),
		max: toLocalInput(END),
		reasons: [SERVER_TEXT.FROZEN_EXTEND],
	});
});

test("結束時間欄：只能延長時下限是目前的結束時間；都不行時停用並列出原因", () => {
	const onlyExtend = endTimeField(
		measure({ canExtend: true, canShorten: false }),
		NOW,
	);
	assert.equal(onlyExtend.min, toLocalInput(END));
	assert.equal(onlyExtend.max, undefined);
	const none = endTimeField(
		measure({
			endTime: undefined,
			extendUnavailabilities: [
				why(Reason.KIND_HAS_NO_END_TIME, SERVER_TEXT.KIND_HAS_NO_END_TIME),
			],
			shortenUnavailabilities: [
				why(Reason.KIND_HAS_NO_END_TIME, SERVER_TEXT.KIND_HAS_NO_END_TIME),
			],
		}),
		NOW,
	);
	assert.deepEqual(none, {
		disabled: true,
		reasons: [SERVER_TEXT.KIND_HAS_NO_END_TIME],
	});
});

test("update_mask 只列實際改過的欄位", () => {
	const m = measure({
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const base = initialUpdateForm(m);
	assert.deepEqual(updatePaths(m, base, base), []);
	assert.deepEqual(updatePaths(m, { ...base, statement: "新的理由" }, base), [
		"statement",
	]);
	const later = toLocalInput(new Date(END.getTime() + 24 * HOUR));
	assert.deepEqual(updatePaths(m, { ...base, endTime: later }, base), [
		"end_time",
	]);
	assert.deepEqual(
		updatePaths(m, { statement: "新的理由", endTime: later }, base),
		["statement", "end_time"],
	);
	// 前後空白不算改；結束時間清空不算改（不支援清除）。
	assert.deepEqual(
		updatePaths(m, { statement: "  原本的理由  ", endTime: "" }, base),
		[],
	);
});

test("不能編輯的欄位不算改：凍結時理由不進 update_mask", () => {
	const m = measure({
		canUpdateStatement: false,
		canExtend: false,
		canShorten: true,
	});
	const base = initialUpdateForm(m);
	assert.deepEqual(updatePaths(m, { ...base, statement: "偷改" }, base), []);
	const sooner = toLocalInput(new Date(END.getTime() - 24 * HOUR));
	assert.deepEqual(updatePaths(m, { ...base, endTime: sooner }, base), [
		"end_time",
	]);
});

test("UpdateMeasure 請求：只帶改過的欄位加 id 與 etag，update_mask 同步", () => {
	const m = measure({
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const base = initialUpdateForm(m);
	const only = buildUpdateMeasure(
		m,
		{ ...base, statement: " 新的理由 " },
		base,
	);
	assert.deepEqual(only.updateMask, { paths: ["statement"] });
	assert.equal(only.measure?.id, "m1");
	assert.equal(only.measure?.etag, "etag-1");
	assert.equal(only.measure?.statement, "新的理由");
	assert.equal(only.measure?.endTime, undefined, "沒改的結束時間不送");

	const later = new Date(END.getTime() + 24 * HOUR);
	const both = buildUpdateMeasure(
		m,
		{ statement: "新的理由", endTime: toLocalInput(later) },
		base,
	);
	assert.deepEqual(both.updateMask, { paths: ["statement", "end_time"] });
	assert.equal(
		timestampDate(both.measure?.endTime as never).getTime(),
		Math.floor(later.getTime() / 60000) * 60000,
	);
	assert.throws(() => buildUpdateMeasure(m, base, base), /沒有改任何欄位/u);
});

test("修改的驗證：理由不可為空、不超過 2000 字；結束時間要晚於現在", () => {
	const m = measure({
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const base = initialUpdateForm(m);
	assert.equal(validateUpdate(m, base, base, NOW), undefined);
	assert.equal(
		validateUpdate(m, { ...base, statement: "   " }, base, NOW),
		"請填寫給賣家的理由。",
	);
	assert.equal(
		validateUpdate(m, { ...base, statement: "字".repeat(2001) }, base, NOW),
		"給賣家的理由最多 2000 字。",
	);
	assert.equal(
		validateUpdate(m, { ...base, statement: "字".repeat(2000) }, base, NOW),
		undefined,
	);
	const past = toLocalInput(new Date(NOW.getTime() - HOUR));
	assert.equal(
		validateUpdate(m, { ...base, endTime: past }, base, NOW),
		"結束時間必須晚於現在。",
	);
});

test("修改的驗證：只能縮短時不能晚於目前的結束時間；只能延長時不能早於", () => {
	const shortenOnly = measure({ canExtend: false, canShorten: true });
	const base = initialUpdateForm(shortenOnly);
	const later = toLocalInput(new Date(END.getTime() + 24 * HOUR));
	const sooner = toLocalInput(new Date(END.getTime() - 24 * HOUR));
	assert.equal(
		validateUpdate(shortenOnly, { ...base, endTime: later }, base, NOW),
		"結束時間不能晚於目前的結束時間。",
	);
	assert.equal(
		validateUpdate(shortenOnly, { ...base, endTime: sooner }, base, NOW),
		undefined,
	);
	const extendOnly = measure({ canExtend: true, canShorten: false });
	assert.equal(
		validateUpdate(extendOnly, { ...base, endTime: sooner }, base, NOW),
		"結束時間不能早於目前的結束時間。",
	);
	assert.equal(
		validateUpdate(extendOnly, { ...base, endTime: later }, base, NOW),
		undefined,
	);
});

// ---------------------------------------------------------------------------
// 裁定

test("裁定的標題與選項文字", () => {
	assert.equal(decideTitle("appeal"), "裁定申覆");
	assert.equal(decideTitle("remediation"), "裁定解除請求");
	assert.equal(decideAcceptLabel("appeal"), "接受（撤銷這個措施）");
	assert.equal(decideAcceptLabel("remediation"), "接受（結束這個措施）");
});

test("裁定：理由必填；還沒選結果不能送出", () => {
	assert.equal(
		validateDecision({ accept: true, reason: "  ", revokeFindings: false }),
		"請填寫裁定理由。",
	);
	assert.equal(
		validateDecision({
			accept: true,
			reason: "字".repeat(2001),
			revokeFindings: false,
		}),
		"裁定理由最多 2000 字。",
	);
	assert.equal(
		canSubmitDecision({ accept: null, reason: "理由", revokeFindings: false }),
		false,
	);
	assert.equal(
		canSubmitDecision({ accept: false, reason: "理由", revokeFindings: false }),
		true,
	);
});

function violationMeasure() {
	return measure({
		basis: MeasureBasis.VIOLATION,
		canRevokeFindings: true,
		findings: [
			create(ViolationSchema, { id: "f1", originMeasureId: "m1" }),
			create(ViolationSchema, { id: "f0", originMeasureId: "m0" }),
		],
	});
}

test("DecideAppeal 請求：帶 etag；一併撤銷認定只在接受、勾選、而且措施有自己建立的認定時送", () => {
	const m = violationMeasure();
	const accept = buildDecideAppeal(m, {
		accept: true,
		reason: " 理由 ",
		revokeFindings: true,
	});
	assert.deepEqual(accept, {
		measureId: "m1",
		decision: MeasureRequestState.ACCEPTED,
		reason: "理由",
		revokeFindings: true,
		etag: "etag-1",
	});
	assert.equal(
		buildDecideAppeal(m, {
			accept: false,
			reason: "理由",
			revokeFindings: true,
		}).revokeFindings,
		false,
		"駁回時送 true 伺服器會拒絕",
	);
	assert.equal(
		buildDecideAppeal(measure({ basis: MeasureBasis.RISK }), {
			accept: true,
			reason: "理由",
			revokeFindings: true,
		}).revokeFindings,
		false,
		"伺服器沒說可以（can_revoke_findings 是 false）就不送",
	);
	assert.throws(
		() =>
			buildDecideAppeal(m, {
				accept: null,
				reason: "理由",
				revokeFindings: false,
			}),
		/還沒選裁定結果/u,
	);
});

test("DecideRemediation 請求：沒有 revoke_findings 欄位", () => {
	const req = buildDecideRemediation(measure({}), {
		accept: false,
		reason: "理由",
		revokeFindings: true,
	});
	assert.deepEqual(req, {
		measureId: "m1",
		decision: MeasureRequestState.REJECTED,
		reason: "理由",
		etag: "etag-1",
	});
});

test("一併撤銷認定只看伺服器的 can_revoke_findings：client 不從依據或認定推導", () => {
	// 違規依據、認定也是自己建立的，但伺服器說不行（例如認定已撤銷）：不送。
	const serverSaysNo = measure({
		basis: MeasureBasis.VIOLATION,
		canRevokeFindings: false,
		findings: [create(ViolationSchema, { id: "f1", originMeasureId: "m1" })],
	});
	assert.equal(
		buildRevoke(serverSaysNo, AdminRevokeReason.OTHER, true).revokeFindings,
		false,
	);
	assert.equal(
		buildDecideAppeal(serverSaysNo, {
			accept: true,
			reason: "理由",
			revokeFindings: true,
		}).revokeFindings,
		false,
	);
	// 風險依據、沒有認定，但伺服器說可以：照伺服器。
	const serverSaysYes = measure({
		basis: MeasureBasis.RISK,
		canRevokeFindings: true,
	});
	assert.equal(
		buildRevoke(serverSaysYes, AdminRevokeReason.OTHER, true).revokeFindings,
		true,
	);
});

test("update_mask 只列管理員自己改過的欄位：版本衝突重讀之後沒動過的欄位不進 mask", () => {
	const opened = measure({
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const baseline = initialUpdateForm(opened);
	const later = toLocalInput(new Date(END.getTime() + 24 * HOUR));
	const form = { ...baseline, endTime: later };
	assert.deepEqual(editedFields(baseline, form), ["end_time"]);
	// 別的管理員在這之間改了理由；重讀後的措施理由與表單裡舊的理由不同。
	const refreshed = measure({
		etag: "etag-2",
		statement: "別人改過的理由",
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const request = buildUpdateMeasure(refreshed, form, baseline);
	assert.deepEqual(request.updateMask, { paths: ["end_time"] });
	assert.equal(request.measure?.statement, undefined, "沒改的理由不送");
	assert.equal(request.measure?.etag, "etag-2");
	// 改回基準的值不算改。
	assert.deepEqual(editedFields(baseline, baseline), []);
});

test("重讀後把沒動過的欄位對到新措施，動過的欄位保留輸入與原本的基準", () => {
	const opened = measure({});
	const baseline = initialUpdateForm(opened);
	const form = { ...baseline, statement: "我改的理由" };
	const later = new Date(END.getTime() + 24 * HOUR);
	const refreshed = measure({
		statement: "別人改過的理由",
		endTime: timestampFromDate(later),
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const next = rebaseForm(refreshed, baseline, form);
	assert.equal(next.form.statement, "我改的理由");
	assert.equal(next.baseline.statement, baseline.statement);
	assert.equal(next.form.endTime, toLocalInput(later));
	assert.equal(next.baseline.endTime, toLocalInput(later));
	assert.deepEqual(editedFields(next.baseline, next.form), ["statement"]);
});

// ---------------------------------------------------------------------------
// 撤銷、延後複審、註記

test("撤銷原因的選項取自契約的 AdminRevokeReason（不含 UNSPECIFIED）；系統寫入的原因在契約裡表示不出來", () => {
	assert.deepEqual(REVOKE_REASON_OPTIONS, [
		AdminRevokeReason.ADMIN_CORRECTION,
		AdminRevokeReason.EXTERNAL_DECISION,
		AdminRevokeReason.OTHER,
	]);
	assert.deepEqual(
		REVOKE_REASON_OPTIONS,
		Object.values(AdminRevokeReason).filter(
			(v) => typeof v === "number" && v !== AdminRevokeReason.UNSPECIFIED,
		),
	);
	for (const option of REVOKE_REASON_OPTIONS) {
		const text = adminRevokeReasonLabel(option);
		assert.ok(
			text !== "" && text !== "未知",
			`${AdminRevokeReason[option]} 有選項文字`,
		);
	}
	assert.deepEqual(
		REVOKE_REASON_OPTIONS.map(adminRevokeReasonLabel),
		["平台更正", "主管機關或法院的決定", "其他"],
		"文字取自共用目錄的 RevokeReason，與賣家端同名",
	);
	assert.equal(adminRevokeReasonLabel(AdminRevokeReason.UNSPECIFIED), "未知");
	assert.throws(
		() => buildRevoke(measure({}), AdminRevokeReason.UNSPECIFIED, false),
		/撤銷原因/u,
	);
	assert.deepEqual(
		buildRevoke(violationMeasure(), AdminRevokeReason.OTHER, true),
		{
			id: "m1",
			reason: AdminRevokeReason.OTHER,
			revokeFindings: true,
			etag: "etag-1",
		},
	);
	assert.equal(
		buildRevoke(measure({}), AdminRevokeReason.OTHER, true).revokeFindings,
		false,
	);
});

test("重讀後伺服器不再允許改的欄位：管理員改過的輸入還原成新措施的值，不留下不會送出的修改", () => {
	const opened = measure({
		canUpdateStatement: true,
		canExtend: true,
		canShorten: true,
	});
	const baseline = initialUpdateForm(opened);
	const edit = toLocalInput(new Date(END.getTime() + 24 * HOUR));
	const form = { statement: "我改的理由", endTime: edit };
	assert.deepEqual(editedFields(baseline, form), ["statement", "end_time"]);
	// 別人在這之間撤銷了措施：三個 can_* 都變成 false，理由與結束時間也是新的值。
	const revoked = measure({
		etag: "etag-2",
		statement: "撤銷時的理由",
		canUpdateStatement: false,
		canExtend: false,
		canShorten: false,
	});
	const next = rebaseForm(revoked, baseline, form);
	assert.deepEqual(next.form, initialUpdateForm(revoked));
	assert.deepEqual(next.baseline, initialUpdateForm(revoked));
	assert.deepEqual(editedFields(next.baseline, next.form), []);
	// 只剩結束時間不許改：理由的輸入保留，結束時間還原。
	const onlyStatement = rebaseForm(
		measure({ canUpdateStatement: true }),
		baseline,
		form,
	);
	assert.equal(onlyStatement.form.statement, "我改的理由");
	assert.equal(onlyStatement.form.endTime, baseline.endTime);
	// 結束時間只能縮短時保留輸入（方向由 validateUpdate 擋，不在這裡靜默丟掉）。
	const shortenOnly = rebaseForm(measure({ canShorten: true }), baseline, form);
	assert.equal(shortenOnly.form.endTime, edit);
	assert.equal(shortenOnly.form.statement, baseline.statement);
});

test("延後複審：期限要晚於現在、30 天內；註記必填（沿用建立措施的規則）", () => {
	const ok = toLocalInput(new Date(NOW.getTime() + 7 * 24 * HOUR));
	assert.equal(
		validateSchedule({ reviewBy: ok, note: "再觀察" }, NOW),
		undefined,
	);
	assert.equal(
		validateSchedule({ reviewBy: "", note: "再觀察" }, NOW),
		"請填寫複審期限。",
	);
	assert.equal(
		validateSchedule(
			{ reviewBy: toLocalInput(new Date(NOW.getTime() - HOUR)), note: "x" },
			NOW,
		),
		"複審期限必須晚於現在。",
	);
	assert.equal(
		validateSchedule(
			{
				reviewBy: toLocalInput(new Date(NOW.getTime() + 31 * 24 * HOUR)),
				note: "x",
			},
			NOW,
		),
		"複審期限最多為現在起 30 天。",
	);
	assert.equal(
		validateSchedule({ reviewBy: ok, note: "  " }, NOW),
		"請填寫內部註記。",
	);
	const req = buildScheduleReview(measure({}), {
		reviewBy: ok,
		note: " 再觀察 ",
	});
	assert.equal(req.id, "m1");
	assert.equal(req.note, "再觀察");
	assert.equal(req.etag, "etag-1");
});

test("註記請求只帶 body（去掉前後空白），不帶 etag", () => {
	assert.deepEqual(buildCreateNote("m1", " 調查經過 "), {
		measureId: "m1",
		note: { body: "調查經過" },
	});
});

// ---------------------------------------------------------------------------
// 待處理請求

test("提出時的版本文字與差異對照", () => {
	assert.equal(
		submittedVersionText(2, 3),
		"賣家提出時是第 2 版，目前是第 3 版",
	);
	const revisions = [
		create(MeasureRevisionSchema, {
			revision: 2,
			statement: "舊理由",
			endTime: timestampFromDate(END),
		}),
		create(MeasureRevisionSchema, { revision: 3, statement: "新理由" }),
	];
	const m = measure({ revisions, currentRevision: 3 });
	const cmp = revisionComparison(m, 2);
	assert.equal(cmp?.submitted.statement, "舊理由");
	assert.equal(cmp?.submitted.endTime?.getTime(), END.getTime());
	assert.equal(cmp?.current.statement, "新理由");
	assert.equal(cmp?.current.endTime, undefined);
	assert.equal(revisionComparison(m, 3), undefined, "版本相同不並列");
	assert.equal(revisionComparison(m, 1), undefined, "找不到那一版不並列");
});

test("佐證連結只收伺服器給的 /api/files/admin/ 相對路徑，經同源 /api/bff；其他不給連結", () => {
	const links = evidenceLinks([
		create(EvidenceSchema, {
			position: 0,
			viewUri: "/api/files/admin/seller/measure_requests/r1/evidence/0",
		}),
		create(EvidenceSchema, {
			position: 1,
			viewUri: "https://evil.example/x",
		}),
		create(EvidenceSchema, { position: 2, viewUri: "" }),
	]);
	assert.deepEqual(links, [
		{
			label: "佐證 1",
			href: "/api/bff/api/files/admin/seller/measure_requests/r1/evidence/0",
		},
		{ label: "佐證 2", href: undefined },
		{ label: "佐證 3", href: undefined },
	]);
});
