import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { PaymentAccountSchema } from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentAccountStatus,
	RequirementCollection,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/**
 * 商家詳情的收款帳戶卡（PaymentAccountCard）實際渲染出來的 HTML。
 *
 * 守的是：「重試建立」只在有 create_failed_time 時出現；not_found 是「尚未建立」而不是讀取失敗；
 * 徽章與收件方式的文字來自共用文案目錄；待補條件原文放在折疊的 <pre>。寫法同 merchant-active-measures-card.test.ts。
 */

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

const { createElement } = requireFromTest("react") as {
	createElement: (type: unknown, props?: unknown) => unknown;
};
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const cardModule = requireFromTest(
	"../src/app/admin/merchants/[id]/PaymentAccountCard",
) as typeof import("../src/app/admin/merchants/[id]/PaymentAccountCard");

const accountModule = requireFromTest(
	"../src/app/admin/merchants/[id]/payment-account",
) as typeof import("../src/app/admin/merchants/[id]/payment-account");

type State = Parameters<typeof cardModule.PaymentAccountCard>[0]["state"];

function render(state: State): string {
	return renderToStaticMarkup(
		createElement(cardModule.PaymentAccountCard, {
			state,
			refreshing: false,
			retrying: false,
			onRefresh: () => {},
			onRetryCreate: () => {},
			onReload: () => {},
		}),
	);
}

function account(init: MessageInitShape<typeof PaymentAccountSchema> = {}) {
	return create(PaymentAccountSchema, {
		storeId: "57",
		provider: "stripe",
		externalAccountId: "acct_1UXVfaID6WkY4CJw",
		status: PaymentAccountStatus.ACTION_REQUIRED,
		requirementCollection: RequirementCollection.PROVIDER,
		statusReason: "requirements_past_due",
		fetchTime: timestampFromDate(new Date("2026-11-03T06:20:00Z")),
		...init,
	});
}

test("ready：顯示目錄的徽章與收件方式、外部帳戶、原因碼與上次讀取，沒有建立失敗時只有重新整理", () => {
	const html = render({ state: "ready", account: account() });
	assert.match(html, /id="payment-account"/);
	assert.match(html, /待設定/);
	assert.match(html, /金流商收件（託管頁面）/);
	assert.match(html, /acct_1UXVfaID6WkY4CJw/);
	assert.match(html, /requirements_past_due/);
	assert.match(html, /2026\/11\/03 14:20/);
	assert.match(html, /重新整理/);
	assert.doesNotMatch(html, /重試建立/);
	assert.doesNotMatch(html, /建立失敗於/);
});

test("有 create_failed_time：顯示建立失敗時間與重試建立", () => {
	const html = render({
		state: "ready",
		account: account({
			externalAccountId: "",
			status: PaymentAccountStatus.NOT_CREATED,
			createFailedTime: timestampFromDate(new Date("2026-11-03T06:20:00Z")),
		}),
	});
	assert.match(html, /建立失敗於 2026\/11\/03 14:20/);
	assert.match(html, /重試建立/);
	// 還沒有外部帳戶：RefreshPaymentAccount 會回 failed_precondition，不給重新整理。
	assert.doesNotMatch(html, /重新整理/);
	assert.match(html, /準備中/);
});

test("轉帳與撥款能力分別顯示", () => {
	const html = render({
		state: "ready",
		account: account({
			status: PaymentAccountStatus.ENABLED,
			transfersActive: true,
			payoutsActive: false,
		}),
	});
	assert.match(html, /轉帳能力<\/div><div class="field-value">已啟用/);
	assert.match(html, /撥款能力<\/div><div class="field-value">未啟用/);
});

test("待補條件原文放在折疊的 pre；空物件不畫", () => {
	const withReq = render({
		state: "ready",
		account: account({ requirements: { entries: [{ code: "x" }] } }),
	});
	assert.match(
		withReq,
		/<details[^>]*>[\s\S]*待補條件（原始內容）[\s\S]*<pre[^>]*>/,
	);
	assert.match(withReq, /&quot;entries&quot;/);
	const empty = render({
		state: "ready",
		account: account({ requirements: {} }),
	});
	assert.doesNotMatch(empty, /待補條件/);
});

test("paymentAccountOf：not_found 是尚未建立，其他錯誤是讀取失敗", () => {
	const notFound = accountModule.paymentAccountOf({
		error: new ConnectError("no", Code.NotFound),
		data: undefined,
	});
	assert.deepEqual(notFound, { state: "notFound" });
	assert.match(render(notFound), /尚未建立（平台審核核准後自動建立）/);
	assert.doesNotMatch(render(notFound), /讀取失敗/);

	const failed = accountModule.paymentAccountOf({
		error: new ConnectError("boom", Code.Unavailable),
		data: undefined,
	});
	assert.equal(failed.state, "error");
	assert.match(render(failed), /收款帳戶讀取失敗/);
	assert.deepEqual(
		accountModule.paymentAccountOf({ error: null, data: undefined }),
		{ state: "loading" },
	);
});

test("paymentAccountOf：規則同 payUniOf（有錯誤就是錯誤，成功卻沒有帳戶是讀取錯誤）；錯誤畫面帶伺服器的錯誤文字", () => {
	assert.equal(
		accountModule.paymentAccountOf({
			error: new ConnectError("boom", Code.Unavailable),
			data: { account: account() },
		}).state,
		"error",
	);
	assert.deepEqual(accountModule.paymentAccountOf({ error: null, data: {} }), {
		state: "error",
	});
	const denied = render(
		accountModule.paymentAccountOf({
			error: new ConnectError("no", Code.PermissionDenied),
			data: undefined,
		}),
	);
	assert.match(denied, /收款帳戶讀取失敗：/);
});

test("rereadFinished：fetch_time 與按下時不同才算完成", () => {
	const t0 = timestampFromDate(new Date("2026-11-03T06:20:00Z"));
	const t1 = timestampFromDate(new Date("2026-11-03T06:21:00Z"));
	const base = accountModule.fetchTimeMs(account({ fetchTime: t0 }));
	assert.equal(
		accountModule.rereadFinished(base, account({ fetchTime: t0 })),
		false,
	);
	assert.equal(
		accountModule.rereadFinished(base, account({ fetchTime: t1 })),
		true,
	);
	assert.equal(accountModule.rereadFinished(base, undefined), false);
	assert.equal(
		accountModule.rereadFinished(undefined, account({ fetchTime: t1 })),
		true,
	);
});

test("接線：頁面的 invalidateKyc 一併重讀收款帳戶；收款帳戶的資料來自 use-store-detail 的 hook", async () => {
	const { readFile } = await import("node:fs/promises");
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	const body = page.slice(page.indexOf("const invalidateKyc"));
	const fn = body.slice(0, body.indexOf("const approveMutation"));
	assert.match(fn, /PaymentAdminService\.method\.getPaymentAccount/u);
	assert.match(page, /usePaymentAccount\(/u);
	const card = await readFile(
		"src/app/admin/merchants/[id]/PaymentAccountCard.tsx",
		"utf8",
	);
	assert.doesNotMatch(card, /useConnectQuery|useToast/u);
});
