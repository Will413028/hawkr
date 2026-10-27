/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ProcurementCandidateSearch } from "@/lib/api/openapi-types";
import AgentTaskPanel from "./AgentTaskPanel";
import type { AgentActivityEvent } from "./activity";

vi.mock("./HawkrAgentSprite", () => ({
	default: ({ state, className }: { state: string; className: string }) =>
		React.createElement("span", {
			className,
			"data-motion": state,
			"aria-hidden": true,
		}),
}));

afterEach(cleanup);

const row: AgentActivityEvent = {
	rowId: "1",
	code: "UNDERSTAND_INPUT",
	label: "理解採購需求",
	status: "RUNNING",
	text: "",
};

const searchRows: AgentActivityEvent[] = [
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
		status: "SUCCEEDED",
		parentRowId: "9001",
		count: "12",
		text: "",
	},
	{
		rowId: "9003",
		code: "SEARCH_EXTERNAL",
		label: "查看外部業者",
		status: "RUNNING",
		parentRowId: "9001",
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

const baseProps: React.ComponentProps<typeof AgentTaskPanel> = {
	stage: "clarifying",
	pending: false,
	liveSteps: [],
	liveActivity: [],
	searchActivity: [],
	motion: "SEARCHING",
	candidateSet: null,
	pollError: false,
	onRetryPolling: () => {},
};

function renderPanel(
	overrides: Partial<React.ComponentProps<typeof AgentTaskPanel>> = {},
) {
	return render(
		React.createElement(AgentTaskPanel, { ...baseProps, ...overrides }),
	);
}

function visibleRowIds() {
	return Array.from(
		screen.getByTestId("agent-activity-rail").querySelectorAll("[data-row-id]"),
		(element) => element.getAttribute("data-row-id"),
	);
}

describe("AgentTaskPanel 當前回合的處理 surface", () => {
	it.each([
		"clarifying",
		"ready",
		"candidates",
	] as const)("%s 階段沒有待處理回合時，不呈現舊列或任務空狀態", (stage) => {
		const { container } = renderPanel({
			stage,
			liveActivity: [row],
			searchActivity: searchRows,
		});
		expect(container.childElementCount).toBe(0);
	});

	it("尚未收到公開步驟時只中性等待，沒有虛構進度或額外 live region", () => {
		const { container } = renderPanel({ pending: true });
		expect(screen.getByText("等待回應…")).toBeTruthy();
		expect(screen.queryByTestId("agent-activity-rail")).toBeNull();
		expect(
			container.querySelector("[data-motion]")?.getAttribute("data-motion"),
		).toBe("SUBMITTING");
		expect(container.querySelector('[role="status"], [aria-live]')).toBeNull();
	});

	it("同 rowId 的公開步驟原位完成，其餘列保留且不設兩列上限", () => {
		const { rerender } = renderPanel({
			pending: true,
			liveActivity: searchRows,
		});
		const originalRow = screen
			.getByTestId("agent-activity-rail")
			.querySelector('[data-row-id="9001"]');
		expect(visibleRowIds()).toEqual(["9001", "9002", "9003", "9004"]);
		rerender(
			React.createElement(AgentTaskPanel, {
				...baseProps,
				pending: true,
				liveActivity: searchRows.map<AgentActivityEvent>((event) =>
					event.rowId === "9001" ? { ...event, status: "SUCCEEDED" } : event,
				),
			}),
		);
		const completedRow = screen
			.getByTestId("agent-activity-rail")
			.querySelector('[data-row-id="9001"]');
		expect(completedRow).toBe(originalRow);
		expect(completedRow?.getAttribute("data-status")).toBe("SUCCEEDED");
		expect(completedRow?.querySelector("[data-motion]")).toBeNull();
		expect(visibleRowIds()).toEqual(["9001", "9002", "9003", "9004"]);
	});

	it("legacy public steps 仍能呈現，不要求一定有 graph Activity", () => {
		renderPanel({
			pending: true,
			liveSteps: [{ code: "UNDERSTAND_INPUT", status: "RUNNING" }],
		});
		expect(screen.getByText("讀你寫的需求")).toBeTruthy();
		expect(screen.getByText("進行中")).toBeTruthy();
	});

	it("回覆落地立即移除思考；下一輪空白等待不繼承前輪快照", () => {
		const { rerender } = renderPanel({ pending: true, liveActivity: [row] });
		expect(screen.getByText("理解採購需求")).toBeTruthy();
		rerender(React.createElement(AgentTaskPanel, baseProps));
		expect(screen.queryByTestId("agent-processing")).toBeNull();
		rerender(
			React.createElement(AgentTaskPanel, { ...baseProps, pending: true }),
		);
		expect(screen.queryByText("理解採購需求")).toBeNull();
		expect(screen.getByText("等待回應…")).toBeTruthy();
	});
});

describe("AgentTaskPanel 搜尋狀態", () => {
	it("從 turn 轉入 searching，只顯示本次搜尋列", () => {
		const { rerender, container } = renderPanel({
			pending: true,
			liveActivity: [row],
		});
		rerender(
			React.createElement(AgentTaskPanel, {
				...baseProps,
				stage: "searching",
				searchActivity: searchRows,
			}),
		);
		expect(visibleRowIds()).toEqual(["9001", "9002", "9003", "9004"]);
		expect(screen.queryByText("理解採購需求")).toBeNull();
		expect(screen.getByText("12 家")).toBeTruthy();
		expect(screen.getByText("等待中")).toBeTruthy();
		expect(screen.queryByText("正在搜尋候選")).toBeNull();
		expect(container.querySelector('[role="status"], [aria-live]')).toBeNull();
	});

	it("沒有搜尋事件時以輪詢的真實 lane 狀態呈現，不回放 turn 列", () => {
		const candidateSet: ProcurementCandidateSearch = {
			requestId: "request-1",
			runId: "run-1",
			candidateSetId: "candidates-1",
			status: "RUNNING",
			platformLaneStatus: "SUCCEEDED",
			externalLaneStatus: "PENDING",
			candidates: { platform: [], external: [] },
		};
		const { rerender } = renderPanel({ pending: true, liveActivity: [row] });
		rerender(
			React.createElement(AgentTaskPanel, {
				...baseProps,
				stage: "searching",
				candidateSet,
			}),
		);
		expect(screen.getByText("正在搜尋候選")).toBeTruthy();
		expect(screen.getByText("已完成")).toBeTruthy();
		expect(screen.getByText("外部業者")).toBeTruthy();
		expect(screen.queryByTestId("agent-activity-rail")).toBeNull();
		expect(screen.queryByText("理解採購需求")).toBeNull();
	});

	it.each([
		{ searchActivity: searchRows },
		{ searchActivity: [] },
	])("搜尋事件與輪詢降級都提供可操作的連線重試入口 %#", ({
		searchActivity,
	}) => {
		const retry = vi.fn();
		renderPanel({
			stage: "searching",
			searchActivity,
			pollError: true,
			onRetryPolling: retry,
		});
		expect(screen.getByRole("alert")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "重試" }));
		expect(retry).toHaveBeenCalledTimes(1);
	});

	it("搜尋完成移除思考，重新搜尋不繼承前次列", () => {
		const { rerender } = renderPanel({
			stage: "searching",
			searchActivity: searchRows,
		});
		rerender(
			React.createElement(AgentTaskPanel, {
				...baseProps,
				stage: "candidates",
				searchActivity: searchRows,
			}),
		);
		expect(screen.queryByTestId("agent-processing")).toBeNull();
		rerender(
			React.createElement(AgentTaskPanel, {
				...baseProps,
				stage: "searching",
				searchActivity: [{ ...searchRows[0], rowId: "9101" }],
			}),
		);
		expect(visibleRowIds()).toEqual(["9101"]);
		expect(screen.queryByText("整理成給你的建議")).toBeNull();
	});
});
