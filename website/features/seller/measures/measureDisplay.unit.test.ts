import { create } from "@bufbuild/protobuf";
import { DurationSchema, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	AppealUnavailableReason,
	MeasureBasis,
	MeasureFindingSchema,
	MeasureKind,
	MeasurePeriodSchema,
	MeasureRequestCloseCause,
	MeasureRequestKind,
	MeasureRequestSchema,
	MeasureRequestState,
	MeasureSchema,
	MeasureState,
	RemediationUnavailableReason,
	RevokeReason,
	RiskGround,
	TerminationGround,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	APPEAL_SENTENCE,
	REMEDIATION_SENTENCE,
	unavailable,
} from "@/tests/support/server-unavailability";
import {
	appealEntry,
	appealHref,
	measureHref,
	measureMeta,
	measurePeriodText,
	measureStatusChips,
	measureStatusDisplay,
	measureTitle,
	rejectionCooldownWarning,
	remediationEntry,
	remediationHref,
	requestNote,
	revokeNote,
	terminationNoticeText,
} from "./measureDisplay";

const ut = uiTextFor("zh-TW");
const measure = (init: Parameters<typeof create<typeof MeasureSchema>>[1]) =>
	create(MeasureSchema, init);
const finding = (category: PolicyCategory, severity: ViolationSeverity) =>
	create(MeasureFindingSchema, { id: "f", category, severity });
const request = (
	kind: MeasureRequestKind,
	state: MeasureRequestState,
	extra: Parameters<typeof create<typeof MeasureRequestSchema>>[1] = {},
) => create(MeasureRequestSchema, { kind, state, ...extra });
const appeal = (state: MeasureRequestState, decisionReason = "") =>
	request(MeasureRequestKind.APPEAL, state, { decisionReason });
// 2026-10-01 02:00 UTC = 台北 10:00
const at = (iso: string) => timestampFromDate(new Date(iso));

describe("依據的名稱", () => {
	it("認定的類別取第一筆（主要依據）；認定可用的每個政策分類都有文字", () => {
		const labels = [
			PolicyCategory.FOOD_SAFETY,
			PolicyCategory.PROHIBITED_GOODS,
			PolicyCategory.UNLICENSED_SALE,
			PolicyCategory.MISLEADING_INFO,
			PolicyCategory.FAKE_CERTIFICATION,
			PolicyCategory.COUNTERFEIT,
			PolicyCategory.IP_INFRINGEMENT,
			PolicyCategory.FRAUD,
			PolicyCategory.OFF_PLATFORM_TRADE,
			PolicyCategory.PRIVACY,
			PolicyCategory.IMPERSONATION,
			PolicyCategory.PLATFORM_ABUSE,
		].map((category) =>
			measureTitle(
				ut,
				measure({
					basis: MeasureBasis.VIOLATION,
					findings: [finding(category, ViolationSeverity.LOW)],
				}),
			),
		);
		expect(new Set(labels).size).toBe(12);
		expect(labels).not.toContain("措施紀錄");
		// 術語表的名稱：「個人資料」本身不是違規的名稱，所以是「不當使用個人資料」。
		expect(labels).toContain("不當使用個人資料");
		expect(labels).toContain("食品安全");
		expect(
			measureTitle(
				ut,
				measure({
					basis: MeasureBasis.VIOLATION,
					findings: [
						finding(PolicyCategory.FRAUD, ViolationSeverity.LOW),
						finding(PolicyCategory.FOOD_SAFETY, ViolationSeverity.LOW),
					],
				}),
			),
		).toBe("詐欺或操縱交易");
	});

	it("認定的類別沒有值或沒有認定：通用的措施紀錄（與 seller-app 同字）", () => {
		expect(
			measureTitle(
				ut,
				measure({
					basis: MeasureBasis.VIOLATION,
					findings: [
						finding(PolicyCategory.UNSPECIFIED, ViolationSeverity.LOW),
					],
				}),
			),
		).toBe("措施紀錄");
		expect(measureTitle(ut, measure({ basis: MeasureBasis.VIOLATION }))).toBe(
			"措施紀錄",
		);
	});

	it("風險依據顯示「風險：{風險類型}」，每個有效值都有不同文字；UNSPECIFIED 用通用文字", () => {
		const labels = [
			RiskGround.TRANSACTION,
			RiskGround.FOOD_SAFETY,
			RiskGround.FRAUD,
			RiskGround.PERSONAL_DATA,
			RiskGround.INFRINGEMENT,
			RiskGround.SECURITY,
			RiskGround.FULFILLMENT,
			RiskGround.EXTERNAL_ORDER,
		].map((riskGround) =>
			measureTitle(ut, measure({ basis: MeasureBasis.RISK, riskGround })),
		);
		expect(new Set(labels).size).toBe(8);
		expect(labels).toEqual([
			"風險：交易風險",
			"風險：食品安全",
			"風險：詐欺",
			"風險：個人資料",
			"風險：侵權",
			"風險：資安",
			"風險：履約",
			"風險：主管機關或司法機關的要求",
		]);
		expect(labels).not.toContain("風險措施");
		expect(
			measureTitle(
				ut,
				measure({
					basis: MeasureBasis.RISK,
					riskGround: RiskGround.UNSPECIFIED,
				}),
			),
		).toBe("風險措施");
	});

	it("依據是 UNSPECIFIED 不是有效值：通用文字，不當成違規", () => {
		expect(measureTitle(ut, measure({ basis: MeasureBasis.UNSPECIFIED }))).toBe(
			"措施紀錄",
		);
	});
});

describe("measureMeta", () => {
	it("嚴重程度（認定才有）加措施；UNSPECIFIED 不顯示", () => {
		expect(
			measureMeta(
				ut,
				measure({
					kind: MeasureKind.SUSPENSION,
					findings: [finding(PolicyCategory.FRAUD, ViolationSeverity.HIGH)],
				}),
			),
		).toBe("嚴重程度：高 · 措施：停權");
		expect(measureMeta(ut, measure({ kind: MeasureKind.SUSPENSION }))).toBe(
			"措施：停權",
		);
		expect(measureMeta(ut, measure({ kind: MeasureKind.WARNING }))).toBe(
			"措施：警告",
		);
		expect(
			measureMeta(ut, measure({ kind: MeasureKind.UNSPECIFIED })),
		).toBeUndefined();
	});
});

describe("measurePeriodText", () => {
	it("警告沒有期間", () => {
		expect(measurePeriodText(ut, measure({ kind: MeasureKind.WARNING }))).toBe(
			undefined,
		);
	});

	it("有結束時間：以臺北時間顯示起訖", () => {
		expect(
			measurePeriodText(
				ut,
				measure({
					period: create(MeasurePeriodSchema, {
						startTime: at("2026-10-01T02:00:00Z"),
						endTime: at("2026-10-08T16:30:00Z"),
					}),
				}),
			),
		).toBe("停權期間：2026/10/01 10:00 至 2026/10/09 00:30");
	});

	it("沒有結束時間：未定結束時間，平台定期複審", () => {
		const text = measurePeriodText(
			ut,
			measure({
				period: create(MeasurePeriodSchema, {
					startTime: at("2026-10-01T02:00:00Z"),
				}),
			}),
		);
		expect(text).toBe(
			"停權期間：2026/10/01 10:00 起，未定結束時間，平台定期複審",
		);
	});
});

describe("measureStatusDisplay", () => {
	it("申覆的狀態優先於措施本身的狀態", () => {
		const state = MeasureState.ACTIVE;
		expect(
			measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				state,
				requests: [appeal(MeasureRequestState.PENDING)],
			})?.label,
		).toBe("申覆審核中");
		expect(
			measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				state: MeasureState.REVOKED,
				requests: [appeal(MeasureRequestState.ACCEPTED)],
			})?.label,
		).toBe("申覆已通過");
		expect(
			measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				state,
				requests: [appeal(MeasureRequestState.REJECTED)],
			})?.label,
		).toBe("申覆已駁回");
	});

	it("沒有請求（或請求已隨措施結案）時看措施的狀態；UNSPECIFIED 不是有效狀態", () => {
		const label = (state: MeasureState, a?: MeasureRequestState) =>
			measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				state,
				requests: a === undefined ? [] : [appeal(a)],
			})?.label;
		expect(label(MeasureState.ACTIVE)).toBe("停權中");
		expect(label(MeasureState.ISSUED)).toBe("已發出");
		expect(label(MeasureState.ENDED)).toBe("已結束");
		expect(label(MeasureState.REVOKED)).toBe("已撤銷");
		expect(label(MeasureState.REVOKED, MeasureRequestState.CLOSED)).toBe(
			"已撤銷",
		);
		expect(label(MeasureState.UNSPECIFIED)).toBeUndefined();
	});

	it("state 是 UNSPECIFIED 的請求沒有標籤，回到措施本身的狀態", () => {
		expect(
			measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				state: MeasureState.ACTIVE,
				requests: [appeal(MeasureRequestState.UNSPECIFIED)],
			})?.label,
		).toBe("停權中");
	});

	it("不認得的請求種類（含 UNSPECIFIED）顯示目錄 other 的文字，與 seller-app 同一個字；不過濾", () => {
		for (const kind of [
			MeasureRequestKind.UNSPECIFIED,
			99 as MeasureRequestKind,
		]) {
			const m = {
				kind: MeasureKind.SUSPENSION,
				state: MeasureState.ACTIVE,
				requests: [request(kind, MeasureRequestState.PENDING)],
			};
			expect(measureStatusDisplay(ut, m)?.label, String(kind)).toBe(
				"請求審核中",
			);
			const decided = {
				...m,
				requests: [request(kind, MeasureRequestState.REJECTED)],
			};
			expect(measureStatusDisplay(ut, decided)?.label, String(kind)).toBe(
				"請求已駁回",
			);
		}
	});
});

describe("三端共用的文字", () => {
	it("警告（ISSUED）不是「生效中」也不是紅色；停權中才是紅色", () => {
		const issued = measureStatusDisplay(ut, {
			kind: MeasureKind.SUSPENSION,
			requests: [],
			state: MeasureState.ISSUED,
		});
		const active = measureStatusDisplay(ut, {
			kind: MeasureKind.SUSPENSION,
			requests: [],
			state: MeasureState.ACTIVE,
		});
		expect(issued).toEqual({ label: "已發出", color: "warning" });
		expect(active).toEqual({ label: "停權中", color: "danger" });
	});

	it("違規依據的標題只取主要依據（第一筆認定），不串接全部", () => {
		expect(
			measureTitle(
				ut,
				measure({
					basis: MeasureBasis.VIOLATION,
					findings: [
						finding(PolicyCategory.FRAUD, ViolationSeverity.LOW),
						finding(PolicyCategory.COUNTERFEIT, ViolationSeverity.LOW),
					],
				}),
			),
		).toBe("詐欺或操縱交易");
	});
});

describe("revokeNote", () => {
	it("依據的認定被撤銷才說明；接受申覆等其他原因不重複說明", () => {
		expect(
			revokeNote(ut, {
				state: MeasureState.REVOKED,
				revokeReason: RevokeReason.FINDING_REVOKED,
			}),
		).toBe("依據的認定已撤銷");
		expect(
			revokeNote(ut, {
				state: MeasureState.REVOKED,
				revokeReason: RevokeReason.APPEAL_ACCEPTED,
			}),
		).toBeUndefined();
		expect(
			revokeNote(ut, {
				state: MeasureState.ACTIVE,
				revokeReason: RevokeReason.FINDING_REVOKED,
			}),
		).toBeUndefined();
	});
});

describe("措施的連結", () => {
	it("單筆措施頁、申覆頁、解除請求頁都在 /seller/measures 之下並編碼 id", () => {
		expect(measureHref({ id: "m 1/2" })).toBe("/seller/measures/m%201%2F2");
		expect(appealHref({ id: "m 1/2" })).toBe(
			"/seller/measures/m%201%2F2/appeal",
		);
		expect(remediationHref({ id: "m 1/2" })).toBe(
			"/seller/measures/m%201%2F2/remediation",
		);
	});
});

describe("rejectionCooldownWarning", () => {
	const days = (seconds: number, nanos = 0) =>
		rejectionCooldownWarning(ut, {
			rejectionCooldown: create(DurationSchema, {
				seconds: BigInt(seconds),
				nanos,
			}),
		});
	const text = (n: number) => `若本次經駁回，將於 ${n} 日內不得再提出解除請求`;

	it("天數是無條件進位、至少 1：不足整天的 Duration 不會少報", () => {
		expect(days(14 * 86400)).toBe(text(14));
		expect(days(6 * 86400 + 23 * 3600)).toBe(text(7)); // 6 日 23 小時
		expect(days(3600)).toBe(text(1)); // 1 小時
		expect(days(7 * 86400 + 1)).toBe(text(8)); // 7 日又 1 秒
		expect(days(0, 1)).toBe(text(1)); // 只有 1 奈秒
		expect(days(7 * 86400)).toBe(text(7)); // 整天不進位
	});

	it("沒有 rejectionCooldown：不顯示警告", () => {
		expect(rejectionCooldownWarning(ut, { rejectionCooldown: undefined })).toBe(
			undefined,
		);
	});
});

const remediation = (
	state: MeasureRequestState,
	extra: Parameters<typeof create<typeof MeasureRequestSchema>>[1] = {},
) => request(MeasureRequestKind.REMEDIATION, state, extra);

const RU = RemediationUnavailableReason;

const PENDING_SENTENCE = REMEDIATION_SENTENCE.REMEDIATION_PENDING;
const APPEAL_PENDING_SENTENCE = REMEDIATION_SENTENCE.APPEAL_PENDING;
const COOLDOWN_SENTENCE = REMEDIATION_SENTENCE.COOLDOWN;

describe("解除請求入口 remediationEntry", () => {
	it("能不能用只看 canRequestRemediation；可用時不看原因、冷卻時間也不反灰", () => {
		expect(remediationEntry(measure({ canRequestRemediation: true }))).toEqual({
			enabled: true,
		});
		expect(
			remediationEntry(
				measure({
					canRequestRemediation: true,
					nextRemediationTime: at("2026-10-20T02:00:00Z"),
				}),
			),
		).toEqual({ enabled: true });
	});

	it("反灰的原因只取 remediationUnavailabilities 的第一筆句子，不依 reason 或 nextRemediationTime 組字", () => {
		const cooldown = at("2026-10-20T02:05:00Z");
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.REMEDIATION_PENDING, PENDING_SENTENCE),
						unavailable(RU.COOLDOWN, COOLDOWN_SENTENCE),
					],
					nextRemediationTime: cooldown,
				}),
			),
		).toEqual({ enabled: false, reason: PENDING_SENTENCE });
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.APPEAL_PENDING, APPEAL_PENDING_SENTENCE),
						unavailable(RU.COOLDOWN, COOLDOWN_SENTENCE),
					],
					nextRemediationTime: cooldown,
				}),
			)?.reason,
		).toBe(APPEAL_PENDING_SENTENCE);
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.COOLDOWN, COOLDOWN_SENTENCE),
						unavailable(RU.APPEAL_PENDING, APPEAL_PENDING_SENTENCE),
					],
					nextRemediationTime: cooldown,
				}),
			),
		).toEqual({ enabled: false, reason: COOLDOWN_SENTENCE });
	});

	it("原因順序是伺服器給的，client 不重排：第一筆不是待處理就不顯示待處理的句子", () => {
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.COOLDOWN, COOLDOWN_SENTENCE),
						unavailable(RU.REMEDIATION_PENDING, PENDING_SENTENCE),
					],
					nextRemediationTime: at("2026-10-20T02:05:00Z"),
				}),
			)?.reason,
		).toBe(COOLDOWN_SENTENCE);
	});

	it("結構性的原因在前：違規措施或已結束的 RISK 措施上有待處理的申覆，也沒有解除請求入口", () => {
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.NOT_RISK, "只有依據風險類型的措施可以請求解除"),
						unavailable(RU.APPEAL_PENDING, APPEAL_PENDING_SENTENCE),
					],
				}),
			),
		).toBeUndefined();
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.NOT_ACTIVE, "這筆措施目前不在生效中"),
						unavailable(RU.APPEAL_PENDING, APPEAL_PENDING_SENTENCE),
					],
				}),
			),
		).toBeUndefined();
	});

	it("冷卻但伺服器沒給句子：反灰、沒有原因文字（client 不用 nextRemediationTime 自己組）", () => {
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [unavailable(RU.COOLDOWN)],
					nextRemediationTime: at("2026-10-20T02:05:00Z"),
				}),
			),
		).toEqual({ enabled: false, reason: undefined });
	});

	it("第一筆是 NOT_RISK 或 NOT_ACTIVE（伺服器說不是 RISK、不是生效中）：不顯示入口", () => {
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [unavailable(RU.NOT_RISK, "任意一句")],
				}),
			),
		).toBeUndefined();
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [unavailable(RU.NOT_ACTIVE, "任意一句")],
				}),
			),
		).toBeUndefined();
		// 沒有任何原因、也不能請求：沒有可顯示的東西
		expect(remediationEntry(measure({}))).toBeUndefined();
		expect(
			remediationEntry(
				measure({
					remediationUnavailabilities: [
						unavailable(RU.UNSPECIFIED, "任意一句"),
					],
				}),
			),
		).toBeUndefined();
	});

	it("不從措施的依據或狀態推導：伺服器說能請求就有入口，說 NOT_RISK 就沒有，與 basis、state 無關", () => {
		expect(
			remediationEntry(
				measure({
					basis: MeasureBasis.VIOLATION,
					state: MeasureState.ENDED,
					canRequestRemediation: true,
				}),
			),
		).toEqual({ enabled: true });
		expect(
			remediationEntry(
				measure({
					basis: MeasureBasis.RISK,
					state: MeasureState.ACTIVE,
					remediationUnavailabilities: [unavailable(RU.NOT_RISK, "任意一句")],
				}),
			),
		).toBeUndefined();
	});
});

describe("請求進度的顯示優先序", () => {
	const decidedAt = (iso: string) => ({ decideTime: at(iso) });
	const RISK_ACTIVE = {
		basis: MeasureBasis.RISK,
		kind: MeasureKind.SUSPENSION,
		state: MeasureState.ACTIVE,
	};

	it("待處理的請求 > 最新已裁定的結果；冷卻時間不進說明行", () => {
		const rejected = remediation(MeasureRequestState.REJECTED, {
			decisionReason: "照片不清楚",
			...decidedAt("2026-10-10T00:00:00Z"),
		});
		const cooldown = at("2026-10-20T02:00:00Z");
		const pending = measure({
			...RISK_ACTIVE,
			requests: [remediation(MeasureRequestState.PENDING), rejected],
			remediationUnavailabilities: [
				unavailable(RU.REMEDIATION_PENDING, PENDING_SENTENCE),
				unavailable(RU.COOLDOWN, COOLDOWN_SENTENCE),
			],
			nextRemediationTime: cooldown,
		});
		expect(measureStatusDisplay(ut, pending)?.label).toBe("解除請求審核中");
		expect(requestNote(ut, pending)).toBe("已提交解除請求，請等候平台複核。");
		const cooling = measure({
			...RISK_ACTIVE,
			requests: [rejected],
			remediationUnavailabilities: [
				unavailable(RU.COOLDOWN, COOLDOWN_SENTENCE),
			],
			nextRemediationTime: cooldown,
		});
		const note = requestNote(ut, cooling);
		expect(note).toBe("平台回覆：照片不清楚");
		expect(note).not.toContain("後可再提出");
		expect(measureStatusDisplay(ut, cooling)?.label).toBe("解除請求已駁回");
		// 冷卻時間只在按鈕的反灰原因
		expect(remediationEntry(cooling)?.reason).toBe(COOLDOWN_SENTENCE);
	});

	it("待處理的申覆：標籤與說明行是申覆的字", () => {
		const m = measure({
			...RISK_ACTIVE,
			requests: [appeal(MeasureRequestState.PENDING)],
		});
		expect(measureStatusDisplay(ut, m)?.label).toBe("申覆審核中");
		expect(requestNote(ut, m)).toBe("已提交申覆，請等候平台複核。");
	});

	it("待處理的申覆：狀態標籤與申覆標籤並列（狀態在前）；沒有待處理請求時只有一個標籤", () => {
		const pending = measure({
			...RISK_ACTIVE,
			requests: [appeal(MeasureRequestState.PENDING)],
		});
		expect(measureStatusChips(ut, pending).map((c) => c.label)).toEqual([
			"停權中",
			"申覆審核中",
		]);
		const decided = measure({
			...RISK_ACTIVE,
			requests: [appeal(MeasureRequestState.REJECTED, "不成立")],
		});
		expect(measureStatusChips(ut, decided).map((c) => c.label)).toEqual([
			"申覆已駁回",
		]);
	});

	it("最新已裁定的請求是 requests 裡第一筆 ACCEPTED／REJECTED（伺服器新到舊），不分種類，不重新比時間", () => {
		const m = measure({
			...RISK_ACTIVE,
			requests: [
				appeal(MeasureRequestState.CLOSED),
				appeal(MeasureRequestState.REJECTED, "申覆不成立"),
				remediation(MeasureRequestState.REJECTED, {
					decisionReason: "照片不清楚",
					// 時間比第一筆新也不影響：順序以伺服器為準
					...decidedAt("2030-01-01T00:00:00Z"),
				}),
			],
		});
		expect(measureStatusDisplay(ut, m)?.label).toBe("申覆已駁回");
		expect(requestNote(ut, m)).toBe("平台回覆：申覆不成立");
	});

	it("接受解除請求後措施已結束：標籤是解除請求已通過，說明行是裁定理由", () => {
		const m = measure({
			basis: MeasureBasis.RISK,
			state: MeasureState.ENDED,
			requests: [
				remediation(MeasureRequestState.ACCEPTED, {
					decisionReason: "已改善",
					...decidedAt("2026-10-12T00:00:00Z"),
				}),
			],
		});
		expect(measureStatusDisplay(ut, m)?.label).toBe("解除請求已通過");
		expect(requestNote(ut, m)).toBe("平台回覆：已改善");
	});

	it("已裁定但沒有裁定理由：有標籤、沒有說明行", () => {
		const m = measure({
			requests: [remediation(MeasureRequestState.REJECTED)],
		});
		expect(measureStatusDisplay(ut, m)?.label).toBe("解除請求已駁回");
		expect(requestNote(ut, m)).toBeUndefined();
	});

	it("標籤與說明只讀請求的狀態，不拿措施的狀態比對（同一組請求在任何措施狀態下結果相同）", () => {
		const requests = [
			remediation(MeasureRequestState.REJECTED, { decisionReason: "不足" }),
		];
		for (const state of [
			MeasureState.ISSUED,
			MeasureState.ACTIVE,
			MeasureState.ENDED,
			MeasureState.REVOKED,
		]) {
			const m = measure({ state, requests });
			expect(measureStatusDisplay(ut, m)?.label).toBe("解除請求已駁回");
			expect(requestNote(ut, m)).toBe("平台回覆：不足");
		}
	});

	it("CLOSED 的請求不顯示標籤也不顯示說明（措施狀態已表達撤銷或結束）", () => {
		const closed = (cause: MeasureRequestCloseCause, state: MeasureState) =>
			measure({
				...RISK_ACTIVE,
				state,
				requests: [
					remediation(MeasureRequestState.CLOSED, {
						closeCause: cause,
						...decidedAt("2026-10-12T00:00:00Z"),
					}),
				],
			});
		const ended = closed(MeasureRequestCloseCause.ENDED, MeasureState.ENDED);
		expect(requestNote(ut, ended)).toBeUndefined();
		expect(measureStatusDisplay(ut, ended)?.label).toBe("已結束");
		const revoked = closed(
			MeasureRequestCloseCause.REVOKED,
			MeasureState.REVOKED,
		);
		expect(requestNote(ut, revoked)).toBeUndefined();
		expect(measureStatusDisplay(ut, revoked)?.label).toBe("已撤銷");
	});
});

/** proto 列舉的全部有效值（數字、不含 UNSPECIFIED = 0）。新增列舉值而目錄沒有文字時，下面的完整性測試會紅。 */
function validValues(e: Record<string, string | number>): number[] {
	return Object.values(e).filter(
		(v): v is number => typeof v === "number" && v !== 0,
	);
}

describe("目錄的完整性（列舉每個有效值都有文字，UNSPECIFIED 沒有）", () => {
	it("PolicyCategory：標題每個值都有不同的文字", () => {
		const titles = validValues(PolicyCategory).map((category) =>
			measureTitle(
				ut,
				measure({
					basis: MeasureBasis.VIOLATION,
					findings: [finding(category, ViolationSeverity.LOW)],
				}),
			),
		);
		expect(new Set(titles).size).toBe(titles.length);
		expect(titles).not.toContain("措施紀錄");
	});

	it("ViolationSeverity 與 MeasureKind：meta 每個值都有文字", () => {
		for (const severity of validValues(ViolationSeverity)) {
			expect(
				measureMeta(
					ut,
					measure({
						kind: MeasureKind.WARNING,
						findings: [finding(PolicyCategory.FRAUD, severity)],
					}),
				),
				String(severity),
			).toMatch(/^嚴重程度：.+ · 措施：警告$/);
		}
		for (const kind of validValues(MeasureKind)) {
			expect(measureMeta(ut, measure({ kind })), String(kind)).toMatch(
				/^措施：.+$/,
			);
		}
	});

	it("TerminationGround：終止合作的 meta 每個值都有文字，UNSPECIFIED 不顯示事由", () => {
		for (const ground of validValues(TerminationGround)) {
			expect(
				measureMeta(
					ut,
					measure({
						kind: MeasureKind.TERMINATION,
						terminationGround: ground,
					}),
				),
				String(ground),
			).toMatch(/^措施：終止合作 · 事由：.+$/);
		}
		expect(
			measureMeta(
				ut,
				measure({
					kind: MeasureKind.TERMINATION,
					terminationGround: TerminationGround.UNSPECIFIED,
				}),
			),
		).toBe("措施：終止合作");
	});

	it("MeasureState：每個有效值都有標籤與顏色，UNSPECIFIED 沒有", () => {
		for (const state of validValues(MeasureState)) {
			const display = measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				requests: [],
				state,
			});
			expect(display?.label, String(state)).toBeTruthy();
			expect(display?.color, String(state)).toBeTruthy();
		}
		expect(
			measureStatusDisplay(ut, {
				kind: MeasureKind.SUSPENSION,
				requests: [],
				state: MeasureState.UNSPECIFIED,
			}),
		).toBeUndefined();
	});

	it("MeasureBasis：CONTRACT 的標題是契約條款；撤銷只說明認定與關聯終止被撤銷的原因", () => {
		expect(measureTitle(ut, measure({ basis: MeasureBasis.CONTRACT }))).toBe(
			"契約條款",
		);
		const note = (revokeReason: RevokeReason) =>
			revokeNote(ut, { state: MeasureState.REVOKED, revokeReason });
		expect(note(RevokeReason.FINDING_REVOKED)).toBeTruthy();
		expect(note(RevokeReason.SOURCE_REVOKED)).toBeTruthy();
		expect(note(RevokeReason.UNSPECIFIED)).toBeUndefined();
		expect(note(RevokeReason.APPEAL_ACCEPTED)).toBeUndefined();
		expect(note(RevokeReason.ADMIN_CORRECTION)).toBeUndefined();
	});

	it("終止合作的文字與目錄相同", () => {
		expect(
			measureStatusDisplay(ut, {
				kind: MeasureKind.TERMINATION,
				requests: [],
				state: MeasureState.SCHEDULED,
			}),
		).toEqual({ label: "預定生效", color: "warning" });
		expect(
			measureMeta(
				ut,
				measure({
					kind: MeasureKind.TERMINATION,
					terminationGround: TerminationGround.NOTICE,
				}),
			),
		).toBe("措施：終止合作 · 事由：預告終止（第 21.2 條）");
		expect(
			revokeNote(ut, {
				state: MeasureState.REVOKED,
				revokeReason: RevokeReason.SOURCE_REVOKED,
			}),
		).toBe("關聯的終止已撤銷");
	});
});

describe("終止合作的呈現", () => {
	const termination = (init: Parameters<typeof measure>[0]) =>
		measure({
			kind: MeasureKind.TERMINATION,
			basis: MeasureBasis.CONTRACT,
			terminationGround: TerminationGround.NOTICE,
			...init,
		});

	it("標題是契約條款，措施與事由在 meta", () => {
		const m = termination({});
		expect(measureTitle(ut, m)).toBe("契約條款");
		expect(measureMeta(ut, m)).toBe(
			"措施：終止合作 · 事由：預告終止（第 21.2 條）",
		);
		// 事由只在終止合作顯示
		expect(
			measureMeta(
				ut,
				measure({
					kind: MeasureKind.SUSPENSION,
					terminationGround: TerminationGround.FRAUD,
				}),
			),
		).toBe("措施：停權");
	});

	it("SCHEDULED 有「預定生效」chip；終止生效後（ACTIVE）是「合作已終止」，不是「停權中」", () => {
		expect(
			measureStatusDisplay(ut, termination({ state: MeasureState.SCHEDULED })),
		).toEqual({ label: "預定生效", color: "warning" });
		expect(
			measureStatusDisplay(ut, termination({ state: MeasureState.ACTIVE }))
				?.label,
		).toBe("合作已終止");
		expect(
			measureStatusDisplay(
				ut,
				measure({ kind: MeasureKind.SUSPENSION, state: MeasureState.ACTIVE }),
			)?.label,
		).toBe("停權中");
	});

	it("期間：終止生效；預告終止已結束再加「合作已於 … 結束」", () => {
		expect(
			measurePeriodText(
				ut,
				termination({ period: { startTime: at("2026-10-01T02:00:00Z") } }),
			),
		).toBe("終止生效：2026/10/01 10:00");
		expect(
			measurePeriodText(
				ut,
				termination({
					period: {
						startTime: at("2026-10-01T02:00:00Z"),
						endTime: at("2026-09-20T02:00:00Z"),
					},
				}),
			),
		).toBe("終止生效：2026/10/01 10:00，合作已於 2026/09/20 10:00 結束");
	});

	it("預告橫幅文字：只有預定生效的終止合作，生效時間讀 period.startTime", () => {
		const base = {
			period: { startTime: at("2026-10-01T02:00:00Z") },
		};
		expect(
			terminationNoticeText(
				ut,
				termination({ ...base, state: MeasureState.SCHEDULED }),
			),
		).toBe(
			"平台已通知將於 2026/10/01 10:00 終止合作。在此之前店家照常營業；如有異議，可提出申覆。",
		);
		expect(
			terminationNoticeText(
				ut,
				termination({ ...base, state: MeasureState.ACTIVE }),
			),
		).toBeUndefined();
		expect(
			terminationNoticeText(
				ut,
				measure({
					...base,
					kind: MeasureKind.SUSPENSION,
					state: MeasureState.SCHEDULED,
				}),
			),
		).toBeUndefined();
		expect(
			terminationNoticeText(ut, termination({ state: MeasureState.SCHEDULED })),
		).toBeUndefined();
	});
});

describe("appealEntry", () => {
	it("能不能用只讀 canAppeal；不能用時原因取伺服器第一項；沒有原因不編造", () => {
		expect(appealEntry(ut, measure({ canAppeal: true }))).toEqual({
			enabled: true,
			deadline: undefined,
		});
		expect(
			appealEntry(
				ut,
				measure({
					canAppeal: false,
					appealUnavailabilities: [
						unavailable(
							AppealUnavailableReason.REQUEST_PENDING,
							APPEAL_SENTENCE.REQUEST_PENDING,
						),
						unavailable(
							AppealUnavailableReason.REVOKED,
							APPEAL_SENTENCE.REVOKED,
						),
					],
				}),
			),
		).toEqual({
			enabled: false,
			reason: APPEAL_SENTENCE.REQUEST_PENDING,
			deadline: undefined,
		});
		expect(
			appealEntry(ut, measure({ canAppeal: false })).reason,
		).toBeUndefined();
		// 即使 state 是 REVOKED，伺服器沒給原因就不顯示原因
		expect(
			appealEntry(
				ut,
				measure({ canAppeal: false, state: MeasureState.REVOKED }),
			).reason,
		).toBeUndefined();
	});

	it("申覆期限只有終止合作顯示，能用與不能用都顯示", () => {
		const deadline = at("2026-10-01T02:00:00Z");
		expect(
			appealEntry(
				ut,
				measure({
					kind: MeasureKind.TERMINATION,
					canAppeal: true,
					appealDeadlineTime: deadline,
				}),
			).deadline,
		).toBe("申覆期限：2026/10/01 10:00");
		expect(
			appealEntry(
				ut,
				measure({
					kind: MeasureKind.SUSPENSION,
					canAppeal: true,
					appealDeadlineTime: deadline,
				}),
			).deadline,
		).toBeUndefined();
		expect(
			appealEntry(
				ut,
				measure({ kind: MeasureKind.TERMINATION, canAppeal: true }),
			).deadline,
		).toBeUndefined();
	});
});
