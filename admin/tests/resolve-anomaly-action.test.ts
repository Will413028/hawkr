import assert from "node:assert/strict";
import test from "node:test";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { PaymentAdminService } from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentAnomalyKind,
	PaymentAnomalyOutcome,
	PaymentAnomalyResolutionReason,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	bodyText,
	byId,
	click,
	clickElement,
	dom,
	mount,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";

const h = React.createElement;
const actionModule = requireFromTest(
	"../src/app/admin/anomalies/ResolveAnomalyAction",
) as typeof import("../src/app/admin/anomalies/ResolveAnomalyAction");
const { ToastProvider } = requireFromTest("../src/components/UI/Toast") as {
	ToastProvider: unknown;
};
const { useQuery } = requireFromTest("@connectrpc/connect-query") as {
	useQuery: (
		method: unknown,
		input: unknown,
	) => { data?: { payments: unknown[] } };
};

const TRANSFER = { slot: "transfer", id: "t1" } as const;

function Host({ targets }: { targets: (typeof TRANSFER)[] }) {
	useQuery(PaymentAdminService.method.listPayments, {});
	useQuery(PaymentAdminService.method.listPaymentTransfers, {});
	useQuery(PaymentAdminService.method.listPaymentRefunds, {});
	return h(actionModule.default, { targets, displayId: "t1" });
}

function setup(resolve: (call: number) => ConnectError | undefined) {
	const calls = { resolve: 0, payments: 0, transfers: 0, refunds: 0 };
	const requests: Array<{
		kind: number;
		id: string;
		outcome: number;
		evidence: string;
		reason: number;
		note: string;
	}> = [];
	const transport = createRouterTransport(({ service }) => {
		service(PaymentAdminService, {
			listPayments: () => {
				calls.payments += 1;
				return { payments: [] };
			},
			listPaymentTransfers: () => {
				calls.transfers += 1;
				return { transfers: [] };
			},
			listPaymentRefunds: () => {
				calls.refunds += 1;
				return { refunds: [] };
			},
			resolvePaymentAnomaly: (req) => {
				calls.resolve += 1;
				requests.push({
					kind: req.kind,
					id: req.id,
					outcome: req.outcome,
					evidence: req.evidence,
					reason: req.reason,
					note: req.note,
				});
				const failure = resolve(calls.resolve);
				if (failure) throw failure;
				return {};
			},
		});
	});
	const view = mount(
		h(ToastProvider, null, h(Host, { targets: [TRANSFER] })),
		transport,
	);
	return { calls, requests, view };
}

const fillReason = async (
	reason: PaymentAnomalyResolutionReason,
	note: string,
) =>
	React.act(async () => {
		setValue(byId("resolve-reason"), String(reason));
		setValue(byId("resolve-note"), note);
	});

const radio = (outcome: "succeeded" | "failed") =>
	dom.window.document.querySelector(
		`input[name="resolve-outcome"][value="${outcome}"]`,
	) as HTMLInputElement;

test("成功結案：送出 kind、id、結果與去空白的證據，之後三個列表重讀並關掉對話框", async () => {
	const { calls, requests, view } = setup(() => undefined);
	await view.render();
	await click("結案");
	await clickElement(radio("succeeded"));
	await React.act(async () => {
		setValue(byId("resolve-evidence"), "  tr_123 ");
	});
	await fillReason(
		PaymentAnomalyResolutionReason.PROVIDER_EXECUTED_NOT_RECORDED,
		"  verified in dashboard  ",
	);
	await click("確認結案");
	await until(() => calls.resolve === 1, "結案已送出");
	assert.deepEqual(requests, [
		{
			kind: PaymentAnomalyKind.TRANSFER,
			id: "t1",
			outcome: PaymentAnomalyOutcome.SUCCEEDED,
			evidence: "tr_123",
			reason: PaymentAnomalyResolutionReason.PROVIDER_EXECUTED_NOT_RECORDED,
			note: "verified in dashboard",
		},
	]);
	await until(
		() => calls.payments === 2 && calls.transfers === 2 && calls.refunds === 2,
		"三個列表重讀",
	);
	assert.equal(dom.window.document.getElementById("resolve-evidence"), null);
	await until(
		() => bodyText().includes(actionModule.RESOLVED_TEXT),
		"成功訊息",
	);
});

test("沒選結果或成功卻沒證據：不送出、對話框顯示原因", async () => {
	const { calls, view } = setup(() => undefined);
	await view.render();
	await click("結案");
	await click("確認結案");
	assert.match(bodyText(), /請選擇核對的結果/);
	await clickElement(radio("succeeded"));
	await click("確認結案");
	assert.match(bodyText(), /結案成功要填金流商的物件 ID/);
	await React.act(async () => {
		setValue(byId("resolve-evidence"), "tr_1");
	});
	await click("確認結案");
	assert.match(bodyText(), /請選擇結案原因/);
	await fillReason(PaymentAnomalyResolutionReason.OTHER, "   ");
	await click("確認結案");
	assert.match(bodyText(), /請填寫說明/);
	assert.equal(calls.resolve, 0);
});

test("失敗結案：不帶證據，即使先選過成功並輸入", async () => {
	const { calls, requests, view } = setup(() => undefined);
	await view.render();
	await click("結案");
	await clickElement(radio("succeeded"));
	await React.act(async () => {
		setValue(byId("resolve-evidence"), "tr_typed");
	});
	await clickElement(radio("failed"));
	await fillReason(PaymentAnomalyResolutionReason.PROVIDER_NOT_EXECUTED, "n");
	await click("確認結案");
	await until(() => calls.resolve === 1, "結案已送出");
	assert.equal(requests[0]?.outcome, PaymentAnomalyOutcome.FAILED);
	assert.equal(requests[0]?.evidence, "");
});

test("伺服器拒絕：對話框留著、顯示錯誤分類的文字、輸入還在、列表不重讀", async () => {
	const { calls, view } = setup(
		() => new ConnectError("證據與金流商的物件不符", Code.InvalidArgument),
	);
	await view.render();
	await click("結案");
	await clickElement(radio("succeeded"));
	await React.act(async () => {
		setValue(byId("resolve-evidence"), "tr_bad");
	});
	await fillReason(PaymentAnomalyResolutionReason.OTHER, "note kept");
	await click("確認結案");
	await until(
		() => dom.window.document.querySelector(".adm-operation-error") !== null,
		"錯誤顯示",
	);
	assert.match(bodyText(), /參數錯誤/);
	assert.doesNotMatch(bodyText(), new RegExp(actionModule.RESOLVED_TEXT));
	assert.equal((byId("resolve-evidence") as HTMLInputElement).value, "tr_bad");
	assert.equal(
		(byId("resolve-note") as HTMLTextAreaElement).value,
		"note kept",
	);
	assert.equal(calls.payments, 1);
	assert.equal(calls.transfers, 1);
	assert.equal(calls.refunds, 1);
});

test("改選結果會清掉不相容的原因，原因選項只列相容的", async () => {
	const { view } = setup(() => undefined);
	await view.render();
	await click("結案");
	await clickElement(radio("succeeded"));
	await fillReason(
		PaymentAnomalyResolutionReason.PROVIDER_EXECUTED_NOT_RECORDED,
		"n",
	);
	await clickElement(radio("failed"));
	const select = byId("resolve-reason") as HTMLSelectElement;
	assert.equal(
		select.value,
		String(PaymentAnomalyResolutionReason.UNSPECIFIED),
	);
	const values = Array.from(select.options).map((o) => o.value);
	assert.equal(
		values.includes(
			String(PaymentAnomalyResolutionReason.PROVIDER_EXECUTED_NOT_RECORDED),
		),
		false,
	);
});
