/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import Pagination from "@/components/shared/Pagination";
import type { PublicStoreView } from "@/lib/seller/public-store";

vi.mock("./StorefrontProductPreviewDrawer", () => ({ default: () => null }));

import { PreviewStoreProvider } from "./PreviewStoreContext";
import StorefrontPreviewBoundary from "./StorefrontPreviewBoundary";

const view = { storeId: "700", storeCode: "123456" } as PublicStoreView;

function inBoundary(children: React.ReactNode) {
	return render(
		React.createElement(
			PreviewStoreProvider,
			{ value: { view, canBePublic: true } },
			React.createElement(StorefrontPreviewBoundary, null, children),
		),
	);
}

afterEach(cleanup);

describe("StorefrontPreviewBoundary", () => {
	it("lets the load-more control run and still cancels write controls", () => {
		const loadMore = vi.fn();
		const subscribe = vi.fn();
		inBoundary(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(
					"div",
					{ "data-storefront-preview-reading-control": "true" },
					React.createElement(
						"button",
						{ type: "button", onClick: loadMore },
						"載入更多動態",
					),
				),
				React.createElement(
					"button",
					{ type: "button", onClick: subscribe },
					"訂閱",
				),
			),
		);
		fireEvent.click(screen.getByText("載入更多動態"));
		fireEvent.click(screen.getByText("訂閱"));
		expect(loadMore).toHaveBeenCalledTimes(1);
		expect(subscribe).not.toHaveBeenCalled();
	});

	it("lets pagination buttons change the page", () => {
		const onPageChange = vi.fn();
		inBoundary(
			React.createElement(Pagination, {
				current: 1,
				total: 3,
				onPageChange,
			}),
		);
		fireEvent.click(screen.getByLabelText("第 2 頁"));
		fireEvent.click(screen.getByText("下一頁"));
		expect(onPageChange).toHaveBeenNthCalledWith(1, 2);
		expect(onPageChange).toHaveBeenNthCalledWith(2, 2);
	});
});
