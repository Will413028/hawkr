import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	LinkedIdentity,
	type TerminationOption,
	TerminationOptionSchema,
	TerminationViolationRequirement,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	MeasureBasis,
	MeasureKind,
	MeasurePeriodSchema,
	MeasureState,
	RevokeReason,
	RiskGround,
	TerminationGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	appealDeadlineText,
	availableTerminationOptions,
	buildCreateTermination,
	citedFindingsFieldFor,
	LINKED_IDENTITY_LABEL,
	linkedIdentityLabel,
	measureBasisLabel,
	measureBasisText,
	measureKindLabel,
	measureStateBadge,
	requestIdFor,
	revokeDetailText,
	revokeReasonLabel,
	scheduledTerminationText,
	TERMINATION_COMMON_TEXT,
	type TerminationChoice,
	type TerminationForm,
	terminationAvailability,
	terminationCreatedText,
	terminationExplanation,
	terminationGroundLabel,
	terminationRequestKey,
	validateTermination,
} from "../src/app/admin/merchants/[id]/measure";

/**
 * 終止合作（2c-3）的 admin 邏輯：標籤表窮舉 enum（新增值時失敗）、建立終止的驗證與請求、說明文字、request_id 的鍵、
 * 預定終止與申覆期限的呈現、關聯終止的可建立條件。時間都由測試傳入，不讀時鐘。
 */

/** 建立終止可選的事由（伺服器給的清單的內容：enum 去掉 UNSPECIFIED 與 LINKED_ACCOUNT）。 */
const SELECTABLE_GROUNDS: readonly TerminationGround[] = Object.values(
	TerminationGround,
).filter(
	(v): v is TerminationGround =>
		typeof v === "number" &&
		v !== TerminationGround.UNSPECIFIED &&
		v !== TerminationGround.LINKED_ACCOUNT,
);

const NOW = new Date("2026-10-15T02:00:00Z");

function numericValues<T extends object>(e: T): number[] {
	return Object.values(e).filter((v): v is number => typeof v === "number");
}

/** 標籤表必須有 enum 每個非 0 值的文字（0 是 UNSPECIFIED，不是有效值），而且表裡沒有多餘的鍵。 */
function assertExhaustive(
	name: string,
	table: Record<number, string>,
	enumObject: object,
) {
	const expected = numericValues(enumObject).filter((v) => v !== 0);
	assert.ok(expected.length > 0, `${name}：enum 讀不到任何值，測試對象已改變`);
	for (const value of expected) {
		assert.ok(table[value], `${name} 的值 ${value} 沒有呈現文字`);
	}
	assert.deepEqual(
		Object.keys(table)
			.map(Number)
			.sort((a, b) => a - b),
		[...expected].sort((a, b) => a - b),
		`${name}：標籤表的鍵必須與 enum（去掉 UNSPECIFIED）完全相同`,
	);
}

/** 文字取自共用文案目錄的 enum：每個非 UNSPECIFIED 的值都查得到文字（不是「未知」），新增值時這裡會失敗。 */
function assertCatalogCovers(
	name: string,
	labelOf: (value: never) => string,
	enumObject: object,
) {
	const expected = numericValues(enumObject).filter((v) => v !== 0);
	assert.ok(expected.length > 0, `${name}：enum 讀不到任何值，測試對象已改變`);
	for (const value of expected) {
		const text = labelOf(value as never);
		assert.ok(
			text !== "" && text !== "未知",
			`${name} 的值 ${value} 在共用文案目錄沒有文字`,
		);
	}
	assert.equal(labelOf(0 as never), "未知", `${name}：UNSPECIFIED 是「未知」`);
	assert.equal(labelOf(9999 as never), "未知", `${name}：不認得的值是「未知」`);
}

/** validateTermination 的錯誤（沒有錯誤是 undefined）。 */
function errorOf(form: TerminationForm): string | undefined {
	return validateTermination(form).error;
}

function option(
	ground: TerminationGround,
	violationRequirement: TerminationViolationRequirement,
	message?: string,
): TerminationOption {
	return create(TerminationOptionSchema, {
		ground,
		violationRequirement,
		...(message ? { explanation: { locale: "zh-TW", message } } : {}),
	});
}

/** 一般事由（店家詳情的 available_terminations）選到的終止合作。 */
function available(
	ground: TerminationGround,
	violationRequirement: TerminationViolationRequirement,
	citedFindingIds: readonly string[] = [],
): TerminationChoice {
	return {
		from: "available",
		option: option(ground, violationRequirement),
		citedFindingIds,
	};
}

/** 關聯帳號（LinkedAccount.termination_option）選到的終止合作。 */
function linked(sourceMeasureId: string): TerminationChoice {
	return {
		from: "linked",
		option: option(
			TerminationGround.LINKED_ACCOUNT,
			TerminationViolationRequirement.FORBIDDEN,
		),
		sourceMeasureId,
	};
}

function form(patch: Partial<TerminationForm> = {}): TerminationForm {
	return {
		choice: available(
			TerminationGround.CEASED_BUSINESS,
			TerminationViolationRequirement.OPTIONAL,
		),
		statement: "店家已停業",
		note: "客服電話確認",
		...patch,
	};
}

/** 已選到終止合作的輸入（buildCreateTermination 與 terminationRequestKey 的參數）。 */
function ready(
	patch: Partial<TerminationForm> & { choice: TerminationChoice },
): TerminationForm & { choice: TerminationChoice } {
	return { ...form(), ...patch, choice: patch.choice };
}

test("文字窮舉 enum：種類、依據、終止事由、撤銷原因（共用目錄）與身分類別（admin 單端的表）", () => {
	assertCatalogCovers("measureKindLabel", measureKindLabel, MeasureKind);
	assertCatalogCovers("measureBasisLabel", measureBasisLabel, MeasureBasis);
	assertCatalogCovers(
		"terminationGroundLabel",
		terminationGroundLabel,
		TerminationGround,
	);
	assertCatalogCovers("revokeReasonLabel", revokeReasonLabel, RevokeReason);
	assertExhaustive(
		"LINKED_IDENTITY_LABEL",
		LINKED_IDENTITY_LABEL,
		LinkedIdentity,
	);
});

test("標籤文字照 proto 註解的標籤：終止合作、預定生效、契約條款、事由、SOURCE_REVOKED", () => {
	assert.equal(measureKindLabel(MeasureKind.TERMINATION), "終止合作");
	assert.equal(
		measureStateBadge(MeasureState.SCHEDULED, MeasureKind.TERMINATION).label,
		"預定生效",
	);
	assert.equal(measureBasisLabel(MeasureBasis.CONTRACT), "契約條款");
	assert.deepEqual(SELECTABLE_GROUNDS.map(terminationGroundLabel), [
		"預告終止（第 21.2 條）",
		"重大違約未改善",
		"停業",
		"解散",
		"破產",
		"必要許可遭撤銷",
		"重大食安事件",
		"詐欺事件",
		"危害平台安全",
		"其他嚴重影響履約的情形",
	]);
	assert.equal(
		terminationGroundLabel(TerminationGround.LINKED_ACCOUNT),
		"關聯帳號",
	);
	assert.equal(
		revokeReasonLabel(RevokeReason.SOURCE_REVOKED),
		"關聯的終止已撤銷",
	);
	assert.deepEqual(
		[
			LinkedIdentity.NATIONAL_ID,
			LinkedIdentity.UBN,
			LinkedIdentity.RESPONSIBLE_PERSON_ID,
			LinkedIdentity.BANK_ACCOUNT,
		].map(linkedIdentityLabel),
		["身分證號", "統編", "負責人身分證號", "銀行帳號"],
	);
	assert.equal(terminationGroundLabel(TerminationGround.UNSPECIFIED), "未知");
	assert.equal(linkedIdentityLabel(LinkedIdentity.UNSPECIFIED), "未知");
});

test("狀態徽章：SCHEDULED 是警示色，不是停權中的紅色", () => {
	assert.equal(
		measureStateBadge(MeasureState.SCHEDULED, MeasureKind.TERMINATION)?.variant,
		"warning",
	);
});

test("依據文字：終止帶事由（契約依據與重大違約的違規依據），種類 UNSPECIFIED 與錯置的 CONTRACT 回「未知」", () => {
	const base = { riskGround: RiskGround.UNSPECIFIED };
	assert.equal(
		measureBasisText({
			...base,
			kind: MeasureKind.TERMINATION,
			basis: MeasureBasis.CONTRACT,
			terminationGround: TerminationGround.NOTICE,
		}),
		"終止合作・契約條款：預告終止（第 21.2 條）",
	);
	assert.equal(
		measureBasisText({
			...base,
			kind: MeasureKind.TERMINATION,
			basis: MeasureBasis.VIOLATION,
			terminationGround: TerminationGround.MATERIAL_BREACH,
		}),
		"終止合作・違規認定：重大違約未改善",
	);
	assert.equal(
		measureBasisText({
			...base,
			kind: MeasureKind.SUSPENSION,
			basis: MeasureBasis.CONTRACT,
			terminationGround: TerminationGround.UNSPECIFIED,
		}),
		"未知",
		"契約依據只配終止合作",
	);
	assert.equal(
		measureBasisText({
			...base,
			kind: MeasureKind.UNSPECIFIED,
			basis: MeasureBasis.CONTRACT,
			terminationGround: TerminationGround.NOTICE,
		}),
		"未知",
	);
});

test("撤銷詳情：連帶撤銷顯示「關聯的終止已撤銷」", () => {
	assert.equal(
		revokeDetailText({
			state: MeasureState.REVOKED,
			revokeTime: timestampFromDate(NOW),
			revokeReason: RevokeReason.SOURCE_REVOKED,
		})?.endsWith("・關聯的終止已撤銷"),
		true,
	);
});

test("驗證：要選到終止合作；理由與註記必填且有上限", () => {
	assert.equal(errorOf(form()), undefined);
	assert.equal(errorOf(form({ choice: undefined })), "請選擇終止事由。");
	assert.equal(errorOf(form({ statement: "  \n " })), "請填寫給賣家的理由。");
	assert.equal(
		errorOf(form({ statement: "字".repeat(2001) })),
		"給賣家的理由最多 2000 字。",
	);
	assert.equal(errorOf(form({ note: "" })), "請填寫內部註記。");
	assert.equal(
		errorOf(form({ note: "字".repeat(2001) })),
		"內部註記最多 2000 字。",
	);
});

test("驗證：依據的認定依選項的需求，不依事由：REQUIRED 至少選一筆、OPTIONAL 不選也能送出、FORBIDDEN 選了也不算", () => {
	const choice = (
		requirement: TerminationViolationRequirement,
		ids: string[],
		ground = TerminationGround.FRAUD,
	) => form({ choice: available(ground, requirement, ids) });
	assert.equal(
		errorOf(choice(TerminationViolationRequirement.REQUIRED, [])),
		"請選擇依據的認定。",
	);
	assert.equal(
		errorOf(choice(TerminationViolationRequirement.REQUIRED, ["f1"])),
		undefined,
	);
	assert.equal(
		errorOf(choice(TerminationViolationRequirement.OPTIONAL, [])),
		undefined,
	);
	assert.equal(
		errorOf(choice(TerminationViolationRequirement.FORBIDDEN, [])),
		undefined,
	);
	// 不比較事由：重大違約若伺服器說 OPTIONAL，就是選填。
	assert.equal(
		errorOf(
			choice(
				TerminationViolationRequirement.OPTIONAL,
				[],
				TerminationGround.MATERIAL_BREACH,
			),
		),
		undefined,
	);
	assert.equal(
		errorOf(form({ choice: linked("9") })),
		undefined,
		"關聯帳號的來源措施由型別保證，不另外檢查",
	);
});

test("請求：事由是選項的 ground，依據只帶選項要的欄位，沒有結束時間與複審期限，理由與註記去掉前後空白", () => {
	const plain = buildCreateTermination(
		"rid",
		"100",
		ready({
			statement: "  店家已停業 ",
			note: " 電話確認\n",
			// 選填而沒選：不送 violation。
			choice: available(
				TerminationGround.CEASED_BUSINESS,
				TerminationViolationRequirement.OPTIONAL,
			),
		}),
	);
	assert.deepEqual(plain, {
		requestId: "rid",
		storeId: "100",
		statement: "店家已停業",
		note: "電話確認",
		measure: {
			case: "termination",
			value: { ground: TerminationGround.CEASED_BUSINESS },
		},
	});
	assert.equal("endTime" in plain, false);
	assert.equal("reviewBy" in plain, false);

	const breach = buildCreateTermination(
		"rid",
		"100",
		ready({
			choice: available(
				TerminationGround.MATERIAL_BREACH,
				TerminationViolationRequirement.REQUIRED,
				["f2", "f1"],
			),
		}),
	);
	// 引用的認定是集合：排序後送出（勾選順序是 f2、f1）。
	assert.deepEqual(breach.measure, {
		case: "termination",
		value: {
			ground: TerminationGround.MATERIAL_BREACH,
			violation: { citedFindingIds: ["f1", "f2"] },
		},
	});

	// 選填而有選：送出；需求是 FORBIDDEN 時殘留的勾選不送（伺服器會拒絕）。
	assert.deepEqual(
		buildCreateTermination(
			"rid",
			"100",
			ready({
				choice: available(
					TerminationGround.FRAUD,
					TerminationViolationRequirement.OPTIONAL,
					["f3"],
				),
			}),
		).measure,
		{
			case: "termination",
			value: {
				ground: TerminationGround.FRAUD,
				violation: { citedFindingIds: ["f3"] },
			},
		},
	);
	assert.deepEqual(
		buildCreateTermination(
			"rid",
			"100",
			ready({
				choice: available(
					TerminationGround.NOTICE,
					TerminationViolationRequirement.FORBIDDEN,
					["f3"],
				),
			}),
		).measure,
		{
			case: "termination",
			value: { ground: TerminationGround.NOTICE },
		},
	);

	const fromLinked = buildCreateTermination(
		"rid",
		"200",
		ready({ choice: linked("9") }),
	);
	assert.deepEqual(fromLinked.measure, {
		case: "termination",
		value: {
			ground: TerminationGround.LINKED_ACCOUNT,
			sourceMeasureId: "9",
		},
	});
	assert.equal(fromLinked.storeId, "200", "關聯終止的目標是該列帳號的店家");
});

test("request_id 的鍵：只含會送出的欄位；同一份內容沿用，內容改過換新的", () => {
	const key = (f: TerminationForm & { choice: TerminationChoice }) =>
		terminationRequestKey("100", f);
	const base = ready({ choice: form().choice as TerminationChoice });
	assert.equal(
		key(base),
		key({ ...base, statement: " 店家已停業  " }),
		"前後空白不算改過",
	);
	assert.equal(
		key(base),
		key({
			...base,
			choice: available(
				TerminationGround.CEASED_BUSINESS,
				TerminationViolationRequirement.FORBIDDEN,
				["f1"],
			),
		}),
		"不送出的欄位不影響鍵",
	);
	assert.notEqual(key(base), key({ ...base, note: "別的註記" }));
	assert.notEqual(
		key(base),
		terminationRequestKey("101", base),
		"換店家是新請求",
	);
	const breach = (ids: string[]) =>
		key(
			ready({
				choice: available(
					TerminationGround.MATERIAL_BREACH,
					TerminationViolationRequirement.REQUIRED,
					ids,
				),
			}),
		);
	assert.notEqual(breach(["f1"]), breach(["f1", "f2"]));
	assert.notEqual(
		key(ready({ choice: linked("9") })),
		key(ready({ choice: linked("10") })),
	);

	let n = 0;
	const generate = () => `id-${++n}`;
	const first = requestIdFor(null, key(base), generate);
	assert.equal(requestIdFor(first, key(base), generate), first);
	assert.notEqual(
		requestIdFor(first, key({ ...base, note: "x" }), generate).id,
		first.id,
	);
});

test("引用的認定是集合：換勾選順序（取消再勾回）是同一個請求、同一個鍵、同一個 request_id", () => {
	const breach = (ids: string[]) =>
		ready({
			choice: available(
				TerminationGround.MATERIAL_BREACH,
				TerminationViolationRequirement.REQUIRED,
				ids,
			),
		});
	const permutations = [
		["f1", "f2", "f3"],
		["f3", "f1", "f2"],
		["f2", "f3", "f1"],
		["f3", "f2", "f1"],
	];
	const first = permutations[0] ?? [];
	for (const ids of permutations) {
		assert.equal(
			terminationRequestKey("100", breach(ids)),
			terminationRequestKey("100", breach(first)),
			`${ids.join(",")} 的鍵要和 ${first.join(",")} 相同`,
		);
		assert.deepEqual(
			buildCreateTermination("rid", "100", breach(ids)),
			buildCreateTermination("rid", "100", breach(first)),
			`${ids.join(",")} 送出的請求要和 ${first.join(",")} 相同`,
		);
	}
	assert.deepEqual(
		buildCreateTermination("rid", "100", breach(["f3", "f1", "f2"])).measure,
		{
			case: "termination",
			value: {
				ground: TerminationGround.MATERIAL_BREACH,
				violation: { citedFindingIds: ["f1", "f2", "f3"] },
			},
		},
	);
	// 不同的集合還是不同的請求。
	assert.notEqual(
		terminationRequestKey("100", breach(["f1", "f2"])),
		terminationRequestKey("100", breach(["f1", "f3"])),
	);
	assert.notEqual(
		terminationRequestKey("100", breach(["f1", "f2"])),
		terminationRequestKey("100", breach(["f1", "f2", "f3"])),
	);

	// 重送：同一組認定換了順序，沿用上一次的 request_id。
	let n = 0;
	const generate = () => `id-${++n}`;
	const sent = requestIdFor(
		null,
		terminationRequestKey("100", breach(["f2", "f1"])),
		generate,
	);
	assert.equal(
		requestIdFor(
			sent,
			terminationRequestKey("100", breach(["f1", "f2"])),
			generate,
		),
		sent,
	);
	// 呼叫端的清單不被改動。
	const selection = ["f3", "f1", "f2"];
	buildCreateTermination("rid", "100", breach(selection));
	terminationRequestKey("100", breach(selection));
	assert.deepEqual(selection, ["f3", "f1", "f2"]);
});

test("說明文字（T7）：第一則取伺服器給的完整說明句（生效時機與建立條件），再加共同說明；client 不補規則、不推算日期或天數", () => {
	const withMessage = (ground: TerminationGround, message: string) =>
		option(ground, TerminationViolationRequirement.OPTIONAL, message);
	assert.deepEqual(terminationExplanation(undefined), []);
	assert.deepEqual(
		terminationExplanation(
			withMessage(TerminationGround.NOTICE, "伺服器的預告句"),
		),
		["伺服器的預告句", TERMINATION_COMMON_TEXT],
	);
	assert.deepEqual(
		terminationExplanation(
			withMessage(TerminationGround.MATERIAL_BREACH, "伺服器的重大違約句"),
		),
		["伺服器的重大違約句", TERMINATION_COMMON_TEXT],
	);
	// 關聯帳號不再有 client 寫的來源說明：條件在伺服器的句子裡。
	assert.deepEqual(
		terminationExplanation(
			withMessage(TerminationGround.LINKED_ACCOUNT, "伺服器的關聯句"),
		),
		["伺服器的關聯句", TERMINATION_COMMON_TEXT],
	);
	// 伺服器沒給說明句：沒有第一則，client 不補。
	assert.deepEqual(
		terminationExplanation(
			option(
				TerminationGround.DISSOLUTION,
				TerminationViolationRequirement.OPTIONAL,
			),
		),
		[TERMINATION_COMMON_TEXT],
	);
});

test("「依據的認定」欄位依選項的需求：REQUIRED 必填、OPTIONAL 選填、FORBIDDEN 與不認得的值不顯示；文字不寫 7 日規則", () => {
	const field = (requirement: TerminationViolationRequirement) =>
		citedFindingsFieldFor(option(TerminationGround.FRAUD, requirement));
	assert.deepEqual(field(TerminationViolationRequirement.REQUIRED), {
		required: true,
		emptyText: "這個帳號沒有生效中的認定，無法以這個事由終止。",
		noteText: "至少選一筆。",
	});
	assert.deepEqual(field(TerminationViolationRequirement.OPTIONAL), {
		required: false,
		emptyText: "這個帳號沒有生效中的認定。",
		noteText: "選填。",
	});
	assert.equal(field(TerminationViolationRequirement.FORBIDDEN), undefined);
	assert.equal(field(TerminationViolationRequirement.UNSPECIFIED), undefined);
	assert.equal(field(99 as TerminationViolationRequirement), undefined);
});

test("終止合作的來源檔沒有預告天數或天數常數：生效時間與天數只由伺服器給", async () => {
	const source = await readFile(
		"src/app/admin/merchants/[id]/measure.ts",
		"utf8",
	);
	assert.doesNotMatch(source, /NOTICE_DAYS|NOTICE_PERIOD/u);
	assert.doesNotMatch(source, /天生效|立即生效/u);
});

function scheduled(start: Date | undefined, patch = {}) {
	return {
		kind: MeasureKind.TERMINATION,
		state: MeasureState.SCHEDULED,
		period: start
			? create(MeasurePeriodSchema, { startTime: timestampFromDate(start) })
			: undefined,
		...patch,
	};
}

test("預定終止：取 SCHEDULED 的終止中生效時間最早的一筆；沒有、或沒有生效時間時不顯示", () => {
	const t1 = new Date("2026-11-14T02:00:00Z");
	const t2 = new Date("2026-11-20T02:00:00Z");
	assert.equal(scheduledTerminationText([]), undefined);
	assert.equal(
		scheduledTerminationText([scheduled(t2), scheduled(t1)]),
		"預定終止：2026/11/14 10:00",
	);
	assert.equal(
		scheduledTerminationText([scheduled(t1, { state: MeasureState.ACTIVE })]),
		undefined,
		"不是 SCHEDULED 的不算",
	);
	assert.equal(
		scheduledTerminationText([scheduled(t1, { kind: MeasureKind.SUSPENSION })]),
		undefined,
		"不是終止的不算",
	);
	assert.equal(
		scheduledTerminationText([scheduled(undefined)]),
		undefined,
		"沒有生效時間不是有效資料，不編一個時間",
	);
});

test("申覆期限：伺服器有給期限才顯示（只看欄位，不看種類）", () => {
	const deadline = timestampFromDate(new Date("2027-05-18T02:00:00Z"));
	assert.equal(
		appealDeadlineText({ appealDeadlineTime: deadline }),
		"申覆期限：2027/05/18 10:00",
	);
	assert.equal(
		appealDeadlineText({ appealDeadlineTime: undefined }),
		undefined,
	);
});

const fraudEffect = option(
	TerminationGround.FRAUD,
	TerminationViolationRequirement.OPTIONAL,
);
const ceasedEffect = option(
	TerminationGround.CEASED_BUSINESS,
	TerminationViolationRequirement.OPTIONAL,
);

test("終止合作的可用性全來自伺服器：旗標與可選事由缺一不可，選項照伺服器給的順序與內容，只丟掉沒有顯示文字的事由", () => {
	assert.deepEqual(terminationAvailability(undefined), {
		canCreate: false,
		options: [],
	});
	// 已關閉的店：伺服器只給第 21.3 條事由，預告不在內；旗標為真就有入口，下拉只有這些（整筆選項帶說明句與依據的需求）。
	assert.deepEqual(
		terminationAvailability({
			canCreateTermination: true,
			availableTerminations: [ceasedEffect, fraudEffect],
		}),
		{
			canCreate: true,
			options: [ceasedEffect, fraudEffect],
		},
	);
	assert.equal(
		terminationAvailability({
			canCreateTermination: false,
			availableTerminations: [fraudEffect],
		}).canCreate,
		false,
		"旗標為假就沒有入口，即使清單有值",
	);
	assert.equal(
		terminationAvailability({
			canCreateTermination: true,
			availableTerminations: [],
		}).canCreate,
		false,
		"旗標為真但沒有可選事由：不留沒有選項的入口",
	);
	// 順序照伺服器給的，不重排；只丟掉沒有顯示文字的事由（UNSPECIFIED、不認得的值）。
	assert.deepEqual(
		availableTerminationOptions([
			option(
				TerminationGround.UNSPECIFIED,
				TerminationViolationRequirement.OPTIONAL,
			),
			option(99 as TerminationGround, TerminationViolationRequirement.OPTIONAL),
			fraudEffect,
			option(
				TerminationGround.NOTICE,
				TerminationViolationRequirement.FORBIDDEN,
			),
		]).map((o) => o.ground),
		[TerminationGround.FRAUD, TerminationGround.NOTICE],
	);
});

test("生效中的終止合作的狀態徽章是「合作已終止」，停權仍是「停權中」；其他狀態不分種類", () => {
	assert.equal(
		measureStateBadge(MeasureState.ACTIVE, MeasureKind.TERMINATION)?.label,
		"合作已終止",
	);
	assert.equal(
		measureStateBadge(MeasureState.ACTIVE, MeasureKind.SUSPENSION)?.label,
		"停權中",
	);
	assert.equal(
		measureStateBadge(MeasureState.REVOKED, MeasureKind.TERMINATION)?.label,
		"已撤銷",
	);
});

test("建立成功提示：生效時間用伺服器回的 period.start_time，沒有就不編時間", () => {
	assert.equal(
		terminationCreatedText({
			period: create(MeasurePeriodSchema, {
				startTime: timestampFromDate(new Date("2026-11-14T02:00:00Z")),
			}),
		}),
		"已建立終止合作，生效時間：2026/11/14 10:00。",
	);
	assert.equal(terminationCreatedText(undefined), "已建立終止合作。");
});

test("終止合作：從案件建立時請求帶 source_case_id，不是就不帶；來源案件算進 request_id 的鍵", () => {
	const base = ready({ choice: form().choice as TerminationChoice });
	const withCase = buildCreateTermination("rid", "100", {
		...base,
		sourceCaseId: "case-1",
	});
	assert.equal(withCase.sourceCaseId, "case-1");
	assert.equal(
		"sourceCaseId" in buildCreateTermination("rid", "100", base),
		false,
	);

	const key = (sourceCaseId?: string) =>
		terminationRequestKey("100", { ...base, sourceCaseId });
	assert.equal(key(undefined), key(""));
	assert.notEqual(key("case-1"), key(undefined));
	assert.notEqual(key("case-1"), key("case-2"));
});
