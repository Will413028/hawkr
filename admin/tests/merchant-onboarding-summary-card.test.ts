import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	OnboardingTaskId,
	OnboardingTaskSchema,
	OnboardingTaskState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/**
 * 商家詳情的開通摘要卡（OnboardingSummaryCard）實際渲染出來的 HTML。
 *
 * 守的是：標題與狀態句逐字來自伺服器、client 不依 id／state／action 自己造文字；UNKNOWN 與不認得的狀態不套用已完成的圖示與色調；
 * 總徽章在有來源讀不到時不宣稱「未開通」。寫法同 merchant-payment-account-card.test.ts。
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
	"../src/app/admin/merchants/[id]/OnboardingSummaryCard",
) as typeof import("../src/app/admin/merchants/[id]/OnboardingSummaryCard");
const summary = requireFromTest(
	"../src/app/admin/merchants/[id]/onboarding-summary",
) as typeof import("../src/app/admin/merchants/[id]/onboarding-summary");

type State = Parameters<typeof cardModule.OnboardingSummaryCard>[0]["state"];

function render(state: State): string {
	return renderToStaticMarkup(
		createElement(cardModule.OnboardingSummaryCard, {
			state,
			onReload: () => {},
		}),
	);
}

function task(
	state: number,
	title = "標題甲-7e4",
	status = "狀態句乙-7e4",
	id: OnboardingTaskId = OnboardingTaskId.PLATFORM_REVIEW,
) {
	return create(OnboardingTaskSchema, {
		id,
		state,
		title: { locale: "zh-TW", message: title },
		status: { locale: "zh-TW", message: status },
	});
}

const ready = (tasks: ReturnType<typeof task>[], complete: boolean): State => ({
	state: "ready",
	tasks,
	complete,
});

test("伺服器的標題與狀態句逐字顯示，說明句與有序清單在", () => {
	const html = render(
		ready([task(OnboardingTaskState.ACTION_REQUIRED)], false),
	);
	assert.match(html, /標題甲-7e4/);
	assert.match(html, /狀態句乙-7e4/);
	assert.match(html, /開通進度/);
	assert.match(html, /以下是賣家看到的開通任務與狀態文字。/);
	assert.match(html, /<ol/);
});

test("順序照回應", () => {
	const html = render(
		ready(
			[
				task(
					OnboardingTaskState.COMPLETED,
					"第一-7e4",
					"句一-7e4",
					OnboardingTaskId.PROFILE,
				),
				task(OnboardingTaskState.LOCKED, "第二-7e4", "句二-7e4"),
				task(
					OnboardingTaskState.IN_PROGRESS,
					"第三-7e4",
					"句三-7e4",
					OnboardingTaskId.PAYMENT_ACCOUNT,
				),
			],
			false,
		),
	);
	const at = (s: string) => html.indexOf(s);
	assert.ok(at("第一-7e4") >= 0);
	assert.ok(at("第一-7e4") < at("句一-7e4"));
	assert.ok(at("句一-7e4") < at("第二-7e4"));
	assert.ok(at("第二-7e4") < at("第三-7e4"));
});

test("UNKNOWN 與不認得的狀態不套用已完成的色調，且各自有狀態標記", () => {
	const completed = render(ready([task(OnboardingTaskState.COMPLETED)], true));
	assert.match(completed, /onboarding-task--success/);
	for (const state of [OnboardingTaskState.UNKNOWN, 0, 99]) {
		const html = render(ready([task(state)], false));
		assert.match(html, /onboarding-task--unknown/);
		assert.doesNotMatch(html, /onboarding-task--success/);
	}
});

test("UNKNOWN 的圖示路徑與已完成的不同", () => {
	const path = (state: number) =>
		/<li[^>]*><svg[^>]*><path d="([^"]+)"/.exec(
			render(ready([task(state)], false)),
		)?.[1];
	const done = path(OnboardingTaskState.COMPLETED);
	assert.ok(done);
	assert.ok(path(OnboardingTaskState.UNKNOWN));
	assert.notEqual(path(OnboardingTaskState.UNKNOWN), done);
	assert.notEqual(path(0), done);
});

test("標題或狀態句是空的：顯示目錄的「未知」，不另造句子", () => {
	const html = render(ready([task(OnboardingTaskState.LOCKED, "", "")], false));
	assert.equal(html.split("未知").length - 1, 2);
});

test("徽章：complete 是已開通；未完成且都認得是未開通；有來源讀不到就沒有徽章", () => {
	const complete = render(ready([task(OnboardingTaskState.COMPLETED)], true));
	assert.match(complete, /已開通/);
	assert.doesNotMatch(complete, /未開通/);

	const incomplete = render(
		ready([task(OnboardingTaskState.ACTION_REQUIRED)], false),
	);
	assert.match(incomplete, /未開通/);

	for (const state of [OnboardingTaskState.UNKNOWN, 99]) {
		const html = render(
			ready(
				[
					task(OnboardingTaskState.COMPLETED),
					task(state, undefined, undefined, OnboardingTaskId.PAYMENT_ACCOUNT),
				],
				false,
			),
		);
		assert.doesNotMatch(html, /未開通/);
		assert.doesNotMatch(html, /已開通/);
	}
});

test("載入中、讀取失敗（有無錯誤）", () => {
	const loading = render({ state: "loading" });
	assert.match(loading, /role="status"/);
	assert.match(loading, /開通進度載入中…/);

	const bare = render({ state: "error" });
	assert.match(bare, /role="alert"/);
	assert.match(bare, /開通進度讀取失敗(?!：)/);
	assert.match(bare, /重試/);

	const withError = render({
		state: "error",
		error: new ConnectError("down", Code.Unavailable),
	});
	assert.match(withError, /開通進度讀取失敗：/);
	assert.match(withError, /重試/);
});

test("onboardingSummaryOf：錯誤優先（not_found 不特殊）、沒資料是載入中、零個任務是讀取錯誤", () => {
	const err = new ConnectError("x", Code.NotFound);
	assert.deepEqual(
		summary.onboardingSummaryOf({ error: err, data: undefined }),
		{ state: "error", error: err },
	);
	assert.deepEqual(
		summary.onboardingSummaryOf({ error: null, data: undefined }),
		{
			state: "loading",
		},
	);
	assert.deepEqual(
		summary.onboardingSummaryOf({
			error: null,
			data: { tasks: [], complete: true },
		}),
		{ state: "error" },
	);
	const t = task(OnboardingTaskState.COMPLETED);
	assert.deepEqual(
		summary.onboardingSummaryOf({
			error: null,
			data: { tasks: [t], complete: true },
		}),
		{ state: "ready", tasks: [t], complete: true },
	);
});

test("呈現表：五個已知狀態各有圖示與色調，UNKNOWN 與未對到的狀態同一筆且不同於已完成", () => {
	const p = summary.onboardingTaskPresentation;
	const known = [
		OnboardingTaskState.COMPLETED,
		OnboardingTaskState.ACTION_REQUIRED,
		OnboardingTaskState.IN_PROGRESS,
		OnboardingTaskState.LOCKED,
		OnboardingTaskState.UNKNOWN,
	];
	const keys = new Set(known.map((s) => `${p(s).icon}/${p(s).tone}`));
	assert.equal(keys.size, known.length);
	assert.deepEqual(
		p(OnboardingTaskState.UNSPECIFIED),
		p(OnboardingTaskState.UNKNOWN),
	);
	assert.deepEqual(
		p(99 as OnboardingTaskState),
		p(OnboardingTaskState.UNKNOWN),
	);
	assert.notEqual(
		p(OnboardingTaskState.UNKNOWN).icon,
		p(OnboardingTaskState.COMPLETED).icon,
	);
	assert.notEqual(
		p(OnboardingTaskState.UNKNOWN).tone,
		p(OnboardingTaskState.COMPLETED).tone,
	);
});

test("接線：頁面在收款帳戶區塊之前渲染開通摘要卡，invalidateKyc 一併重讀開通任務；卡不接資料", async () => {
	const { readFile } = await import("node:fs/promises");
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	const onboardingAt = page.indexOf("<OnboardingSummaryCard");
	const paymentAt = page.indexOf("<PaymentAccountSection");
	assert.ok(onboardingAt >= 0);
	assert.ok(onboardingAt < paymentAt);
	assert.match(page, /useStoreOnboarding\(/u);
	const kycAt = page.indexOf("const invalidateKyc");
	assert.ok(kycAt >= 0);
	const body = page.slice(kycAt);
	const approveAt = body.indexOf("const approveMutation");
	assert.ok(approveAt > 0);
	const fn = body.slice(0, approveAt);
	assert.match(fn, /SellerAdminService\.method\.getStoreOnboarding/u);
	const card = await readFile(
		"src/app/admin/merchants/[id]/OnboardingSummaryCard.tsx",
		"utf8",
	);
	assert.doesNotMatch(card, /useConnectQuery|useQuery|useToast/u);
});
