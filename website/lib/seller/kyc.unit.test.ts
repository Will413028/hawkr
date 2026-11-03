import { create, toJson } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	KYCApplicationSchema,
	KYCField,
	KYCIssueCode,
	KYCStatus,
	SaveKYCDraftRequestSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { parseDate } from "@hawkr/api/proto/date";
import { describe, expect, it, vi } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	adoptExistingApplication,
	adoptSavedDocs,
	baselineAfterSave,
	buildSaveKycDraftRequest,
	EMPTY_KYC_DOCS,
	EMPTY_KYC_FORM,
	fetchMyKyc,
	type KycDocs,
	type KycForm,
	kycDocsOf,
	kycDocumentNotice,
	kycDraftDiscardable,
	kycEditable,
	kycFieldLabel,
	kycFieldOfPath,
	kycFormKeyOfDraftPath,
	kycFormOf,
	kycInputErrors,
	kycIssueMessages,
	mergeLatestForm,
	missingItemsOf,
	resumeSectionOf,
	withPayoutEmailDefault,
} from "./kyc";

const ut = uiTextFor("zh-TW");

function draft(
	form: Partial<KycForm>,
	options: {
		baseline?: Partial<KycForm>;
		docs?: Partial<KycDocs>;
		baselineDocs?: Partial<KycDocs>;
		etag?: string;
	} = {},
) {
	return buildSaveKycDraftRequest({
		form: { ...EMPTY_KYC_FORM, ...form },
		docs: { ...EMPTY_KYC_DOCS, ...options.docs },
		baseline: { ...EMPTY_KYC_FORM, ...options.baseline },
		baselineDocs: { ...EMPTY_KYC_DOCS, ...options.baselineDocs },
		etag: options.etag,
	});
}

function json(request: ReturnType<typeof draft>) {
	if (!request) throw new Error("expected a request");
	return toJson(SaveKYCDraftRequestSchema, request) as {
		application: Record<string, unknown> & { etag?: string };
		updateMask: string;
	};
}

describe("SaveKYCDraft：建立與更新", () => {
	it("還沒有申請：不帶 etag、subject 一定設定（建立）", () => {
		const request = draft({ sellerType: "business" });
		expect(request?.application?.etag).toBe("");
		expect(request?.application?.subject.case).toBe("business");
	});

	it("已經有申請：每次寫入都帶 etag，只列有改的欄位", () => {
		const request = draft(
			{ productType: "水果", payoutContactEmail: "pay@example.com" },
			{ baseline: { productType: "水果" }, etag: "e1" },
		);
		const body = json(request);
		expect(body.application.etag).toBe("e1");
		expect(body.updateMask).toBe("payoutContactEmail");
		expect(body.application.payoutContactEmail).toBe("pay@example.com");
	});

	it("沒有任何改動不送請求", () => {
		expect(
			draft(
				{ payoutContactEmail: "a@b.co" },
				{ baseline: { payoutContactEmail: "a@b.co" }, etag: "e1" },
			),
		).toBeNull();
	});

	it("格式不合的收款聯絡信箱不送：伺服器會因為一欄不合法整筆不寫入", () => {
		const request = draft(
			{ productType: "水果", payoutContactEmail: "not-an-email" },
			{ etag: "e1" },
		);
		expect(json(request).updateMask).toBe("productType");
	});

	it("收款聯絡信箱清空：列進 mask、值是空字串（草稿可以空）", () => {
		const request = draft(
			{ payoutContactEmail: "" },
			{ baseline: { payoutContactEmail: "a@b.co" }, etag: "e1" },
		);
		expect(json(request).updateMask).toBe("payoutContactEmail");
	});

	it("建立：預設帶入的收款聯絡信箱隨建立一起送", () => {
		const request = draft({
			sellerType: "individual",
			payoutContactEmail: "login@example.com",
		});
		expect(request?.application?.payoutContactEmail).toBe("login@example.com");
		expect(request?.application?.subject.case).toBe("individual");
	});

	it("表單不再有身分、銀行、證件與英文名稱：請求的 mask 不會列到它們", () => {
		const request = draft(
			{
				sellerType: "business",
				companyName: "山田有限公司",
				taxId: "12345678",
				payoutContactEmail: "a@b.co",
			},
			{ etag: "e1" },
		);
		expect(json(request).updateMask).not.toMatch(
			/realName|idCard|bank|englishName|birthDate|repIdCardNo/,
		);
	});

	it("組織型態不是請求的一部分", () => {
		const request = draft(
			{
				sellerType: "business",
				companyName: "山田有限公司",
				taxId: "12345678",
				companyCountyCode: "63000",
				companyTownCode: "63000010",
				companyAddressDetail: "路 1 號",
			},
			{ etag: "e1" },
		);
		const text = JSON.stringify(json(request));
		expect(text).not.toMatch(/companyType|company_type/);
		expect(json(request).updateMask).toContain("business.companyName");
		expect(json(request).updateMask).toContain("business.companyAddress");
	});

	it("公司地址送鄉鎮市區代碼與明細；只選了縣市、還沒選鄉鎮市區時不送", () => {
		const request = draft(
			{
				sellerType: "business",
				companyCountyCode: "63000",
				companyTownCode: "63000010",
				companyAddressDetail: "路 1 號",
			},
			{ etag: "e1" },
		);
		const body = json(request);
		expect(body.updateMask).toBe("business.companyAddress");
		expect(
			(body.application.business as Record<string, unknown>).companyAddress,
		).toEqual({ townCode: "63000010", addressDetail: "路 1 號" });
		expect(
			draft(
				{
					sellerType: "business",
					companyCountyCode: "63000",
					companyAddressDetail: "路 1 號",
				},
				{ etag: "e1" },
			),
		).toBeNull();
	});

	it("公司地址全部清空：列進 mask、沒有值", () => {
		const request = draft(
			{ sellerType: "business" },
			{
				etag: "e1",
				baseline: {
					sellerType: "business",
					companyCountyCode: "63000",
					companyTownCode: "63000010",
					companyAddressDetail: "路 1 號",
				},
			},
		);
		expect(json(request).updateMask).toBe("business.companyAddress");
	});

	it("補充文件：新上傳的送 staged token，保留的送 asset id，移除的列進 mask 但沒有值", () => {
		const request = draft(
			{ sellerType: "business" },
			{
				etag: "e1",
				docs: {
					companyDocuments: [
						{ assetId: "", stagedToken: "tok", preview: "blob:x" },
					],
				},
				baselineDocs: {
					companyDocuments: [
						{ assetId: "a1", stagedToken: "", preview: "u" },
						{ assetId: "a2", stagedToken: "", preview: "u" },
					],
				},
			},
		);
		const body = json(request);
		expect(body.updateMask).toBe("business.companyDocuments");
		expect(
			(body.application.business as Record<string, unknown>).companyDocuments,
		).toEqual([{ stagedToken: "tok" }]);
	});
});

describe("存檔後的 baseline（以送出的值為基準，與伺服器正規化無關）", () => {
	const request = (form: KycForm, baseline: KycForm) =>
		buildSaveKycDraftRequest({
			form,
			docs: EMPTY_KYC_DOCS,
			baseline,
			baselineDocs: EMPTY_KYC_DOCS,
			etag: "e2",
		});

	it("送出後沿用送出的值，不會每次都重送", () => {
		const sent = { ...EMPTY_KYC_FORM, payoutContactEmail: "a@b.co" };
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, [
			"payout_contact_email",
		]);
		expect(baseline.payoutContactEmail).toBe("a@b.co");
		expect(request(sent, baseline)).toBeNull();
	});

	it("伺服器正規化（去空白、電話去橫線）不會讓已送出的欄位被當成又改了而重送", () => {
		const sent: KycForm = {
			...EMPTY_KYC_FORM,
			sellerType: "business",
			payoutContactEmail: "A@B.co ",
			companyPhone: "02-12345678",
		};
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, [
			"payout_contact_email",
			"business.company_phone",
		]);
		expect(request(sent, baseline)).toBeNull();
	});

	it("輸入原樣保留：打字中的尾端空白不會被換掉（表單不由伺服器的值覆寫）", () => {
		const sent: KycForm = { ...EMPTY_KYC_FORM, productType: "水果 " };
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, ["product_type"]);
		const typed: KycForm = { ...sent, productType: "水果、蔬菜" };
		expect(baseline.productType).toBe("水果 ");
		expect(json(request(typed, baseline)).updateMask).toBe("productType");
	});

	it("沒送出的欄位（不合格式的收款聯絡信箱）baseline 不變，仍與輸入不同", () => {
		const sent: KycForm = {
			...EMPTY_KYC_FORM,
			productType: "水果",
			payoutContactEmail: "bad",
		};
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, ["product_type"]);
		expect(baseline.payoutContactEmail).toBe("");
		expect(baseline.productType).toBe("水果");
	});
});

describe("收款聯絡信箱預設帶入登入信箱（D23）", () => {
	it("還沒有值：帶入登入信箱", () => {
		expect(
			withPayoutEmailDefault(EMPTY_KYC_FORM, "login@example.com")
				.payoutContactEmail,
		).toBe("login@example.com");
	});

	it("已經有值（賣家改過的）不覆蓋", () => {
		const form = { ...EMPTY_KYC_FORM, payoutContactEmail: "pay@example.com" };
		expect(
			withPayoutEmailDefault(form, "login@example.com").payoutContactEmail,
		).toBe("pay@example.com");
	});

	it("登入沒有信箱（例如 LINE）：維持空，不帶入空白字串以外的東西", () => {
		expect(
			withPayoutEmailDefault(EMPTY_KYC_FORM, undefined).payoutContactEmail,
		).toBe("");
		expect(
			withPayoutEmailDefault(EMPTY_KYC_FORM, "  ").payoutContactEmail,
		).toBe("");
	});

	it("帶入的值與 baseline 不同：自動存檔會把它寫進申請", () => {
		const defaulted = withPayoutEmailDefault(
			EMPTY_KYC_FORM,
			"login@example.com",
		);
		const request = buildSaveKycDraftRequest({
			form: defaulted,
			docs: EMPTY_KYC_DOCS,
			baseline: EMPTY_KYC_FORM,
			baselineDocs: EMPTY_KYC_DOCS,
			etag: "e1",
		});
		expect(json(request).updateMask).toBe("payoutContactEmail");
	});

	it("輸入檢查：格式錯誤有訊息，空的（草稿）沒有", () => {
		expect(kycInputErrors(EMPTY_KYC_FORM).payoutEmailErr).toBeNull();
		expect(
			kycInputErrors({ ...EMPTY_KYC_FORM, payoutContactEmail: "bad" })
				.payoutEmailErr,
		).toBe("Email 格式不正確");
	});
});

describe("建立時別處已經建立了申請", () => {
	const storeDefaults: KycForm = {
		...EMPTY_KYC_FORM,
		sellerType: "business",
		companyPhone: "02-1234-5678",
	};
	const existing = create(KYCApplicationSchema, {
		etag: "e9",
		payoutContactEmail: "pay@example.com",
		subject: { case: "business", value: { companyName: "範例鮮果" } },
	});

	it("舊做法的陷阱：合併把店家預設值當成使用者的修改保留", () => {
		const merged = mergeLatestForm(
			storeDefaults,
			EMPTY_KYC_FORM,
			kycFormOf(existing),
		);
		expect(merged.companyPhone).toBe("02-1234-5678");
	});

	it("原樣採用既有申請：不帶店家預設值，也不會被當成待存的修改", () => {
		const adopted = adoptExistingApplication(existing);
		expect(adopted.form.companyPhone).toBe("");
		expect(adopted.form.payoutContactEmail).toBe("pay@example.com");
		expect(adopted.etag).toBe("e9");
		expect(
			buildSaveKycDraftRequest({
				form: adopted.form,
				docs: adopted.docs,
				baseline: adopted.baseline,
				baselineDocs: adopted.baselineDocs,
				etag: adopted.etag,
			}),
		).toBeNull();
	});
});

describe("續填：位置與缺項由 missing_fields 決定", () => {
	it("第一個還有缺項的區塊", () => {
		expect(
			resumeSectionOf([
				KYCField.KYC_FIELD_PAYOUT_CONTACT_EMAIL,
				KYCField.KYC_FIELD_BUSINESS_TAX_ID,
			]),
		).toBe("company");
		expect(resumeSectionOf([KYCField.KYC_FIELD_PAYOUT_CONTACT_EMAIL])).toBe(
			"other",
		);
	});

	it("沒有缺項不指定位置", () => {
		expect(resumeSectionOf([])).toBeNull();
	});

	it("缺項清單依欄位順序列出區塊與名詞；表單沒有的欄位與 UNSPECIFIED 略過", () => {
		expect(
			missingItemsOf(ut, [
				KYCField.KYC_FIELD_BUSINESS_TAX_ID,
				KYCField.KYC_FIELD_PAYOUT_CONTACT_EMAIL,
				KYCField.KYC_FIELD_BANK_NAME,
				KYCField.KYC_FIELD_UNSPECIFIED,
			]),
		).toEqual([
			{ sectionId: "company", label: "統一編號" },
			{ sectionId: "other", label: "收款聯絡信箱" },
		]);
	});

	it("送出缺項的欄位路徑對應回欄位", () => {
		expect(kycFieldOfPath("business.tax_id")).toBe(
			KYCField.KYC_FIELD_BUSINESS_TAX_ID,
		);
		expect(kycFieldOfPath("contact_email")).toBeUndefined();
		// 重複欄位裡的單一元素（公司文件的第 3 份被拒絕）指到整個欄位。
		expect(kycFieldOfPath("business.company_documents[2]")).toBe(
			KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
		);
		expect(kycFieldOfPath("unknown[1]")).toBeUndefined();
		expect(kycFieldOfPath("business.company_address.town_code")).toBe(
			KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
		);
		expect(kycFieldOfPath("business.company_address.address_detail")).toBe(
			KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
		);
		expect(kycFieldOfPath("nope")).toBeUndefined();
	});
});

describe("補充文件欄位的錯誤提示", () => {
	it("欄位名稱加伺服器的訊息，註明第幾張；文字欄位不是補充文件", () => {
		expect(
			kycDocumentNotice(
				ut,
				"business.company_documents[0]",
				KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
				"x",
			),
		).toBe("補充文件（第 1 張）：x");
		expect(
			kycDocumentNotice(
				ut,
				"business.tax_id",
				KYCField.KYC_FIELD_BUSINESS_TAX_ID,
				"x",
			),
		).toBeUndefined();
	});
});

describe("申請 → 表單", () => {
	it("地址、賣家型態與收款聯絡信箱", () => {
		const form = kycFormOf(
			create(KYCApplicationSchema, {
				payoutContactEmail: "pay@example.com",
				subject: {
					case: "business",
					value: {
						taxId: "12345678",
						companyAddress: { townCode: "63000010", addressDetail: "某路" },
					},
				},
			}),
		);
		expect(form.sellerType).toBe("business");
		expect(form.companyAddressDetail).toBe("某路");
		expect(form.companyTownCode).toBe("63000010");
		expect(form.companyCountyCode).toBe("63000");
		expect(form.payoutContactEmail).toBe("pay@example.com");
		expect(parseDate("1990-02-03")).toEqual({ year: 1990, month: 2, day: 3 });
		expect(parseDate("bad")).toBeUndefined();
	});
});

describe("版本衝突：保留輸入、顯示最新", () => {
	it("使用者改過的欄位保留，沒改的換成最新版本", () => {
		const oldBaseline = {
			...EMPTY_KYC_FORM,
			productType: "舊",
			payoutContactEmail: "old@example.com",
		};
		const current = { ...oldBaseline, productType: "我改的" };
		const latest = {
			...oldBaseline,
			productType: "別處改的",
			payoutContactEmail: "new@example.com",
		};
		const merged = mergeLatestForm(current, oldBaseline, latest);
		expect(merged.productType).toBe("我改的");
		expect(merged.payoutContactEmail).toBe("new@example.com");
	});

	it("存檔回應把剛傳的補充文件換成申請上的圖片，保留本機預覽", () => {
		const staged = { assetId: "", stagedToken: "tok", preview: "blob:x" };
		const sent: KycDocs = { companyDocuments: [staged] };
		const saved: KycDocs = {
			companyDocuments: [{ assetId: "a1", stagedToken: "", preview: "/dl/a1" }],
		};
		expect(adoptSavedDocs(sent, sent, saved).companyDocuments).toEqual([
			{ assetId: "a1", stagedToken: "", preview: "blob:x" },
		]);
		const changed: KycDocs = { companyDocuments: [] };
		expect(adoptSavedDocs(changed, sent, saved).companyDocuments).toEqual([]);
	});

	it("申請的補充文件取 download_uri 當預覽", () => {
		const docs = kycDocsOf(
			create(KYCApplicationSchema, {
				subject: {
					case: "business",
					value: {
						companyDocuments: [{ assetId: "a1", downloadUri: "/dl/a1" }],
					},
				},
			}),
		);
		expect(docs.companyDocuments).toEqual([
			{ assetId: "a1", stagedToken: "", preview: "/dl/a1" },
		]);
	});
});

describe("放棄草稿", () => {
	const app = (status: KYCStatus) => create(KYCApplicationSchema, { status });
	it("只有草稿而且身分沒有鎖定時可以放棄", () => {
		expect(kycDraftDiscardable(app(KYCStatus.KYC_STATUS_DRAFT), false)).toBe(
			true,
		);
		expect(kycDraftDiscardable(app(KYCStatus.KYC_STATUS_DRAFT), true)).toBe(
			false,
		);
		expect(kycDraftDiscardable(app(KYCStatus.KYC_STATUS_REJECTED), false)).toBe(
			false,
		);
	});
});

describe("可編輯的狀態", () => {
	const app = (status: KYCStatus) => create(KYCApplicationSchema, { status });
	it("沒有申請、草稿、被退回可以編輯；審核中與已通過不行", () => {
		expect(kycEditable(null)).toBe(true);
		expect(kycEditable(app(KYCStatus.KYC_STATUS_DRAFT))).toBe(true);
		expect(kycEditable(app(KYCStatus.KYC_STATUS_REJECTED))).toBe(true);
		expect(kycEditable(app(KYCStatus.KYC_STATUS_SUBMITTED))).toBe(false);
		expect(kycEditable(app(KYCStatus.KYC_STATUS_APPROVED))).toBe(false);
		expect(kycEditable(app(KYCStatus.KYC_STATUS_UNSPECIFIED))).toBe(false);
	});

	it("與 flow.can_resubmit 無關：狀態只看申請自己的 status", () => {
		const submitted = create(KYCApplicationSchema, {
			status: KYCStatus.KYC_STATUS_SUBMITTED,
			flow: { canResubmit: true },
		});
		expect(kycEditable(submitted)).toBe(false);
	});
});

describe("GetMyKYC", () => {
	it("not_found 是沒有申請，其他錯誤照丟", async () => {
		const getMyKYC = vi
			.fn()
			.mockRejectedValue(new ConnectError("x", Code.NotFound));
		await expect(fetchMyKyc(() => ({ getMyKYC }) as never)).resolves.toBeNull();
		getMyKYC.mockRejectedValue(new ConnectError("x", Code.Unavailable));
		await expect(
			fetchMyKyc(() => ({ getMyKYC }) as never),
		).rejects.toBeInstanceOf(ConnectError);
	});
});

describe("SaveKYCDraft 的欄位錯誤", () => {
	it("欄位錯誤的路徑對應回表單欄位", () => {
		expect(kycFormKeyOfDraftPath("application.business.tax_id")).toBe("taxId");
		expect(kycFormKeyOfDraftPath("application.payout_contact_email")).toBe(
			"payoutContactEmail",
		);
		expect(kycFormKeyOfDraftPath("application.english_name")).toBeUndefined();
		expect(kycFormKeyOfDraftPath("application.etag")).toBeUndefined();
	});
});

describe("待處理的退回問題", () => {
	const issue = (
		field: KYCField,
		message: string,
		documentAssetId = "",
		code = KYCIssueCode.KYC_ISSUE_CODE_OTHER,
	) => ({ field, message, documentAssetId, code });
	const app = (...issues: ReturnType<typeof issue>[]) =>
		create(KYCApplicationSchema, {
			lastRejection: { reason: "資料不清楚", issues },
			outstandingIssues: issues,
		});

	it("沒有申請或沒有待處理的問題：什麼都不標", () => {
		const none = {
			form: {},
			companyDocumentByAsset: {},
			companyDocuments: null,
		};
		expect(kycIssueMessages(null)).toEqual(none);
		expect(kycIssueMessages(undefined)).toEqual(none);
		expect(kycIssueMessages(app())).toEqual(none);
	});

	it("只讀伺服器算的 outstanding_issues，不看 last_rejection 的全部問題", () => {
		const application = create(KYCApplicationSchema, {
			lastRejection: {
				reason: "x",
				issues: [
					issue(KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME, "名稱已改過"),
					issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, "與登記不符"),
				],
			},
			outstandingIssues: [
				issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, "與登記不符"),
			],
		});
		expect(kycIssueMessages(application).form).toEqual({
			taxId: "與登記不符",
		});
	});

	it("欄位的說明對到表單欄位，同一處的多項說明接在一起", () => {
		const messages = kycIssueMessages(
			app(
				issue(KYCField.KYC_FIELD_PAYOUT_CONTACT_EMAIL, "信箱無法收信"),
				issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, "與登記不符"),
				issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, "請重新確認"),
				issue(KYCField.KYC_FIELD_UNSPECIFIED, "不該出現"),
			),
		);
		expect(messages.form).toEqual({
			payoutContactEmail: "信箱無法收信",
			taxId: "與登記不符；請重新確認",
		});
	});

	it("表單已經沒有的欄位（舊申請的身分、銀行問題）不對到任何表單位置，名稱仍在", () => {
		const messages = kycIssueMessages(
			app(
				issue(KYCField.KYC_FIELD_ID_CARD_FRONT, "反光", "asset-front"),
				issue(KYCField.KYC_FIELD_BANK_CODE, "銀行代碼不存在"),
			),
		);
		expect(messages.form).toEqual({});
		expect(messages.companyDocuments).toBeNull();
		expect(kycFieldLabel(ut, KYCField.KYC_FIELD_ID_CARD_FRONT)).toBe(
			"身分證正面照",
		);
	});

	it("公司文件的說明以 asset ID 對到那一張，沒有指到特定一張的另外放", () => {
		const messages = kycIssueMessages(
			app(
				issue(KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, "過期", "co-2"),
				issue(KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, "請補登記文件"),
			),
		);
		expect(messages.companyDocumentByAsset).toEqual({ "co-2": "過期" });
		expect(messages.companyDocuments).toBe("請補登記文件");
	});
});
