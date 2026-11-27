/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import AgentActivityRail from "./AgentActivityRail";
import type { AgentActivityEvent } from "./activity";

afterEach(cleanup);

function activityRow(
	overrides: Partial<AgentActivityEvent> & Pick<AgentActivityEvent, "rowId">,
): AgentActivityEvent {
	return {
		code: "SEARCH_CANDIDATES",
		label: "搜尋候選供應商",
		status: "RUNNING",
		text: "",
		...overrides,
	};
}

function renderRail(activity: AgentActivityEvent[]) {
	return render(
		React.createElement(AgentActivityRail, {
			steps: [],
			activity,
			motion: "SEARCHING" as const,
		}),
	);
}

describe("AgentActivityRail 的完成狀態來自 status 而不是 text", () => {
	it("SUCCEEDED 且 text 空的列顯示 label，且不標「進行中」", () => {
		renderRail([activityRow({ rowId: "9001", status: "SUCCEEDED" })]);

		expect(screen.getByText("搜尋候選供應商")).toBeTruthy();
		expect(screen.queryByText("進行中")).toBeNull();
	});
});

describe("AgentActivityRail 每一列都標出狀態", () => {
	it("等待中／進行中／已完成三種列各自顯示自己的狀態字", () => {
		renderRail([
			activityRow({ rowId: "1", label: "理解採購需求", status: "SUCCEEDED" }),
			activityRow({ rowId: "2", label: "搜尋候選供應商", status: "RUNNING" }),
			activityRow({ rowId: "3", label: "整理候選清單", status: "PENDING" }),
		]);

		expect(screen.getByText("已完成")).toBeTruthy();
		expect(screen.getByText("進行中")).toBeTruthy();
		expect(screen.getByText("等待中")).toBeTruthy();
	});
});

describe("AgentActivityRail 的巢狀 lane 子列", () => {
	it("子列渲染在父列底下的巢狀清單裡", () => {
		const { container } = renderRail([
			activityRow({ rowId: "9001", label: "搜尋候選供應商" }),
			activityRow({
				rowId: "9002",
				label: "平台賣家",
				status: "SUCCEEDED",
				parentRowId: "9001",
				count: "12",
			}),
		]);

		const group = container
			.querySelector('[data-row-id="9001"]')
			?.closest("li");
		expect(group).toBeTruthy();
		expect(group?.querySelector('[data-row-id="9002"]')).not.toBeNull();
		expect(
			group?.querySelector('.hawkr-agent-rail__children [data-row-id="9002"]'),
		).not.toBeNull();
	});

	it("已完成的 lane 顯示「12 家」，進行中的 lane 顯示「已整理 7 家」", () => {
		renderRail([
			activityRow({ rowId: "9001", label: "搜尋候選供應商" }),
			activityRow({
				rowId: "9002",
				label: "平台賣家",
				status: "SUCCEEDED",
				parentRowId: "9001",
				count: "12",
			}),
			activityRow({
				rowId: "9003",
				label: "外部業者",
				status: "RUNNING",
				parentRowId: "9001",
				count: "7",
			}),
		]);

		expect(screen.getByText("12 家")).toBeTruthy();
		expect(screen.getByText("已整理 7 家")).toBeTruthy();
	});

	it("count 缺席的列不顯示家數——缺席不是 0 家", () => {
		const { container } = renderRail([
			activityRow({ rowId: "9001", label: "搜尋候選供應商" }),
			activityRow({
				rowId: "9002",
				label: "平台賣家",
				status: "SUCCEEDED",
				parentRowId: "9001",
			}),
		]);

		expect(container.querySelector(".hawkr-agent-rail__count")).toBeNull();
	});

	it("count 為 0 時照樣顯示「0 家」，那是「真的一家都沒找到」", () => {
		renderRail([
			activityRow({ rowId: "9001", label: "搜尋候選供應商" }),
			activityRow({
				rowId: "9002",
				label: "平台賣家",
				status: "SUCCEEDED",
				parentRowId: "9001",
				count: "0",
			}),
		]);

		expect(screen.getByText("0 家")).toBeTruthy();
	});
});
