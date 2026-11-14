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
	retryPurge?: () => ReturnType<typeof account> | ConnectError;
	deleteMapping?: () => ConnectError | undefined;
}) {
	const calls = {
		get: 0,
		refresh: 0,
		retry: 0,
		retryPurge: 0,
		deleteMapping: 0,
	};
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
			retryPaymentAccountPurge: () => {
				calls.retryPurge += 1;
				const r = handlers.retryPurge?.();
				if (r instanceof ConnectError) throw r;
				return { account: r };
			},
			deletePaymentAccountMapping: () => {
				calls.deleteMapping += 1;
				const r = handlers.deleteMapping?.();
				if (r) throw r;
				return {};
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

const blocked = () =>
	account({
		purgeBlockedTime: at(2),
		purgeBlockedReason: "provider_refused：no",
	});

test("重試關閉：回應的帳戶直接寫進快取（不再讀一次）、待辦重讀、notice 成功", async () => {
	const { calls, notices, view, queueInvalidated } = setup({
		get: blocked,
		retryPurge: () => account({ statusReason: "after_retry_purge" }),
	});
	await view.render();
	assert.match(bodyText(), /收款帳戶無法關閉/);
	await click("重試關閉");
	await until(() => bodyText().includes("after_retry_purge"), "快取已寫入");
	assert.equal(calls.retryPurge, 1);
	assert.equal(calls.get, 1);
	assert.doesNotMatch(bodyText(), /收款帳戶無法關閉/);
	assert.deepEqual(notices, [
		null,
		{ tone: "success", text: sectionModule.PURGE_RETRY_QUEUED_TEXT },
	]);
	assert.equal(queueInvalidated(), true);
});

test("已在金流商處理，刪除對應：先確認、確認前不送出；確認後送出、待辦與帳戶重讀、notice 成功", async () => {
	const { calls, notices, view, queueInvalidated } = setup({
		get: (n) => (n === 1 ? blocked() : new ConnectError("gone", Code.NotFound)),
	});
	await view.render();
	await click("已在金流商處理，刪除對應");
	assert.match(bodyText(), /確定已在金流商處理這個收款帳戶嗎？/);
	assert.match(
		bodyText(),
		/這個動作不會呼叫金流商：只刪除平台這邊的帳戶對應，並記下是誰、什麼時候刪的。刪除之後平台找不回外部帳戶，請先確認金流商那邊已經關閉或處理。/,
	);
	assert.equal(calls.deleteMapping, 0, "確認前不得送出");
	await click("返回");
	assert.equal(calls.deleteMapping, 0);
	assert.doesNotMatch(bodyText(), /確定已在金流商處理/);
	await click("已在金流商處理，刪除對應");
	await click("刪除對應");
	await until(() => bodyText().includes("尚未建立"), "帳戶重讀後是尚未建立");
	assert.equal(calls.deleteMapping, 1);
	assert.equal(calls.get, 2);
	assert.deepEqual(notices, [
		{ tone: "success", text: sectionModule.MAPPING_DELETED_TEXT },
	]);
	assert.equal(queueInvalidated(), true);
});

test("刪除對應失敗：對話框留著並就地顯示原因，不宣告成功", async () => {
	const { calls, notices, view } = setup({
		get: blocked,
		deleteMapping: () => new ConnectError("denied", Code.PermissionDenied),
	});
	await view.render();
	await click("已在金流商處理，刪除對應");
	await click("刪除對應");
	await until(
		() =>
			bodyText().includes("確定已在金流商處理這個收款帳戶嗎？") &&
			calls.deleteMapping === 1,
		"送出一次",
	);
	assert.deepEqual(notices, []);
	assert.match(bodyText(), /確定已在金流商處理這個收款帳戶嗎？/);
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
