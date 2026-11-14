import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { renderToStaticMarkup } from "react-dom/server";
import SellerAppPaymentReturnPage from "../app/[locale]/(public)/seller-app/payment-return/page";
import { returnStateOf } from "../app/[locale]/(public)/seller-app/payment-return/ReturnHeading";
import { requiredAccessForPath } from "../lib/auth/redirect";
import { surfaceOfPath } from "../lib/intl/paths";
import { siteMiddleware } from "../middleware";

// 賣家 App 在外部瀏覽器開金流商的連結，完成後回到這一頁：瀏覽器沒有 hawkr 的登入。
const PATHS = [
	"/seller-app/payment-return?state=done",
	"/seller-app/payment-return?state=expired",
	"/en/seller-app/payment-return?state=done",
];

test("the seller app return page is not under any protected prefix", () => {
	for (const path of PATHS) {
		const pathname = path.split("?")[0] as string;
		assert.equal(requiredAccessForPath(pathname), null, path);
	}
	assert.equal(surfaceOfPath("/seller-app/payment-return"), "seller");
});

test("an unauthenticated request is not redirected to login", async () => {
	for (const path of PATHS) {
		const response = await siteMiddleware(
			new NextRequest(`http://localhost:3000${path}`),
		);
		assert.equal(response.headers.get("location"), null, path);
	}
});

async function render(locale: string): Promise<string> {
	return renderToStaticMarkup(
		await SellerAppPaymentReturnPage({ params: Promise.resolve({ locale }) }),
	);
}

test("the page tells the seller to go back to the seller app, in both locales", async () => {
	assert.match(await render("zh-TW"), /請回到 hawkr 賣家 App/);
	assert.match(await render("en"), /return to the hawkr Seller app/);
});

test("the heading reads the state: expired is different, anything else is done", () => {
	assert.equal(returnStateOf("expired"), "expired");
	assert.equal(returnStateOf("done"), "done");
	assert.equal(returnStateOf("nonsense"), "done");
	assert.equal(returnStateOf(null), "done");
});
