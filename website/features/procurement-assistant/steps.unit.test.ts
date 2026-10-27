import { describe, expect, it } from "vitest";
import {
	isPublicAgentStepCode,
	normalizeSteps,
	stepIcon,
	stepLabel,
} from "./steps";

/**
 * 2026-09-03：後端 StepNarrator 的敘述（step.text）開始隨 steps[] 一起送來，
 * 並取代前端的固定短句。這推翻了 steps.ts 原本「後端文字一律丟棄、使用者看見
 * 的文字只能由前端 allowlist 決定」的規則——那段註解已在同一次改動更新。
 *
 * 退路必須留著：narrator 逾時或被 guard 擋下時 text 缺席，此時仍要顯示固定短句
 * 而不是空白。
 */
describe("stepLabel：後端敘述優先，缺席時退回固定短句", () => {
	it("有 text 時直接顯示它，不套前端模板", () => {
		expect(
			stepLabel({
				code: "VALIDATE_SLOTS",
				text: "甘藍條件有效，無拒絕欄位。",
				safeArgs: { item: "甘藍", city: "桃園市" },
			}),
		).toBe("甘藍條件有效，無拒絕欄位。");
	});

	it("text 缺席時退回既有的動態模板（既有行為不變）", () => {
		expect(
			stepLabel({
				code: "PICK_NEXT_QUESTION",
				safeArgs: { item: "甘藍" },
			}),
		).toBe("已經抓到【甘藍】，看還缺什麼");
	});

	it("text 與 safeArgs 都缺席時退回固定短句，不留空白", () => {
		expect(stepLabel({ code: "VALIDATE_SLOTS" })).toBe("核對條件合不合理");
	});

	it("wire label 仍然一律丟棄——這條沒有跟著 text 一起放寬", () => {
		expect(
			stepLabel({ code: "VALIDATE_SLOTS", label: "後端送來的 label" }),
		).toBe("核對條件合不合理");
	});
});

describe("normalizeSteps：text 的取值與清理", () => {
	const wire = (extra: Record<string, unknown>) => [
		{ code: "VALIDATE_SLOTS", status: "SUCCEEDED", ...extra },
	];

	it("帶回 text 並去除前後空白", () => {
		expect(normalizeSteps(wire({ text: "  甘藍條件有效。  " }))[0].text).toBe(
			"甘藍條件有效。",
		);
	});

	it("空字串／全空白視為沒有敘述（讓 stepLabel 走退路，不顯示空行）", () => {
		expect(normalizeSteps(wire({ text: "   " }))[0].text).toBeUndefined();
		expect(normalizeSteps(wire({ text: "" }))[0].text).toBeUndefined();
	});

	it("非字串的 text 一律丟棄", () => {
		expect(normalizeSteps(wire({ text: 42 }))[0].text).toBeUndefined();
	});
});

describe("stepIcon", () => {
	it("已知 code 對到專屬 icon，未知 code 降級成 clock", () => {
		expect(stepIcon("VALIDATE_SLOTS")).toBe("filter");
		expect(stepIcon("NOT_A_REAL_CODE")).toBe("clock");
	});
});

/**
 * 2026-09-03：後端開始在 steps[] 送出 CROP_GATE（品項分類判定）。這一碼以前
 * 只走即時 Activity 事件、不進持久化的 steps[]，因此不在本檔的兩張表裡——而
 * normalizeSteps 是 deny-by-default，漏列不會報錯、只會讓這一碼被整個丟掉。
 * 這組測試就是釘住「兩張表都補齊了」。
 */
describe("CROP_GATE：新接進 steps[] 的品項分類判定", () => {
	it("通過白名單，不被 normalizeSteps 丟掉", () => {
		const steps = normalizeSteps([
			{
				code: "CROP_GATE",
				status: "SUCCEEDED",
				text: "品項「高麗菜」的分類判斷已完成。",
			},
		]);
		expect(steps).toHaveLength(1);
		expect(steps[0].code).toBe("CROP_GATE");
	});

	it("有後端敘述時顯示敘述，缺席時退回固定短句", () => {
		expect(
			stepLabel({
				code: "CROP_GATE",
				text: "品項尚待買家從 3 個候選作物中選擇。",
			}),
		).toBe("品項尚待買家從 3 個候選作物中選擇。");
		expect(stepLabel({ code: "CROP_GATE" })).toBe("核對品項分類");
	});

	it("有自己的 icon，不與同輪相鄰的步驟撞圖、也不落到 clock 降級", () => {
		expect(stepIcon("CROP_GATE")).toBe("folder");
		expect(stepIcon("CROP_GATE")).not.toBe(stepIcon("VALIDATE_SLOTS"));
		expect(stepIcon("CROP_GATE")).not.toBe(stepIcon("PICK_NEXT_QUESTION"));
		expect(stepIcon("CROP_GATE")).not.toBe("clock");
	});
});

// T23 為候選搜尋階段新增了父列碼 SEARCH_CANDIDATES（backend domain/steps.go），
// 同一批把 FAILED 移出 code 軸（失敗只住 status 軸，T23 D3）。前端這兩張表當時
// 沒跟上：SEARCH_CANDIDATES 不在白名單 ⇒ normalizeSteps 靜默丟掉整列；不在
// STEP_ICON ⇒ 設計稿 01 的「搜尋候選供應商」父列拿到 clock 退化圖示。
describe("T23 對 step 碼表的增刪", () => {
	it("SEARCH_CANDIDATES 是公開碼，不會被 normalizeSteps 丟掉", () => {
		expect(isPublicAgentStepCode("SEARCH_CANDIDATES")).toBe(true);
		expect(
			normalizeSteps([{ code: "SEARCH_CANDIDATES", status: "RUNNING" }]),
		).toHaveLength(1);
	});

	it("SEARCH_CANDIDATES 有自己的圖示，不落到 clock 退化", () => {
		expect(stepIcon("SEARCH_CANDIDATES")).not.toBe("clock");
	});

	it("FAILED 不再是 code 軸的成員——失敗只住 status 軸", () => {
		expect(isPublicAgentStepCode("FAILED")).toBe(false);
	});
});
