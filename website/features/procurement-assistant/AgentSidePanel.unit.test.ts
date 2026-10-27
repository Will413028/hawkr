/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import AgentSidePanel from "./AgentSidePanel";

afterEach(cleanup);

function ResultDraft() {
	const [draft, setDraft] = React.useState("");
	return React.createElement("input", {
		"aria-label": "詢價備註",
		value: draft,
		onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
			setDraft(event.target.value),
	});
}

describe("AgentSidePanel", () => {
	it("只呈現採購成果，收起再打開仍保留草稿", () => {
		const onClose = vi.fn();
		const panel = (hidden: boolean) =>
			React.createElement(
				AgentSidePanel,
				{
					hidden,
					onClose,
				},
				React.createElement(ResultDraft),
			);
		const { rerender } = render(panel(false));
		expect(
			screen.getByRole("complementary", { name: "採購成果" }),
		).toBeTruthy();
		expect(screen.queryByRole("tab")).toBeNull();
		fireEvent.change(screen.getByLabelText("詢價備註"), {
			target: { value: "分兩批配送" },
		});
		rerender(panel(true));
		expect(screen.queryByRole("complementary")).toBeNull();
		rerender(panel(false));
		expect((screen.getByLabelText("詢價備註") as HTMLInputElement).value).toBe(
			"分兩批配送",
		);
	});

	it("回到對話通知外層處理焦點", () => {
		const onClose = vi.fn();
		render(React.createElement(AgentSidePanel, { onClose }, "候選資料"));
		fireEvent.click(screen.getByRole("button", { name: "回到對話" }));
		expect(onClose).toHaveBeenCalledTimes(1);
	});
});
