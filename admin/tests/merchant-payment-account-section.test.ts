import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { createConnectQueryKey } from "@connectrpc/connect-query";
import {
	PaymentAccountSchema,
	PaymentAdminService,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import { ReviewQueueAdminService } from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PaymentAccountStatus } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	bodyText,
	click,
	mount,
	React,
	requireFromTest,
	until,
} from "./fixtures/dom-harness";

/**
 * 收款帳戶區塊（PaymentAccountSection）掛進 DOM 後的行為：結果走頁面的 notice、重新整理與重試建立的回應直接寫進快取（不再讀一次）、
 * 成功後待辦列表與開通任務重讀、讀取失敗與 not_found 的畫面。
 */

const h = React.createElement;
const sectionModule = requireFromTest(
	"../src/app/admin/merchants/[id]/PaymentAccountCard",
) as typeof import("../src/app/admin/merchants/[id]/PaymentAccountCard");
const hooks = requireFromTest(
	"../src/app/admin/merchants/[id]/use-store-detail",
) as typeof import("../src/app/admin/merchants/[id]/use-store-detail");

const at = (minute: number) =>
	timestampFromDate(new Date(Date.UTC(2026, 10, 3, 6, minute)));
function account(
	init: Parameters<typeof create<typeof PaymentAccountSchema>>[1],
) {
	return create(PaymentAccountSchema, {
		storeId: "57",
		provider: "stripe",
		externalAccountId: "acct_1",
		status: PaymentAccountStatus.ACTION_REQUIRED,
		fetchTime: at(0),
		...init,
	});
}

type Notice = { tone: string; text: string | readonly string[] } | null;
const queueKey = createConnectQueryKey({
	schema: ReviewQueueAdminService.method.listReviewQueue,
	cardinality: undefined,
});

function Host({ notices }: { notices: Notice[] }) {
	const payment = hooks.usePaymentAccount("57");
	return h(sectionModule.PaymentAccountSection, {
		storeId: "57",
		payment,
		onNotice: (n: Notice) => notices.push(n),
	});
}

function setup(handlers: {
	get: (call: number) => ReturnType<typeof account> | ConnectError;
	refresh?: () => ReturnType<typeof account> | ConnectError;
	retry?: () => ReturnType<typeof account>;
}) {
	const calls = { get: 0, refresh: 0, retry: 0 };
	const transport = createRouterTransport(({ service }) => {
		service(PaymentAdminService, {
			getPaymentAccount: () => {
				calls.get += 1;
				const r = handlers.get(calls.get);
				if (r instanceof ConnectError) throw r;
				return { account: r };
			},
			refreshPaymentAccount: () => {
				calls.refresh += 1;
				const r = handlers.refresh?.();
				if (r instanceof ConnectError) throw r;
				return { account: r };
			},
			retryPaymentAccountCreate: () => {
				calls.retry += 1;
				return { account: handlers.retry?.() };
			},
		});
	});
	const notices: Notice[] = [];
	const view = mount(h(Host, { notices }), transport);
	view.queryClient.setQueryData(queueKey, { items: [] });
	const queueInvalidated = () =>
		view.queryClient.getQueryState(queueKey)?.isInvalidated === true;
	return { calls, notices, view, queueInvalidated };
}

test("重新整理：回應的帳戶直接寫進快取（不再讀一次）、notice 先清空再成功、待辦與開通任務重讀", async () => {
	const onboardingKey = createConnectQueryKey({
		schema: SellerAdminService.method.getStoreOnboarding,
		cardinality: undefined,
	});
	const { calls, notices, view, queueInvalidated } = setup({
		get: () => account({ statusReason: "before_refresh" }),
		refresh: () => account({ statusReason: "after_refresh", fetchTime: at(5) }),
	});
	view.queryClient.setQueryData(onboardingKey, { tasks: [], complete: false });
	await view.render();
	assert.match(bodyText(), /before_refresh/);
	await click("重新整理");
	await until(() => bodyText().includes("after_refresh"), "快取已寫入");
	assert.equal(calls.refresh, 1);
	assert.equal(calls.get, 1);
	assert.doesNotMatch(bodyText(), /before_refresh/);
	assert.deepEqual(notices, [
		null,
		{ tone: "success", text: sectionModule.REFRESHED_TEXT },
	]);
	assert.equal(queueInvalidated(), true);
	assert.equal(
		view.queryClient.getQueryState(onboardingKey)?.isInvalidated,
		true,
	);
});

test("重新整理失敗：notice 是錯誤、畫面不變、待辦與開通任務不重讀", async () => {
	const onboardingKey = createConnectQueryKey({
		schema: SellerAdminService.method.getStoreOnboarding,
		cardinality: undefined,
	});
	const { calls, notices, view, queueInvalidated } = setup({
		get: () => account({ statusReason: "before_refresh" }),
		refresh: () => new ConnectError("down", Code.Unavailable),
	});
	view.queryClient.setQueryData(onboardingKey, { tasks: [], complete: false });
	await view.render();
	await click("重新整理");
	await until(
		() => notices.some((n) => n?.tone === "error"),
		"重新整理失敗的結果已顯示",
	);
	assert.equal(notices[0], null);
	assert.equal(calls.get, 1);
	assert.match(bodyText(), /before_refresh/);
	assert.equal(queueInvalidated(), false);
	assert.equal(
		view.queryClient.getQueryState(onboardingKey)?.isInvalidated,
		false,
	);
});

test("重試建立：回應的帳戶直接寫進快取（不再讀一次）、待辦重讀、notice 成功", async () => {
	const { calls, notices, view, queueInvalidated } = setup({
		get: () =>
			account({
				externalAccountId: "",
				status: PaymentAccountStatus.NOT_CREATED,
				createFailedTime: at(1),
			}),
		retry: () =>
			account({
				externalAccountId: "acct_new",
				status: PaymentAccountStatus.UNDER_REVIEW,
			}),
	});
	await view.render();
	assert.match(bodyText(), /建立失敗於/);
	await click("重試建立");
	await until(() => bodyText().includes("acct_new"), "快取已寫入");
	assert.equal(calls.retry, 1);
	assert.equal(calls.get, 1);
	assert.doesNotMatch(bodyText(), /重試建立/);
	assert.deepEqual(notices, [
		null,
		{ tone: "success", text: sectionModule.RETRY_QUEUED_TEXT },
	]);
	assert.equal(queueInvalidated(), true);
});

test("讀取：not_found 是尚未建立；其他錯誤是讀取失敗，重試會再讀", async () => {
	const notFound = setup({
		get: () => new ConnectError("no", Code.NotFound),
	});
	await notFound.view.render();
	assert.match(bodyText(), /尚未建立/);
	assert.doesNotMatch(bodyText(), /讀取失敗/);
	await notFound.view.unmount();

	const failing = setup({
		get: (n) =>
			n === 1 ? new ConnectError("x", Code.Unavailable) : account({}),
	});
	await failing.view.render();
	assert.match(bodyText(), /收款帳戶讀取失敗/);
	await click("重試");
	await until(() => bodyText().includes("acct_1"), "重試後顯示帳戶");
	assert.equal(failing.calls.get, 2);
});

test("開通任務：重試建立成功讓開通任務查詢重讀", async () => {
	const onboardingKey = createConnectQueryKey({
		schema: SellerAdminService.method.getStoreOnboarding,
		cardinality: undefined,
	});
	const { view } = setup({
		get: () =>
			account({
				externalAccountId: "",
				status: PaymentAccountStatus.NOT_CREATED,
				createFailedTime: at(1),
			}),
		retry: () => account({ externalAccountId: "acct_new" }),
	});
	view.queryClient.setQueryData(onboardingKey, { tasks: [], complete: false });
	await view.render();
	await click("重試建立");
	await until(() => bodyText().includes("acct_new"), "快取已寫入");
	assert.equal(
		view.queryClient.getQueryState(onboardingKey)?.isInvalidated,
		true,
	);
});
