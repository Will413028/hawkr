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
 * 收款帳戶區塊（PaymentAccountSection）掛進 DOM 後的行為：結果走頁面的 notice、重新整理後依 fetch_time 重讀（有上限、卸載即停）、
 * 重試建立的回應直接寫進快取、成功後待辦列表重讀、讀取失敗與 not_found 的畫面。
 */

const h = React.createElement;
const { useState } = requireFromTest("react") as typeof import("react");
const sectionModule = requireFromTest(
	"../src/app/admin/merchants/[id]/PaymentAccountCard",
) as typeof import("../src/app/admin/merchants/[id]/PaymentAccountCard");
const hooks = requireFromTest(
	"../src/app/admin/merchants/[id]/use-store-detail",
) as typeof import("../src/app/admin/merchants/[id]/use-store-detail");
const accountModule = requireFromTest(
	"../src/app/admin/merchants/[id]/payment-account",
) as typeof import("../src/app/admin/merchants/[id]/payment-account");

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

function Host({ notices, rereadMs }: { notices: Notice[]; rereadMs: number }) {
	const payment = hooks.usePaymentAccount("57");
	return h(sectionModule.PaymentAccountSection, {
		storeId: "57",
		payment,
		onNotice: (n: Notice) => notices.push(n),
		rereadMs,
	});
}

/** 與商家詳情頁相同的掛法：查詢在頁面層，區塊可以先卸載（切換分頁）而查詢還在。 */
function PageHost({
	notices,
	rereadMs,
}: {
	notices: Notice[];
	rereadMs: number;
}) {
	const payment = hooks.usePaymentAccount("57");
	const [shown, setShown] = useState(true);
	if (!shown) return null;
	return h(
		"div",
		null,
		h("button", { type: "button", onClick: () => setShown(false) }, "切換分頁"),
		h(sectionModule.PaymentAccountSection, {
			storeId: "57",
			payment,
			onNotice: (n: Notice) => notices.push(n),
			rereadMs,
		}),
	);
}

function setup(
	handlers: {
		get: (call: number) => ReturnType<typeof account> | ConnectError;
		/** 讓第 call 次 GetPaymentAccount 卡住直到回傳的 promise 完成。 */
		getGate?: (call: number) => Promise<void> | undefined;
		refresh?: () => undefined | ConnectError;
		/** 讓 RefreshPaymentAccount 卡住直到這個 promise 完成。 */
		refreshGate?: Promise<void>;
		retry?: () => ReturnType<typeof account>;
	},
	rereadMs = 5,
	host: typeof Host = Host,
) {
	const calls = { get: 0, refresh: 0, retry: 0 };
	const transport = createRouterTransport(({ service }) => {
		service(PaymentAdminService, {
			getPaymentAccount: async () => {
				calls.get += 1;
				const call = calls.get;
				await handlers.getGate?.(call);
				const r = handlers.get(call);
				if (r instanceof ConnectError) throw r;
				return { account: r };
			},
			refreshPaymentAccount: async () => {
				calls.refresh += 1;
				await handlers.refreshGate;
				const r = handlers.refresh?.();
				if (r instanceof ConnectError) throw r;
				return {};
			},
			retryPaymentAccountCreate: () => {
				calls.retry += 1;
				return { account: handlers.retry?.() };
			},
		});
	});
	const notices: Notice[] = [];
	const view = mount(h(host, { notices, rereadMs }), transport);
	view.queryClient.setQueryData(queueKey, { items: [] });
	const queueInvalidated = () =>
		view.queryClient.getQueryState(queueKey)?.isInvalidated === true;
	return { calls, notices, view, queueInvalidated };
}

const sleep = (ms: number) =>
	React.act(async () => {
		await new Promise((resolve) => setTimeout(resolve, ms));
	});

test("重新整理：結果走頁面 notice（先清空再顯示成功）、待辦重讀，每隔一段時間重讀到 fetch_time 變了就停", async () => {
	const { calls, notices, view, queueInvalidated } = setup({
		get: (n) => account({ fetchTime: n >= 3 ? at(5) : at(0) }),
	});
	await view.render();
	assert.equal(calls.get, 1);
	await click("重新整理");
	assert.equal(calls.refresh, 1);
	assert.deepEqual(notices, [
		null,
		{ tone: "success", text: sectionModule.REFRESH_QUEUED_TEXT },
	]);
	await until(() => calls.get >= 3, "第二次重讀");
	await sleep(40);
	assert.equal(calls.get, 3, "fetch_time 變了就不再重讀");
	assert.equal(queueInvalidated(), true);
});

test("重新整理：fetch_time 一直沒變時重讀有次數上限", async () => {
	const { calls, view } = setup({ get: () => account({}) });
	await view.render();
	await click("重新整理");
	await until(
		() => calls.get === 1 + accountModule.REFRESH_REREAD_MAX_ATTEMPTS,
		"重讀到上限",
	);
	await sleep(40);
	assert.equal(calls.get, 1 + accountModule.REFRESH_REREAD_MAX_ATTEMPTS);
});

test("重新整理：卸載後計時器停止，不再重讀", async () => {
	// 間隔遠大於測試時長：計時器不可能自己觸發，只有卸載清掉它才不會漏出去，不靠牆鐘競速。
	const { calls, view } = setup({ get: () => account({}) }, 60_000);
	await view.render();
	await click("重新整理");
	await until(() => calls.refresh === 1, "重新整理已送出");
	await view.unmount();
	await sleep(50);
	assert.equal(calls.get, 1);
});

test("重新整理：RPC 還在等時卸載，回應到了也不啟動重讀", async () => {
	let release: () => void = () => {};
	const refreshGate = new Promise<void>((resolve) => {
		release = resolve;
	});
	const { calls, view } = setup({ get: () => account({}), refreshGate }, 5);
	await view.render();
	await click("重新整理");
	await until(() => calls.refresh === 1, "重新整理已送出");
	await view.unmount();
	release();
	await sleep(80);
	assert.equal(calls.get, 1);
});

test("重新整理失敗：notice 是錯誤、不重讀、待辦不重讀", async () => {
	const { calls, notices, view, queueInvalidated } = setup({
		get: () => account({}),
		refresh: () => new ConnectError("down", Code.Unavailable),
	});
	await view.render();
	await click("重新整理");
	assert.equal(notices[0], null);
	assert.equal(notices[1]?.tone, "error");
	await sleep(40);
	assert.equal(calls.get, 1);
	assert.equal(queueInvalidated(), false);
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

test("開通任務：重試建立成功與重讀完成都讓開通任務查詢重讀（重新整理剛排入與未完成的重讀不算），重新整理失敗不重讀", async () => {
	const onboardingKey = createConnectQueryKey({
		schema: SellerAdminService.method.getStoreOnboarding,
		cardinality: undefined,
	});
	const invalidated = (view: ReturnType<typeof setup>["view"]) =>
		view.queryClient.getQueryState(onboardingKey)?.isInvalidated === true;

	const retry = setup({
		get: () =>
			account({
				externalAccountId: "",
				status: PaymentAccountStatus.NOT_CREATED,
				createFailedTime: at(1),
			}),
		retry: () => account({ externalAccountId: "acct_new" }),
	});
	retry.view.queryClient.setQueryData(onboardingKey, {
		tasks: [],
		complete: false,
	});
	await retry.view.render();
	await click("重試建立");
	await until(() => bodyText().includes("acct_new"), "快取已寫入");
	assert.equal(invalidated(retry.view), true);
	await retry.view.unmount();

	// 第 2 次讀仍是舊 fetch_time（重讀未完成），第 3 次才變；第 3 次卡住，讓「還沒完成」的斷言有可觀察的時間點。
	let releaseThird: () => void = () => {};
	const thirdGate = new Promise<void>((resolve) => {
		releaseThird = resolve;
	});
	const reread = setup({
		get: (n) => account({ fetchTime: n >= 3 ? at(5) : at(0) }),
		getGate: (n) => (n === 3 ? thirdGate : undefined),
	});
	reread.view.queryClient.setQueryData(onboardingKey, {
		tasks: [],
		complete: false,
	});
	await reread.view.render();
	await click("重新整理");
	assert.equal(reread.calls.refresh, 1);
	await until(() => reread.calls.get >= 3, "第一次重讀已處理、第二次重讀送出");
	assert.equal(
		invalidated(reread.view),
		false,
		"fetch_time 還沒變：排入重新整理與未完成的重讀都不重讀開通任務",
	);
	releaseThird();
	await until(() => invalidated(reread.view), "重讀完成後開通任務重讀");
	await reread.view.unmount();

	const failing = setup({
		get: () => account({}),
		refresh: () => new ConnectError("down", Code.Unavailable),
	});
	failing.view.queryClient.setQueryData(onboardingKey, {
		tasks: [],
		complete: false,
	});
	await failing.view.render();
	await click("重新整理");
	await until(
		() => failing.notices.some((n) => n?.tone === "error"),
		"重新整理失敗的結果已顯示",
	);
	assert.equal(invalidated(failing.view), false);
	await failing.view.unmount();
});

test("開通任務：重讀還在途中區塊就卸載（頁面層的查詢還在），帳戶更新後開通任務仍然重讀，且不再排下一輪", async () => {
	const onboardingKey = createConnectQueryKey({
		schema: SellerAdminService.method.getStoreOnboarding,
		cardinality: undefined,
	});
	let release: () => void = () => {};
	const gate = new Promise<void>((resolve) => {
		release = resolve;
	});
	const { calls, view } = setup(
		{
			get: (n) => account({ fetchTime: n >= 2 ? at(5) : at(0) }),
			getGate: (n) => (n === 2 ? gate : undefined),
		},
		5,
		PageHost,
	);
	view.queryClient.setQueryData(onboardingKey, { tasks: [], complete: false });
	await view.render();
	await click("重新整理");
	await until(() => calls.get === 2, "重讀已送出");
	await click("切換分頁");
	assert.equal(
		view.queryClient.getQueryState(onboardingKey)?.isInvalidated,
		false,
	);
	release();
	await until(
		() => view.queryClient.getQueryState(onboardingKey)?.isInvalidated === true,
		"帳戶更新後開通任務重讀",
	);
	await sleep(40);
	assert.equal(calls.get, 2);
	await view.unmount();
});
