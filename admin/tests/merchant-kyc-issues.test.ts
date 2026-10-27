import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	type KYCApplication,
	KYCApplicationSchema,
	KYCReviewAction,
	RevealKYCSensitiveResponseSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCField,
	KYCIssueCode,
	KYCIssueSchema,
	KYCStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	buildRejectRequest,
	type DocumentChoice,
	documentChoices,
	ISSUE_ASSET_FORBIDDEN_TEXT,
	ISSUE_ASSET_REQUIRED_TEXT,
	ISSUE_MESSAGE_MAX,
	type IssueFieldOption,
	issuableFieldOptions,
	KYC_ISSUE_CODE_LABEL,
	KYC_ISSUE_CODE_OPTIONS,
	type KycIssueDraft,
	kycIssueCodeLabel,
	REJECT_TEXT_MAX,
	type ReviewDraft,
	revealFrom,
	serverIssueView,
	validateIssueDraft,
	validateRejectDraft,
} from "../src/app/admin/merchants/[id]/kyc-application";

/**
 * 退回的逐項問題（2b-0 契約：RejectKYCRequest.issues）：原因碼的呈現文字、可以指出問題的欄位、加入一項問題與送出前的檢查、
 * 送出的請求形狀，以及賣家尚未處理的問題與審核紀錄的呈現。
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
const dialogModule = requireFromTest(
	"../src/app/admin/merchants/[id]/RejectKycDialog",
) as { default: unknown };
const render = (component: unknown, props: Record<string, unknown>): string =>
	renderToStaticMarkup(createElement(component, props));
const noop = () => {};

const COMPANY = KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS;

/** 伺服器給的可指出問題的欄位（model.IssuableFields，依 KYCField 數字順序）；測試只模擬它的輸出，不自己推規則。 */
const SHARED_FIELDS = [
	KYCField.KYC_FIELD_REAL_NAME,
	KYCField.KYC_FIELD_BIRTH_DATE,
	KYCField.KYC_FIELD_ID_CARD_FRONT,
	KYCField.KYC_FIELD_ID_CARD_BACK,
	KYCField.KYC_FIELD_BANK_BOOK,
	KYCField.KYC_FIELD_PRODUCT_TYPE,
];
const INDIVIDUAL_ISSUABLE = [
	...SHARED_FIELDS,
	KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO,
];
const BUSINESS_ISSUABLE = [
	...SHARED_FIELDS,
	KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME,
	KYCField.KYC_FIELD_BUSINESS_TAX_ID,
	KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO,
	KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
];

function individual(
	patch: MessageInitShape<typeof KYCApplicationSchema> = {},
): KYCApplication {
	return create(KYCApplicationSchema, {
		id: "app1",
		status: KYCStatus.KYC_STATUS_SUBMITTED,
		etag: "etag-1",
		realName: "王小明",
		subject: { case: "individual", value: { idCardNoMasked: "A12****789" } },
		issuableFields: INDIVIDUAL_ISSUABLE,
		...patch,
	});
}

function business(
	patch: MessageInitShape<typeof KYCApplicationSchema> = {},
): KYCApplication {
	return individual({
		subject: {
			case: "business",
			value: { companyName: "範例生鮮有限公司", taxId: "12345678" },
		},
		issuableFields: BUSINESS_ISSUABLE,
		...patch,
	});
}

const EXPIRE = new Date("2026-10-11T05:00:00Z");

function reveal(
	documents: MessageInitShape<
		typeof RevealKYCSensitiveResponseSchema
	>["documents"],
) {
	return revealFrom(
		"app1",
		create(RevealKYCSensitiveResponseSchema, {
			expireTime: timestampFromDate(EXPIRE),
			documents,
		}),
	);
}

function issue(patch: Partial<KycIssueDraft> = {}): KycIssueDraft {
	return {
		key: "issue-1",
		field: KYCField.KYC_FIELD_REAL_NAME,
		documentAssetId: "",
		documentLabel: "",
		code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
		message: "姓名與證件不符",
		...patch,
	};
}

// ---------------------------------------------------------------------------
// 原因碼

test("原因碼的文字窮舉 KYCIssueCode：每個非 UNSPECIFIED 的值都有文字，下拉選項與 enum 完全相同，新增 enum 值時這裡會失敗", () => {
	const all = Object.values(KYCIssueCode).filter(
		(v): v is KYCIssueCode => typeof v === "number",
	);
	const expected = all.filter(
		(c) => c !== KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED,
	);
	assert.ok(expected.length >= 6, "enum 讀不到值，測試對象已改變");
	for (const code of expected) {
		assert.ok(
			(KYC_ISSUE_CODE_LABEL as Record<number, string>)[code],
			`${KYCIssueCode[code]} 沒有呈現文字`,
		);
	}
	assert.deepEqual([...KYC_ISSUE_CODE_OPTIONS].sort(), [...expected].sort());
	assert.equal(
		kycIssueCodeLabel(KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED),
		"未知",
	);
	assert.equal(kycIssueCodeLabel(9999 as KYCIssueCode), "未知");
});

// ---------------------------------------------------------------------------
// 可以指出問題的欄位

test("可以指出問題的欄位由伺服器給（issuable_fields）：選項照它的順序，只加上呈現文字與「是不是文件」；UNSPECIFIED 不列", () => {
	const ind = issuableFieldOptions(individual());
	const biz = issuableFieldOptions(business());
	const fields = (o: IssueFieldOption[]) => o.map((x) => x.field);
	assert.deepEqual(fields(ind), INDIVIDUAL_ISSUABLE);
	assert.deepEqual(fields(biz), BUSINESS_ISSUABLE);
	assert.ok(ind.every((o) => o.label !== "-" && o.label !== ""));
	// 沒有寫死的型態對欄位表：伺服器給什麼就是什麼（個人的清單裡沒有統編、企業的沒有個人身分證字號只是伺服器的輸出）。
	assert.deepEqual(
		fields(
			issuableFieldOptions(
				individual({
					issuableFields: [
						KYCField.KYC_FIELD_UNSPECIFIED,
						KYCField.KYC_FIELD_WEBSITE_URL,
					],
				}),
			),
		),
		[KYCField.KYC_FIELD_WEBSITE_URL],
	);
	assert.deepEqual(
		issuableFieldOptions(individual({ issuableFields: [] })),
		[],
	);
	// 文件類欄位只有四個，其餘都不是文件。
	assert.deepEqual(
		biz.filter((o) => o.document).map((o) => o.field),
		[
			KYCField.KYC_FIELD_ID_CARD_FRONT,
			KYCField.KYC_FIELD_ID_CARD_BACK,
			KYCField.KYC_FIELD_BANK_BOOK,
			KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
		],
	);
});

// ---------------------------------------------------------------------------
// 指定文件

test("只有顯示全文之後才能指定某一張文件：asset_id 來自顯示全文的回應，沒有 asset_id 的不列，補充文件依順序編號", () => {
	assert.deepEqual(documentChoices(undefined), [], "沒有顯示全文：不能指定");
	const choices = documentChoices(
		reveal([
			{
				field: KYCField.KYC_FIELD_ID_CARD_FRONT,
				position: 0,
				assetId: "a-front",
			},
			{ field: COMPANY, position: 0, assetId: "a-c0" },
			{ field: COMPANY, position: 1, assetId: "a-c1" },
			{ field: COMPANY, position: 2, assetId: "" },
			{
				field: KYCField.KYC_FIELD_UNSPECIFIED,
				position: 0,
				assetId: "a-x",
			},
		]),
	);
	assert.deepEqual(choices, [
		{
			field: KYCField.KYC_FIELD_ID_CARD_FRONT,
			assetId: "a-front",
			label: "身分證正面照",
		},
		{
			field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
			assetId: "a-c0",
			label: "補充文件 1",
		},
		{
			field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
			assetId: "a-c1",
			label: "補充文件 2",
		},
	]);
});

// ---------------------------------------------------------------------------
// 加入一項問題前的檢查

const BIZ_OPTIONS = issuableFieldOptions(business());
const CHOICES: DocumentChoice[] = [
	{
		field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
		assetId: "a-c0",
		label: "補充文件 1",
	},
	{
		field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
		assetId: "a-c1",
		label: "補充文件 2",
	},
];

const base = {
	field: KYCField.KYC_FIELD_REAL_NAME,
	documentAssetId: "",
	code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
	message: "姓名與證件不符",
};

test("加入一項問題：欄位、原因、說明都要有效；原因碼是 UNSPECIFIED 一律擋下", () => {
	assert.equal(validateIssueDraft(base, BIZ_OPTIONS, CHOICES, []), undefined);
	assert.equal(
		validateIssueDraft(
			{ ...base, code: KYCIssueCode.KYC_ISSUE_CODE_UNSPECIFIED },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		"請選擇原因。",
	);
	assert.equal(
		validateIssueDraft(
			{ ...base, field: KYCField.KYC_FIELD_UNSPECIFIED },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		"請選擇有問題的欄位。",
	);
	// 不是這份申請的欄位（個人欄位出現在企業申請）也擋下。
	assert.equal(
		validateIssueDraft(
			{ ...base, field: KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		"請選擇有問題的欄位。",
	);
	// 每個有效的原因碼都過。
	for (const code of KYC_ISSUE_CODE_OPTIONS) {
		assert.equal(
			validateIssueDraft({ ...base, code }, BIZ_OPTIONS, CHOICES, []),
			undefined,
		);
	}
});

test("說明必填、去掉前後空白後 1 到 500 字（以字元計，不是 UTF-16 單位）", () => {
	const check = (message: string) =>
		validateIssueDraft({ ...base, message }, BIZ_OPTIONS, CHOICES, []);
	assert.equal(check(""), "請填寫給賣家看的說明。");
	assert.equal(check("   \n "), "請填寫給賣家看的說明。");
	assert.equal(check("字".repeat(ISSUE_MESSAGE_MAX)), undefined);
	assert.equal(check("字".repeat(ISSUE_MESSAGE_MAX + 1)), "說明最多 500 字。");
	assert.equal(
		check("😀".repeat(ISSUE_MESSAGE_MAX)),
		undefined,
		"emoji 是一個字元",
	);
	assert.equal(
		check(`  ${"字".repeat(ISSUE_MESSAGE_MAX)}  `),
		undefined,
		"前後空白不計",
	);
});

test("文件：只有文件類欄位能指定一張，而且那一張要屬於這個欄位；缺件只標欄位；同一個欄位（文件）不重複", () => {
	const docField = KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS;
	const withAsset = { ...base, field: docField, documentAssetId: "a-c0" };
	assert.equal(
		validateIssueDraft(withAsset, BIZ_OPTIONS, CHOICES, []),
		undefined,
	);
	// 缺少文件：只標欄位，不帶 asset。
	assert.equal(
		validateIssueDraft(
			{ ...base, field: docField, code: KYCIssueCode.KYC_ISSUE_CODE_MISSING },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		undefined,
	);
	// 非文件欄位不能帶文件。
	assert.equal(
		validateIssueDraft(
			{ ...base, documentAssetId: "a-c0" },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		"這個欄位不是文件，不能指定文件。",
	);
	// 不存在、或屬於別的欄位的文件。
	assert.equal(
		validateIssueDraft(
			{ ...withAsset, documentAssetId: "nope" },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		"找不到指定的文件，請重新選擇。",
	);
	assert.equal(
		validateIssueDraft(
			{ ...base, field: KYCField.KYC_FIELD_BANK_BOOK, documentAssetId: "a-c0" },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		"找不到指定的文件，請重新選擇。",
	);
	// 同一個欄位（同一張）已經有一項；同一個欄位的另一張可以。
	const existing = [issue({ field: docField, documentAssetId: "a-c0" })];
	assert.match(
		validateIssueDraft(withAsset, BIZ_OPTIONS, CHOICES, existing) ?? "",
		/已經有一項問題/u,
	);
	assert.equal(
		validateIssueDraft(
			{ ...withAsset, documentAssetId: "a-c1" },
			BIZ_OPTIONS,
			CHOICES,
			existing,
		),
		undefined,
	);
	assert.match(
		validateIssueDraft(base, BIZ_OPTIONS, CHOICES, [issue()]) ?? "",
		/已經有一項問題/u,
	);
});

test("文件類欄位依原因碼（與伺服器同一條規則）：看不清楚、過期、不一致、格式不符必須指定是哪一張；缺少不可以指定；其他兩者皆可", () => {
	const docField = KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS;
	const required = [
		KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
		KYCIssueCode.KYC_ISSUE_CODE_EXPIRED,
		KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
		KYCIssueCode.KYC_ISSUE_CODE_INVALID,
	];
	for (const code of required) {
		assert.match(
			validateIssueDraft(
				{ ...base, field: docField, code },
				BIZ_OPTIONS,
				CHOICES,
				[],
			) ?? "",
			new RegExp(ISSUE_ASSET_REQUIRED_TEXT, "u"),
			`${KYCIssueCode[code]} 沒有 asset 要擋下`,
		);
		assert.equal(
			validateIssueDraft(
				{ ...base, field: docField, code, documentAssetId: "a-c0" },
				BIZ_OPTIONS,
				CHOICES,
				[],
			),
			undefined,
			`${KYCIssueCode[code]} 帶 asset 可以`,
		);
	}
	// 還沒顯示全文（沒有可選的文件）：同一個錯誤再說明怎麼辦。
	assert.match(
		validateIssueDraft(
			{ ...base, field: docField, code: required[0] },
			BIZ_OPTIONS,
			[],
			[],
		) ?? "",
		/請先在頁面上「顯示全文」/u,
	);
	// 缺少：不可以帶 asset；不帶可以。
	assert.equal(
		validateIssueDraft(
			{
				...base,
				field: docField,
				code: KYCIssueCode.KYC_ISSUE_CODE_MISSING,
				documentAssetId: "a-c0",
			},
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		ISSUE_ASSET_FORBIDDEN_TEXT,
	);
	assert.equal(
		validateIssueDraft(
			{ ...base, field: docField, code: KYCIssueCode.KYC_ISSUE_CODE_MISSING },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		undefined,
	);
	// 其他：兩者皆可。
	for (const documentAssetId of ["", "a-c0"]) {
		assert.equal(
			validateIssueDraft(
				{
					...base,
					field: docField,
					code: KYCIssueCode.KYC_ISSUE_CODE_OTHER,
					documentAssetId,
				},
				BIZ_OPTIONS,
				CHOICES,
				[],
			),
			undefined,
		);
	}
	// 非文件欄位不受這條規則影響：看不清楚不需要 asset。
	assert.equal(
		validateIssueDraft(
			{ ...base, code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE },
			BIZ_OPTIONS,
			CHOICES,
			[],
		),
		undefined,
	);
});

// ---------------------------------------------------------------------------
// 送出前的檢查與請求

function draft(patch: Partial<ReviewDraft> = {}): ReviewDraft {
	return {
		issues: [issue()],
		rejectReason: "證件需要重新上傳",
		internalNote: "",
		...patch,
	};
}

test("退回前：至少一項問題、總理由必填、總理由與內部備註各最多 1000 字", () => {
	assert.equal(validateRejectDraft(draft()), undefined);
	assert.match(
		validateRejectDraft(draft({ issues: [] })) ?? "",
		/至少指出一項問題/u,
	);
	assert.equal(
		validateRejectDraft(draft({ rejectReason: "  " })),
		"請填寫退回原因，它會回傳給賣家。",
	);
	assert.equal(
		validateRejectDraft(draft({ rejectReason: "字".repeat(REJECT_TEXT_MAX) })),
		undefined,
	);
	assert.equal(
		validateRejectDraft(
			draft({ rejectReason: "字".repeat(REJECT_TEXT_MAX + 1) }),
		),
		"退回原因最多 1000 字。",
	);
	assert.equal(
		validateRejectDraft(
			draft({ internalNote: "字".repeat(REJECT_TEXT_MAX + 1) }),
		),
		"內部備註最多 1000 字。",
	);
	assert.equal(
		validateRejectDraft(draft({ internalNote: "字".repeat(REJECT_TEXT_MAX) })),
		undefined,
		"內部備註選填",
	);
});

test("RejectKYC 的請求：帶讀到的 etag、依加入順序的 issues（欄位、文件 asset、原因碼、說明）、去掉前後空白的總理由與內部備註", () => {
	const request = buildRejectRequest(
		{ id: "app1", etag: "3.1" },
		draft({
			issues: [
				issue({
					key: "issue-1",
					field: KYCField.KYC_FIELD_BIRTH_DATE,
					message: "  生日不對 ",
				}),
				issue({
					key: "issue-2",
					field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
					documentAssetId: "a-c1",
					documentLabel: "補充文件 2",
					code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
					message: "看不清楚",
				}),
				issue({
					key: "issue-3",
					field: KYCField.KYC_FIELD_ID_CARD_FRONT,
					code: KYCIssueCode.KYC_ISSUE_CODE_MISSING,
					message: "缺少正面",
				}),
			],
			rejectReason: " 證件要重傳 ",
			internalNote: " 電話確認過 ",
		}),
	);
	assert.deepEqual(request, {
		id: "app1",
		etag: "3.1",
		reason: "證件要重傳",
		issues: [
			{
				field: KYCField.KYC_FIELD_BIRTH_DATE,
				documentAssetId: "",
				code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
				message: "生日不對",
			},
			{
				field: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
				documentAssetId: "a-c1",
				code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
				message: "看不清楚",
			},
			{
				field: KYCField.KYC_FIELD_ID_CARD_FRONT,
				documentAssetId: "",
				code: KYCIssueCode.KYC_ISSUE_CODE_MISSING,
				message: "缺少正面",
			},
		],
		internalNote: "電話確認過",
	});
	// 草稿裡的 key、顯示名稱不送出。
	assert.ok(
		request.issues.every((i) => !("key" in i) && !("documentLabel" in i)),
	);
});

// ---------------------------------------------------------------------------
// 退回對話框

function renderDialog(patch: Record<string, unknown> = {}): string {
	return render(dialogModule.default, {
		draft: draft(),
		onDraftChange: noop,
		options: BIZ_OPTIONS,
		choices: [] as DocumentChoice[],
		blockedReason: undefined as string | undefined,
		onClose: noop,
		onConfirm: async () => ({ ok: true }),
		...patch,
	});
}

test("退回對話框：原因下拉只有六個有效原因（加一個「請選擇」的空選項），已指出的問題逐項列出可移除，總理由與內部備註各一欄", () => {
	const html = renderDialog();
	const codeSelect =
		html.match(/<select id="adm-reject-kyc-code"[\s\S]*?<\/select>/u)?.[0] ??
		"";
	const labels = [
		...codeSelect.matchAll(/<option[^>]*>([^<]*)<\/option>/gu),
	].map((m) => m[1]);
	assert.deepEqual(labels, [
		"請選擇原因",
		...KYC_ISSUE_CODE_OPTIONS.map(
			(c) => KYC_ISSUE_CODE_LABEL[c as keyof typeof KYC_ISSUE_CODE_LABEL],
		),
	]);
	assert.match(html, /已指出的問題/u);
	assert.match(html, /姓名/u);
	assert.match(html, /姓名與證件不符/u);
	assert.match(html, /移除/u);
	assert.match(html, /退回原因（必填，賣家看得到）/u);
	assert.match(html, /證件需要重新上傳/u);
	assert.match(html, /內部備註（選填，賣家看不到）/u);
	assert.doesNotMatch(
		html,
		/toggle-circle|aria-pressed/u,
		"第一輪的欄位勾選框已刪除",
	);
});

test("退回對話框：沒有問題時說還沒有；沒顯示全文時不能指定文件並說明原因的路（選到文件欄位才出現）；不能退回時保留輸入並禁用送出", () => {
	const empty = renderDialog({ draft: draft({ issues: [] }) });
	assert.match(empty, /還沒有/u);
	const serverText =
		"店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。";
	const blocked = renderDialog({ blockedReason: serverText });
	assert.ok(blocked.includes(serverText), "伺服器的句子原樣顯示");
	assert.match(blocked, /你輸入的內容還在，可以關閉視窗後確認申請的狀態。/u);
	assert.doesNotMatch(
		blocked,
		/這份申請目前不能退回/u,
		"client 不再自己寫原因",
	);
	assert.match(blocked, /證件需要重新上傳/u, "輸入還在");
	assert.match(
		blocked,
		/type="submit"[^>]*disabled|disabled=""[^>]*type="submit"/u,
	);
	const open = renderDialog();
	assert.doesNotMatch(open, /type="submit"[^>]*disabled/u);
});

// ---------------------------------------------------------------------------
// 賣家尚未處理的問題與審核紀錄

const SERVER_ISSUES = [
	{
		field: KYCField.KYC_FIELD_REAL_NAME,
		code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
		message: "姓名與證件不符",
		documentAssetId: "",
	},
	{
		field: KYCField.KYC_FIELD_ID_CARD_BACK,
		code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
		message: "反面反光",
		documentAssetId: "a-back",
	},
];

test("伺服器的問題清單轉成文字：欄位、原因碼、說明，指定了文件的註明", () => {
	const views = SERVER_ISSUES.map((i) =>
		serverIssueView(create(KYCIssueSchema, i)),
	);
	assert.deepEqual(views, [
		{
			field: "真實姓名",
			code: "與其他資料不一致",
			message: "姓名與證件不符",
			document: undefined,
		},
		{
			field: "身分證反面照",
			code: "看不清楚",
			message: "反面反光",
			document: "指定的文件",
		},
	]);
});

test("被退回的申請顯示總理由，加上賣家尚未處理的問題（伺服器的 outstanding_issues）；全部處理完還沒重送時說明這一點", () => {
	const rejected = individual({
		status: KYCStatus.KYC_STATUS_REJECTED,
		reviews: [
			{
				id: "r1",
				action: KYCReviewAction.KYC_REVIEW_ACTION_REJECT,
				reviewerAdminId: "9",
				reason: "證件要重傳",
			},
		],
		outstandingIssues: SERVER_ISSUES,
	});
	const html = render(cards.KycRejectionNotice, { application: rejected });
	assert.match(html, /失敗原因：證件要重傳/u);
	assert.match(html, /賣家尚未處理的問題/u);
	assert.match(html, /姓名與證件不符/u);
	assert.match(html, /反面反光/u);
	assert.match(html, /與其他資料不一致/u);
	assert.match(html, /看不清楚/u);
	assert.match(html, /指定的文件/u);

	const handled = render(cards.KycRejectionNotice, {
		application: individual({
			status: KYCStatus.KYC_STATUS_REJECTED,
			reviews: [
				{
					id: "r1",
					action: KYCReviewAction.KYC_REVIEW_ACTION_REJECT,
					reviewerAdminId: "9",
					reason: "證件要重傳",
				},
			],
		}),
	});
	assert.match(handled, /賣家已處理退回的所有項目，還沒有重新送出/u);
	assert.doesNotMatch(handled, /賣家尚未處理的問題/u);
	// 沒有退回紀錄的理由時用固定說法。
	assert.match(
		render(cards.KycRejectionNotice, {
			application: individual({ status: KYCStatus.KYC_STATUS_REJECTED }),
		}),
		/審核未通過，請重新上傳/u,
	);
});

test("審核紀錄：退回的逐項問題與內部備註（只有後台看得到），審核者只顯示 admin ID", () => {
	const html = render(cards.KycReviewHistoryCard, {
		application: individual({
			reviews: [
				{ id: "r0", action: KYCReviewAction.KYC_REVIEW_ACTION_SUBMIT },
				{
					id: "r1",
					action: KYCReviewAction.KYC_REVIEW_ACTION_REJECT,
					reviewerAdminId: "9",
					reason: "證件要重傳",
					issues: SERVER_ISSUES,
					internalNote: "電話確認過",
				},
			],
		}),
	});
	assert.match(html, /管理員 9/u);
	assert.match(html, /賣家/u);
	assert.match(html, /姓名與證件不符/u);
	assert.match(html, /看不清楚/u);
	assert.match(html, /內部備註：電話確認過/u);
	assert.doesNotMatch(html, /要補件：/u, "舊的欄位清單已經不存在");
});

test("頁面與元件沒有留下第一輪的欄位勾選與 BUSINESS_COMPANY_DOCUMENTS 合併", async () => {
	const { readFile } = await import("node:fs/promises");
	for (const file of [
		"src/app/admin/merchants/[id]/page.tsx",
		"src/app/admin/merchants/[id]/KycApplicationCards.tsx",
		"src/app/admin/merchants/[id]/kyc-application.ts",
		"src/app/admin/merchants/[id]/RejectKycDialog.tsx",
	]) {
		const source = await readFile(file, "utf8");
		assert.doesNotMatch(
			source,
			/toggleField|requiredFieldsOf|flaggedFieldsOf|FieldFlag|toggleFlag|aria-pressed/u,
			file,
		);
	}
	const css = await readFile(
		"src/styles/pages/MerchantCertificationDetail.css",
		"utf8",
	);
	assert.doesNotMatch(css, /toggle-circle|with-toggle/u);
});
