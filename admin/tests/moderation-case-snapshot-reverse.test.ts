import assert from "node:assert/strict";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	CaseAction,
	CaseDecision,
	CaseSchema,
	CaseSnapshotSchema,
	CaseState,
	ModerationAdminService,
	type ReverseHideRequest,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";

/** 案件詳情：被檢舉內容快照（五種 oneof、聊天前後文與墓碑）與撤銷隱藏。 */

const { ToastProvider } = requireFromTest("../src/components/UI/Toast") as {
	ToastProvider: unknown;
};
const Page = requireFromTest("../src/app/admin/moderation/cases/[id]/page")
	.default as unknown;
const { canReverse } = requireFromTest(
	"../src/app/admin/moderation/case-decide",
) as { canReverse: (k: unknown) => boolean };
const h = React.createElement;
const T0 = Date.UTC(2026, 10, 1);

function closedHide(patch: MessageInitShape<typeof CaseSchema> = {}) {
	return create(CaseSchema, {
		id: "case-1",
		target: { kind: ReportTargetKind.REVIEW, id: "rv-1" },
		state: CaseState.CLOSED,
		decision: CaseDecision.VIOLATION,
		category: PolicyCategory.SPAM,
		action: CaseAction.HIDE,
		createTime: timestampFromMs(T0),
		etag: "e1",
		...patch,
	} as MessageInitShape<typeof CaseSchema>);
}

function setup(
	current: () => ReturnType<typeof closedHide>,
	snapshot?: MessageInitShape<typeof CaseSnapshotSchema>,
	onReverse: (req: ReverseHideRequest) => unknown = () => ({}),
) {
	const reverses: ReverseHideRequest[] = [];
	const transport: Transport = createRouterTransport(({ service }) => {
		service(ModerationAdminService, {
			getCase: () => ({
				case: current(),
				reports: [],
				snapshot: snapshot ? create(CaseSnapshotSchema, snapshot) : undefined,
			}),
			reverseHide: (req: ReverseHideRequest) => {
				reverses.push(req);
				return onReverse(req);
			},
		} as never);
	});
	return { reverses, transport };
}

async function open(transport: Transport) {
	navigation.params = { id: "case-1" };
	const m = mount(h(ToastProvider, null, h(Page)), transport);
	await m.render();
	return m;
}

test("canReverse：只有已結案、隱藏、未撤銷", () => {
	assert.equal(canReverse(closedHide()), true);
	assert.equal(canReverse(closedHide({ state: CaseState.OPEN })), false);
	assert.equal(canReverse(closedHide({ action: CaseAction.WARN })), false);
	assert.equal(
		canReverse(closedHide({ reversal: { reversedBy: "a", reason: "x" } })),
		false,
	);
});

test("快照：評價顯示評分、內文與店家回覆", async () => {
	const { transport } = setup(() => closedHide(), {
		content: {
			case: "review",
			value: {
				authorId: "b-9",
				storeId: "s-3",
				rating: 2,
				body: "很差的評價內文",
				replyBody: "店家的回覆內文",
			},
		},
	});
	await open(transport);
	const text = bodyText();
	assert.ok(text.includes("被檢舉內容"));
	assert.ok(text.includes("很差的評價內文"));
	assert.ok(text.includes("店家的回覆內文"));
	assert.ok(text.includes("b-9"));
});

const otherKinds = [
	[
		"評價回覆",
		{
			case: "reviewReply",
			value: {
				storeId: "s-1",
				reviewId: "rv-8",
				reviewerId: "b-4",
				body: "回覆內文",
			},
		},
		"回覆內文",
	],
	[
		"買家",
		{ case: "buyer", value: { displayName: "小明", avatar: "" } },
		"小明",
	],
	[
		"社群",
		{
			case: "social",
			value: { authorKind: "STORE", authorId: "5", body: "貼文內文" },
		},
		"貼文內文",
	],
] as const;

for (const [name, content, expected] of otherKinds) {
	test(`快照：${name}顯示自己的欄位`, async () => {
		const { transport } = setup(() => closedHide(), {
			content: content as never,
		});
		await open(transport);
		assert.ok(bodyText().includes(expected), expected);
	});
}

test("快照：聊天前後文依序顯示，已隱藏與已抹除是墓碑、不顯示內文", async () => {
	const { transport } = setup(
		() =>
			closedHide({
				target: { kind: ReportTargetKind.CHAT_MESSAGE, id: "m-1" },
			}),
		{
			content: {
				case: "chat",
				value: {
					authorKind: "BUYER",
					authorId: "b-1",
					body: "被檢舉的訊息",
					conversationId: "conv-1",
					seq: BigInt(5),
					context: [
						{
							seq: BigInt(4),
							senderKind: "STORE",
							senderId: "7",
							body: "正常前文",
						},
						{
							seq: BigInt(6),
							senderKind: "BUYER",
							senderId: "b-1",
							body: "祕密內文",
							hidden: true,
						},
						{
							seq: BigInt(7),
							senderKind: "BUYER",
							senderId: "b-1",
							body: "抹除內文",
							erased: true,
						},
					],
				},
			},
		},
	);
	await open(transport);
	const text = bodyText();
	assert.ok(text.includes("正常前文"));
	assert.ok(text.includes("（已隱藏）"));
	assert.ok(text.includes("（已抹除）"));
	assert.ok(!text.includes("祕密內文"));
	assert.ok(!text.includes("抹除內文"));
	assert.ok(text.indexOf("#4") < text.indexOf("#6"));
});

test("沒有快照：顯示已清除的說明", async () => {
	const { transport } = setup(() => closedHide());
	await open(transport);
	assert.ok(bodyText().includes("沒有內容快照"));
});

test("撤銷隱藏：可撤銷的案件有按鈕", async () => {
	await open(setup(() => closedHide()).transport);
	assert.ok(buttonByText("撤銷隱藏"));
});

test("撤銷隱藏：處置不是隱藏的案件沒有按鈕", async () => {
	await open(setup(() => closedHide({ action: CaseAction.WARN })).transport);
	assert.equal(buttonByText("撤銷隱藏"), undefined);
});

test("撤銷隱藏：空白原因不送出，送出帶案件 ID 與原因", async () => {
	const { transport, reverses } = setup(() => closedHide());
	await open(transport);
	await click("撤銷隱藏");
	setValue(byId("moderation-reverse-reason"), "   ");
	await click("確認撤銷");
	assert.equal(reverses.length, 0);
	assert.ok(bodyText().includes("請填寫撤銷原因"));
	setValue(byId("moderation-reverse-reason"), "誤判");
	await click("確認撤銷");
	await until(() => reverses.length === 1, "ReverseHide 的請求");
	assert.equal(reverses[0]?.id, "case-1");
	assert.equal(reverses[0]?.reason, "誤判");
	await until(
		() => maybeById("moderation-reverse-reason") === null,
		"對話框關閉",
	);
});

test("撤銷隱藏：超過 500 字不送出", async () => {
	const { transport, reverses } = setup(() => closedHide());
	await open(transport);
	await click("撤銷隱藏");
	setValue(byId("moderation-reverse-reason"), "字".repeat(501));
	await click("確認撤銷");
	assert.equal(reverses.length, 0);
	assert.ok(bodyText().includes("最多 500 字"));
});

test("撤銷隱藏：FAILED_PRECONDITION 顯示專用說明並保留對話框", async () => {
	const { transport } = setup(
		() => closedHide(),
		undefined,
		() => {
			throw new ConnectError("precondition", Code.FailedPrecondition);
		},
	);
	await open(transport);
	await click("撤銷隱藏");
	setValue(byId("moderation-reverse-reason"), "誤判");
	await click("確認撤銷");
	await until(() => bodyText().includes("無法撤銷"), "前置條件說明");
	assert.ok(maybeById("moderation-reverse-reason"));
});

test("已撤銷：顯示撤銷資訊，沒有撤銷按鈕", async () => {
	const { transport } = setup(() =>
		closedHide({
			reversal: {
				reverseTime: timestampFromMs(T0 + 9000),
				reversedBy: "admin-7",
				reason: "申訴成立",
			},
		}),
	);
	await open(transport);
	assert.equal(buttonByText("撤銷隱藏"), undefined);
	const text = bodyText();
	assert.ok(text.includes("admin-7"));
	assert.ok(text.includes("申訴成立"));
});
