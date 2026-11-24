import { describe, expect, it } from "vitest";
import {
	type AgentActivityEvent,
	activityLabel,
	activityStatusLabel,
	groupActivityRows,
	lastRunningActivity,
	mergeLiveActivity,
} from "./activity";

describe("activityLabel", () => {
	it("entry snapshot 顯示通用佔位文字", () => {
		expect(
			activityLabel({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "",
				status: "RUNNING",
				text: "",
			}),
		).toBe("處理中…");
	});

	it("completion snapshot 直接顯示模型文字", () => {
		expect(
			activityLabel({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "",
				status: "SUCCEEDED",
				text: "抓到你要的蘋果",
			}),
		).toBe("抓到你要的蘋果");
	});

	it("超長文字在展示層截斷", () => {
		const longText = "字".repeat(100);
		expect(
			activityLabel({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "",
				status: "SUCCEEDED",
				text: longText,
			}),
		).toBe(`${"字".repeat(80)}…`);
	});

	it("截斷不得切壞 surrogate pair（emoji 剛好落在第 80 個字元邊界）", () => {
		// 79 個全形字 + 1 個 emoji（占 2 個 UTF-16 code unit）+ 20 個全形字：
		// 舊版用 `.length`／`.slice` 以 UTF-16 code unit 計數，剛好在 emoji 中間
		// 切一刀，留下孤兒 surrogate（顯示成 U+FFFD）。改用 code point 陣列後，
		// emoji 完整落在第 80 個 code point，不會被切開。用 \u{...} code point
		// escape（U+1F600，運行期跟字面 emoji 字元是同一個 surrogate pair）
		// 寫，不讓本檔的 no-emoji gate（scripts/check-no-emoji.ts）把測試資料
		// 誤判成 UI 文字。
		const emoji = "\u{1F600}";
		const text = `${"字".repeat(79)}${emoji}${"字".repeat(20)}`;
		expect(
			activityLabel({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				label: "",
				status: "SUCCEEDED",
				text,
			}),
		).toBe(`${"字".repeat(79)}${emoji}…`);
	});
});

// T23 候選搜尋 rail（backend PR #3157）。這條 run 沒有 StepNarrator，所以它的
// 每一列 `text` 恆為空字串——顯示文字只能來自後端的固定字串表 `label`
// （domain.StepLabels，wire 上恆非空）。舊版消費端把 `text === ""` 當成「還在
// 跑」，於是這四列會永遠停在「處理中…」配旋轉彈珠，一列都不會完成。
describe("T23 候選搜尋 rail 的列（無 narrator，text 恆空）", () => {
	it("text 空但 label 有值時顯示 label，不是「處理中…」", () => {
		expect(
			activityLabel({
				rowId: "1",
				code: "SEARCH_CANDIDATES",
				label: "搜尋候選供應商",
				status: "SUCCEEDED",
				text: "",
			}),
		).toBe("搜尋候選供應商");
	});
});

// 設計稿 01 每一列右側都有狀態字，不只進行中那一列。文案是買家面的，跟 CodeMeta
// 的 PROCUREMENT_STEP_STATUS_LABEL（「待執行」／「執行中」，偏管理端）刻意分開——
// 同 CandidateSearchProgress 的 LANE_STATUS_LABEL 慣例。
describe("activityStatusLabel", () => {
	it("涵蓋設計稿 01 畫出來的三種狀態", () => {
		expect(activityStatusLabel("PENDING")).toBe("等待中");
		expect(activityStatusLabel("RUNNING")).toBe("進行中");
		expect(activityStatusLabel("SUCCEEDED")).toBe("已完成");
	});

	it("值域裡其餘四個狀態都有文案，沒有一個會顯示空白", () => {
		for (const status of [
			"WAITING_FOR_INPUT",
			"FAILED",
			"SKIPPED",
			"CANCELLED",
		] as const) {
			expect(activityStatusLabel(status)).not.toBe("");
		}
	});
});

function row(
	over: Partial<AgentActivityEvent> & Pick<AgentActivityEvent, "rowId">,
): AgentActivityEvent {
	return {
		code: "SEARCH_CANDIDATES",
		label: "搜尋候選供應商",
		status: "RUNNING",
		text: "",
		...over,
	};
}

// 設計稿 01 的兩條 lane 掛在「搜尋候選供應商」底下。後端只保證一層
// （candidate_search_test.go：父列不得帶 parentRowId），所以這裡也只做一層。
describe("groupActivityRows", () => {
	it("子列收進父列底下，top-level 維持抵達順序", () => {
		const grouped = groupActivityRows([
			row({ rowId: "9001", label: "搜尋候選供應商" }),
			row({ rowId: "9002", label: "翻平台上的賣家", parentRowId: "9001" }),
			row({ rowId: "9003", label: "找平台外的業者", parentRowId: "9001" }),
			row({ rowId: "9004", label: "整理成給你的建議", status: "PENDING" }),
		]);

		expect(grouped.map((r) => r.rowId)).toEqual(["9001", "9004"]);
		expect(grouped[0].children.map((c) => c.rowId)).toEqual(["9002", "9003"]);
		expect(grouped[1].children).toEqual([]);
	});

	it("父列還沒抵達的子列升級成 top-level，不靜默丟掉", () => {
		const grouped = groupActivityRows([
			row({ rowId: "9002", label: "翻平台上的賣家", parentRowId: "9001" }),
		]);

		expect(grouped.map((r) => r.rowId)).toEqual(["9002"]);
	});

	it("升級成 top-level 的子列留在它抵達的位置，不被推到最後面", () => {
		const grouped = groupActivityRows([
			row({ rowId: "8999", label: "找平台外的業者", parentRowId: "8000" }),
			row({ rowId: "9001", label: "搜尋候選供應商" }),
		]);

		expect(grouped.map((r) => r.rowId)).toEqual(["8999", "9001"]);
	});

	it("只支援一層：掛在子列底下的孫列升級成 top-level，不做第二層縮排", () => {
		const grouped = groupActivityRows([
			row({ rowId: "9001" }),
			row({ rowId: "9002", parentRowId: "9001" }),
			row({ rowId: "9003", parentRowId: "9002" }),
		]);

		expect(grouped.map((r) => r.rowId)).toEqual(["9001", "9003"]);
		expect(grouped[0].children.map((c) => c.rowId)).toEqual(["9002"]);
	});
});

describe("mergeLiveActivity", () => {
	const base: AgentActivityEvent = {
		rowId: "9001",
		code: "SEARCH_CANDIDATES",
		label: "搜尋候選供應商",
		status: "PENDING",
		text: "",
	};

	it("同一個 rowId 就地覆寫，不長成第二列", () => {
		const merged = mergeLiveActivity([base], { ...base, status: "RUNNING" });
		expect(merged).toHaveLength(1);
		expect(merged[0].status).toBe("RUNNING");
	});

	it("新的 rowId 依抵達順序附加在後面", () => {
		const merged = mergeLiveActivity([base], {
			...base,
			rowId: "9002",
			code: "SEARCH_PLATFORM",
		});
		expect(merged.map((e) => e.rowId)).toEqual(["9001", "9002"]);
	});

	it("空 text 的 entry 事件不得覆蓋已顯示的完成敘述（reconnect replay 防呆）", () => {
		const completed = { ...base, text: "已找到 12 家" };
		const events = [completed];
		const merged = mergeLiveActivity(events, { ...base, text: "" });
		expect(merged[0].text).toBe("已找到 12 家");
		// 回傳的必須是**同一個陣列 reference**，不是內容相同的新陣列：呼叫端
		// （useRunActivityStream 的 setActivity）靠
		// 這個 identity 才不會每收到一次 replay 就重繪整條軌。
		expect(merged).toBe(events);
	});
});

describe("lastRunningActivity", () => {
	const rows: AgentActivityEvent[] = [
		{
			rowId: "9001",
			code: "SEARCH_CANDIDATES",
			label: "搜尋候選供應商",
			status: "RUNNING",
			text: "",
		},
		{
			rowId: "9002",
			code: "SEARCH_PLATFORM",
			label: "翻平台上的賣家",
			status: "RUNNING",
			text: "",
		},
		{
			rowId: "9004",
			code: "SUMMARIZE",
			label: "整理成給你的建議",
			status: "PENDING",
			text: "",
		},
	];

	it("取最後一個 RUNNING 的列，不是最後抵達的列", () => {
		// declarePlan 開場就把整份清單以 PENDING 送出，所以最後抵達的是還沒跑到
		// 的 SUMMARIZE——播報它等於講一句當下為假的話。
		expect(lastRunningActivity(rows)?.code).toBe("SEARCH_PLATFORM");
	});

	it("沒有任何 RUNNING 時回 null", () => {
		expect(
			lastRunningActivity(
				rows.map((r) => ({ ...r, status: "SUCCEEDED" as const })),
			),
		).toBeNull();
	});

	it("空清單回 null", () => {
		expect(lastRunningActivity([])).toBeNull();
	});
});

// 回歸守門：mergeLiveActivity 的「entry 不得覆寫已完成的 text」護欄看 text。候選搜尋四列的 text 恆空，
// 護欄對它們永遠不觸發，status 才推得動；把護欄簡化成看 status 會鎖死這四列。
describe("mergeLiveActivity 對沒有 narrator 的列", () => {
	it("text 恆空的列仍能依序推進 PENDING → RUNNING → SUCCEEDED", () => {
		const base = {
			rowId: "9002",
			code: "SEARCH_PLATFORM",
			label: "翻平台上的賣家",
			text: "",
		} as const;

		let rows = mergeLiveActivity([], { ...base, status: "PENDING" });
		rows = mergeLiveActivity(rows, { ...base, status: "RUNNING" });
		rows = mergeLiveActivity(rows, {
			...base,
			status: "SUCCEEDED",
			count: "12",
		});

		expect(rows).toHaveLength(1);
		expect(rows[0].status).toBe("SUCCEEDED");
		expect(rows[0].count).toBe("12");
	});
});
