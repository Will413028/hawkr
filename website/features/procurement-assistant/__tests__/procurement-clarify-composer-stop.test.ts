/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { ProcurementClarifyComposer } from "../ProcurementClarifyChat";
import { RUN_CANCEL_COPY } from "../runtime-snapshot-contract";
import type { RunCancelState } from "../useProcurementSession";

/**
 * 「停止處理」那顆按鈕。
 *
 * 這一顆按鈕在後端補上取消能力之前是刻意接不上的 placeholder（`disabled` ＋
 * 文案「規劃中」），因為當時沒有任何執行路徑會讀取消命令。後端把取消觀察器
 * 接上 turn pipeline、並把 `runVersion` 放進 runtime snapshot 之後，這裡鎖住
 * 兩件事：
 *
 *   1. **它真的接上了**——有 `onClick`、拿得到 token 時按得下去。把它退回
 *      placeholder 的實作會在這裡紅。
 *   2. **拿不到 token 時仍然按不下去**——不送出一個必然被 400 擋下的請求，
 *      而且按鈕不會消失（消失會讓人以為自己漏看了）。
 *
 * 第三件是文案：四個 disposition 各講各的話，三個終態的句子都不得讓買家以為
 * 自己的停止把一個已完成的結果作廢了。
 *
 * 本 repo 沒有 `@testing-library/jest-dom`，所以斷言直接讀 DOM 屬性，不用
 * `toBeDisabled()` 這類 matcher。
 */

class NoopResizeObserver {
	observe() {}
	unobserve() {}
	disconnect() {}
}

beforeAll(() => {
	vi.stubGlobal("ResizeObserver", NoopResizeObserver);
	// composer 的 scroll hook 會問 prefers-reduced-motion；jsdom 沒有這支 API。
	vi.stubGlobal("matchMedia", (query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addEventListener: () => {},
		removeEventListener: () => {},
		addListener: () => {},
		removeListener: () => {},
		dispatchEvent: () => false,
	}));
});

afterEach(cleanup);

function renderComposer(
	overrides: {
		pending?: boolean;
		canStop?: boolean;
		stopState?: RunCancelState;
		onStop?: () => void;
	} = {},
) {
	const onStop = overrides.onStop ?? vi.fn();
	render(
		React.createElement(ProcurementClarifyComposer, {
			pending: overrides.pending ?? true,
			value: "",
			onValueChange: () => {},
			onSubmit: () => {},
			canStop: overrides.canStop ?? true,
			stopState: overrides.stopState ?? { status: "idle" },
			onStop,
		}),
	);
	return { onStop };
}

/**
 * 用 testid 而不是可及名稱：命令在途時按鈕進 loading 態，可及名稱會換成
 * loadingLabel，用名稱查會在那一個 case 上假紅。
 */
function stopButton(): HTMLButtonElement {
	return screen.getByTestId("agent-stop-run") as HTMLButtonElement;
}

/** SendButton 在 loading 時也會掛 role="status"，所以用 testid 指名這一則。 */
function notice(): string {
	return screen.getByTestId("agent-stop-status").textContent ?? "";
}

describe("composer 的停止鈕", () => {
	it("拿得到這一輪的 run 與版本時按得下去，並呼叫 onStop", () => {
		const { onStop } = renderComposer();
		const button = stopButton();
		expect(button.disabled).toBe(false);
		fireEvent.click(button);
		expect(onStop).toHaveBeenCalledTimes(1);
	});

	it("文案不再標「規劃中」——能力已經上線，標成未上線就是另一種誤導", () => {
		renderComposer();
		expect(stopButton().textContent ?? "").toContain("停止處理");
		expect(stopButton().textContent ?? "").not.toContain("規劃中");
	});

	it("拿不到 run 或版本時看得到但按不下去，且說明為什麼", () => {
		const { onStop } = renderComposer({ canStop: false });
		const button = stopButton();
		expect(button).toBeTruthy();
		expect(button.disabled).toBe(true);
		fireEvent.click(button);
		expect(onStop).not.toHaveBeenCalled();
		expect(
			screen.getByText(/目前無法確認這一輪的執行狀態，暫時不能停止/),
		).toBeTruthy();
	});

	it("不在執行中就沒有停止鈕", () => {
		renderComposer({ pending: false });
		expect(screen.queryByTestId("agent-stop-run")).toBeNull();
	});

	it("命令在途時不得重複送出", () => {
		const { onStop } = renderComposer({ stopState: { status: "requesting" } });
		const button = stopButton();
		expect(button.disabled).toBe(true);
		fireEvent.click(button);
		expect(onStop).not.toHaveBeenCalled();
	});

	it("REQUESTED 講「正在停止」，按鈕收起來不再接受第二次", () => {
		renderComposer({
			stopState: { status: "settled", disposition: "REQUESTED" },
		});
		expect(notice()).toBe(RUN_CANCEL_COPY.REQUESTED);
		expect(stopButton().disabled).toBe(true);
	});

	it("另外三個 disposition 都講「這一輪已經到終點」，不宣稱結果被作廢", () => {
		for (const disposition of [
			"ALREADY_CANCELLED",
			"ALREADY_TERMINAL",
			"TOO_LATE",
		] as const) {
			cleanup();
			renderComposer({ stopState: { status: "settled", disposition } });
			const text = notice();
			expect(text).toBe(RUN_CANCEL_COPY[disposition]);
			expect(text).not.toContain("正在停止");
			expect(text).not.toContain("已為您取消");
			expect(stopButton().disabled).toBe(true);
		}
	});

	it("CAS 版本過期且換不到新 token：要買家重新整理確認，不謊報已取消", () => {
		renderComposer({ stopState: { status: "stale" } });
		const text = notice();
		expect(text).toContain("請重新整理確認目前結果");
		expect(text).not.toContain("已停止");
		expect(stopButton().disabled).toBe(true);
	});

	it("命令沒有成立（非 CAS 的錯誤）時可以重試", () => {
		const { onStop } = renderComposer({
			stopState: { status: "error", detail: "停止沒有送出，請重試。" },
		});
		expect(notice()).toBe("停止沒有送出，請重試。");
		const button = stopButton();
		expect(button.disabled).toBe(false);
		fireEvent.click(button);
		expect(onStop).toHaveBeenCalledTimes(1);
	});
});
