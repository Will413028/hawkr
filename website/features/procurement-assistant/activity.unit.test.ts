import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
	type AgentActivityEvent,
	activityLabel,
	activityStatusLabel,
	groupActivityRows,
	normalizeActivityEvent,
} from "./activity";

describe("normalizeActivityEvent", () => {
	it("解析合法 entry snapshot（text 空字串）", () => {
		expect(
			normalizeActivityEvent({ rowId: "123", code: "EXTRACT_SLOTS", text: "" }),
		).toEqual({
			rowId: "123",
			code: "EXTRACT_SLOTS",
			label: "",
			status: "RUNNING",
			text: "",
		});
	});

	it("解析合法 completion snapshot", () => {
		expect(
			normalizeActivityEvent({
				rowId: "123",
				code: "EXTRACT_SLOTS",
				text: "抓到你要的蘋果",
			}),
		).toEqual({
			rowId: "123",
			code: "EXTRACT_SLOTS",
			label: "",
			status: "SUCCEEDED",
			text: "抓到你要的蘋果",
		});
	});

	it("非物件回傳 null", () => {
		expect(normalizeActivityEvent("not an object")).toBeNull();
		expect(normalizeActivityEvent(null)).toBeNull();
		expect(normalizeActivityEvent(undefined)).toBeNull();
	});

	it("缺 rowId 或 rowId 是空字串回傳 null", () => {
		expect(
			normalizeActivityEvent({ code: "EXTRACT_SLOTS", text: "" }),
		).toBeNull();
		expect(
			normalizeActivityEvent({ rowId: "", code: "EXTRACT_SLOTS", text: "" }),
		).toBeNull();
	});

	it("缺 code 或 code 是空字串回傳 null", () => {
		expect(normalizeActivityEvent({ rowId: "1", text: "" })).toBeNull();
		expect(
			normalizeActivityEvent({ rowId: "1", code: "", text: "" }),
		).toBeNull();
	});

	it("text 不是字串回傳 null", () => {
		expect(
			normalizeActivityEvent({ rowId: "1", code: "EXTRACT_SLOTS", text: null }),
		).toBeNull();
	});
});

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

// 把「後端真的送出來的 bytes」接上「前端 fail-closed 的消費端」。在此之前這
// 兩半各自被測，中間沒有人把它們連起來：
//   - agui-fixtures.unit.test.ts 只驗 AG-UI 信封。@ag-ui/core 的
//     CustomEventSchema 是 BaseEventSchema.extend({ ..., value: z.any() })，
//     z.any() 連 undefined 都收，所以它對 activity payload 什麼都不保證。
//   - 本檔其餘案例全是手寫 inline 物件，碰不到真實 fixture。
// 於是後端若把 rowId 改名成 row_id：gateway 的 golden test 會過（fixture 由同一份
// 程式重生）、zod 閘門也會過（z.any()），而 normalizeActivityEvent 會對每一筆
// 回 null——整條 rail 從 UI 消失，卻沒有任何測試轉紅。這個 describe 補的就是
// 這一刀。
//
// 涵蓋範圍誠實標注：它只釘住一個 canonical frame，而它的意義建立在這份 fixture
// 就是 gateway golden test 逐位元組比對的那一份上——那是它成立的基礎，不是它的弱點。
const FIXTURE_DIR = join(
	fileURLToPath(new URL("../../", import.meta.url)),
	"../backend/gateway/internal/proxy/testdata/agui-frames",
);

/** 讀一份 .sse fixture，回傳每個 frame 的 `value`（AG-UI CUSTOM 的酬載）。 */
function fixtureValues(name: string): unknown[] {
	return readFileSync(join(FIXTURE_DIR, name), "utf8")
		.split("\n")
		.filter((line) => line.startsWith("data:"))
		.map((line) => JSON.parse(line.slice(5).replace(/^\s/, "")))
		.map((frame: { value?: unknown }) => frame.value);
}

describe("normalizeActivityEvent 對 gateway 產生的真實 frame", () => {
	it("custom-activity.sse（graph 路徑）的 value 通得過 fail-closed 解析", () => {
		const values = fixtureValues("custom-activity.sse");
		expect(values).toHaveLength(1);
		const event = normalizeActivityEvent(values[0]);

		// 不是 null ⇒ 後端的欄位名與型別仍然對得上消費端的契約。
		expect(event).not.toBeNull();
		expect(event?.rowId).toBe("7489");
		expect(event?.code).toBe("EXTRACT_SLOTS");
		expect(event?.text).toBe("整理你的採購條件");
		// T23 之後恆存在的兩欄。fixture 停在舊形狀時這兩行是唯一會轉紅的地方。
		expect(event?.label).toBe("抓出品項、數量和交期");
		expect(event?.status).toBe("SUCCEEDED");
	});
});

// 這個 describe 的價值在「跨 repo 逐位元組閘門讓這份 fixture 等於 gateway 真實
// 輸出」這個前提上：它把後端真的送出來的 bytes 一路餵到 groupActivityRows，
// 涵蓋設計稿 01 右欄那三件既有 fixture 零覆蓋的事——沒有 narrator 的空 text、
// 巢狀 lane 子列、以及「缺席 ≠ 0」的家數。
describe("候選搜尋 rail 的真實 frame（T23，無 narrator）", () => {
	const events = fixtureValues("custom-activity-candidate-search-rail.sse").map(
		(value) => {
			const event = normalizeActivityEvent(value);
			expect(event).not.toBeNull();
			return event as NonNullable<ReturnType<typeof normalizeActivityEvent>>;
		},
	);

	it("三列的 text 全空，顯示文字一律來自 label", () => {
		expect(events.map((e) => e.text)).toEqual(["", "", ""]);
		expect(events.map(activityLabel)).toEqual([
			"搜尋候選供應商",
			"翻平台上的賣家",
			"整理成給你的建議",
		]);
	});

	it("狀態來自 status 軸，含尚未跑到的 PENDING 列", () => {
		expect(events.map((e) => e.status)).toEqual([
			"RUNNING",
			"SUCCEEDED",
			"PENDING",
		]);
	});

	it("lane 子列掛在父列底下，未跑到的步驟仍是 top-level", () => {
		const grouped = groupActivityRows(events);
		expect(grouped.map((r) => r.rowId)).toEqual(["9001", "9004"]);
		expect(grouped[0].children.map((c) => c.code)).toEqual(["SEARCH_PLATFORM"]);
	});

	it("只有真的有計數概念的那一列帶 count，其餘是 undefined 而不是 0", () => {
		expect(events.map((e) => e.count)).toEqual([undefined, "12", undefined]);
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

describe("normalizeActivityEvent 的 status 軸", () => {
	it("wire 帶合法 status 時直接採用", () => {
		expect(
			normalizeActivityEvent({
				rowId: "1",
				code: "SEARCH_CANDIDATES",
				label: "搜尋候選供應商",
				text: "",
				status: "PENDING",
			})?.status,
		).toBe("PENDING");
	});

	it("status 缺席時退回舊的 text 推導，不讓部署窗口內的列整批消失", () => {
		expect(
			normalizeActivityEvent({ rowId: "1", code: "EXTRACT_SLOTS", text: "" })
				?.status,
		).toBe("RUNNING");
		expect(
			normalizeActivityEvent({
				rowId: "1",
				code: "EXTRACT_SLOTS",
				text: "抓到你要的蘋果",
			})?.status,
		).toBe("SUCCEEDED");
	});

	it("未知 status 退回推導而不是丟掉整列——status 不是 privacy boundary，丟列只會讓 rail 無聲變短", () => {
		const event = normalizeActivityEvent({
			rowId: "1",
			code: "EXTRACT_SLOTS",
			label: "抓出品項、數量和交期",
			text: "",
			status: "SOMETHING_NEW",
		});
		expect(event).not.toBeNull();
		expect(event?.status).toBe("RUNNING");
	});
});

// 「缺席 ≠ 帶零值」是 T23 wire 的明文不變式（graph_dispatch.go 的註解：補 "0"／""
// 會讓 consumer 分不出「沒有」與「是 0」）。flow 53 實測過這件事：PLATFORM 查到
// 0 家時送 count:"0"，而父列與 SUMMARIZE 整個欄位不存在。
describe("normalizeActivityEvent 的 parentRowId／count", () => {
	it("有值時原樣保留（count 是十進位字串，不轉 number）", () => {
		const event = normalizeActivityEvent({
			rowId: "9002",
			code: "SEARCH_PLATFORM",
			label: "翻平台上的賣家",
			text: "",
			status: "SUCCEEDED",
			parentRowId: "9001",
			count: "12",
		});
		expect(event?.parentRowId).toBe("9001");
		expect(event?.count).toBe("12");
	});

	it('count 為 "0" 是真的 0 家，不得當成缺席', () => {
		expect(
			normalizeActivityEvent({
				rowId: "9002",
				code: "SEARCH_PLATFORM",
				label: "翻平台上的賣家",
				text: "",
				status: "SUCCEEDED",
				parentRowId: "9001",
				count: "0",
			})?.count,
		).toBe("0");
	});

	it('缺席時是 undefined，不得補成 "0" 或空字串', () => {
		const event = normalizeActivityEvent({
			rowId: "9001",
			code: "SEARCH_CANDIDATES",
			label: "搜尋候選供應商",
			text: "",
			status: "RUNNING",
		});
		expect(event?.parentRowId).toBeUndefined();
		expect(event?.count).toBeUndefined();
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

describe("mergeLiveActivity（住 activity.ts，兩條 transport 共用）", () => {
	const base: AgentActivityEvent = {
		rowId: "9001",
		code: "SEARCH_CANDIDATES",
		label: "搜尋候選供應商",
		status: "PENDING",
		text: "",
	};

	it("同一個 rowId 就地覆寫，不長成第二列", async () => {
		const { mergeLiveActivity } = await import("./activity");
		const merged = mergeLiveActivity([base], { ...base, status: "RUNNING" });
		expect(merged).toHaveLength(1);
		expect(merged[0].status).toBe("RUNNING");
	});

	it("新的 rowId 依抵達順序附加在後面", async () => {
		const { mergeLiveActivity } = await import("./activity");
		const merged = mergeLiveActivity([base], {
			...base,
			rowId: "9002",
			code: "SEARCH_PLATFORM",
		});
		expect(merged.map((e) => e.rowId)).toEqual(["9001", "9002"]);
	});

	it("空 text 的 entry 事件不得覆蓋已顯示的完成敘述（reconnect replay 防呆）", async () => {
		const { mergeLiveActivity } = await import("./activity");
		const completed = { ...base, text: "已找到 12 家" };
		const events = [completed];
		const merged = mergeLiveActivity(events, { ...base, text: "" });
		expect(merged[0].text).toBe("已找到 12 家");
		// 回傳的必須是**同一個陣列 reference**，不是內容相同的新陣列：呼叫端
		// （useRunActivityStream 的 setActivity）靠
		// 這個 identity 才不會每收到一次 replay 就重繪整條軌。
		expect(merged).toBe(events);
	});

	it("parseActivityEventJson 對壞掉的 JSON 回 null，不丟例外", async () => {
		const { parseActivityEventJson } = await import("./activity");
		expect(parseActivityEventJson("{not json")).toBeNull();
	});

	it("parseActivityEventJson 對合法 frame value 回正規化後的列", async () => {
		const { parseActivityEventJson } = await import("./activity");
		const event = parseActivityEventJson(
			JSON.stringify({
				rowId: "9002",
				code: "SEARCH_PLATFORM",
				text: "",
				count: "12",
			}),
		);
		expect(event?.count).toBe("12");
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

	it("取最後一個 RUNNING 的列，不是最後抵達的列", async () => {
		const { lastRunningActivity } = await import("./activity");
		// declarePlan 開場就把整份清單以 PENDING 送出，所以最後抵達的是還沒跑到
		// 的 SUMMARIZE——播報它等於講一句當下為假的話。
		expect(lastRunningActivity(rows)?.code).toBe("SEARCH_PLATFORM");
	});

	it("沒有任何 RUNNING 時回 null", async () => {
		const { lastRunningActivity } = await import("./activity");
		expect(
			lastRunningActivity(
				rows.map((r) => ({ ...r, status: "SUCCEEDED" as const })),
			),
		).toBeNull();
	});

	it("空清單回 null", async () => {
		const { lastRunningActivity } = await import("./activity");
		expect(lastRunningActivity([])).toBeNull();
	});
});
