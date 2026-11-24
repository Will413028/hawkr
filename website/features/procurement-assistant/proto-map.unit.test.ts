import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { StoreCardSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import {
	BlockKind,
	CandidateSchema,
	ClarifyQuestionSchema,
	GetCandidateSearchResponseSchema,
	GetSnapshotResponseSchema,
	InterruptKind,
	ItemResolutionKind,
	OpenConfirmationSchema,
	QuestionOptionSchema,
	RejectionReason,
	RequestStatus,
	RunFailureReason,
	RunStatus,
	SlotKey,
	SlotsSchema,
	TurnResultSchema,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { describe, expect, it } from "vitest";
import {
	candidateFromProto,
	candidateSearchFromProto,
	slotKeyCode,
	slotKeyFromCode,
	slotsFromProto,
	snapshotFromProto,
	turnResultFromProto,
} from "./proto-map";

describe("slot key", () => {
	it("契約的 SlotKey 與畫面代碼雙向對得上", () => {
		for (const key of [
			SlotKey.ITEM,
			SlotKey.QUANTITY,
			SlotKey.UNIT,
			SlotKey.PACKAGING,
			SlotKey.BUDGET_PER_UNIT,
			SlotKey.DELIVERY_CITY,
			SlotKey.DELIVERY_DATE,
			SlotKey.EXACT_MATCH_REQUIRED,
			SlotKey.ITEM_DISAMBIGUATION,
		]) {
			expect(slotKeyFromCode(slotKeyCode(key))).toBe(key);
		}
		expect(slotKeyFromCode("nope")).toBe(SlotKey.UNSPECIFIED);
	});
});

describe("slotsFromProto", () => {
	it("日期、金額與品項判定轉成畫面用的型別", () => {
		const slots = slotsFromProto(
			create(SlotsSchema, {
				item: "高麗菜",
				quantity: 10,
				budgetPerUnit: 800n,
				deliveryDate: { year: 2026, month: 8, day: 20 },
				itemResolution: {
					kind: ItemResolutionKind.CATEGORY_CONFIRMED,
					label: "甘藍",
					displayValue: "甘藍（分類）",
				},
			}),
		);
		expect(slots).toMatchObject({
			item: "高麗菜",
			quantity: 10,
			budgetPerUnit: 800,
			deliveryDate: "2026-08-20",
			itemResolution: {
				kind: "CATEGORY_CONFIRMED",
				displayValue: "甘藍（分類）",
			},
		});
	});

	it("沒有顯示字串的品項判定整個丟掉", () => {
		const slots = slotsFromProto(
			create(SlotsSchema, {
				itemResolution: {
					kind: ItemResolutionKind.BUYER_DESCRIBED,
					label: "x",
				},
			}),
		);
		expect(slots.itemResolution).toBeUndefined();
	});
});

describe("turnResultFromProto", () => {
	it("中斷、問題、區塊與被拒欄位都轉成畫面代碼", () => {
		const view = turnResultFromProto(
			create(TurnResultSchema, {
				requestId: "req-1",
				revisionId: "rev-1",
				conversationId: "conv-1",
				reply: "好",
				blocks: [
					{ kind: BlockKind.WARNING, text: "注意" },
					{ kind: BlockKind.UNSPECIFIED, text: "丟掉" },
				],
				rejected: [
					{ key: SlotKey.QUANTITY, reason: RejectionReason.QUANTITY_RANGE },
				],
				missingRequired: [SlotKey.UNIT],
				question: create(ClarifyQuestionSchema, {
					slotKey: SlotKey.ITEM_DISAMBIGUATION,
					prompt: "哪一種",
					options: [
						create(QuestionOptionSchema, { optionId: "a", label: "A" }),
					],
					canSkip: true,
				}),
				interrupt: { interruptId: "int-1", kind: InterruptKind.CLARIFICATION },
			}),
		);
		expect(view.blocks).toEqual([{ kind: "WARNING", text: "注意" }]);
		expect(view.rejected).toEqual([
			{ key: "quantity", reason: RejectionReason.QUANTITY_RANGE },
		]);
		expect(view.missingRequired).toEqual(["unit"]);
		expect(view.question).toMatchObject({
			slotKey: "crop_disambiguation",
			canSkip: true,
			unitOptions: undefined,
		});
		expect(view.interrupt).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
		});
	});

	it("沒有中斷與問題時兩者都是 null", () => {
		const view = turnResultFromProto(
			create(TurnResultSchema, { requestId: "r", revisionId: "v" }),
		);
		expect(view.interrupt).toBeNull();
		expect(view.question).toBeNull();
	});
});

describe("snapshotFromProto", () => {
	it("openConfirmation 帶問題與處理中旗標", () => {
		const view = snapshotFromProto(
			create(GetSnapshotResponseSchema, {
				requestId: "req-1",
				revisionId: "rev-1",
				status: RequestStatus.MATCHING,
				openConfirmation: create(OpenConfirmationSchema, {
					interruptId: "int-1",
					kind: InterruptKind.CLARIFICATION,
					processing: true,
				}),
			}),
		);
		expect(view.status).toBe("MATCHING");
		expect(view.openConfirmation).toEqual({
			interruptId: "int-1",
			reason: "CLARIFICATION",
			question: null,
			processing: true,
		});
		expect(view.publishedPublication).toBeNull();
	});
});

describe("candidates", () => {
	const store = create(StoreCardSchema, {
		id: "store-1",
		storeCode: "ab12",
		name: "阿水果行",
	});

	it("店家卡片決定名稱與分組鍵；沒有名稱時用呼叫端給的關閉店家文字", () => {
		const named = candidateFromProto(
			create(CandidateSchema, {
				id: "c1",
				store,
				selected: true,
				factsTime: timestampFromDate(new Date("2026-08-20T00:00:00Z")),
			}),
			"已關閉的店家",
		);
		expect(named).toMatchObject({
			candidateId: "c1",
			displayName: "阿水果行",
			storeId: "store-1",
			storeCode: "ab12",
			lane: "PLATFORM",
			selected: true,
			factsAsOf: "2026-08-20T00:00:00.000Z",
		});
		const closed = candidateFromProto(
			create(CandidateSchema, {
				id: "c2",
				store: create(StoreCardSchema, { id: "store-2" }),
			}),
			"已關閉的店家",
		);
		expect(closed.displayName).toBe("已關閉的店家");
	});

	it("搜尋結果的失敗原因是列舉名稱，成功時是空字串", () => {
		const failed = candidateSearchFromProto(
			create(GetCandidateSearchResponseSchema, {
				requestId: "r",
				runId: "run",
				status: RunStatus.FAILED,
				failureReason: RunFailureReason.WORKER_LOST,
			}),
			"x",
		);
		expect(failed).toMatchObject({
			status: "FAILED",
			failureReason: "WORKER_LOST",
		});
		const ok = candidateSearchFromProto(
			create(GetCandidateSearchResponseSchema, {
				status: RunStatus.SUCCEEDED,
				candidates: [create(CandidateSchema, { id: "c1", store })],
			}),
			"x",
		);
		expect(ok.failureReason).toBe("");
		expect(ok.candidates.platform).toHaveLength(1);
		expect(ok.candidates.external).toEqual([]);
	});
});
