import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createRouterTransport,
	type Transport,
} from "@connectrpc/connect";
import {
	CaseInquiryKind,
	CaseInquiryReplyChannel,
	CaseInquirySchema,
	CaseInquiryState,
	CaseInquiryWithdrawCause,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import {
	FDAChannel,
	FDAReferralReplySchema,
	FDAReferralSchema,
	FDAReferralWaiverSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { DeadlineSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import {
	bodyText,
	buttonByText,
	byId,
	click,
	dom,
	maybeById,
	mount,
	navigation,
	React,
	requireFromTest,
	setValue,
	until,
} from "./fixtures/dom-harness";
import {
	CASE_ID,
	kase,
	STORE_ID,
	T0,
	ts,
	withMessage,
} from "./fixtures/report-fixtures";

/**
 * 案件詳情頁的詢問賣家與轉報 v2（3c-b-5）：詢問區塊的卡片與按鈕（只看伺服器的 can_*）、讀取失敗只影響詢問區塊、
 * 詢問／回覆（附件上傳）／撤回三個對話框的請求、轉報的追蹤日期與回覆與不轉報、頁首的等賣家與結案提示。
 */

const Page = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/page",
).default as unknown;
const ReplyDialog = requireFromTest(
	"../src/app/admin/violation-reports/cases/[id]/RecordInquiryReplyDialog",
).default as unknown;
const h = React.createElement;

type Req = Record<string, unknown>;
type Handler = (req: Req) => unknown;

function transportWith(gov: Record<string, Handler>): Transport {
	return createRouterTransport(({ service }) => {
		service(SellerGovernanceAdminService, {
			listReports: () => ({ reports: [] }),
			listMeasures: () => ({ measures: [] }),
			listCaseNotes: () => ({ notes: [] }),
			listCaseInquiries: () => ({ inquiries: [] }),
			...gov,
		} as never);
		service(SellerAdminService, {
			getStore: () => ({ store: { id: STORE_ID, accountId: "acc-1" } }),
		} as never);
	});
}

let current: { unmount: () => Promise<void> } | undefined;
async function openPage(transport: Transport) {
	await current?.unmount();
	navigation.params = { id: CASE_ID };
	const m = mount(h(Page), transport);
	current = m;
	await m.render();
	return m;
}

const UUID_V4 =
	/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u;

const INQUIRY_ID = "0190b000-0000-7000-8000-0000000000a1";

function inquiry(
	patch: Parameters<typeof create<typeof CaseInquirySchema>>[1],
) {
	return create(CaseInquirySchema, {
		id: INQUIRY_ID,
		caseId: CASE_ID,
		storeId: STORE_ID,
		kind: CaseInquiryKind.RECORDS,
		message: "請提供進貨紀錄",
		createTime: ts(T0),
		createdByAdminId: "9001",
		replyDeadline: create(DeadlineSchema, {
			startTime: ts(T0),
			dueTime: ts(new Date(T0.getTime() + 3 * 86400000)),
		}),
		state: CaseInquiryState.OPEN,
		replyEvidenceMax: 5,
		...patch,
	});
}

/** 請求的純資料（去掉 protobuf 的 $typeName），測試只看欄位。 */
const plain = (value: unknown) =>
	JSON.parse(
		JSON.stringify(value, (key, v) => (key === "$typeName" ? undefined : v)),
	);

const footerButton = (text: string) =>
	[...dom.window.document.querySelectorAll(".modal-footer button")].find(
		(b) => b.textContent === text,
	) as HTMLElement | undefined;

async function submit(text: string) {
	const button = footerButton(text);
	assert.ok(button, `找不到對話框的按鈕「${text}」`);
	await React.act(async () => {
		button.click();
	});
}

const fill = (id: string, value: string) =>
	React.act(async () => {
		setValue(byId(id), value);
	});

test("詢問區塊：固定說明、空狀態、每筆一張卡（{種類}・{狀態}、建立、期限、說明）；按鈕只看 can_*", async () => {
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canCreateInquiry: true }) }),
		}),
	);
	assert.match(bodyText(), /詢問賣家/u);
	assert.match(
		bodyText(),
		/這裡只記下詢問；賣家端的畫面上線前，系統不會通知賣家，請另外聯絡賣家。詢問只寫你轉述的內容，不要附上通報原文或通報人的資料。/u,
	);
	assert.match(bodyText(), /還沒有詢問/u);
	assert.ok(buttonByText("詢問賣家"), "頁首按鈕");

	await openPage(
		transportWith({
			getCase: () => ({ case: kase({}) }),
			listCaseInquiries: () => ({
				inquiries: [
					inquiry({ canRecordReply: true, canWithdraw: true }),
					inquiry({
						id: "i-2",
						kind: CaseInquiryKind.EXPLANATION,
						state: CaseInquiryState.WITHDRAWN,
						withdrawCause: CaseInquiryWithdrawCause.CASE_CLOSED,
						withdrawTime: ts(T0),
						// 伺服器只在 OPEN 時給 reply_deadline。
						replyDeadline: undefined,
					}),
				],
			}),
		}),
	);
	assert.ok(
		buttonByText("詢問賣家") === undefined,
		"沒有 can_create_inquiry 就沒有頁首按鈕",
	);
	assert.match(bodyText(), /提供紀錄・等賣家回覆/u);
	assert.match(bodyText(), /請說明・已撤回/u);
	assert.match(bodyText(), /請提供進貨紀錄/u);
	assert.equal(
		dom.window.document.querySelectorAll("[data-inquiry-id]").length,
		2,
	);
	assert.equal(buttonsText("記下回覆").length, 1, "只有第一筆能記回覆");
	assert.equal(buttonsText("撤回").length, 1);
	assert.match(bodyText(), /已於 2026\/10\/18 10:00 撤回（案件結案時撤回）/u);
	assert.match(
		bodyText(),
		/結案或撤回之後才收到的回覆，請寫在案件的內部註記。/u,
	);
	// 相對時間隨現在的時鐘變，這裡只看有走 DeadlineText（「剩…」或「已逾期…」）；已撤回的卡沒有期限（伺服器不給），顯示「-」。
	assert.match(bodyText(), /期限(剩|已逾期) /u, "OPEN 的期限用相對時間");
	assert.match(
		bodyText(),
		/請說明・已撤回.*期限-給賣家的說明/u,
		"不是 OPEN 沒有期限",
	);
});

function buttonsText(text: string) {
	return [...dom.window.document.querySelectorAll("button")].filter(
		(b) => b.textContent === text,
	);
}

test("已回覆的卡：管道、收到的時間、記下、內容與附件連結（機敏檔，新分頁開啟）", async () => {
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({}) }),
			listCaseInquiries: () => ({
				inquiries: [
					inquiry({
						state: CaseInquiryState.REPLIED,
						reply: {
							body: "已附進貨單",
							channel: CaseInquiryReplyChannel.EMAIL,
							receiveTime: ts(T0),
							recordTime: ts(T0),
							recordedByAdminId: "9002",
							evidence: [
								{
									position: 0,
									viewUri: `/api/files/admin/seller/case_inquiries/${INQUIRY_ID}/evidence/0`,
								},
								{ position: 1, viewUri: "https://evil.example/x" },
							],
						},
					}),
				],
			}),
		}),
	);
	assert.match(bodyText(), /提供紀錄・已回覆/u);
	assert.match(bodyText(), /Email/u);
	assert.match(bodyText(), /已附進貨單/u);
	const links = [
		...dom.window.document.querySelectorAll("[data-inquiry-id] a"),
	] as HTMLAnchorElement[];
	assert.deepEqual(
		links.map((a) => [a.textContent, a.getAttribute("href"), a.rel, a.target]),
		[
			[
				"附件 1",
				`/api/bff/api/files/admin/seller/case_inquiries/${INQUIRY_ID}/evidence/0`,
				"noopener noreferrer",
				"_blank",
			],
		],
		"不是後台檔案路徑的網址不給連結，只顯示名稱",
	);
	assert.match(bodyText(), /附件 2/u);
});

test("詢問讀取失敗：只影響詢問區塊，顯示失敗句與重試；重試成功後列出", async () => {
	let calls = 0;
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canRecordFdaReferral: true }) }),
			listCaseInquiries: () => {
				calls++;
				if (calls === 1) throw new ConnectError("x", Code.Unavailable);
				return { inquiries: [inquiry({})] };
			},
		}),
	);
	await until(() => bodyText().includes("重試"), "失敗與重試");
	assert.match(bodyText(), /案件詳情/u, "其他區塊照常");
	assert.ok(maybeById("case-referral-title"));
	await click("重試");
	await until(() => bodyText().includes("請提供進貨紀錄"), "重試後列出");
});

test("詢問賣家：說明必填；請求帶種類、去掉前後空白的說明與期限；失敗後重送沿用同一個 request_id，改內容換新的", async () => {
	const requests: Req[] = [];
	let fail = true;
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canCreateInquiry: true }) }),
			createCaseInquiry: (req) => {
				requests.push(req);
				if (fail) throw new ConnectError("x", Code.Unavailable);
				return { inquiry: inquiry({}), case: kase({}) };
			},
		}),
	);
	await click("詢問賣家");
	await fill("create-case-inquiry-kind", String(CaseInquiryKind.RECORDS));
	await submit("記下詢問");
	assert.match(bodyText(), /請填寫給賣家的說明。/u);
	assert.equal(requests.length, 0, "沒填說明不送出");

	await fill("create-case-inquiry-message", "  請提供 10 月的進貨紀錄  ");
	await fill("create-case-inquiry-due", "2026-10-25T10:00");
	await submit("記下詢問");
	await until(() => requests.length === 1, "第一次送出");
	await until(() => !footerButton("送出中…"), "失敗後可再送");
	const first = requests[0];
	assert.equal(first.caseId, CASE_ID);
	assert.equal(first.kind, CaseInquiryKind.RECORDS);
	assert.equal(first.message, "請提供 10 月的進貨紀錄");
	assert.ok(first.dueTime, "有選期限就帶");
	assert.match(String(first.requestId), /^[0-9a-f-]{36}$/u);

	await submit("記下詢問");
	await until(() => requests.length === 2, "重送");
	assert.equal(
		requests[1].requestId,
		first.requestId,
		"同一份內容重送同一個 id",
	);

	await fill("create-case-inquiry-message", "請改提供 9 月的");
	fail = false;
	await submit("記下詢問");
	await until(() => requests.length === 3, "改內容後送出");
	assert.notEqual(requests[2].requestId, first.requestId);
	await until(
		() => maybeById("create-case-inquiry-kind") === null,
		"成功後關閉",
	);
});

test("詢問賣家：不選期限就不帶期限欄位（由伺服器取 72 小時）", async () => {
	const requests: Req[] = [];
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canCreateInquiry: true }) }),
			createCaseInquiry: (req) => {
				requests.push(req);
				return { inquiry: inquiry({}), case: kase({}) };
			},
		}),
	);
	await click("詢問賣家");
	await fill("create-case-inquiry-kind", String(CaseInquiryKind.EXPLANATION));
	await fill("create-case-inquiry-message", "請說明");
	await submit("記下詢問");
	await until(() => requests.length === 1, "送出");
	assert.equal(requests[0].dueTime, undefined);
	assert.equal(requests[0].kind, CaseInquiryKind.EXPLANATION);
});

test("記下回覆：附件上傳成機敏檔再送出 staged token（依選檔順序）；送出失敗後再按一次不重傳", async () => {
	const requests: Req[] = [];
	const uploaded: string[] = [];
	let fail = true;
	const transport = transportWith({});
	const m = mount(
		h(ReplyDialog, {
			inquiry: inquiry({ canRecordReply: true }),
			onClose: () => {},
			upload: async (file: File) => {
				uploaded.push(file.name);
				return `token-${file.name}`;
			},
			onConfirm: async (req: Req) => {
				requests.push(req);
				if (fail) throw new ConnectError("x", Code.Unavailable);
			},
		}),
		transport,
	);
	await m.render({ settle: false });
	const time = byId<HTMLInputElement>("record-inquiry-reply-time");
	// 瀏覽器會把秒為 0 的值正規化成整分，所以只能看有開放秒的輸入（預設值精確到秒由 toLocalInputSeconds 的測試守）。
	assert.equal(time.step, "1", "輸入框開放到秒");
	await fill(
		"record-inquiry-reply-channel",
		String(CaseInquiryReplyChannel.PHONE),
	);
	await fill("record-inquiry-reply-body", " 已電話回覆 ");
	const input = byId<HTMLInputElement>("record-inquiry-reply-files");
	const pdf = new dom.window.File(["%PDF-1.4"], "invoice.pdf", {
		type: "application/pdf",
	});
	const png = new dom.window.File(
		[new Uint8Array([137, 80, 78, 71])],
		"shot.png",
		{
			type: "image/png",
		},
	);
	await React.act(async () => {
		Object.defineProperty(input, "files", {
			value: [pdf, png],
			configurable: true,
		});
		input.dispatchEvent(new dom.window.Event("change", { bubbles: true }));
	});
	assert.match(bodyText(), /invoice\.pdf/u);
	assert.match(bodyText(), /shot\.png/u);
	await submit("記下回覆");
	await until(() => requests.length === 1, "第一次送出");
	assert.deepEqual(uploaded, ["invoice.pdf", "shot.png"]);
	assert.deepEqual(requests[0].evidenceTokens, [
		"token-invoice.pdf",
		"token-shot.png",
	]);
	assert.equal(requests[0].id, INQUIRY_ID);
	assert.equal(requests[0].channel, CaseInquiryReplyChannel.PHONE);
	assert.equal(requests[0].body, "已電話回覆");
	assert.ok(requests[0].receiveTime, "收到的時間預設現在");

	fail = false;
	await until(() => !footerButton("送出中…"), "失敗後可再送");
	await submit("記下回覆");
	await until(() => requests.length === 2, "重送");
	assert.deepEqual(uploaded, ["invoice.pdf", "shot.png"], "沒有重傳");
});

async function mountReplyDialog(
	props: Record<string, unknown>,
	onConfirm: (req: Req) => Promise<void> = async () => {},
) {
	const m = mount(
		h(ReplyDialog, {
			inquiry: inquiry({ canRecordReply: true }),
			onClose: () => {},
			onConfirm,
			...props,
		}),
		transportWith({}),
	);
	await m.render({ settle: false });
	return m;
}

async function pickFiles(count: number) {
	const input = byId<HTMLInputElement>("record-inquiry-reply-files");
	await React.act(async () => {
		Object.defineProperty(input, "files", {
			value: Array.from(
				{ length: count },
				(_, i) =>
					new dom.window.File(["x"], `f${i}.pdf`, { type: "application/pdf" }),
			),
			configurable: true,
		});
		input.dispatchEvent(new dom.window.Event("change", { bubbles: true }));
	});
}

test("記下回覆：上傳失敗一律顯示固定的失敗句（J15），不顯示內部錯誤字串，不送出請求", async () => {
	const requests: Req[] = [];
	await mountReplyDialog(
		{
			upload: async () => {
				throw new Error("機敏檔上傳失敗：缺少 stagedToken");
			},
		},
		async (req) => {
			requests.push(req);
		},
	);
	await pickFiles(1);
	await submit("記下回覆");
	await until(
		() => bodyText().includes("記下失敗，沒有寫入；請確認網路狀態後再試一次。"),
		"失敗句",
	);
	assert.doesNotMatch(bodyText(), /機敏檔上傳失敗|stagedToken/u);
	assert.equal(requests.length, 0);
});

test("記下回覆：onConfirm 的 Connect 錯誤仍顯示伺服器的句子", async () => {
	await mountReplyDialog({}, async () => {
		throw withMessage(Code.InvalidArgument, "回覆內容太長了。");
	});
	await submit("記下回覆");
	await until(() => bodyText().includes("回覆內容太長了。"), "伺服器句");
});

test("記下回覆：附件上限取自伺服器（reply_evidence_max）；超過在上傳之前擋下，剛好就上傳", async () => {
	const uploaded: string[] = [];
	const requests: Req[] = [];
	await mountReplyDialog(
		{
			inquiry: inquiry({ canRecordReply: true, replyEvidenceMax: 2 }),
			upload: async (file: File) => {
				uploaded.push(file.name);
				return `t-${file.name}`;
			},
		},
		async (req) => {
			requests.push(req);
		},
	);
	assert.match(bodyText(), /附件（選填，最多 2 份）/u);
	await pickFiles(3);
	await submit("記下回覆");
	await until(
		() => bodyText().includes("附件最多 2 份，請移除多的檔案。"),
		"超過上限",
	);
	assert.deepEqual(uploaded, [], "沒有上傳");
	assert.equal(requests.length, 0);

	await pickFiles(2);
	assert.doesNotMatch(bodyText(), /請移除多的檔案/u, "改選之後清掉提示");
	await submit("記下回覆");
	await until(() => requests.length === 1, "送出");
	assert.deepEqual(uploaded, ["f0.pdf", "f1.pdf"]);
});

test("記下回覆與撤回：已記過／已撤回時顯示橫幅（這次沒有寫入）", async () => {
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({}) }),
			listCaseInquiries: () => ({
				inquiries: [inquiry({ canRecordReply: true, canWithdraw: true })],
			}),
			recordCaseInquiryReply: () => ({
				inquiry: inquiry({ state: CaseInquiryState.REPLIED }),
				case: kase({}),
				alreadyRecorded: true,
			}),
			withdrawCaseInquiry: () => ({
				inquiry: inquiry({ state: CaseInquiryState.WITHDRAWN }),
				case: kase({}),
				alreadyWithdrawn: true,
			}),
		}),
	);
	await click("記下回覆");
	await fill(
		"record-inquiry-reply-channel",
		String(CaseInquiryReplyChannel.OTHER),
	);
	await fill("record-inquiry-reply-body", "電話裡說的");
	await submit("記下回覆");
	await until(
		() => bodyText().includes("這筆詢問先前已記下回覆，這次沒有再記一筆。"),
		"已記過橫幅",
	);
	await click("撤回");
	assert.match(bodyText(), /撤回詢問/u);
	assert.match(
		bodyText(),
		/撤回之後賣家不必再回覆。賣家端的畫面上線前，請另外告知賣家。/u,
	);
	await submit("撤回");
	await until(() => bodyText().includes("這筆詢問先前已撤回。"), "已撤回橫幅");
});

test("撤回詢問：請求只帶詢問 id；伺服器的失敗句顯示在對話框裡", async () => {
	const requests: Req[] = [];
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({}) }),
			listCaseInquiries: () => ({
				inquiries: [inquiry({ canWithdraw: true })],
			}),
			withdrawCaseInquiry: (req) => {
				requests.push(req);
				throw withMessage(Code.FailedPrecondition, "這筆詢問已回覆。");
			},
		}),
	);
	await click("撤回");
	await submit("撤回");
	await until(() => bodyText().includes("這筆詢問已回覆。"), "失敗句");
	assert.deepEqual(requests.map(plain), [{ id: INQUIRY_ID }]);
});

test("等賣家：頁首狀態旁有「等賣家」標記；結案對話框加提示（不擋送出）", async () => {
	await openPage(
		transportWith({
			getCase: () => ({
				case: kase({
					awaitingSeller: true,
					openInquiryId: INQUIRY_ID,
					canDecide: true,
				}),
			}),
		}),
	);
	assert.match(
		dom.window.document.querySelector(".header-title-row")?.textContent ?? "",
		/進行中.*等賣家/u,
	);
	await click("結案");
	await until(
		() => bodyText().includes("還有一筆詢問在等賣家回覆；結案會撤回它。"),
		"結案提示",
	);
	assert.ok(footerButton("確認結案")?.hasAttribute("disabled") === false);

	await openPage(transportWith({ getCase: () => ({ case: kase({}) }) }));
	assert.doesNotMatch(
		dom.window.document.querySelector(".header-title-row")?.textContent ?? "",
		/等賣家/u,
	);
});

const REFERRAL_TIME = new Date("2026-10-01T02:00:00Z");

test("轉報區塊：追蹤日期的 warning 只看伺服器的 follow_up_overdue（不比日期）；有回覆列出「{日期}：{內容}」；沒有回覆說還沒有回覆", async () => {
	const reply = create(FDAReferralReplySchema, {
		body: "已收到，辦理中",
		receiveDate: { year: 2026, month: 10, day: 5 },
		recordTime: ts(T0),
	});
	const referral = (followUpOverdue: boolean, replies: unknown[] = []) =>
		create(FDAReferralSchema, {
			channel: FDAChannel.FDA_CHANNEL_ONLINE_FORM,
			referenceNo: "FDA-9",
			reportTime: ts(REFERRAL_TIME),
			// 日期早就過了：旗標是 false 時仍不可以有 warning，證明畫面不自己比日期。
			followUpDate: { year: 2020, month: 10, day: 2 },
			followUpOverdue,
			replies: replies as never,
		});
	const warning = () => dom.window.document.querySelector(".adm-sla--warning");
	const show = (followUpOverdue: boolean, replies: unknown[] = []) =>
		openPage(
			transportWith({
				getCase: () => ({
					case: kase({ fdaReferral: referral(followUpOverdue, replies) }),
				}),
			}),
		);

	await show(true);
	assert.match(bodyText(), /追蹤日期 2020\/10\/02/u);
	assert.match(bodyText(), /還沒有回覆/u);
	assert.ok(warning(), "伺服器說逾期");

	await show(false);
	assert.match(bodyText(), /追蹤日期 2020\/10\/02/u);
	assert.ok(warning() === null, "伺服器說沒逾期，日期再舊也不 warning");

	// 有回覆而且新的追蹤日期又逾期：伺服器說逾期就 warning（跨輪）。
	await show(true, [reply]);
	assert.match(bodyText(), /2026\/10\/05：已收到，辦理中/u);
	assert.doesNotMatch(bodyText(), /還沒有回覆/u);
	assert.ok(warning(), "有回覆但新的追蹤日期逾期");

	await show(false, [reply]);
	assert.ok(warning() === null, "有回覆且伺服器說沒逾期");
});

test("轉報區塊：不轉報顯示「已於 {時間} 標記不轉報：{原因}」；按鈕只看 can_*", async () => {
	await openPage(
		transportWith({
			getCase: () => ({
				case: kase({
					fdaReferralWaiver: create(FDAReferralWaiverSchema, {
						reason: "主管機關已經介入",
						waiveTime: ts(T0),
					}),
				}),
			}),
		}),
	);
	assert.match(
		bodyText(),
		/已於 2026\/10\/18 10:00 標記不轉報：主管機關已經介入/u,
	);
	assert.ok(buttonByText("不轉報") === undefined);
	assert.ok(buttonByText("記下主管機關的回覆") === undefined);

	await openPage(
		transportWith({
			getCase: () => ({
				case: kase({
					canRecordFdaReferral: true,
					canWaiveFdaReferral: true,
					canRecordFdaReferralReply: true,
				}),
			}),
		}),
	);
	assert.ok(buttonByText("不轉報"));
	assert.ok(buttonByText("記錄已轉報主管機關"));
	assert.ok(buttonByText("記下主管機關的回覆"));
});

test("不轉報：原因去掉前後空白送出；已標記過時顯示橫幅", async () => {
	const requests: Req[] = [];
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canWaiveFdaReferral: true }) }),
			waiveFDAReferral: (req) => {
				requests.push(req);
				return { case: kase({}), alreadyWaived: true };
			},
		}),
	);
	await click("不轉報");
	assert.match(bodyText(), /例如主管機關已經介入。標記之後不能再記錄轉報。/u);
	await fill("waive-fda-referral-reason", "  主管機關已經介入 ");
	await submit("標記不轉報");
	await until(() => requests.length === 1, "送出");
	assert.deepEqual(plain(requests[0]), {
		caseId: CASE_ID,
		reason: "主管機關已經介入",
	});
	await until(() => bodyText().includes("這個案件先前已標記不轉報。"), "橫幅");
});

test("記下主管機關的回覆：收到的日期預設今天，請求帶日期、內容、下一個追蹤日期；勾「不再追蹤」時追蹤日期欄停用並清空", async () => {
	const requests: Req[] = [];
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canRecordFdaReferralReply: true }) }),
			recordFDAReferralReply: (req) => {
				requests.push(req);
				return { case: kase({}) };
			},
		}),
	);
	await click("記下主管機關的回覆");
	assert.match(
		byId<HTMLInputElement>("fda-referral-reply-date").value,
		/^\d{4}-\d{2}-\d{2}$/u,
	);
	await fill("fda-referral-reply-date", "2026-10-19");
	await fill("fda-referral-reply-body", " 已受理 ");
	await fill("fda-referral-reply-next", "2026-11-19");
	await submit("記下回覆");
	await until(() => requests.length === 1, "送出");
	const { requestId, ...rest } = plain(requests[0]);
	assert.match(requestId, UUID_V4);
	assert.deepEqual(rest, {
		caseId: CASE_ID,
		body: "已受理",
		receiveDate: { year: 2026, month: 10, day: 19 },
		nextFollowUpDate: { year: 2026, month: 11, day: 19 },
		clearFollowUp: false,
	});

	await click("記下主管機關的回覆");
	await fill("fda-referral-reply-next", "2026-12-01");
	await React.act(async () => {
		byId<HTMLInputElement>("fda-referral-reply-clear").click();
	});
	assert.equal(byId<HTMLInputElement>("fda-referral-reply-next").value, "");
	assert.equal(
		byId<HTMLInputElement>("fda-referral-reply-next").disabled,
		true,
	);
});

test("記下主管機關的回覆：request_id 是 UUIDv4；同一份內容失敗後重送沿用同一個，內容改了換新的", async () => {
	const requests: Req[] = [];
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canRecordFdaReferralReply: true }) }),
			recordFDAReferralReply: (req) => {
				requests.push(req);
				if (requests.length < 3) {
					throw new ConnectError("暫時失敗", Code.Unavailable);
				}
				return { case: kase({}) };
			},
		}),
	);
	await click("記下主管機關的回覆");
	await fill("fda-referral-reply-body", "第一版");
	await submit("記下回覆");
	await until(() => requests.length === 1, "第一次送出");
	await until(() => !footerButton("送出中…"), "可重送");
	await submit("記下回覆");
	await until(() => requests.length === 2, "原樣重送");
	await until(() => !footerButton("送出中…"), "可再送");
	await fill("fda-referral-reply-body", "第二版");
	await submit("記下回覆");
	await until(() => requests.length === 3, "改過內容");
	const ids = requests.map((r) => r.requestId as string);
	for (const id of ids) assert.match(id, UUID_V4);
	assert.equal(ids[0], ids[1], "同一份內容重送沿用同一個");
	assert.notEqual(ids[1], ids[2], "內容改了換新的");
});

test("記錄轉報：追蹤日期選填，有填就帶 follow_up_date；沒填不帶（伺服器取 30 天後）", async () => {
	const requests: Req[] = [];
	await openPage(
		transportWith({
			getCase: () => ({ case: kase({ canRecordFdaReferral: true }) }),
			recordFDAReferral: (req) => {
				requests.push(req);
				return { case: kase({}), alreadyRecorded: false };
			},
		}),
	);
	for (const followUp of ["2026-11-30", ""]) {
		await click("記錄已轉報主管機關");
		assert.match(bodyText(), /追蹤日期（預設 30 天後）/u);
		await fill(
			"record-fda-referral-channel",
			String(FDAChannel.FDA_CHANNEL_OTHER),
		);
		if (followUp) await fill("record-fda-referral-follow-up", followUp);
		await submit("記錄轉報");
		await until(
			() => maybeById("record-fda-referral-channel") === null,
			"對話框關閉",
		);
	}
	assert.deepEqual(plain(requests[0].followUpDate), {
		year: 2026,
		month: 11,
		day: 30,
	});
	assert.equal(requests[1].followUpDate, undefined);
});
