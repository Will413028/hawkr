import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { BadRequestSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	AppealUnavailableReason,
	MeasureRequestKind,
	MeasureSchema,
	MeasureState,
	RemediationUnavailableReason,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import { defaultErrorTextFor } from "@/lib/i18n/ui-text";
import {
	appealBlockedReason,
	appealClosedNotice,
	appealFieldErrors,
	fetchHasMeasures,
	fetchMyMeasure,
	fetchMyMeasures,
	remediationBlockedReason,
	remediationClosedNotice,
	settleAppealFailure,
	settleRemediationFailure,
	submitMeasureAppeal,
	submitMeasureRemediation,
} from "./measures";

const defaults = defaultErrorTextFor("zh-TW");
const measure = (id: string) => create(MeasureSchema, { id });

describe("fetchMyMeasures", () => {
	it("依 next_page_token 讀完所有分頁，依序合併", async () => {
		const listMyMeasures = vi
			.fn()
			.mockResolvedValueOnce({
				measures: [measure("1"), measure("2")],
				nextPageToken: "t2",
			})
			.mockResolvedValueOnce({
				measures: [measure("3")],
				nextPageToken: "t3",
			})
			.mockResolvedValueOnce({
				measures: [measure("4")],
				nextPageToken: "",
			});
		const result = await fetchMyMeasures(
			MeasureState.UNSPECIFIED,
			() => ({ listMyMeasures }) as never,
		);
		expect(result.map((m) => m.id)).toEqual(["1", "2", "3", "4"]);
		expect(listMyMeasures.mock.calls.map(([req]) => req.pageToken)).toEqual([
			"",
			"t2",
			"t3",
		]);
		for (const [req] of listMyMeasures.mock.calls) {
			expect(req.pageSize).toBe(100);
		}
	});

	it("每一頁都帶同一個 state 篩選（公告頁只讀 ACTIVE）", async () => {
		const listMyMeasures = vi
			.fn()
			.mockResolvedValueOnce({ measures: [], nextPageToken: "t2" })
			.mockResolvedValueOnce({ measures: [], nextPageToken: "" });
		await fetchMyMeasures(
			MeasureState.ACTIVE,
			() => ({ listMyMeasures }) as never,
		);
		expect(listMyMeasures.mock.calls.map(([req]) => req.state)).toEqual([
			MeasureState.ACTIVE,
			MeasureState.ACTIVE,
		]);
	});

	it("token 沒有前進時停下，不無限迴圈", async () => {
		const listMyMeasures = vi
			.fn()
			.mockResolvedValue({ measures: [], nextPageToken: "same" });
		await expect(
			fetchMyMeasures(
				MeasureState.UNSPECIFIED,
				() => ({ listMyMeasures }) as never,
			),
		).rejects.toThrow();
		expect(listMyMeasures).toHaveBeenCalledTimes(2);
	});
});

describe("fetchMyMeasure", () => {
	it("以 measure_id 讀單筆；回應沒有措施是契約違反", async () => {
		const getMyMeasure = vi.fn(async () => ({ measure: measure("m1") }));
		expect(
			(await fetchMyMeasure("m1", () => ({ getMyMeasure }) as never)).id,
		).toBe("m1");
		expect(getMyMeasure).toHaveBeenCalledWith({ measureId: "m1" });
		await expect(
			fetchMyMeasure("m1", () => ({ getMyMeasure: async () => ({}) }) as never),
		).rejects.toThrow("GetMyMeasure");
	});
});

describe("submitMeasureAppeal", () => {
	it("帶措施 id、理由、機敏 staged token 與讀到的 etag", async () => {
		const updated = create(MeasureSchema, {
			id: "m1",
			state: MeasureState.ACTIVE,
		});
		const submitAppeal = vi.fn(async () => ({ measure: updated }));
		const result = await submitMeasureAppeal(
			{ id: "m1", etag: "e7" },
			{ reason: "說明", evidenceStagedTokens: ["t1", "t2"] },
			() => ({ submitAppeal }) as never,
		);
		expect(result).toBe(updated);
		expect(submitAppeal).toHaveBeenCalledWith({
			measureId: "m1",
			reason: "說明",
			evidenceStagedTokens: ["t1", "t2"],
			etag: "e7",
		});
	});

	it("回應沒有措施是契約違反", async () => {
		await expect(
			submitMeasureAppeal(
				{ id: "m1", etag: "e" },
				{ reason: "r", evidenceStagedTokens: [] },
				() => ({ submitAppeal: async () => ({}) }) as never,
			),
		).rejects.toThrow("SubmitAppeal");
	});
});

describe("query keys", () => {
	it("申覆後 invalidate 的 family 同時命中各個 state 的列表與單筆", async () => {
		const { queryKeys } = await import("@/lib/query/keys");
		const id = { accountId: "a1", activeRole: "seller" } as never;
		const family = queryKeys.seller.measures(id);
		const startsWithFamily = (key: readonly unknown[]) =>
			family.every((part, i) => key[i] === part);
		expect(
			startsWithFamily(queryKeys.seller.measureList(id, { state: 0 })),
		).toBe(true);
		expect(
			startsWithFamily(queryKeys.seller.measureList(id, { state: 2 })),
		).toBe(true);
		expect(startsWithFamily(queryKeys.seller.measure(id, "m1"))).toBe(true);
		expect(
			startsWithFamily(
				queryKeys.seller.measureList(
					{ accountId: "a2", activeRole: "seller" } as never,
					{ state: 0 },
				),
			),
		).toBe(false);
	});
});

const sentence = (reason: number, message?: string) => ({
	reason,
	message: message === undefined ? undefined : { locale: "zh-TW", message },
});

describe("appealClosedNotice", () => {
	const AU = AppealUnavailableReason;
	const noticeOf = (
		unavailabilities: ReturnType<typeof sentence>[],
		extra: Parameters<typeof create<typeof MeasureSchema>>[1] = {},
	) =>
		appealClosedNotice(
			create(MeasureSchema, {
				appealUnavailabilities: unavailabilities as never,
				...extra,
			}),
		);

	it("只讀伺服器原因的第一筆句子，後面的原因與 reason 本身都不影響；不看 requests 或 state", () => {
		expect(
			noticeOf([
				sentence(
					AU.ALREADY_APPEALED,
					"這筆措施已提出過申覆，每筆措施只能申覆一次",
				),
				sentence(AU.REVOKED, "這筆措施已撤銷，不需要申覆"),
			]),
		).toBe("這筆措施已提出過申覆，每筆措施只能申覆一次");
		expect(
			noticeOf([sentence(AU.APPEAL_WINDOW_ENDED, "伺服器的任意一句")]),
		).toBe("伺服器的任意一句");
		// 有申覆請求、已撤銷，但伺服器沒給原因：不自己推導，用通用說明
		expect(
			noticeOf([], {
				requests: [{ kind: MeasureRequestKind.APPEAL }],
				state: MeasureState.REVOKED,
			}),
		).toBe("這項措施目前不能提出申覆");
	});

	it("沒有原因、沒有句子或句子是空字串：通用說明，不依 reason 自己組字", () => {
		expect(noticeOf([])).toBe("這項措施目前不能提出申覆");
		expect(noticeOf([sentence(AU.REVOKED)])).toBe("這項措施目前不能提出申覆");
		expect(noticeOf([sentence(AU.UNSPECIFIED, "")])).toBe(
			"這項措施目前不能提出申覆",
		);
	});
});

describe("appealBlockedReason", () => {
	it("第一筆句子；沒有時 undefined", () => {
		expect(
			appealBlockedReason(
				create(MeasureSchema, {
					appealUnavailabilities: [
						sentence(
							AppealUnavailableReason.REVOKED,
							"這筆措施已撤銷，不需要申覆",
						),
					] as never,
				}),
			),
		).toBe("這筆措施已撤銷，不需要申覆");
		expect(appealBlockedReason(create(MeasureSchema))).toBeUndefined();
	});
});

describe("remediationBlockedReason", () => {
	const RU = RemediationUnavailableReason;
	const reasonOf = (unavailabilities: ReturnType<typeof sentence>[]) =>
		remediationBlockedReason(
			create(MeasureSchema, {
				remediationUnavailabilities: unavailabilities as never,
				nextRemediationTime: { seconds: BigInt(1792461900), nanos: 0 },
			}),
		);

	it("只取第一筆的句子：冷卻的時間由伺服器組進句子，client 不再用 nextRemediationTime 組字", () => {
		expect(
			reasonOf([
				sentence(
					RU.COOLDOWN,
					"解除請求目前在冷卻期內，2026/10/20 10:25 後可再提出",
				),
				sentence(RU.APPEAL_PENDING, "另一句"),
			]),
		).toBe("解除請求目前在冷卻期內，2026/10/20 10:25 後可再提出");
		expect(
			reasonOf([
				sentence(
					RU.REMEDIATION_PENDING,
					"這筆措施的解除請求審核中，審核完成前不能再提出",
				),
				sentence(RU.COOLDOWN, "另一句"),
			]),
		).toBe("這筆措施的解除請求審核中，審核完成前不能再提出");
	});

	it("沒有句子（沒有 message、空字串、空陣列）沒有原因文字；關閉說明退回通用文字", () => {
		for (const unavailabilities of [
			[sentence(RU.NOT_RISK)],
			[sentence(RU.UNSPECIFIED, "")],
			[],
		]) {
			expect(reasonOf(unavailabilities)).toBeUndefined();
		}
		expect(
			remediationClosedNotice(
				create(MeasureSchema, {
					remediationUnavailabilities: [sentence(RU.NOT_ACTIVE)] as never,
				}),
			),
		).toBe("這項措施目前不能請求解除");
		expect(
			remediationClosedNotice(
				create(MeasureSchema, {
					remediationUnavailabilities: [
						sentence(RU.NOT_RISK, "只有依據風險類型的措施可以請求解除"),
					] as never,
				}),
			),
		).toBe("只有依據風險類型的措施可以請求解除");
	});
});

describe("settleAppealFailure", () => {
	it("非衝突、非 failed_precondition 的失敗不重讀", async () => {
		const reload = vi.fn();
		const out = await settleAppealFailure(
			new Error("x"),
			defaults,
			"m1",
			reload,
		);
		expect(out.kind).toBe("message");
		expect(reload).not.toHaveBeenCalled();
	});
});

describe("submitMeasureRemediation", () => {
	it("呼叫 requestRemediation（不是 submitAppeal），帶措施 id、理由、token 與 etag", async () => {
		const requestRemediation = vi
			.fn()
			.mockResolvedValue({ measure: measure("m1") });
		const submitAppeal = vi.fn();
		const result = await submitMeasureRemediation(
			{ id: "m1", etag: "e1" },
			{ reason: "已改善", evidenceStagedTokens: ["t1"] },
			() => ({ requestRemediation, submitAppeal }) as never,
		);
		expect(result.id).toBe("m1");
		expect(submitAppeal).not.toHaveBeenCalled();
		expect(requestRemediation).toHaveBeenCalledWith({
			measureId: "m1",
			reason: "已改善",
			evidenceStagedTokens: ["t1"],
			etag: "e1",
		});
	});

	it("回應沒有措施：丟錯", async () => {
		await expect(
			submitMeasureRemediation(
				{ id: "m1", etag: "e1" },
				{ reason: "x", evidenceStagedTokens: [] },
				() => ({ requestRemediation: vi.fn().mockResolvedValue({}) }) as never,
			),
		).rejects.toThrow("RequestRemediation");
	});
});

describe("fetchHasMeasures", () => {
	it("只讀第一筆", async () => {
		const listMyMeasures = vi
			.fn()
			.mockResolvedValue({ measures: [measure("1")], nextPageToken: "x" });
		expect(await fetchHasMeasures(() => ({ listMyMeasures }) as never)).toBe(
			true,
		);
		expect(listMyMeasures).toHaveBeenCalledWith({ pageSize: 1 });
	});

	it("沒有措施是 false", async () => {
		const listMyMeasures = vi.fn().mockResolvedValue({ measures: [] });
		expect(await fetchHasMeasures(() => ({ listMyMeasures }) as never)).toBe(
			false,
		);
	});
});

describe("appealFieldErrors", () => {
	it("以精確的 key 對到理由、整個佐證欄位與單張", () => {
		expect(
			appealFieldErrors({
				reason: "理由有問題",
				evidence_staged_tokens: "最多 5 張",
				"evidence_staged_tokens[2]": "已過期",
			}),
		).toEqual({
			reason: "理由有問題",
			evidence: "最多 5 張",
			evidenceByIndex: { 2: "已過期" },
		});
	});

	it("有索引的路徑不算整個欄位的錯誤", () => {
		const result = appealFieldErrors({ "evidence_staged_tokens[0]": "已過期" });
		expect(result.evidence).toBeUndefined();
		expect(result.evidenceByIndex).toEqual({ 0: "已過期" });
	});

	it("對不上任何欄位時退回整體說明，沒有就用第一個欄位的文字", () => {
		expect(appealFieldErrors({ etag: "格式錯誤" }, "資料有誤").other).toBe(
			"資料有誤",
		);
		expect(appealFieldErrors({ etag: "格式錯誤" }).other).toBe("格式錯誤");
	});
});

describe("settle*Failure 的欄位錯誤", () => {
	const invalid = () =>
		new ConnectError("invalid", Code.InvalidArgument, undefined, [
			{
				desc: BadRequestSchema,
				value: {
					fieldViolations: [
						{
							field: "reason",
							description: "invalid",
							localizedMessage: { locale: "zh-TW", message: "理由不可空白" },
						},
					],
				},
			},
		]);

	it.each([
		["申覆", settleAppealFailure],
		["解除請求", settleRemediationFailure],
	])("%s：invalid_argument 回欄位錯誤、不重讀", async (_name, settle) => {
		const reload = vi.fn();
		const out = await settle(invalid(), defaults, "m1", reload);
		expect(out).toEqual({
			kind: "fields",
			fields: { reason: "理由不可空白", evidenceByIndex: {} },
		});
		expect(reload).not.toHaveBeenCalled();
	});
});
