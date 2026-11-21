import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/Link", () => ({
	default: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) =>
		React.createElement("a", props),
}));

import SellerStorefrontPreviewBanner from "./SellerStorefrontPreviewBanner";

const PUBLIC_TEXT =
	"以下是買家看到的店面樣子。預覽期間不會送出訂閱、聊天、收藏、按讚、留言、分享或購買操作。";
const NOT_PUBLIC_TEXT =
	"你的店家還沒公開，買家目前看不到這個店面。以下是公開後買家會看到的樣子。";

function html(canBePublic: boolean): string {
	return renderToStaticMarkup(
		React.createElement(SellerStorefrontPreviewBanner, { canBePublic }),
	);
}

describe("preview banner copy", () => {
	it("keeps the existing notice when the store can be public", () => {
		const out = html(true);
		expect(out).toContain(PUBLIC_TEXT);
		expect(out).not.toContain(NOT_PUBLIC_TEXT);
	});

	it("says buyers cannot see the storefront yet when it cannot be public", () => {
		const out = html(false);
		expect(out).toContain(NOT_PUBLIC_TEXT);
		expect(out).not.toContain(PUBLIC_TEXT);
	});
});
