import { describe, expect, it } from "vitest";
import type { ProcurementCandidate } from "@/lib/api/openapi-types";
import {
	candidatesToToggleForSeller,
	canGroupBySeller,
	countSelectedRecipients,
	groupPlatformCandidates,
	sellerSelectionState,
} from "./candidate-grouping";

function candidate(
	overrides: Partial<ProcurementCandidate> & { candidateId: string },
): ProcurementCandidate {
	return {
		sourceId: `prod-${overrides.candidateId}`,
		lane: "PLATFORM",
		displayName: `候選 ${overrides.candidateId}`,
		score: 90,
		matchedFacts: [],
		unverifiableFacts: [],
		factsAsOf: "2026-08-22T02:00:00Z",
		selected: false,
		...overrides,
	};
}

describe("canGroupBySeller", () => {
	// 這是整個「不做假分頁」約定的前端執行點：後端還沒部署 #2377 時所有候選都沒有
	// storeId，分組必須自動退場，而不是渲染一堆「商家 undefined」的空名片。
	it("後端尚未提供 storeId 時不得分組", () => {
		expect(
			canGroupBySeller([
				candidate({ candidateId: "1" }),
				candidate({ candidateId: "2" }),
			]),
		).toBe(false);
	});

	it("任一筆帶得出 storeId 就分組（部署窗口內新舊資料混雜）", () => {
		expect(
			canGroupBySeller([
				candidate({ candidateId: "1" }),
				candidate({ candidateId: "2", storeId: "1001" }),
			]),
		).toBe(true);
	});

	it("空清單不分組", () => {
		expect(canGroupBySeller([])).toBe(false);
	});
});

describe("groupPlatformCandidates", () => {
	// 後端 #2377 驗收條件 2 的前端對應面：同店的多個商品必須落在同一組，
	// 否則使用者會看到同一家商家出現兩次。
	it("同一家商家的多個商品合成一組", () => {
		const { groups, ungrouped } = groupPlatformCandidates([
			candidate({ candidateId: "1", storeId: "1001", sellerName: "青果行 A" }),
			candidate({ candidateId: "2", storeId: "1001", sellerName: "青果行 A" }),
			candidate({ candidateId: "3", storeId: "2002", sellerName: "青果行 B" }),
		]);

		expect(groups).toHaveLength(2);
		expect(groups[0].storeId).toBe("1001");
		expect(groups[0].candidates.map((c) => c.candidateId)).toEqual(["1", "2"]);
		expect(groups[1].storeId).toBe("2002");
		expect(groups[1].candidates.map((c) => c.candidateId)).toEqual(["3"]);
		expect(ungrouped).toEqual([]);
	});

	// 群組順序＝首次出現順序。後端已依 rank 排好候選，重排會把最相關的推到下面。
	it("群組依候選首次出現順序排列，不重排", () => {
		const { groups } = groupPlatformCandidates([
			candidate({ candidateId: "1", storeId: "2002" }),
			candidate({ candidateId: "2", storeId: "1001" }),
		]);
		expect(groups.map((g) => g.storeId)).toEqual(["2002", "1001"]);
	});

	// 靜默丟棄候選比不分組更糟：使用者會以為系統只找到這幾筆。
	it("帶不出商家身分的候選進 ungrouped，不得被丟掉", () => {
		const { groups, ungrouped } = groupPlatformCandidates([
			candidate({ candidateId: "1", storeId: "1001" }),
			candidate({ candidateId: "2" }),
		]);
		expect(groups).toHaveLength(1);
		expect(ungrouped.map((c) => c.candidateId)).toEqual(["2"]);
	});

	it("缺店名時退回可讀標題，不留空字串", () => {
		const { groups } = groupPlatformCandidates([
			candidate({ candidateId: "1", storeId: "1001", sellerName: "" }),
		]);
		expect(groups[0].sellerName).toBe("商家 1001");
	});

	// storeCode 缺值不得退回 storeId 組連結：`/sellers/{storeId}` 那條路徑正在被收掉。
	it("缺 storeCode 時 storeCode 為 null，不得用 storeId 頂替", () => {
		const { groups } = groupPlatformCandidates([
			candidate({ candidateId: "1", storeId: "1001" }),
		]);
		expect(groups[0].storeCode).toBeNull();
	});

	it("同店但只有第一筆帶店名/logo 時仍取得到群組標題資料", () => {
		const { groups } = groupPlatformCandidates([
			candidate({
				candidateId: "1",
				storeId: "1001",
				sellerName: "青果行 A",
				storeCode: "25ABCDEFGH",
				logoUrl: "https://media.test/i/original/seller/1001/logo.jpg",
			}),
			candidate({ candidateId: "2", storeId: "1001" }),
		]);
		expect(groups[0].sellerName).toBe("青果行 A");
		expect(groups[0].storeCode).toBe("25ABCDEFGH");
		expect(groups[0].logoUrl).toBe(
			"https://media.test/i/original/seller/1001/logo.jpg",
		);
	});
});

describe("群組的查核結果與時間", () => {
	// 後端目前對同一次搜尋的每筆 PLATFORM 候選算出的 matched/unverifiable 完全相同
	// （candidate_platform_lane.go 只依 slots 計算）。用聯集而不是取第一筆，是為了讓
	// 後端日後改成逐候選判定時這裡仍然誠實。
	it("matched/unverifiable 取聯集並依 key 去重", () => {
		const { groups } = groupPlatformCandidates([
			candidate({
				candidateId: "1",
				storeId: "1001",
				matchedFacts: [{ key: "item", label: "品項" }],
				unverifiableFacts: [{ key: "quantity", reason: "需向商家確認" }],
			}),
			candidate({
				candidateId: "2",
				storeId: "1001",
				matchedFacts: [
					{ key: "item", label: "品項" },
					{ key: "budget_per_unit", label: "預算" },
				],
				unverifiableFacts: [{ key: "quantity", reason: "需向商家確認" }],
			}),
		]);
		expect(groups[0].matchedFacts.map((f) => f.key)).toEqual([
			"item",
			"budget_per_unit",
		]);
		expect(groups[0].unverifiableFacts).toHaveLength(1);
	});

	it("factsAsOf 取群組內最新、score 取最高", () => {
		const { groups } = groupPlatformCandidates([
			candidate({
				candidateId: "1",
				storeId: "1001",
				factsAsOf: "2026-08-21T00:00:00Z",
				score: 3,
			}),
			candidate({
				candidateId: "2",
				storeId: "1001",
				factsAsOf: "2026-08-22T00:00:00Z",
				score: 6,
			}),
		]);
		expect(groups[0].factsAsOf).toBe("2026-08-22T00:00:00Z");
		expect(groups[0].score).toBe(6);
	});
});

describe("賣家層勾選", () => {
	function group(selectedFlags: boolean[]) {
		return groupPlatformCandidates(
			selectedFlags.map((selected, i) =>
				candidate({
					candidateId: String(i + 1),
					storeId: "1001",
					selected,
				}),
			),
		).groups[0];
	}

	// 三態而不是布林：部分勾選必須表達得出來，否則使用者對「這家我選了嗎」得不到正確答案。
	it("勾選狀態是 none / partial / all 三態", () => {
		expect(sellerSelectionState(group([false, false]))).toBe("none");
		expect(sellerSelectionState(group([true, false]))).toBe("partial");
		expect(sellerSelectionState(group([true, true]))).toBe("all");
	});

	// 詢價單的收件人是商家，一家只需要問一次——選這家＝底下候選全帶進去。
	it("未選時勾選 → 把全部候選切成已選", () => {
		const { candidates, nextSelected } = candidatesToToggleForSeller(
			group([false, false]),
		);
		expect(nextSelected).toBe(true);
		expect(candidates.map((c) => c.candidateId)).toEqual(["1", "2"]);
	});

	// 使用者點一個「部分勾選」的框，預期是補齊而不是清空。
	it("部分勾選時往全選走，且只送需要改變的那幾筆", () => {
		const { candidates, nextSelected } = candidatesToToggleForSeller(
			group([true, false]),
		);
		expect(nextSelected).toBe(true);
		expect(candidates.map((c) => c.candidateId)).toEqual(["2"]);
	});

	it("全選時勾選 → 全部取消", () => {
		const { candidates, nextSelected } = candidatesToToggleForSeller(
			group([true, true]),
		);
		expect(nextSelected).toBe(false);
		expect(candidates.map((c) => c.candidateId)).toEqual(["1", "2"]);
	});

	// 已經是目標狀態的不重送：每一筆都會打一次 toggle API，重送等於白白製造請求。
	it("已在目標狀態的候選不重送", () => {
		const { candidates } = candidatesToToggleForSeller(
			group([true, true, true]),
		);
		expect(candidates).toHaveLength(3); // 目標是取消，三筆都要改
		const partial = candidatesToToggleForSeller(group([true, true, false]));
		expect(partial.candidates).toHaveLength(1); // 目標是補齊，只有一筆要改
	});
});

/**
 * 底部行動列講「已選 N 家」，而勾選契約是 per-candidate 的。這一組釘的就是那個換算：
 * 說出來的數字必須跟畫面上被勾起來的**列數**一致，不是打出去的請求數。
 */
describe("countSelectedRecipients", () => {
	it("同一家商家底下勾多筆只算一家", () => {
		expect(
			countSelectedRecipients([
				candidate({ candidateId: "1", storeId: "77", selected: true }),
				candidate({ candidateId: "2", storeId: "77", selected: true }),
				candidate({ candidateId: "3", storeId: "88", selected: true }),
			]),
		).toBe(2);
	});

	// EXTERNAL lane 的站外業者沒有 storeId，畫面上本來就是一家一列。
	it("帶不出 storeId 的候選各自算一家", () => {
		expect(
			countSelectedRecipients([
				candidate({ candidateId: "1", lane: "EXTERNAL", selected: true }),
				candidate({ candidateId: "2", lane: "EXTERNAL", selected: true }),
			]),
		).toBe(2);
	});

	// 部署窗口內混到的舊候選走 ungrouped、各自成列，與收斂後的商家列相加。
	it("收斂得了的與收斂不了的相加", () => {
		expect(
			countSelectedRecipients([
				candidate({ candidateId: "1", storeId: "77", selected: true }),
				candidate({ candidateId: "2", storeId: "77", selected: true }),
				candidate({ candidateId: "3", selected: true }),
			]),
		).toBe(2);
	});

	it("一筆都沒選是 0", () => {
		expect(countSelectedRecipients([])).toBe(0);
	});
});
