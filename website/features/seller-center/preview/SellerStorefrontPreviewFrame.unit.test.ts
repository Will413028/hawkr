import { Code, ConnectError } from "@connectrpc/connect";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ useSellerSelf: vi.fn() }));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: mocks.useSellerSelf,
}));
vi.mock("@/components/shared/TruckSpinner", () => ({ TruckMark: () => null }));
vi.mock("@/features/seller/components/SellerPageShell", () => ({
	default: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock("@/features/seller/components/SellerProfileV2", () => ({
	default: () => null,
}));
vi.mock("@/features/seller/components/AppDownloadBannerV2", () => ({
	default: ({ deepLink }: { deepLink: string }) =>
		React.createElement("i", { "data-deep-link": deepLink }),
}));
vi.mock("./SellerStorefrontPreviewBanner", () => ({ default: () => null }));
vi.mock("./StorefrontPreviewBoundary", () => ({
	default: ({ children }: { children: React.ReactNode }) => children,
}));

import SellerStorefrontPreviewFrame from "./SellerStorefrontPreviewFrame";

function html(): string {
	return renderToStaticMarkup(
		React.createElement(SellerStorefrontPreviewFrame, null, null),
	);
}

function loaded(storeCode: string) {
	mocks.useSellerSelf.mockReturnValue({
		data: {
			store: { id: "700", storeCode, storeName: "S" },
			stats: {},
			capabilities: [],
		},
		loading: false,
		error: null,
	});
}

beforeEach(() => {
	mocks.useSellerSelf.mockReset();
});

describe("preview frame", () => {
	it("keeps the retry message for a genuine read failure", () => {
		mocks.useSellerSelf.mockReturnValue({
			data: null,
			loading: false,
			error: new ConnectError("down", Code.Unavailable),
		});
		expect(html()).toContain("店面預覽暫時載入不到，請稍後再試。");
	});

	it("links the app banner by store code", () => {
		loaded("123456");
		expect(html()).toContain('data-deep-link="hawkr://seller/123456"');
	});

	it("omits the app banner when the store has no code", () => {
		loaded("");
		expect(html()).not.toContain("data-deep-link");
	});
});
