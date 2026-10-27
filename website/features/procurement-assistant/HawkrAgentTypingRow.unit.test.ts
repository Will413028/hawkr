/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import type { AgentMotionState } from "./agent-motion";
import HawkrAgentTypingRow from "./HawkrAgentTypingRow";

afterEach(cleanup);

function renderRow(
	props: Partial<React.ComponentProps<typeof HawkrAgentTypingRow>> = {},
) {
	// 包一層 <ul>：這一列是 transcript 清單的項目，直接掛在容器 div 底下會是
	// 不合法的 DOM 巢狀，測出來的結構就不是實際渲染的那個。
	return render(
		React.createElement(
			"ul",
			null,
			React.createElement(HawkrAgentTypingRow, {
				motion: "SUBMITTING" as AgentMotionState,
				ariaLabel: "Hawkr Agent 正在整理",
				...props,
			}),
		),
	);
}

describe("HawkrAgentTypingRow", () => {
	it("有進度文字時顯示那一行字，不落回三點指示", () => {
		renderRow({ label: "正在查詢公開商務名單" });
		expect(screen.getByText("正在查詢公開商務名單")).toBeTruthy();
		expect(document.querySelector(".thread-page__typing")).toBeNull();
	});

	it("沒有進度文字時給中性三點指示，不編造階段（§V.10.6-2）", () => {
		renderRow();
		const dots = document.querySelector(".thread-page__typing");
		expect(dots).toBeTruthy();
		// 三點是裝飾，語意由整列的 aria-label 承擔。
		expect(dots?.getAttribute("aria-hidden")).toBe("true");
		expect(document.querySelector(".hawkr-agent-typing-row__label")).toBeNull();
	});

	it("空字串跟缺席一樣落到三點——不畫一條看不見的空行", () => {
		renderRow({ label: "" });
		expect(document.querySelector(".thread-page__typing")).toBeTruthy();
	});

	it("整列掛在既有的 hawkr-agent-typing-row 版位上，並帶呼叫端給的名字", () => {
		renderRow({
			label: "正在搜尋候選供應商",
			ariaLabel: "Hawkr Agent 正在找貨",
		});
		const row = document.querySelector("li.hawkr-agent-typing-row");
		expect(row).toBeTruthy();
		expect(row?.getAttribute("aria-label")).toBe("Hawkr Agent 正在找貨");
	});

	it("不是 live region——朗讀由 workspace 那顆常駐 sr-only status 統一負責", () => {
		// 這一列若自己帶 role=status，同一句話會被播兩次（軌本身 aria-hidden 的
		// 前提就是播報只有一個來源）。
		renderRow({ label: "正在查詢公開商務名單" });
		const row = document.querySelector("li.hawkr-agent-typing-row");
		expect(row?.getAttribute("role")).toBeNull();
		expect(row?.getAttribute("aria-live")).toBeNull();
	});

	it("標誌掛在列上，且旁邊已有等值文字時整顆視為裝飾", () => {
		renderRow({ label: "正在查詢公開商務名單", motion: "SEARCHING" });
		const sprite = document.querySelector(".hawkr-agent-sprite");
		// 先確認真的畫出來了，否則下面那條 aria 斷言會空過。
		expect(sprite).toBeTruthy();
		expect(sprite?.getAttribute("data-motion")).toBe("SEARCHING");
		// 處理中的狀態用標誌的載入動態。
		expect(sprite?.querySelector(".il-logo")).toBeTruthy();
		// 傳 label="" ＝ 整顆 aria-hidden，不跟旁邊那行字重複朗讀。
		expect(sprite?.getAttribute("aria-hidden")).toBe("true");
		expect(sprite?.getAttribute("role")).toBeNull();
	});
});
