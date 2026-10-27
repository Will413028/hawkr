import assert from "node:assert/strict";
import test from "node:test";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	MeasureBasis,
	MeasureKind,
	MeasureState,
	RevokeReason,
	RiskGround,
	TerminationGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	buildCreateRiskMeasure,
	defaultReviewBy,
	fromLocalInput,
	isResend,
	MEASURE_CONFLICT_TEXT,
	measureBasisText,
	measureStateBadge,
	NOTE_MAX_LENGTH,
	RISK_GROUND_OPTIONS,
	type RiskMeasureForm,
	requestIdFor,
	requestKey,
	revokeDetailText,
	riskGroundLabel,
	STATEMENT_MAX_LENGTH,
	sourceCaseField,
	toLocalInput,
	validateRiskMeasure,
} from "../src/app/admin/merchants/[id]/measure";
import { actionErrorText, runAction } from "../src/lib/connect-action";

/**
 * 商家詳情的措施邏輯（measure.ts）：風險類型與狀態的文字窮舉 enum、建立風險措施的輸入驗證與請求、request_id 的沿用規則、
 * 結束措施的失敗處理。時間都由測試傳入固定的 now，不讀時鐘。
 */

// 固定的時刻（台北 2026-10-13 10:00）；測試不讀時鐘，也不跟著機器時區。
const NOW = new Date("2026-10-13T02:00:00Z");

function addDays(d: Date, days: number): Date {
	return new Date(d.getTime() + days * 24 * 60 * 60 * 1000);
}

function form(patch: Partial<RiskMeasureForm> = {}): RiskMeasureForm {
	return {
		ground: RiskGround.FRAUD,
		statement: "疑似詐欺，先行停權",
		endTime: "",
		reviewBy: toLocalInput(defaultReviewBy(NOW)),
		note: "客服轉來三筆檢舉",
		...patch,
	};
}

function numericValues<T extends object>(e: T): number[] {
	return Object.values(e).filter((v): v is number => typeof v === "number");
}

test("風險類型的文字窮舉 enum：每個非 UNSPECIFIED 的值都有文字，新增值時這裡會失敗", () => {
	const expected = numericValues(RiskGround).filter(
		(v) => v !== RiskGround.UNSPECIFIED,
	);
	assert.ok(expected.length > 0, "enum 讀不到任何值，測試對象已改變");
	for (const ground of expected) {
		const text = riskGroundLabel(ground);
		assert.ok(
			text !== "" && text !== "未知",
			`風險類型 ${RiskGround[ground]}（${ground}）在共用文案目錄沒有文字`,
		);
	}
	assert.deepEqual(
		[...RISK_GROUND_OPTIONS].sort(),
		expected.sort(),
		"下拉選項必須與 enum（去掉 UNSPECIFIED）完全相同",
	);
	assert.equal(riskGroundLabel(RiskGround.UNSPECIFIED), "未知");
	assert.equal(riskGroundLabel(99 as RiskGround), "未知");
});

test("措施狀態窮舉：五個狀態各有徽章（文字取自共用目錄），UNSPECIFIED 與不認得的值是灰色的「未知」，不被當成有效狀態", () => {
	for (const state of numericValues(MeasureState).filter(
		(v) => v !== MeasureState.UNSPECIFIED,
	)) {
		const badge = measureStateBadge(state, MeasureKind.SUSPENSION);
		assert.notEqual(badge.label, "未知", `${MeasureState[state]} 沒有徽章文字`);
	}
	for (const state of [MeasureState.UNSPECIFIED, 99 as MeasureState]) {
		assert.deepEqual(measureStateBadge(state, MeasureKind.SUSPENSION), {
			label: "未知",
			variant: "gray",
		});
	}
	// 生效中的終止合作不是停權：文字取自目錄的 select。
	assert.deepEqual(
		measureStateBadge(MeasureState.ACTIVE, MeasureKind.TERMINATION),
		{ label: "合作已終止", variant: "danger" },
	);
	assert.equal(
		measureStateBadge(MeasureState.ACTIVE, MeasureKind.SUSPENSION).variant,
		"danger",
	);
	assert.equal(
		measureStateBadge(MeasureState.REVOKED, MeasureKind.SUSPENSION).label,
		"已撤銷",
	);
});

test("依據文字：風險帶風險類型，違規指向認定，UNSPECIFIED 回「未知」", () => {
	const base = {
		kind: MeasureKind.SUSPENSION,
		riskGround: RiskGround.FRAUD,
		terminationGround: TerminationGround.UNSPECIFIED,
	};
	assert.equal(
		measureBasisText({ ...base, basis: MeasureBasis.RISK }),
		"停權・風險：詐欺",
	);
	assert.equal(
		measureBasisText({ ...base, basis: MeasureBasis.VIOLATION }),
		"停權・違規認定",
	);
	assert.equal(
		measureBasisText({ ...base, basis: MeasureBasis.UNSPECIFIED }),
		"未知",
	);
	// 不認得的種類或依據：整句是「未知」，和 UNSPECIFIED 一致，不組出「未知・…」的半句。
	assert.equal(
		measureBasisText({ ...base, basis: 9999 as MeasureBasis }),
		"未知",
	);
	assert.equal(
		measureBasisText({
			...base,
			kind: 9999 as MeasureKind,
			basis: MeasureBasis.RISK,
		}),
		"未知",
	);
	assert.equal(
		measureBasisText({
			...base,
			kind: MeasureKind.UNSPECIFIED,
			basis: MeasureBasis.RISK,
		}),
		"未知",
	);
	assert.equal(
		measureBasisText({
			...base,
			kind: MeasureKind.TERMINATION,
			basis: 9999 as MeasureBasis,
			terminationGround: TerminationGround.FRAUD,
		}),
		"未知",
	);
	assert.equal(
		measureBasisText({
			kind: MeasureKind.WARNING,
			basis: MeasureBasis.VIOLATION,
			riskGround: RiskGround.UNSPECIFIED,
			terminationGround: TerminationGround.UNSPECIFIED,
		}),
		"警告・違規認定",
	);
});

test("複審期限預設為現在起 7 天，datetime-local 字串可來回轉換（到分鐘）", () => {
	const review = defaultReviewBy(NOW);
	assert.equal(review.getTime() - NOW.getTime(), 7 * 24 * 60 * 60 * 1000);
	const text = toLocalInput(review);
	assert.match(text, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/u);
	assert.equal(text, "2026-10-20T10:00", "台北時間，不跟著機器時區");
	assert.equal(fromLocalInput(text)?.getTime(), review.getTime());
	assert.equal(
		fromLocalInput("2026-10-13T10:00")?.toISOString(),
		"2026-10-13T02:00:00.000Z",
	);
	assert.equal(fromLocalInput(""), undefined);
	assert.equal(fromLocalInput("明天"), undefined);
});

test("預設的輸入通過驗證", () => {
	assert.equal(validateRiskMeasure(form(), NOW), undefined);
});

test("UNSPECIFIED 不是有效的風險類型：擋下，不會送出", () => {
	assert.equal(
		validateRiskMeasure(form({ ground: RiskGround.UNSPECIFIED }), NOW),
		"請選擇風險類型。",
	);
});

test("給賣家的理由與內部註記：去掉空白後必填，上限 2000 字（以字元計，不是 UTF-16 code unit）", () => {
	assert.ok(validateRiskMeasure(form({ statement: "   " }), NOW));
	assert.ok(validateRiskMeasure(form({ note: "" }), NOW));
	assert.equal(
		validateRiskMeasure(
			form({ statement: "字".repeat(STATEMENT_MAX_LENGTH) }),
			NOW,
		),
		undefined,
	);
	assert.ok(
		validateRiskMeasure(
			form({ statement: "字".repeat(STATEMENT_MAX_LENGTH + 1) }),
			NOW,
		),
	);
	assert.equal(
		validateRiskMeasure(form({ note: "𠀀".repeat(NOTE_MAX_LENGTH) }), NOW),
		undefined,
	);
	assert.ok(
		validateRiskMeasure(form({ note: "字".repeat(NOTE_MAX_LENGTH + 1) }), NOW),
	);
});

test("複審期限：必填、必須晚於現在、最多現在起 30 天（含第 30 天整）", () => {
	assert.ok(validateRiskMeasure(form({ reviewBy: "" }), NOW), "必填");
	assert.ok(
		validateRiskMeasure(form({ reviewBy: toLocalInput(NOW) }), NOW),
		"等於現在不行",
	);
	assert.ok(
		validateRiskMeasure(
			form({ reviewBy: toLocalInput(addDays(NOW, -1)) }),
			NOW,
		),
		"過去不行",
	);
	assert.equal(
		validateRiskMeasure(
			form({ reviewBy: toLocalInput(addDays(NOW, 30)) }),
			NOW,
		),
		undefined,
		"剛好 30 天可以",
	);
	assert.ok(
		validateRiskMeasure(
			form({ reviewBy: toLocalInput(addDays(NOW, 31)) }),
			NOW,
		),
		"超過 30 天不行",
	);
});

test("結束時間：選填；填了必須晚於現在", () => {
	assert.equal(validateRiskMeasure(form({ endTime: "" }), NOW), undefined);
	assert.equal(
		validateRiskMeasure(form({ endTime: toLocalInput(addDays(NOW, 90)) }), NOW),
		undefined,
		"結束時間不受 30 天的複審上限限制",
	);
	assert.ok(
		validateRiskMeasure(form({ endTime: toLocalInput(addDays(NOW, -1)) }), NOW),
	);
	assert.ok(validateRiskMeasure(form({ endTime: toLocalInput(NOW) }), NOW));
});

test("CreateMeasure 請求：風險依據的停權，理由與註記去掉前後空白，沒填結束時間就不送", () => {
	const f = form({ statement: "  理由  ", note: " 註記 " });
	const req = buildCreateRiskMeasure("rid-1", "100", f);
	assert.equal(req.requestId, "rid-1");
	assert.equal(req.storeId, "100");
	assert.equal(req.measure?.case, "suspension", "種類就是設定的那支輸入");
	assert.equal(req.statement, "理由");
	assert.equal(req.note, "註記");
	const suspension =
		req.measure?.case === "suspension" ? req.measure.value : undefined;
	assert.ok(suspension, "風險依據的停權是 suspension 輸入");
	assert.equal(suspension.endTime, undefined, "開放式措施不送 end_time");
	assert.deepEqual(suspension.basis, {
		case: "risk",
		value: { ground: RiskGround.FRAUD },
	});
	assert.equal(
		suspension.reviewBy &&
			timestampDate(suspension.reviewBy as never).getTime(),
		fromLocalInput(f.reviewBy)?.getTime(),
	);
	const withEnd = buildCreateRiskMeasure(
		"rid-1",
		"100",
		form({ endTime: toLocalInput(addDays(NOW, 3)) }),
	);
	assert.ok(
		withEnd.measure?.case === "suspension" && withEnd.measure.value.endTime,
		"填了結束時間就送，放在停權輸入裡",
	);
	assert.equal(
		Object.keys(withEnd).includes("endTime") ||
			Object.keys(withEnd).includes("reviewBy") ||
			Object.keys(withEnd).includes("kind") ||
			Object.keys(withEnd).includes("basis"),
		false,
		"頂層沒有舊欄位",
	);
});

test("組請求時時間解析不出來就丟例外，不送缺值的請求", () => {
	assert.throws(() => buildCreateRiskMeasure("r", "1", form({ reviewBy: "" })));
	assert.throws(() =>
		buildCreateRiskMeasure("r", "1", form({ endTime: "亂填" })),
	);
});

// ---------------------------------------------------------------------------
// request_id

test("request_id：同一份內容重送沿用同一個；內容改過換新的；去掉前後空白不算改過", () => {
	let n = 0;
	const generate = () => `id-${++n}`;
	const key = requestKey("100", form());
	const first = requestIdFor(null, key, generate);
	assert.equal(first.id, "id-1");
	const resend = requestIdFor(first, requestKey("100", form()), generate);
	assert.equal(resend.id, "id-1", "同一份內容的重送必須沿用同一個 request_id");
	assert.equal(n, 1, "沿用時不得再產生");
	const padded = requestIdFor(
		first,
		requestKey("100", form({ statement: "  疑似詐欺，先行停權 " })),
		generate,
	);
	assert.equal(padded.id, "id-1", "只差前後空白是同一份內容");
	for (const changed of [
		form({ ground: RiskGround.SECURITY }),
		form({ statement: "改過的理由" }),
		form({ note: "改過的註記" }),
		form({ endTime: toLocalInput(addDays(NOW, 3)) }),
		form({ reviewBy: toLocalInput(addDays(NOW, 9)) }),
	]) {
		assert.notEqual(
			requestIdFor(first, requestKey("100", changed), generate).id,
			"id-1",
			"內容改過必須換新的 request_id（否則伺服器回同 id 不同內容的 invalid_argument）",
		);
	}
	assert.notEqual(
		requestIdFor(first, requestKey("200", form()), generate).id,
		"id-1",
		"不同店家是不同的請求",
	);
});

test("request_id：重新開啟（沒有上一個 slot）一定是新的", () => {
	let n = 0;
	const generate = () => `id-${++n}`;
	const key = requestKey("100", form());
	const a = requestIdFor(null, key, generate);
	const b = requestIdFor(null, key, generate);
	assert.notEqual(a.id, b.id);
});

// ---------------------------------------------------------------------------
// 失敗處理

function versionConflict(): ConnectError {
	return new ConnectError("etag mismatch", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
		},
	]);
}

function withMessage(code: Code, message: string): ConnectError {
	return new ConnectError("raw", code, undefined, [
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message },
		},
	]);
}

test("VERSION_CONFLICT：重讀一次、回 conflict 的說明", async () => {
	let reloads = 0;
	const result = await runAction(() => Promise.reject(versionConflict()), {
		reload: async () => {
			reloads++;
		},
		fallback: "結束失敗",
		conflictText: MEASURE_CONFLICT_TEXT,
	});
	assert.deepEqual(result, {
		ok: false,
		kind: "conflict",
		text: MEASURE_CONFLICT_TEXT,
		code: Code.Aborted,
		noResponse: false,
	});
	assert.equal(reloads, 1);
});

test("重讀本身失敗也不丟例外，結果仍是 conflict；但不能說「頁面已重新載入」，改用伺服器自己的句子", async () => {
	const result = await runAction(() => Promise.reject(versionConflict()), {
		reload: () => Promise.reject(new Error("網路")),
		fallback: "結束失敗",
		conflictText: MEASURE_CONFLICT_TEXT,
	});
	assert.equal(!result.ok && result.kind, "conflict");
	assert.ok(!result.ok && result.text !== MEASURE_CONFLICT_TEXT);
	assert.ok(!result.ok && !result.text.includes("已重新載入"));
});

test("版本衝突的說明可以依 ErrorInfo.metadata 挑", async () => {
	const result = await runAction(() => Promise.reject(versionConflict()), {
		reload: async () => {},
		fallback: "fb",
		conflictText: (metadata) =>
			metadata.conflictTarget === "x" ? "X" : "other",
	});
	assert.equal(!result.ok && result.text, "other");
});

test("其他失敗：顯示伺服器的訊息、不重讀；網路層失敗與沒有訊息時用 fallback；成功回 ok 與寫入的回應", async () => {
	let reloads = 0;
	const reload = async () => {
		reloads++;
	};
	assert.deepEqual(
		await runAction(async () => "寫入的回應", {
			reload,
			fallback: "fb",
			conflictText: MEASURE_CONFLICT_TEXT,
		}),
		{ ok: true, value: "寫入的回應" },
	);
	assert.deepEqual(
		await runAction(
			() =>
				Promise.reject(
					withMessage(Code.FailedPrecondition, "這個措施已經結束"),
				),
			{ reload, fallback: "fb", conflictText: MEASURE_CONFLICT_TEXT },
		),
		{
			ok: false,
			kind: "message",
			text: "這個措施已經結束",
			code: Code.FailedPrecondition,
			noResponse: false,
		},
	);
	assert.deepEqual(
		await runAction(() => Promise.reject(new TypeError("Failed to fetch")), {
			reload,
			fallback: "fb",
			conflictText: MEASURE_CONFLICT_TEXT,
		}),
		{ ok: false, kind: "message", text: "fb", noResponse: true },
	);
	assert.equal(reloads, 0, "只有版本衝突才重讀");
});

test("actionErrorText：欄位錯誤合併成伺服器給的訊息或第一個欄位的說明；網路層失敗用 fallback", () => {
	const fields = new ConnectError("bad", Code.InvalidArgument, undefined, [
		{
			desc: BadRequestSchema,
			value: {
				fieldViolations: [
					{
						field: "review_by",
						description: "review_by exceeds 30 days",
						localizedMessage: {
							locale: "zh-TW",
							message: "複審期限超過 30 天",
						},
					},
				],
			},
		},
	]);
	assert.equal(
		actionErrorText(fields, "fb", MEASURE_CONFLICT_TEXT),
		"複審期限超過 30 天",
	);
	assert.equal(
		actionErrorText(new TypeError("x"), "fb", MEASURE_CONFLICT_TEXT),
		"fb",
	);
	assert.equal(
		actionErrorText(versionConflict(), "fb", MEASURE_CONFLICT_TEXT),
		MEASURE_CONFLICT_TEXT,
	);
});

test("三端共用的文字（seller.proto 各值註解的標籤）：風險類型、措施狀態、種類逐一對照", () => {
	assert.deepEqual(RISK_GROUND_OPTIONS.map(riskGroundLabel), [
		"交易風險",
		"食品安全",
		"詐欺",
		"個人資料",
		"侵權",
		"資安",
		"履約",
		"主管機關或司法機關的要求",
	]);
	assert.deepEqual(
		[
			MeasureState.ISSUED,
			MeasureState.SCHEDULED,
			MeasureState.ACTIVE,
			MeasureState.ENDED,
			MeasureState.REVOKED,
		].map((state) => measureStateBadge(state, MeasureKind.SUSPENSION).label),
		["已發出", "預定生效", "停權中", "已結束", "已撤銷"],
	);
	assert.notEqual(
		measureStateBadge(MeasureState.ISSUED, MeasureKind.SUSPENSION).variant,
		"danger",
	);
});

test("種類 UNSPECIFIED 不顯示成停權：依據文字回「未知」", () => {
	assert.equal(
		measureBasisText({
			kind: MeasureKind.UNSPECIFIED,
			basis: MeasureBasis.RISK,
			riskGround: RiskGround.FRAUD,
			terminationGround: TerminationGround.UNSPECIFIED,
		}),
		"未知",
	);
});

test("已撤銷的措施顯示撤銷時間與原因（含連帶撤銷）；沒撤銷的不顯示", () => {
	const revokeTime = timestampFromDate(new Date("2026-10-13T02:00:00Z"));
	assert.equal(
		revokeDetailText({
			state: MeasureState.REVOKED,
			revokeTime,
			revokeReason: RevokeReason.FINDING_REVOKED,
		}),
		"2026/10/13 10:00・依據的認定已撤銷",
	);
	assert.equal(
		revokeDetailText({
			state: MeasureState.REVOKED,
			revokeTime,
			revokeReason: RevokeReason.APPEAL_ACCEPTED,
		}),
		"2026/10/13 10:00・申覆成立",
	);
	assert.equal(
		revokeDetailText({
			state: MeasureState.REVOKED,
			revokeTime: undefined,
			revokeReason: RevokeReason.UNSPECIFIED,
		}),
		"-・未知",
	);
	assert.equal(
		revokeDetailText({
			state: MeasureState.ENDED,
			revokeTime: undefined,
			revokeReason: RevokeReason.UNSPECIFIED,
		}),
		undefined,
	);
});

test("同一份內容的重送不再用現在的時間擋：結束時間、複審期限過了也放行；格式與必填照常檢查", () => {
	const sent = form({
		endTime: toLocalInput(addDays(NOW, 1)),
		reviewBy: toLocalInput(addDays(NOW, 7)),
	});
	const later = addDays(NOW, 10);
	assert.ok(validateRiskMeasure(sent, later), "新的送出仍受時間檢查");
	assert.equal(validateRiskMeasure(sent, later, true), undefined);
	assert.ok(validateRiskMeasure({ ...sent, reviewBy: "" }, later, true));
	assert.ok(validateRiskMeasure({ ...sent, statement: " " }, later, true));
	const key = requestKey("11", sent);
	assert.equal(isResend(null, key), false);
	assert.equal(isResend({ key, id: "r1" }, key), true);
	assert.equal(isResend({ key: "other", id: "r1" }, key), false);
});

// ---------------------------------------------------------------------------
// 從案件建立：來源案件（source_case_id）

test("sourceCaseField：沒有案件（沒給或空字串）時請求不帶這個欄位，有就帶", () => {
	assert.deepEqual(sourceCaseField(undefined), {});
	assert.deepEqual(sourceCaseField(""), {});
	assert.deepEqual(sourceCaseField("case-1"), { sourceCaseId: "case-1" });
});

test("風險措施：從案件建立時請求帶 source_case_id，不是就不帶；來源案件算進 request_id 的鍵（換案件是不同的請求）", () => {
	const withCase = buildCreateRiskMeasure(
		"rid",
		"100",
		form({ sourceCaseId: "case-1" }),
	);
	assert.equal(withCase.sourceCaseId, "case-1");
	const plain = buildCreateRiskMeasure("rid", "100", form());
	assert.equal("sourceCaseId" in plain, false);

	const key = (sourceCaseId?: string) =>
		requestKey("100", form({ sourceCaseId }));
	assert.equal(key(undefined), key(""), "沒有案件與空字串是同一個請求");
	assert.notEqual(key("case-1"), key(undefined));
	assert.notEqual(key("case-1"), key("case-2"));
});
