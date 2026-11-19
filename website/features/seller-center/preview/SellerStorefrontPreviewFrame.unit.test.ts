import { Code, ConnectError } from "@connectrpc/connect";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ useSellerSelf: vi.fn() }));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: mocks.useSellerSelf,
}));
vi.mock("@/components/shared/TruckSpinner", () => ({ TruckMark: () => null }));

import SellerStorefrontPreviewFrame from "./SellerStorefrontPreviewFrame";

function html(): string {
	return renderToStaticMarkup(
		React.createElement(SellerStorefrontPreviewFrame, null, null),
	);
}

beforeEach(() => mocks.useSellerSelf.mockReset());

describe("preview frame without a loaded store", () => {
	it("shows the shared go-to-onboarding transition when the account has no store", () => {
		mocks.useSellerSelf.mockReturnValue({
			data: null,
			loading: false,
			error: new ConnectError("no store", Code.NotFound),
		});
		const out = html();
		expect(out).toContain("正在前往建立賣場頁…");
		expect(out).not.toContain("店面預覽暫時載入不到");
	});

	it("keeps the retry message for a genuine read failure", () => {
		mocks.useSellerSelf.mockReturnValue({
			data: null,
			loading: false,
			error: new ConnectError("down", Code.Unavailable),
		});
		expect(html()).toContain("店面預覽暫時載入不到，請稍後再試。");
	});
});
