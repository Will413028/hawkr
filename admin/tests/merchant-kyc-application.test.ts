import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { createQueryOptions } from "@connectrpc/connect-query";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	GetKYCApplicationResponseSchema,
	type KYCApplication,
	KYCApplicationSchema,
	KYCRegistrationCheckSchema,
	KYCRegistrationOutcome,
	KYCRegistrationStatus,
	type PayUniRegistration,
	PayUniRegistrationSchema,
	RevealKYCSensitiveResponseSchema,
	SellerAdminService,
	StoreActionUnavailabilitySchema,
	StoreActionUnavailableReason,
	StoreSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCField,
	KYCFlowStatus,
	KYCIssueCode,
	KYCStatus,
	PayUniBindStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient } from "@tanstack/react-query";
import {
	activeReveal,
	applyRequeryResult,
	documentSrc,
	draftAfter,
	EMPTY_REVIEW_DRAFT,
	gcisView,
	idCardNoField,
	isDocumentField,
	KYC_CONFLICT_TEXT,
	kycApplicationOf,
	kycApplicationQueryKey,
	kycFieldLabel,
	payUniOf,
	payUniPollInterval,
	type ReviewDraft,
	ratingText,
	rebindPresentation,
	registrationOverdue,
	registrationPollInterval,
	registrationWarning,
	revealFrom,
	revealRemainingMs,
	revealUntilText,
	reviewActions,
	reviewBlockedLabel,
	sensitiveValue,
	writeRequeryResult,
} from "../src/app/admin/merchants/[id]/kyc-application";
import { runAction } from "../src/lib/connect-action";

/**
 * 商家詳情的商家認證（KYC）審核與 PayUni：顯示判斷（kyc-application.ts）與實際渲染出來的 HTML。
 * 釘的是：核准與退回只在審核中、版本衝突時輸入不丟、顯示全文前後與到期收回、綁定失敗時是「重新綁定」而不是核准、
 * GCIS 並列的不一致與無法比對標記、背景工作（登記資料查詢、PayUni 綁定）的退避輪詢。退回的逐項問題在 merchant-kyc-issues.test.ts，
 * 顯示全文的到期與放大圖在 merchant-reveal-state.test.ts。所有時間都是固定的值（不讀系統時鐘）。
 */

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

interface ReactLike {
	createElement: (
		type: unknown,
		props?: unknown,
		...children: unknown[]
	) => unknown;
}
const { createElement } = requireFromTest("react") as ReactLike;
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const cards = requireFromTest(
	"../src/app/admin/merchants/[id]/KycApplicationCards",
) as Record<string, unknown>;
const payuniCards = requireFromTest(
	"../src/app/admin/merchants/[id]/PayUniCards",
) as Record<string, unknown>;

const render = (component: unknown, props: Record<string, unknown>): string =>
	renderToStaticMarkup(createElement(component, props));

// ---------------------------------------------------------------------------
// 測試資料

const COMPANY = KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS;

function application(
	patch: MessageInitShape<typeof KYCApplicationSchema> = {},
): KYCApplication {
	return create(KYCApplicationSchema, {
		id: "app1",
		storeId: "100",
		accountId: "7",
		status: KYCStatus.KYC_STATUS_SUBMITTED,
		etag: "etag-1",
		realName: "王小明",
		bankName: "測試銀行",
		bankCode: "004",
		bankAccountMasked: "****5678",
		subject: {
			case: "individual",
			value: { idCardNoMasked: "A12****789" },
		},
		documents: [
			{ field: KYCField.KYC_FIELD_ID_CARD_FRONT, position: 0 },
			{ field: KYCField.KYC_FIELD_ID_CARD_BACK, position: 0 },
			{ field: KYCField.KYC_FIELD_BANK_BOOK, position: 0 },
		],
		...patch,
	});
}

function businessApplication(
	patch: MessageInitShape<typeof KYCApplicationSchema> = {},
): KYCApplication {
	return application({
		subject: {
			case: "business",
			value: {
				companyName: "範例生鮮有限公司",
				taxId: "12345678",
				repIdCardNoMasked: "B22****456",
				companyRegisterDate: { year: 2020, month: 3, day: 5 },
			},
		},
		documents: [
			{ field: COMPANY, position: 0 },
			{ field: COMPANY, position: 1 },
		],
		...patch,
	});
}

const QUERY_TIME = new Date("2026-10-11T04:30:00Z"); // 台北 12:30

function foundCheck(
	patch: MessageInitShape<typeof KYCRegistrationCheckSchema> = {},
) {
	const base: MessageInitShape<typeof KYCRegistrationCheckSchema> = {
		seq: BigInt(1),
		outcome: KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_FOUND,
		queryTime: timestampFromDate(QUERY_TIME),
		registration: {
			taxId: "12345678",
			name: "範例生鮮有限公司",
			address: "臺北市中正區忠孝東路一段1號",
			setupDate: { year: 2020, month: 3, day: 5 },
			organizationType: "有限公司",
			status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE,
			statusText: "核准設立",
		},
	};
	return create(KYCRegistrationCheckSchema, {
		...base,
		...patch,
	} as MessageInitShape<typeof KYCRegistrationCheckSchema>);
}

const EXPIRE = new Date("2026-10-11T05:00:00Z"); // 台北 13:00
const BEFORE_EXPIRE = new Date("2026-10-11T04:59:59Z");

function revealResponse() {
	return create(RevealKYCSensitiveResponseSchema, {
		idCardNo: "A123456789",
		repIdCardNo: "B223456789",
		bankAccount: "0012345678",
		expireTime: timestampFromDate(EXPIRE),
		documents: [
			{
				field: KYCField.KYC_FIELD_ID_CARD_FRONT,
				position: 0,
				viewUri:
					"/api/files/admin/seller/kyc_applications/1/documents/id_card_front/0",
			},
			{
				field: KYCField.KYC_FIELD_ID_CARD_BACK,
				position: 0,
				viewUri:
					"/api/files/admin/seller/kyc_applications/1/documents/id_card_back/0",
			},
			{
				field: KYCField.KYC_FIELD_BANK_BOOK,
				position: 0,
				viewUri:
					"/api/files/admin/seller/kyc_applications/1/documents/bank_book/0",
			},
			{
				field: COMPANY,
				position: 0,
				viewUri:
					"/api/files/admin/seller/kyc_applications/1/documents/company_document/0",
			},
			{
				field: COMPANY,
				position: 1,
				viewUri:
					"/api/files/admin/seller/kyc_applications/1/documents/company_document/1",
			},
		],
	});
}

const noop = () => {};

/** 判斷「查詢是否超過期限」用的固定現在時間（早於 deadline）。 */
const NOW = new Date("2026-10-11T04:31:00Z");

// ---------------------------------------------------------------------------
// 核准、退回與重新查詢：只讀伺服器的旗標與原因（前提 2026-10-15）

const U = StoreActionUnavailableReason;
const why = (reason: StoreActionUnavailableReason, message?: string) =>
	create(StoreActionUnavailabilitySchema, {
		reason,
		message: message ? { message, locale: "zh-TW" } : undefined,
	});

// 伺服器真的會送的句子（model.StoreActionUnavailableText）。
const CLOSED_TEXT =
	"店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。";
const PENDING_DELETION_TEXT = "店家等待刪除，這份商家認證申請不再審核。";
const SUSPENDED_TEXT = "帳號停權中，停權解除之後才能核准商家認證。";
const NOT_SUBMITTED_TEXT = "只有審核中的申請能核准或退回。";

/** 伺服器對「審核中、營運中、沒有停權」的個人型申請送出的規則。 */
function reviewable(
	patch: MessageInitShape<typeof KYCApplicationSchema> = {},
): KYCApplication {
	return application({
		canApprove: true,
		canReject: true,
		canRequeryRegistration: false,
		requeryRegistrationUnavailabilities: [
			why(U.KYC_NOT_BUSINESS, "只有公司型的申請有登記資料可以查詢。"),
		],
		...patch,
	});
}

test("核准與退回只讀伺服器的旗標：不從申請狀態推導（已核准的申請伺服器說能核准就是能，審核中說不能就是不能）", () => {
	const all = Object.values(KYCStatus).filter(
		(v): v is KYCStatus => typeof v === "number",
	);
	assert.ok(all.length >= 5, "enum 讀不到值，測試對象已改變");
	for (const status of all) {
		const yes = reviewActions(
			reviewable({ status, canApprove: true, canReject: true }),
		);
		assert.equal(yes.canApprove, true, `核准 @ ${KYCStatus[status]}`);
		assert.equal(yes.canReject, true, `退回 @ ${KYCStatus[status]}`);
		const no = reviewActions(
			reviewable({ status, canApprove: false, canReject: false }),
		);
		assert.equal(no.canApprove, false, `核准 @ ${KYCStatus[status]}`);
		assert.equal(no.canReject, false, `退回 @ ${KYCStatus[status]}`);
	}
});

test("申請還沒讀到、沒有 ID 或 etag：不能核准與退回（資料完整性，不是業務規則）", () => {
	const none = { canApprove: false, canReject: false, canRequery: false };
	assert.deepEqual(reviewActions(undefined), none);
	assert.equal(reviewActions(reviewable({ etag: "" })).canApprove, false);
	assert.equal(reviewActions(reviewable({ etag: "" })).canReject, false);
	assert.equal(reviewActions(reviewable({ id: "" })).canApprove, false);
	assert.equal(reviewActions(reviewable({ id: "" })).canReject, false);
});

test("停權時伺服器只擋核准：可以退回，原因是停權的那一句", () => {
	const suspended = reviewable({
		canApprove: false,
		approveUnavailabilities: [why(U.ACCOUNT_SUSPENDED, SUSPENDED_TEXT)],
	});
	const actions = reviewActions(suspended);
	assert.equal(actions.canApprove, false);
	assert.equal(actions.canReject, true);
	assert.equal(reviewBlockedLabel(suspended), SUSPENDED_TEXT);
});

test("店家已關閉或等待刪除：核准、退回都不能，提示是那一句", () => {
	for (const text of [CLOSED_TEXT, PENDING_DELETION_TEXT]) {
		const reason =
			text === CLOSED_TEXT ? U.STORE_CLOSED : U.STORE_PENDING_DELETION;
		const closed = reviewable({
			canApprove: false,
			canReject: false,
			approveUnavailabilities: [why(reason, text)],
			rejectUnavailabilities: [why(reason, text)],
			requeryRegistrationUnavailabilities: [why(reason, text)],
		});
		assert.deepEqual(reviewActions(closed), {
			canApprove: false,
			canReject: false,
			canRequery: false,
		});
		assert.equal(reviewBlockedLabel(closed), text);
	}
});

test("真實的伺服器形狀：終止合作關店（帳號仍有生效中的措施）核准列 [已關閉, 停權]、等待刪除加停權列 [等待刪除, 停權]：只顯示第一句，不會同時說「暫停審核」與「停權解除之後才能核准」", () => {
	for (const [first, text] of [
		[U.STORE_CLOSED, CLOSED_TEXT],
		[U.STORE_PENDING_DELETION, PENDING_DELETION_TEXT],
	] as const) {
		const app = reviewable({
			canApprove: false,
			canReject: false,
			approveUnavailabilities: [
				why(first, text),
				why(U.ACCOUNT_SUSPENDED, SUSPENDED_TEXT),
			],
			rejectUnavailabilities: [why(first, text)],
		});
		const label = reviewBlockedLabel(app);
		assert.equal(label, text);
		assert.notEqual(label, SUSPENDED_TEXT);
	}
});

test("第一個原因是結構性的（申請不在審核中）：沒有入口、不顯示任何句子，即使後面還有別的原因；可以核准時沒有句子", () => {
	const notSubmitted = reviewable({
		status: KYCStatus.KYC_STATUS_APPROVED,
		canApprove: false,
		canReject: false,
		approveUnavailabilities: [
			why(U.KYC_NOT_SUBMITTED, NOT_SUBMITTED_TEXT),
			why(U.STORE_CLOSED, CLOSED_TEXT),
		],
		rejectUnavailabilities: [
			why(U.KYC_NOT_SUBMITTED, NOT_SUBMITTED_TEXT),
			why(U.STORE_CLOSED, CLOSED_TEXT),
		],
	});
	assert.equal(reviewBlockedLabel(notSubmitted), undefined);
	assert.equal(reviewBlockedLabel(reviewable()), undefined);
	assert.equal(reviewBlockedLabel(undefined), undefined);
});

test("重新查詢只讀 can_requery_registration：公司型審核中的申請才有", () => {
	const business = businessApplication({
		canApprove: true,
		canReject: true,
		canRequeryRegistration: true,
	});
	assert.equal(reviewActions(business).canRequery, true);
	assert.equal(reviewActions(reviewable()).canRequery, false);
	// 旗標是唯一來源：申請狀態與賣家型態不參與。
	assert.equal(
		reviewActions(
			businessApplication({
				status: KYCStatus.KYC_STATUS_APPROVED,
				canRequeryRegistration: false,
			}),
		).canRequery,
		false,
	);
});

// ---------------------------------------------------------------------------
// 版本衝突：輸入保留

/** 一份填到一半的退回草稿。 */
function aDraft(): ReviewDraft {
	return {
		issues: [
			{
				key: "issue-1",
				field: KYCField.KYC_FIELD_REAL_NAME,
				documentAssetId: "",
				documentLabel: "",
				code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
				message: "姓名與證件不符",
			},
		],
		rejectReason: "身分證已過期",
		internalNote: "備註",
	};
}

function versionConflict(): ConnectError {
	return new ConnectError("etag mismatch", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
		},
	]);
}

test("VERSION_CONFLICT：重讀申請一次、回 conflict 的說明；草稿（逐項問題、退回理由、內部備註）原封不動", async () => {
	let reloads = 0;
	const result = await runAction(() => Promise.reject(versionConflict()), {
		reload: async () => {
			reloads++;
		},
		fallback: "退回失敗",
		conflictText: KYC_CONFLICT_TEXT,
	});
	assert.deepEqual(result, {
		ok: false,
		kind: "conflict",
		text: KYC_CONFLICT_TEXT,
		code: Code.Aborted,
		noResponse: false,
	});
	assert.equal(reloads, 1);

	const draft = aDraft();
	assert.equal(draftAfter(draft, result), draft, "同一個物件：沒有被重設");
	assert.equal(draftAfter(draft, result).rejectReason, "身分證已過期");
	assert.equal(draftAfter(draft, result).internalNote, "備註");
	assert.deepEqual(
		draftAfter(draft, result).issues.map((i) => i.field),
		[KYCField.KYC_FIELD_REAL_NAME],
		"逐項問題也保留",
	);
});

test("重讀本身失敗也不丟例外，結果仍是 conflict（輸入照樣保留）；不說「已重新讀取」，改回伺服器的句子", async () => {
	const result = await runAction(() => Promise.reject(versionConflict()), {
		reload: () => Promise.reject(new Error("網路")),
		fallback: "核准失敗",
		conflictText: KYC_CONFLICT_TEXT,
	});
	assert.equal(result.ok, false);
	assert.equal(!result.ok && result.kind, "conflict");
	assert.ok(!result.ok && result.text !== KYC_CONFLICT_TEXT);
	assert.ok(!result.ok && result.text.length > 0);
});

test("其他失敗：顯示伺服器的訊息、不重讀、草稿保留；網路層失敗用 fallback", async () => {
	let reloads = 0;
	const reload = async () => {
		reloads++;
	};
	const rejected = new ConnectError(
		"identity conflict",
		Code.FailedPrecondition,
		undefined,
		[
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "這份申請目前無法核准" },
			},
		],
	);
	const message = await runAction(() => Promise.reject(rejected), {
		reload,
		fallback: "核准失敗",
		conflictText: KYC_CONFLICT_TEXT,
	});
	assert.deepEqual(message, {
		ok: false,
		kind: "message",
		text: "這份申請目前無法核准",
		code: Code.FailedPrecondition,
		noResponse: false,
	});
	const network = await runAction(
		() => Promise.reject(new TypeError("Failed to fetch")),
		{
			reload,
			fallback: "核准失敗，請確認網路",
			conflictText: KYC_CONFLICT_TEXT,
		},
	);
	assert.deepEqual(network, {
		ok: false,
		kind: "message",
		text: "核准失敗，請確認網路",
		noResponse: true,
	});
	assert.equal(reloads, 0, "非版本衝突不重讀");
	const draft = aDraft();
	assert.equal(draftAfter(draft, message), draft);
	assert.equal(draftAfter(draft, network), draft);
});

test("成功才清空草稿", async () => {
	const result = await runAction(async () => undefined, {
		reload: async () => undefined,
		fallback: "x",
		conflictText: KYC_CONFLICT_TEXT,
	});
	assert.deepEqual(result, { ok: true, value: undefined });
	assert.equal(draftAfter(aDraft(), result), EMPTY_REVIEW_DRAFT);
});

// ---------------------------------------------------------------------------
// 退回欄位對應 KYCField

test("退回欄位的文字窮舉 KYCField：每個非 UNSPECIFIED 的值都有文字，新增 enum 值時這裡會失敗", () => {
	const fields = Object.values(KYCField).filter(
		(v): v is KYCField => typeof v === "number",
	);
	const expected = fields.filter((f) => f !== KYCField.KYC_FIELD_UNSPECIFIED);
	assert.ok(expected.length > 10, "enum 讀不到值，測試對象已改變");
	for (const field of expected) {
		const text = kycFieldLabel(field);
		assert.ok(
			text !== "" && text !== "未知",
			`${KYCField[field]} 在共用文案目錄沒有文字`,
		);
	}
	assert.equal(kycFieldLabel(KYCField.KYC_FIELD_UNSPECIFIED), "未知");
	assert.equal(kycFieldLabel(9999 as KYCField), "未知");
	// 與賣家端同一組名詞（L0 A1 收斂字）。
	assert.equal(kycFieldLabel(KYCField.KYC_FIELD_REAL_NAME), "真實姓名");
	assert.equal(kycFieldLabel(KYCField.KYC_FIELD_BIRTH_DATE), "出生日期");
	assert.equal(kycFieldLabel(KYCField.KYC_FIELD_WEBSITE_URL), "官方網站");
	assert.equal(kycFieldLabel(KYCField.KYC_FIELD_ID_CARD_FRONT), "身分證正面照");
	assert.equal(
		kycFieldLabel(KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS),
		"補充文件",
	);
});

test("文件類的欄位只有四個（證件正反面、存摺、補充文件）：申請上的文件與退回的問題用同一套欄位表示，沒有另一套證件位置", () => {
	const documentFields = Object.values(KYCField)
		.filter((v): v is KYCField => typeof v === "number")
		.filter(isDocumentField);
	assert.deepEqual(
		documentFields.sort((x, y) => x - y),
		[
			KYCField.KYC_FIELD_ID_CARD_FRONT,
			KYCField.KYC_FIELD_ID_CARD_BACK,
			KYCField.KYC_FIELD_BANK_BOOK,
			KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
		],
	);
	assert.equal(isDocumentField(KYCField.KYC_FIELD_REAL_NAME), false);
	assert.equal(isDocumentField(KYCField.KYC_FIELD_UNSPECIFIED), false);
});

test("身分證字號的退回欄位：個人是 INDIVIDUAL_ID_CARD_NO，企業是負責人的 BUSINESS_REP_ID_CARD_NO", () => {
	assert.equal(
		idCardNoField(application()),
		KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO,
	);
	assert.equal(
		idCardNoField(businessApplication()),
		KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO,
	);
});

// ---------------------------------------------------------------------------
// 顯示全文：前後、到期收回

test("顯示全文前：身分證字號與銀行帳號是遮蔽值，證件位置是「需先顯示全文」，沒有任何影像網址", () => {
	const html =
		render(cards.KycIdentityCard as never, {
			application: application(),
			reveal: undefined,
			onPreview: noop,
		}) +
		render(cards.KycDocumentsCard as never, {
			application: application(),
			reveal: undefined,
			onPreview: noop,
		});
	assert.match(html, /A12\*\*\*\*789/u);
	assert.match(html, /\*\*\*\*5678/u);
	assert.doesNotMatch(html, /A123456789|0012345678/u, "不得出現明文");
	assert.equal(
		(html.match(/需先顯示全文/gu) ?? []).length,
		3,
		"身分證正反面與存摺各一處",
	);
	assert.doesNotMatch(html, /\/api\/bff|<img/u, "沒有顯示全文前不載入任何影像");
});

test("顯示全文後：明文與影像（同源 /api/bff 加上伺服器給的 view_uri），沒有直連後端", () => {
	const reveal = revealFrom("app1", revealResponse());
	assert.ok(reveal);
	const html =
		render(cards.KycIdentityCard as never, {
			application: application(),
			reveal,
			onPreview: noop,
		}) +
		render(cards.KycDocumentsCard as never, {
			application: application(),
			reveal,
			onPreview: noop,
		});
	assert.match(html, /A123456789/u, "個人的身分證字號用 id_card_no");
	assert.doesNotMatch(html, /B223456789/u, "個人申請不顯示負責人的號碼");
	assert.match(html, /0012345678/u);
	assert.doesNotMatch(html, /A12\*\*\*\*789/u);
	assert.doesNotMatch(html, /需先顯示全文/u);
	assert.match(
		html,
		/src="[^"]*api%2Fbff[^"]*documents%2Fid_card_front%2F0|\/api\/bff\/api\/files\/admin\/seller\/kyc_applications\/1\/documents\/id_card_front\/0/u,
	);
	for (const src of html.match(/src="[^"]*"/gu) ?? []) {
		assert.doesNotMatch(src, /^src="https?:/u, `不得直連：${src}`);
	}
});

test("證件標籤取自目錄（個人與企業同一組字）：身分證正面照、身分證反面照；企業的「負責人」由區塊標題表達", () => {
	for (const [app, title] of [
		[application(), "賣家身分資訊"],
		[businessApplication(), "公司負責人身分資訊"],
	] as const) {
		const html = render(cards.KycIdentityCard as never, {
			application: app,
			reveal: undefined,
			onPreview: noop,
		});
		assert.match(html, /身分證正面照/u);
		assert.match(html, /身分證反面照/u);
		assert.doesNotMatch(html, /身分證\(正\)|身分證\(反\)/u);
		assert.match(html, new RegExp(title, "u"));
	}
});

test("企業申請顯示全文：身分證字號欄是負責人的 rep_id_card_no", () => {
	const reveal = revealFrom("app1", revealResponse());
	const html = render(cards.KycIdentityCard as never, {
		application: businessApplication(),
		reveal,
		onPreview: noop,
	});
	assert.match(html, /B223456789/u);
	assert.doesNotMatch(html, /A123456789/u);
});

test("到期收回：到期前仍有效，到期那一刻起回到遮蔽值與「需先顯示全文」", () => {
	const reveal = revealFrom("app1", revealResponse());
	assert.ok(reveal);
	assert.ok(activeReveal(reveal, "app1", BEFORE_EXPIRE), "到期前還在");
	assert.equal(
		activeReveal(reveal, "app1", EXPIRE),
		undefined,
		"剛好到期就收回",
	);
	assert.equal(
		activeReveal(reveal, "app1", new Date(EXPIRE.getTime() + 1)),
		undefined,
	);
	assert.equal(revealRemainingMs(reveal, BEFORE_EXPIRE), 1000);
	assert.equal(revealRemainingMs(reveal, new Date(EXPIRE.getTime() + 5000)), 0);

	const render1 = (now: Date) =>
		render(cards.KycIdentityCard as never, {
			application: application(),
			reveal: activeReveal(reveal, "app1", now),
			onPreview: noop,
		});
	assert.match(render1(BEFORE_EXPIRE), /A123456789/u);
	const expired = render1(EXPIRE);
	assert.doesNotMatch(expired, /A123456789/u);
	assert.match(expired, /A12\*\*\*\*789/u);
	assert.match(expired, /需先顯示全文/u);
});

test("別份申請的顯示全文不套用；回應沒有到期時間不當成可顯示", () => {
	const reveal = revealFrom("app1", revealResponse());
	assert.equal(activeReveal(reveal, "app2", BEFORE_EXPIRE), undefined);
	assert.equal(activeReveal(reveal, undefined, BEFORE_EXPIRE), undefined);
	assert.equal(activeReveal(null, "app1", BEFORE_EXPIRE), undefined);
	assert.equal(
		revealFrom(
			"app1",
			create(RevealKYCSensitiveResponseSchema, { idCardNo: "X" }),
		),
		undefined,
		"沒有期限的明文不存在",
	);
	assert.equal(revealFrom("", revealResponse()), undefined, "沒有申請 ID");
});

test("「全文顯示到 HH:mm」是台北時間", () => {
	const reveal = revealFrom("app1", revealResponse());
	assert.ok(reveal);
	assert.equal(revealUntilText(reveal), "全文顯示到 13:00");
	const html = render(cards.KycRevealBar as never, {
		reveal,
		onReveal: noop,
		onHide: noop,
	});
	assert.match(html, /全文顯示到 13:00/u);
	assert.match(html, /重新顯示全文/u);
	assert.match(html, /收起全文/u, "顯示中可以提早收起");
	const before = render(cards.KycRevealBar as never, {
		reveal: undefined,
		onReveal: noop,
		onHide: noop,
	});
	assert.match(before, /顯示全文/u);
	assert.doesNotMatch(before, /全文顯示到|重新顯示全文|收起全文/u);
});

test("敏感值：明文是空字串時退回遮蔽值，兩者都沒有回 -", () => {
	assert.equal(sensitiveValue("A12****789", "A123456789", true), "A123456789");
	assert.equal(sensitiveValue("A12****789", "A123456789", false), "A12****789");
	assert.equal(sensitiveValue("A12****789", "", true), "A12****789");
	assert.equal(sensitiveValue("", "", true), "-");
});

test("證件影像網址只收伺服器的相對路徑，經同源 /api/bff；絕對網址、協定相對、.. 都不收", () => {
	assert.equal(
		documentSrc(
			"/api/files/admin/seller/kyc_applications/1/documents/bank_book/0",
		),
		"/api/bff/api/files/admin/seller/kyc_applications/1/documents/bank_book/0",
	);
	for (const bad of [
		"",
		"https://gateway.example.com/api/files/admin/x",
		"//evil.example.com/api/files/admin/x",
		"/api/files/admin/../../secret",
		"/api/files/app/x/1",
		"/api/files/admin/x?token=1",
		"/api/files/admin/x y",
	]) {
		assert.equal(documentSrc(bad), undefined, bad);
	}
});

test("顯示全文後影像的 view_uri 不合法：該位置顯示「無法檢視」，不猜網址", () => {
	const res = create(RevealKYCSensitiveResponseSchema, {
		idCardNo: "A123456789",
		expireTime: timestampFromDate(EXPIRE),
		documents: [
			{
				field: KYCField.KYC_FIELD_ID_CARD_FRONT,
				position: 0,
				viewUri: "https://gateway.example.com/x",
			},
		],
	});
	const html = render(cards.KycIdentityCard as never, {
		application: application(),
		reveal: revealFrom("app1", res),
		onPreview: noop,
	});
	assert.doesNotMatch(html, /gateway\.example\.com|<img/u);
	assert.match(html, /無法檢視/u);
});

// ---------------------------------------------------------------------------
// GCIS 並列（併在基本資料卡的右欄）

const STORE = create(StoreSchema, {
	id: "100",
	storeName: "範例鮮果",
	contactPhone: "02-12345678",
	supportEmail: "seller@example.com",
});

function renderBasicInfo(
	app: KYCApplication,
	patch: Record<string, unknown> = {},
): string {
	return render(cards.KycBasicInfoCard as never, {
		store: STORE,
		application: app,
		now: NOW,
		canRequery: false,
		requerying: false,
		onRequery: noop,
		...patch,
	});
}

/** 進行中的查詢：要求時間台北 12:31，期限台北 13:00 以前由測試指定。 */
function pendingCheck(deadline: Date) {
	return create(KYCRegistrationCheckSchema, {
		seq: BigInt(2),
		requestTime: timestampFromDate(new Date("2026-10-11T04:31:00Z")),
		deadlineTime: timestampFromDate(deadline),
	});
}

test("不一致與無法比對各有標記，其餘列沒有；查到的登記值與申請值並列", () => {
	const app = businessApplication({
		registrationCheck: foundCheck({
			mismatchedFields: [KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME],
			unverifiableFields: [KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS],
			registration: {
				taxId: "12345678",
				name: "範例鮮果有限公司",
				address: "",
				organizationType: "有限公司",
				status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE,
				statusText: "核准設立",
			},
		}),
	});
	const view = gcisView(app, NOW);
	assert.ok(view);
	const marks = Object.fromEntries(view.rows.map((r) => [r.key, r.mark]));
	assert.deepEqual(marks, {
		company_name: "mismatch",
		tax_id: undefined,
		company_address: "unverifiable",
		company_register_date: undefined,
		organization_type: undefined,
		registration_status: undefined,
	});
	const name = view.rows.find((r) => r.key === "company_name");
	assert.equal(name?.applicant, "範例生鮮有限公司");
	assert.equal(name?.registry, "範例鮮果有限公司");
	assert.equal(
		view.rows.find((r) => r.key === "company_address")?.registry,
		"-",
		"無法比對的來源值缺漏",
	);

	const html = renderBasicInfo(app);
	assert.equal((html.match(/不一致/gu) ?? []).length, 1);
	assert.equal((html.match(/無法比對/gu) ?? []).length, 1);
	assert.match(html, /範例鮮果有限公司/u);
	assert.match(html, /範例生鮮有限公司/u, "申請的值在左欄");
	assert.match(html, /商業登記（GCIS）/u, "右欄的欄頭");
	assert.match(
		html,
		/組織型態/u,
		"組織型態列顯示 GCIS 快照的 organization_type",
	);
	assert.match(html, /查詢時間 2026\/10\/11 12:30・查到/u);
});

test("GCIS 並列併在申請內容的基本資料卡裡，沿用 info-grid／info-field，沒有另外的卡片或表格", () => {
	const html = renderBasicInfo(
		businessApplication({ registrationCheck: foundCheck() }),
	);
	assert.equal(
		(html.match(/class="detail-card"/gu) ?? []).length,
		1,
		"只有基本資料這一張卡",
	);
	assert.doesNotMatch(html, /<table/u);
	assert.match(html, /class="info-field"/u);
	assert.equal(
		renderBasicInfo(application()).includes("商業登記（GCIS）"),
		false,
		"個人型申請沒有並列",
	);
});

test("沒有任何標記時不畫「不一致」「無法比對」", () => {
	const html = renderBasicInfo(
		businessApplication({ registrationCheck: foundCheck() }),
	);
	assert.doesNotMatch(html, /不一致|無法比對/u);
});

test("沒有查到（NOT_FOUND、UNAVAILABLE）：欄頭說明結果，登記值是 -，即使 mismatched 有值也不標記", () => {
	const notFound = gcisView(
		businessApplication({
			registrationCheck: foundCheck({
				outcome: KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_NOT_FOUND,
				registration: undefined,
				mismatchedFields: [KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME],
			}),
		}),
		NOW,
	);
	assert.deepEqual(notFound?.header, {
		kind: "notFound",
		text: "查詢時間 2026/10/11 12:30・查無此統編",
	});
	assert.ok(notFound?.rows.every((r) => r.mark === undefined));
	assert.equal(
		notFound?.rows.find((r) => r.key === "company_name")?.registry,
		"-",
	);

	const unavailable = gcisView(
		businessApplication({
			registrationCheck: foundCheck({
				outcome: KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_UNAVAILABLE,
				registration: undefined,
			}),
		}),
		NOW,
	);
	assert.deepEqual(unavailable?.header, {
		kind: "unavailable",
		text: "查詢時間 2026/10/11 12:30・查詢失敗",
	});
});

test("欄頭：查詢中帶「最晚 HH:mm 會有結果」、超過期限提示重新查詢、沒有登記資料（含 outcome 是 UNSPECIFIED）；個人申請沒有並列", () => {
	const deadline = new Date("2026-10-11T05:00:00Z"); // 台北 13:00
	const pending = gcisView(
		businessApplication({ pendingRegistrationCheck: pendingCheck(deadline) }),
		NOW,
	);
	assert.deepEqual(pending?.header, {
		kind: "pending",
		text: "登記資料查詢中…",
		detail: "最晚 13:00 會有結果",
		previous: undefined,
	});
	const withOlder = gcisView(
		businessApplication({
			pendingRegistrationCheck: pendingCheck(deadline),
			registrationCheck: foundCheck(),
		}),
		NOW,
	);
	assert.equal(withOlder?.header.kind, "pending");
	assert.equal(
		withOlder?.header.previous,
		"查詢時間 2026/10/11 12:30・查到",
		"查詢中時上一筆仍標明",
	);
	// 到了期限還沒有終態：不再說「查詢中」，提示重新查詢。
	const overdue = gcisView(
		businessApplication({ pendingRegistrationCheck: pendingCheck(deadline) }),
		deadline,
	);
	assert.deepEqual(overdue?.header, {
		kind: "overdue",
		text: "登記資料查詢超過預定時間仍未完成",
		detail: "請按「重新查詢」重新送出。",
		previous: undefined,
	});
	assert.deepEqual(gcisView(businessApplication(), NOW)?.header, {
		kind: "none",
		text: "沒有登記資料",
	});
	assert.equal(
		gcisView(
			businessApplication({
				registrationCheck: foundCheck({
					outcome: KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_UNSPECIFIED,
				}),
			}),
			NOW,
		)?.header.kind,
		"none",
		"UNSPECIFIED 不是結果",
	);
	assert.equal(gcisView(application(), NOW), undefined);
});

test("登記現況警示：非核准設立、或伺服器標記查詢當天在停業期間內（suspended_at_query）才顯示，文字帶停業期間；前端不自己比對日期", () => {
	assert.equal(registrationWarning(foundCheck()), undefined, "核准設立不警示");
	assert.equal(registrationWarning(undefined), undefined);
	const dissolved = foundCheck({
		registration: {
			status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_TERMINATED,
			statusText: "解散",
			suspendStartDate: { year: 2026, month: 1, day: 1 },
			suspendEndDate: { year: 2026, month: 12, day: 31 },
		},
	});
	assert.equal(
		registrationWarning(dissolved),
		"解散（停業期間 2026-01-01 至 2026-12-31）",
	);
	const suspendedNow = foundCheck({
		registration: {
			status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE,
			statusText: "核准設立",
			suspendStartDate: { year: 2026, month: 10, day: 1 },
			suspendEndDate: { year: 2026, month: 10, day: 31 },
			suspendedAtQuery: true,
		},
	});
	assert.equal(
		registrationWarning(suspendedNow),
		"核准設立（停業期間 2026-10-01 至 2026-10-31）",
	);
	const suspendedBefore = foundCheck({
		registration: {
			status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE,
			statusText: "核准設立",
			suspendStartDate: { year: 2025, month: 1, day: 1 },
			suspendEndDate: { year: 2025, month: 3, day: 1 },
		},
	});
	assert.equal(
		registrationWarning(suspendedBefore),
		undefined,
		"停業期間已結束且現況是核准設立：不警示",
	);
	// 日期落在期間內但伺服器沒有標記：只信伺服器（日期的判斷只有一處）。
	const datesInsideButNotFlagged = foundCheck({
		registration: {
			status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_ACTIVE,
			statusText: "核准設立",
			suspendStartDate: { year: 2026, month: 10, day: 1 },
			suspendEndDate: { year: 2026, month: 10, day: 31 },
			suspendedAtQuery: false,
		},
	});
	assert.equal(registrationWarning(datesInsideButNotFlagged), undefined);
	const unknown = foundCheck({
		registration: {
			status: KYCRegistrationStatus.KYC_REGISTRATION_STATUS_UNKNOWN,
			statusText: "不在對照表的現況",
		},
	});
	assert.equal(registrationWarning(unknown), "不在對照表的現況");
	assert.equal(
		registrationWarning(
			foundCheck({
				outcome: KYCRegistrationOutcome.KYC_REGISTRATION_OUTCOME_NOT_FOUND,
				registration: undefined,
			}),
		),
		undefined,
	);
});

test("「重新查詢」只在 canRequery 時出現；查詢中不能再按（伺服器保證有出口），超過期限又可以按", () => {
	const deadline = new Date("2026-10-11T05:00:00Z");
	const base = businessApplication({ registrationCheck: foundCheck() });
	assert.match(renderBasicInfo(base, { canRequery: true }), /重新查詢/u);
	assert.doesNotMatch(
		renderBasicInfo(base, { canRequery: false }),
		/重新查詢/u,
	);
	const pending = businessApplication({
		pendingRegistrationCheck: pendingCheck(deadline),
	});
	const pendingHtml = renderBasicInfo(pending, { canRequery: true });
	assert.match(pendingHtml, /查詢中…/u);
	assert.match(pendingHtml, /disabled/u);
	assert.match(pendingHtml, /登記資料查詢中…/u);
	assert.match(pendingHtml, /最晚 13:00 會有結果/u);
	const overdueHtml = renderBasicInfo(pending, {
		canRequery: true,
		now: deadline,
	});
	assert.doesNotMatch(overdueHtml, /disabled/u, "超過期限時可以重新查詢");
	assert.match(overdueHtml, /登記資料查詢超過預定時間仍未完成/u);
	assert.match(overdueHtml, /重新查詢/u);
});

test("進行中的查詢依伺服器的期限退避輪詢：間隔隨已等待的時間成長、有上限、不晚於期限後一秒；終態與超過期限不再輪詢", () => {
	const request = new Date("2026-10-11T04:31:00Z");
	const deadline = new Date("2026-10-11T06:11:00Z"); // 100 分鐘後
	const pending = pendingCheck(deadline);
	const at = (secondsAfterRequest: number) =>
		registrationPollInterval(
			pending,
			new Date(request.getTime() + secondsAfterRequest * 1000),
		);
	assert.equal(at(0), 3000, "剛要求時用最短的間隔");
	assert.equal(at(60), 15_000, "等越久間隔越長（已等待時間的 1/4）");
	assert.equal(at(600), 60_000, "上限 60 秒");
	assert.equal(at(3600), 60_000);
	// 離期限不到一個間隔：下一次讀取落在期限之後一秒，才看得到期限那一刻的終態。
	assert.equal(at(100 * 60 - 10), 11_000);
	// 期限到了還沒有終態（伺服器的錯誤）：不再輪詢，畫面提示重新查詢。
	assert.equal(at(100 * 60), false);
	assert.equal(at(100 * 60 + 1), false);
	assert.equal(
		registrationOverdue(pending, new Date(deadline.getTime() - 1)),
		false,
	);
	assert.equal(registrationOverdue(pending, deadline), true);
	// 沒有進行中的查詢（已有終態）、或沒有期限（不該發生）都不輪詢。
	assert.equal(registrationPollInterval(undefined, request), false);
	assert.equal(
		registrationPollInterval(
			create(KYCRegistrationCheckSchema, { seq: BigInt(2) }),
			request,
		),
		false,
	);
	assert.equal(
		registrationOverdue(create(KYCRegistrationCheckSchema, {}), request),
		true,
	);
});

// ---------------------------------------------------------------------------
// PayUni

// 伺服器真的會送的重新綁定句子（model.StoreActionUnavailableText）。
const TERMINAL_TEXT =
	"PayUni 端的商店已是終止狀態，這家店無法再綁定；賣家要重新入駐才能收款。";
const REBIND_CLOSED_TEXT = "店家已關閉，不能重新綁定 PayUni。";
const REBIND_PENDING_DELETION_TEXT = "店家等待刪除，不能重新綁定 PayUni。";
const SELLER_EDITING_TEXT = "賣家正在修改商家認證，重新核准後會自動重新綁定。";
const BOUND_TEXT = "只有綁定失敗的註冊能重新綁定。";

/** 伺服器對一筆綁定失敗、可以重新綁定的註冊送出的形狀（階段與可用性來自伺服器，client 不推導）。 */
function registration(
	patch: MessageInitShape<typeof PayUniRegistrationSchema> = {},
): PayUniRegistration {
	return create(PayUniRegistrationSchema, {
		storeId: "100",
		merId: "HAWK100",
		bindStatus: PayUniBindStatus.BOUND,
		flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW,
		canRebind: false,
		rebindUnavailabilities: [why(U.PAYUNI_BOUND, BOUND_TEXT)],
		...patch,
	});
}

function failedRegistration(
	patch: MessageInitShape<typeof PayUniRegistrationSchema> = {},
): PayUniRegistration {
	return registration({
		bindStatus: PayUniBindStatus.FAILED,
		flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED,
		canRebind: true,
		rebindUnavailabilities: [],
		...patch,
	});
}

const NOT_FOUND_RESULT = {
	error: new ConnectError("未註冊", Code.NotFound),
	data: undefined,
};

function renderPayUni(reg: PayUniRegistration): string {
	return render(payuniCards.PayUniStatusCard as never, {
		state: payUniOf({ error: null, data: { registration: reg } }),
		hasApplication: true,
		rebinding: false,
		onRebind: noop,
		onRetry: noop,
	});
}

test("綁定失敗而且伺服器說能重新綁定：卡片顯示「重新綁定」；綁定中、已綁定沒有鍵也沒有句子", () => {
	const failed = failedRegistration({
		bindErrorCode: "AGENT01001",
		bindAttempts: 3,
	});
	assert.deepEqual(rebindPresentation(failed), {
		canRebind: true,
		reason: undefined,
	});
	const html = renderPayUni(failed);
	assert.match(html, /重新綁定/u);
	assert.match(html, /待重新綁定/u, "徽章與清單同一組文字");
	assert.match(html, /AGENT01001/u);
	assert.match(html, /已嘗試 3 次/u);

	// 已綁定（BOUND）：第一個原因是結構性的，伺服器雖然附了句子也不顯示。
	const bound = renderPayUni(registration());
	assert.doesNotMatch(bound, /重新綁定/u);
	assert.ok(!bound.includes(BOUND_TEXT), "結構性的原因不顯示句子");
	// 綁定中（PENDING）：PAYUNI_BIND_IN_PROGRESS 沒有 message（不是失敗原因），也沒有任何句子，不顯示通用文字。
	const pending = registration({
		bindStatus: PayUniBindStatus.PENDING,
		flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_BINDING,
		rebindUnavailabilities: [why(U.PAYUNI_BIND_IN_PROGRESS)],
	});
	assert.deepEqual(rebindPresentation(pending), {
		canRebind: false,
		reason: undefined,
	});
	const pendingHtml = renderPayUni(pending);
	assert.doesNotMatch(pendingHtml, /重新綁定|目前不能操作/u);
});

test("不能重新綁定而且不是結構性的原因：不顯示「重新綁定」，顯示伺服器的第一句（終止、已關閉、等待刪除、賣家正在修改）", () => {
	for (const [reason, text] of [
		[U.PAYUNI_MERCHANT_TERMINAL, TERMINAL_TEXT],
		[U.STORE_CLOSED, REBIND_CLOSED_TEXT],
		[U.STORE_PENDING_DELETION, REBIND_PENDING_DELETION_TEXT],
		[U.KYC_NOT_APPROVED, SELLER_EDITING_TEXT],
	] as const) {
		const reg = failedRegistration({
			canRebind: false,
			rebindUnavailabilities: [why(reason, text)],
		});
		assert.deepEqual(rebindPresentation(reg), {
			canRebind: false,
			reason: text,
		});
		const html = renderPayUni(reg);
		assert.doesNotMatch(html, /hk-btn__text">重新綁定</u, "沒有重新綁定的按鈕");
		assert.ok(html.includes(text), text);
	}
});

test("PayUni 端已終止而且 BOUND 的註冊：第一個原因是終止，仍顯示終止的句子（不被後面的結構性原因吃掉）；只顯示第一句", () => {
	const reg = registration({
		rebindUnavailabilities: [
			why(U.PAYUNI_MERCHANT_TERMINAL, TERMINAL_TEXT),
			why(U.STORE_CLOSED, REBIND_CLOSED_TEXT),
			why(U.PAYUNI_BOUND, BOUND_TEXT),
		],
	});
	assert.equal(rebindPresentation(reg).reason, TERMINAL_TEXT);
	const html = renderPayUni(reg);
	assert.ok(html.includes(TERMINAL_TEXT));
	assert.ok(!html.includes(REBIND_CLOSED_TEXT), "只顯示第一個原因");
});

test("PayUni 綁定中（PENDING）以退避輪詢到終態：間隔隨距離上次更新的時間成長、上限 60 秒、沒有次數上限；BOUND、FAILED 與沒有註冊都不輪詢", () => {
	const updated = new Date("2026-10-11T04:31:00Z");
	const pending = registration({
		bindStatus: PayUniBindStatus.PENDING,
		updateTime: timestampFromDate(updated),
	});
	const at = (seconds: number) =>
		payUniPollInterval(pending, new Date(updated.getTime() + seconds * 1000));
	assert.equal(at(0), 5000, "最短 5 秒");
	assert.equal(at(100), 25_000);
	assert.equal(at(10_000), 60_000, "上限 60 秒");
	assert.equal(at(86_400), 60_000, "沒有次數上限：一天之後仍在輪詢");
	for (const bindStatus of [
		PayUniBindStatus.BOUND,
		PayUniBindStatus.FAILED,
		PayUniBindStatus.UNSPECIFIED,
	]) {
		assert.equal(
			payUniPollInterval(registration({ bindStatus }), updated),
			false,
			PayUniBindStatus[bindStatus],
		);
	}
	assert.equal(payUniPollInterval(undefined, updated), false);
});

test("綁定失敗時卡片沒有「核准」入口：只有重新綁定（核准是申請的事，不在 PayUni 卡）", () => {
	const html = renderPayUni(failedRegistration());
	assert.doesNotMatch(html, /通過審核|核准/u);
	assert.match(html, /重新綁定/u);
});

test("PayUni 徽章只讀伺服器給的階段（flow_status），與清單同一組文字；不從 bind_status、status_codes 推導", () => {
	const expected: [KYCFlowStatus, string][] = [
		[KYCFlowStatus.KYC_FLOW_STATUS_BINDING, "PayUni 綁定中"],
		[KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED, "待重新綁定"],
		[KYCFlowStatus.KYC_FLOW_STATUS_APPROVED, "已核准"],
		[KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW, "PayUni 審核中"],
		[KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO, "PayUni 待補件"],
		[KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED, "PayUni 審核未通過"],
	];
	for (const [flowStatus, label] of expected) {
		// bindStatus 故意放一個與階段矛盾的值：徽章不得看它。
		const html = renderPayUni(
			registration({ flowStatus, bindStatus: PayUniBindStatus.UNSPECIFIED }),
		);
		assert.ok(html.includes(label), `${KYCFlowStatus[flowStatus]} → ${label}`);
	}
	// UNSPECIFIED 與不認得的值是「未知」（灰色），不當成核准。
	for (const flowStatus of [KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED, 99]) {
		const html = renderPayUni(registration({ flowStatus }));
		assert.ok(html.includes("未知"), String(flowStatus));
		assert.doesNotMatch(html, />已核准</u);
	}
});

test("PayUni 卡：最後同步是 status_time、審核意見直接顯示 status_codes.reason、要補的欄位、沒有 freshness 與原碼徽章", () => {
	const html = renderPayUni(
		registration({
			flowStatus: KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
			statusTime: timestampFromDate(new Date("2026-10-11T04:30:00Z")),
			problem: {
				code: "MER_92",
				fields: [KYCField.KYC_FIELD_BANK_BOOK, KYCField.KYC_FIELD_UNSPECIFIED],
				reason: "存摺影像不清楚",
			},
			statusCodes: {
				merStatus: "91",
				memberStatus: "5",
				reason: "存摺影像不清楚",
			},
		}),
	);
	assert.match(html, /最後同步：2026\/10\/11 12:30/u);
	assert.match(html, /存摺影像不清楚/u);
	assert.match(html, /需補資料：存摺封面/u, "UNSPECIFIED 不顯示成欄位");
	assert.doesNotMatch(
		html,
		/狀態碼|商店 91|會員 5/u,
		"K9：原碼不顯示，狀態看流程狀態",
	);
	assert.match(html, /HAWK100/u);
	assert.doesNotMatch(html, /剛同步|資料較舊|資料可能延遲/u);
	// 沒有審核意見時不畫那一行。
	assert.doesNotMatch(
		renderPayUni(registration({ statusCodes: { reason: "" } })),
		/payuni-reason-text/u,
	);
});

test("第一次核准前（not_found）：有申請時顯示尚未註冊，沒有申請時整張卡不畫；讀取失敗可重試", () => {
	const state = payUniOf(NOT_FOUND_RESULT);
	assert.deepEqual(state, { state: "notFound" });
	const props = {
		state,
		rebinding: false,
		onRebind: noop,
		onRetry: noop,
	};
	assert.match(
		render(payuniCards.PayUniStatusCard as never, {
			...props,
			hasApplication: true,
		}),
		/尚未向 PayUni 註冊/u,
	);
	assert.equal(
		render(payuniCards.PayUniStatusCard as never, {
			...props,
			hasApplication: false,
		}),
		"",
	);
	const error = payUniOf({
		error: new ConnectError("壞了", Code.Unavailable),
		data: undefined,
	});
	assert.deepEqual(error, { state: "error" });
	const html = render(payuniCards.PayUniStatusCard as never, {
		...props,
		state: error,
		hasApplication: true,
	});
	assert.match(html, /PayUni 狀態讀取失敗/u);
	assert.match(html, /重試/u);
	assert.deepEqual(payUniOf({ error: null, data: undefined }), {
		state: "loading",
	});
	assert.deepEqual(
		payUniOf({ error: null, data: {} }),
		{ state: "error" },
		"成功卻沒有註冊不當成尚未註冊",
	);
});

test("聯絡資訊卡標題表明是店家目前的聯絡資料（不是 PayUni 收到的值）；讀店家自己的欄位；沒有註冊或全空時不畫", () => {
	const store = create(StoreSchema, {
		id: "100",
		contactName: "王小明",
		contactMobile: "0912345678",
		contactPhone: "02-12345678",
		supportEmail: "seller@example.com",
	});
	const html = render(payuniCards.PayUniContactCard as never, {
		store,
		registration: registration(),
	});
	for (const text of [
		"王小明",
		"0912345678",
		"02-12345678",
		"seller@example.com",
	]) {
		assert.match(html, new RegExp(text, "u"));
	}
	assert.match(html, /店家目前的聯絡資料/u);
	assert.match(html, /不一定是 PayUni 已經收到的值/u);
	assert.doesNotMatch(
		html,
		/PayUni 聯絡資訊/u,
		"舊標題暗示這是送給 PayUni 的值",
	);
	assert.equal(
		render(payuniCards.PayUniContactCard as never, {
			store,
			registration: undefined,
		}),
		"",
	);
	assert.equal(
		render(payuniCards.PayUniContactCard as never, {
			store: create(StoreSchema, { id: "100" }),
			registration: registration(),
		}),
		"",
	);
});

// ---------------------------------------------------------------------------
// 申請的讀取狀態與評分

test("申請的讀取狀態：沒有摘要不發請求；not_found 是沒有申請；成功卻沒有申請是錯誤", () => {
	assert.deepEqual(
		kycApplicationOf(undefined, { error: null, data: undefined }),
		{
			state: "none",
		},
	);
	assert.deepEqual(kycApplicationOf("", { error: null, data: undefined }), {
		state: "none",
	});
	assert.deepEqual(
		kycApplicationOf("app1", {
			error: new ConnectError("沒有", Code.NotFound),
			data: undefined,
		}),
		{ state: "none" },
	);
	assert.deepEqual(
		kycApplicationOf("app1", {
			error: new ConnectError("壞了", Code.Unavailable),
			data: undefined,
		}),
		{ state: "error" },
	);
	assert.deepEqual(kycApplicationOf("app1", { error: null, data: undefined }), {
		state: "loading",
	});
	assert.deepEqual(kycApplicationOf("app1", { error: null, data: {} }), {
		state: "error",
	});
	const app = application();
	assert.deepEqual(
		kycApplicationOf("app1", { error: null, data: { application: app } }),
		{ state: "ready", application: app },
	);
});

test("評分卡：只顯示讀到且大於 0 的平均分；讀不到、沒有評價、不是數字都不渲染", () => {
	assert.equal(ratingText({ ratingAverage: 4.5 }), "4.5");
	assert.equal(ratingText({ ratingAverage: 4 }), "4.0");
	assert.equal(ratingText({ ratingAverage: 0 }), undefined);
	assert.equal(ratingText({}), undefined);
	assert.equal(ratingText(undefined), undefined);
	assert.equal(ratingText({ ratingAverage: Number.NaN }), undefined);
});

// ---------------------------------------------------------------------------
// 接線（頁面與 hook 讀的是契約的哪個方法）

test("接線：核准與退回帶 etag；顯示全文、重新查詢、重新綁定各走自己的方法；評分讀公開的 GetStore", async () => {
	const { readFile } = await import("node:fs/promises");
	const [page, hooks] = await Promise.all([
		readFile("src/app/admin/merchants/[id]/page.tsx", "utf8"),
		readFile("src/app/admin/merchants/[id]/use-store-detail.ts", "utf8"),
	]);
	assert.match(
		page,
		/approveMutation\.mutateAsync\(\{\s*id: application\.id,\s*etag: application\.etag,?\s*\}\)/u,
	);
	assert.match(
		page,
		/rejectMutation\.mutateAsync\(buildRejectRequest\(application, draft\)\)/u,
		"退回帶讀到的 etag 與逐項問題（buildRejectRequest）",
	);
	assert.doesNotMatch(page, /requiredFields/u, "退回不再送勾選的欄位");
	assert.match(
		page,
		/SellerAdminService\.method\.revealKYCSensitive,\s*\{ gcTime: 0 \}/u,
		"顯示全文的回應含明文：不留在 mutation cache",
	);
	for (const method of [
		"approveKYC",
		"rejectKYC",
		"revealKYCSensitive",
		"requeryKYCRegistration",
		"rebindPayUni",
	]) {
		assert.match(
			page,
			new RegExp(`SellerAdminService\\.method\\.${method}`, "u"),
			method,
		);
	}
	assert.match(hooks, /kycApplicationQueryOptions\(/u);
	assert.match(
		hooks,
		/registrationPollInterval\(/u,
		"登記資料查詢依期限退避輪詢",
	);
	assert.match(hooks, /payUniPollInterval\(/u, "PayUni 綁定中退避輪詢");
	assert.doesNotMatch(
		hooks,
		/REGISTRATION_POLL_MS|registrationCheckPending/u,
		"寫死的 5 秒輪詢已刪除",
	);
	assert.match(hooks, /SellerAdminService\.method\.getPayUniRegistration/u);
	assert.match(hooks, /SellerPublicService\.method\.getStore/u);
	assert.match(
		hooks,
		/\{ storeCode \}/u,
		"公開的 GetStore 以 store_code 讀，不是 id",
	);
	// 舊 REST 全部不再出現。
	assert.doesNotMatch(page, /\/api\/v1\/admin\/(sellers|certifications)/u);
	assert.doesNotMatch(
		page,
		/\b(SellerProfile|CertificationDetail|fetchSecureAsset|SecureImage|payuniWebsiteUrl|platformStorefrontUrl|PayuniStatusBadge|FRESHNESS_LABEL)\b/u,
	);
});

test("頁面載入的結構：店家 loading、找不到、讀取失敗各自一個畫面（NotFound 不顯示英文 error.message）", async () => {
	const { readFile } = await import("node:fs/promises");
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /if \(loading\) \{/u);
	assert.match(
		page,
		/error instanceof ConnectError && error\.code === Code\.NotFound/u,
	);
	assert.match(page, /讀取失敗：/u);
	assert.match(page, /找不到商家資料/u);
});

test("重新查詢成功：回應那筆進行中的查詢直接寫進申請的快取（只換 pending_registration_check，其餘照舊），不整筆重讀", () => {
	const deadline = new Date("2026-10-11T05:00:00Z");
	const cached = {
		application: businessApplication({ registrationCheck: foundCheck() }),
	};
	const check = pendingCheck(deadline);
	const next = applyRequeryResult(cached, check);
	assert.ok(next?.application);
	assert.equal(next.application.pendingRegistrationCheck?.seq, BigInt(2));
	assert.equal(next.application.pendingRegistrationCheck, check);
	// 其餘（etag、審核中的狀態、已完成的查詢）原樣保留；原快取物件不被改動。
	assert.equal(next.application.etag, cached.application.etag);
	assert.equal(
		next.application.registrationCheck,
		cached.application.registrationCheck,
	);
	assert.equal(cached.application.pendingRegistrationCheck, undefined);
	// 沒有快取、或回應沒有查詢時原樣回傳（交給正常的讀取）。
	assert.equal(applyRequeryResult(undefined, check), undefined);
	assert.equal(applyRequeryResult(cached, undefined), cached);
	// 寫進去之後頁面立刻是查詢中，輪詢以伺服器的期限接手。
	assert.equal(
		gcisView(next.application, NOW)?.header.kind,
		"pending",
		"寫入快取後立刻顯示查詢中",
	);
	assert.notEqual(
		registrationPollInterval(next.application.pendingRegistrationCheck, NOW),
		false,
	);
});

test("重新查詢成功：寫入落在 useQuery 讀的那筆快取（同一個 transport、input、cardinality 的 key）", () => {
	const transport = createRouterTransport(() => {});
	const queryClient = new QueryClient();
	const application = businessApplication({ registrationCheck: foundCheck() });
	const readKey = createQueryOptions(
		SellerAdminService.method.getKYCApplication,
		{ id: application.id },
		{ transport },
	).queryKey;
	assert.deepEqual(kycApplicationQueryKey(transport, application.id), readKey);
	queryClient.setQueryData(
		kycApplicationQueryKey(transport, application.id),
		create(GetKYCApplicationResponseSchema, { application }),
	);
	const check = pendingCheck(new Date("2026-10-11T05:00:00Z"));
	writeRequeryResult(queryClient, transport, application.id, check);
	const read = queryClient.getQueryData<{ application?: KYCApplication }>(
		readKey,
	);
	assert.equal(read?.application?.pendingRegistrationCheck, check);
	assert.equal(read?.application?.etag, application.etag);
	assert.equal(queryClient.getQueryCache().getAll().length, 1);
});

test("接線：useKycApplication 的讀取用 kycApplicationQueryOptions，與寫入同一個 key 來源", async () => {
	const { readFile } = await import("node:fs/promises");
	const hook = await readFile(
		"src/app/admin/merchants/[id]/use-store-detail.ts",
		"utf8",
	);
	const body = hook.slice(hook.indexOf("export function useKycApplication"));
	const fn = body.slice(0, body.indexOf("\n}\n"));
	assert.match(
		fn,
		/\.\.\.kycApplicationQueryOptions\(transport, applicationId\)/u,
	);
	assert.doesNotMatch(fn, /getKYCApplication/u);
});

test("接線：重新查詢成功用 writeRequeryResult 寫入，不 invalidate 整個申請", async () => {
	const { readFile } = await import("node:fs/promises");
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(
		page,
		/requeryMutation = useConnectMutation\(\s*SellerAdminService\.method\.requeryKYCRegistration,\s*\);/u,
		"重新查詢的 mutation 沒有 onSuccess: invalidateKyc",
	);
	assert.match(page, /writeRequeryResult\(/u);
	assert.doesNotMatch(page, /queryClient\.setQueryData\(/u);
});
