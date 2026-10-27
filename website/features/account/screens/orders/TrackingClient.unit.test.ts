/** @vitest-environment jsdom */

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { LogisticsTrack } from "@/lib/logistics/repo";

const queryLogistics = vi.fn<(orderNo: string) => Promise<LogisticsTrack>>();

vi.mock("@/lib/logistics/repo", () => ({
	queryLogistics: (orderNo: string) => queryLogistics(orderNo),
}));
vi.mock("@/lib/hooks/useOrders", () => ({
	useOrder: () => ({ order: null }),
}));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children?: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
// 空狀態的插圖換成一個純標記：本測試問的是「顯不顯示空狀態」，不是插圖長什麼樣。
vi.mock("@/features/account/components/AccountEmptyIcon", () => ({
	AccountEmptyStateIcon: () => React.createElement("span"),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({ children }: { children?: React.ReactNode }) =>
		React.createElement("button", { type: "button" }, children),
}));

const { default: TrackingClient } = await import("./TrackingClient");

/**
 * 沒有託運單號 ⇒ 一律是空狀態。即時配送（3003，Uber）已刪除
 * （ADR 2026-10-27-general-ecommerce-scope-cut D4），沒有「無單號但顯示狀態」的例外。
 */
describe("TrackingClient 對沒有託運單號的訂單", () => {
	afterEach(() => {
		cleanup();
		queryLogistics.mockReset();
	});

	function track(overrides: Partial<LogisticsTrack>): LogisticsTrack {
		return {
			orderNo: "O2026091200001",
			shipTradeNo: "",
			shippingMethod: "",
			status: "",
			statusLabel: "",
			...overrides,
		};
	}

	for (const shippingMethod of ["HOME_DELIVERY", "3003"]) {
		it(`${shippingMethod} 沒有託運單號但有狀態 ⇒ 空狀態，沒有即時配送區塊`, async () => {
			queryLogistics.mockResolvedValue(
				track({ shippingMethod, status: "30", statusLabel: "配送中" }),
			);
			render(
				React.createElement(TrackingClient, { orderNo: "O2026091200001" }),
			);

			await waitFor(() => {
				expect(screen.queryByText(/尚無物流追蹤資訊/)).not.toBeNull();
			});
			expect(screen.queryByText(/即時配送/)).toBeNull();
		});
	}

	/**
	 * W6-5 的 forbidden 格。
	 *
	 * 🔴 舊行為把**所有**失敗壓成同一句「請稍後再試」＋一顆重試鍵，包含 403。
	 * 對無權查看的人來說，那顆鍵永遠不會亮——他會一直按、一直看到同一句話。
	 * 「不外露 HTTP 碼」是對的，但**可行動性**必須外露，那是兩件事。
	 */
	describe("失敗的可行動性", () => {
		function apiError(status: number) {
			return Object.assign(new Error("boom"), { status });
		}

		it("[關鍵] 403 ⇒ 說清楚是權限問題，且不給無效的重試鍵", async () => {
			queryLogistics.mockRejectedValue(apiError(403));
			render(
				React.createElement(TrackingClient, { orderNo: "O2026091200001" }),
			);

			await waitFor(() => {
				expect(screen.queryByTestId("tracking-forbidden")).not.toBeNull();
			});
			expect(screen.queryByText("重試")).toBeNull();
			// 原始 HTTP 碼不得外露。
			expect(screen.queryByText(/403/)).toBeNull();
		});

		it("[陽性對照] 暫時性失敗（500）⇒ 仍然給重試鍵", async () => {
			// 沒有這一條，一個「永遠不給重試鍵」的實作會讓上面那條通過。
			queryLogistics.mockRejectedValue(apiError(500));
			render(
				React.createElement(TrackingClient, { orderNo: "O2026091200001" }),
			);

			await waitFor(() => {
				expect(screen.queryByTestId("tracking-error")).not.toBeNull();
			});
			expect(screen.queryByText("重試")).not.toBeNull();
		});
	});
});
