import { create, toJson } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	KYCApplicationSchema,
	KYCField,
	KYCFlowStatus,
	KYCIssueCode,
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
	kycEditable,
	kycFieldOfPath,
	kycFormKeyOfDraftPath,
	kycFormOf,
	kycIssueMessages,
	mergeLatestForm,
	missingItemsOf,
	resumeSectionOf,
} from "./kyc";

const ut = uiTextFor("zh-TW");

const VALID_ID = "A123456789";

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
			{ realName: "王小明", englishName: "Wang" },
			{ baseline: { englishName: "Wang" }, etag: "e1" },
		);
		const body = json(request);
		expect(body.application.etag).toBe("e1");
		expect(body.updateMask).toBe("realName");
	});

	it("沒有任何改動不送請求", () => {
		expect(
			draft({ realName: "A" }, { baseline: { realName: "A" }, etag: "e1" }),
		).toBeNull();
	});

	it("格式不合的值不送：伺服器會因為一欄不合法整筆不寫入", () => {
		const request = draft(
			{
				realName: "王小明",
				idCardNo: "A000000000",
				bankAccount: "12",
			},
			{ etag: "e1" },
		);
		expect(json(request).updateMask).toBe("realName");
	});

	it("個人的身分證號寫在 individual，企業的寫在 business.rep_id_card_no", () => {
		expect(json(draft({ idCardNo: VALID_ID }, { etag: "e1" })).updateMask).toBe(
			"individual.idCardNo",
		);
		expect(
			json(
				draft({ sellerType: "business", idCardNo: VALID_ID }, { etag: "e1" }),
			).updateMask,
		).toBe("business.repIdCardNo");
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

	it("證件：新上傳的送 staged token，保留的送 asset id，移除的列進 mask 但沒有值", () => {
		const request = draft(
			{},
			{
				etag: "e1",
				docs: {
					idCardFront: { assetId: "", stagedToken: "tok", preview: "blob:x" },
					idCardBack: null,
				},
				baselineDocs: {
					idCardFront: { assetId: "a1", stagedToken: "", preview: "u" },
					idCardBack: { assetId: "a2", stagedToken: "", preview: "u" },
				},
			},
		);
		const body = json(request);
		expect(body.updateMask).toBe("idCardFront,idCardBack");
		expect(body.application.idCardFront).toEqual({ stagedToken: "tok" });
		expect(body.application.idCardBack).toBeUndefined();
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

	it("身分證號與銀行帳號送出後沿用送出的值，不會每次都重送", () => {
		const sent = {
			...EMPTY_KYC_FORM,
			idCardNo: VALID_ID,
			bankAccount: "12345678",
		};
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, [
			"individual.id_card_no",
			"bank_account",
		]);
		expect(baseline.idCardNo).toBe(VALID_ID);
		expect(request(sent, baseline)).toBeNull();
	});

	it("伺服器正規化（去空白、電話去橫線）不會讓已送出的欄位被當成又改了而重送", () => {
		const sent: KycForm = {
			...EMPTY_KYC_FORM,
			sellerType: "business",
			englishName: "Wang Farm ",
			companyPhone: "02-12345678",
		};
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, [
			"english_name",
			"business.company_phone",
		]);
		expect(request(sent, baseline)).toBeNull();
	});

	it("輸入原樣保留：打字中的尾端空白不會被換掉（表單不由伺服器的值覆寫）", () => {
		const sent: KycForm = { ...EMPTY_KYC_FORM, englishName: "Wang " };
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, ["english_name"]);
		const typed: KycForm = { ...sent, englishName: "Wang Farm" };
		expect(baseline.englishName).toBe("Wang ");
		expect(json(request(typed, baseline)).updateMask).toBe("englishName");
	});

	it("沒送出的欄位（打到一半的日期、不合格式的帳號）baseline 不變，仍與輸入不同", () => {
		const sent: KycForm = {
			...EMPTY_KYC_FORM,
			englishName: "Wang",
			birthDate: "1988-02-3",
			bankAccount: "123",
		};
		const baseline = baselineAfterSave(EMPTY_KYC_FORM, sent, ["english_name"]);
		expect(baseline.birthDate).toBe("");
		expect(baseline.bankAccount).toBe("");
		expect(baseline.englishName).toBe("Wang");
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
		realName: "王小明",
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
		expect(adopted.form.realName).toBe("王小明");
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
				KYCField.KYC_FIELD_BANK_ACCOUNT,
				KYCField.KYC_FIELD_ENGLISH_NAME,
			]),
		).toBe("bank");
		expect(resumeSectionOf([KYCField.KYC_FIELD_BUSINESS_TAX_ID])).toBe(
			"company",
		);
		expect(resumeSectionOf([KYCField.KYC_FIELD_ID_CARD_FRONT])).toBe(
			"identity",
		);
		expect(resumeSectionOf([KYCField.KYC_FIELD_ENGLISH_NAME])).toBe("other");
	});

	it("沒有缺項不指定位置", () => {
		expect(resumeSectionOf([])).toBeNull();
	});

	it("缺項清單依表單控制項去重：銀行是一個選擇器，BANK_NAME 與 BANK_CODE 只列一列「銀行名稱」", () => {
		const expected = [
			{ sectionId: "bank", label: "銀行名稱" },
			{ sectionId: "other", label: "英文姓名" },
		];
		expect(
			missingItemsOf(ut, [
				KYCField.KYC_FIELD_BANK_NAME,
				KYCField.KYC_FIELD_BANK_CODE,
				KYCField.KYC_FIELD_ENGLISH_NAME,
			]),
		).toEqual(expected);
		// 順序不影響：列 proto 順序最前面的欄位
		expect(
			missingItemsOf(ut, [
				KYCField.KYC_FIELD_BANK_CODE,
				KYCField.KYC_FIELD_BANK_NAME,
				KYCField.KYC_FIELD_ENGLISH_NAME,
			]),
		).toEqual(expected);
		// 只缺其中一個：列那個欄位自己的名詞
		expect(missingItemsOf(ut, [KYCField.KYC_FIELD_BANK_CODE])).toEqual([
			{ sectionId: "bank", label: "銀行代碼" },
		]);
	});

	it("缺項清單：不同控制項即使名詞相同也各列一列；同一個控制項的個人與公司負責人身分證字號列各自的名詞", () => {
		expect(
			missingItemsOf(ut, [
				KYCField.KYC_FIELD_ID_CARD_FRONT,
				KYCField.KYC_FIELD_ID_CARD_BACK,
			]),
		).toEqual([
			{ sectionId: "identity", label: "身分證正面照" },
			{ sectionId: "identity", label: "身分證反面照" },
		]);
		expect(
			missingItemsOf(ut, [KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO]),
		).toEqual([{ sectionId: "identity", label: "身分證字號" }]);
		expect(
			missingItemsOf(ut, [KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO]),
		).toEqual([{ sectionId: "identity", label: "負責人身分證字號" }]);
		expect(missingItemsOf(ut, [KYCField.KYC_FIELD_UNSPECIFIED])).toEqual([]);
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

describe("證件欄位的錯誤提示", () => {
	it("證件名稱加伺服器的訊息，補充文件註明第幾張；文字欄位不是證件", () => {
		expect(
			kycDocumentNotice(
				ut,
				"bank_book",
				KYCField.KYC_FIELD_BANK_BOOK,
				"圖片無法使用，請重新上傳",
			),
		).toBe("存摺封面：圖片無法使用，請重新上傳");
		expect(
			kycDocumentNotice(
				ut,
				"business.company_documents[0]",
				KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
				"x",
			),
		).toBe("補充文件（第 1 張）：x");
		expect(
			kycDocumentNotice(ut, "real_name", KYCField.KYC_FIELD_REAL_NAME, "x"),
		).toBeUndefined();
	});
});

describe("申請 → 表單", () => {
	it("日期、地址與賣家型態", () => {
		const form = kycFormOf(
			create(KYCApplicationSchema, {
				realName: "王小明",
				birthDate: { year: 1990, month: 2, day: 3 },
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
		expect(form.birthDate).toBe("1990-02-03");
		expect(form.companyAddressDetail).toBe("某路");
		expect(form.companyTownCode).toBe("63000010");
		expect(form.companyCountyCode).toBe("63000");
		expect(form.idCardNo).toBe("");
		expect(parseDate("1990-02-03")).toEqual({ year: 1990, month: 2, day: 3 });
		expect(parseDate("bad")).toBeUndefined();
	});
});

describe("版本衝突：保留輸入、顯示最新", () => {
	it("使用者改過的欄位保留，沒改的換成最新版本", () => {
		const oldBaseline = {
			...EMPTY_KYC_FORM,
			realName: "舊",
			englishName: "Old",
		};
		const current = { ...oldBaseline, realName: "我改的" };
		const latest = { ...oldBaseline, realName: "別處改的", englishName: "New" };
		const merged = mergeLatestForm(current, oldBaseline, latest);
		expect(merged.realName).toBe("我改的");
		expect(merged.englishName).toBe("New");
	});

	it("存檔回應把剛傳的證件換成申請上的圖片，保留本機預覽", () => {
		const staged = { assetId: "", stagedToken: "tok", preview: "blob:x" };
		const sent: KycDocs = { ...EMPTY_KYC_DOCS, idCardFront: staged };
		const saved: KycDocs = {
			...EMPTY_KYC_DOCS,
			idCardFront: { assetId: "a1", stagedToken: "", preview: "/dl/a1" },
		};
		expect(adoptSavedDocs(sent, sent, saved).idCardFront).toEqual({
			assetId: "a1",
			stagedToken: "",
			preview: "blob:x",
		});
		const changed: KycDocs = { ...EMPTY_KYC_DOCS, idCardFront: null };
		expect(adoptSavedDocs(changed, sent, saved).idCardFront).toBeNull();
	});

	it("申請的證件取 download_uri 當預覽", () => {
		const docs = kycDocsOf(
			create(KYCApplicationSchema, {
				idCardFront: { assetId: "a1", downloadUri: "/dl/a1" },
			}),
		);
		expect(docs.idCardFront).toEqual({
			assetId: "a1",
			stagedToken: "",
			preview: "/dl/a1",
		});
	});
});

describe("可編輯的狀態", () => {
	const app = (status: KYCFlowStatus, canResubmit = false) =>
		create(KYCApplicationSchema, { flow: { status, canResubmit } });
	it("沒有申請，或伺服器說能重送（canResubmit）時可以編輯，與流程狀態的值無關", () => {
		expect(kycEditable(null)).toBe(true);
		expect(kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT, true))).toBe(
			true,
		);
		expect(kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_REJECTED, true))).toBe(
			true,
		);
		// 綁定失敗而且賣家能自己修正（F17）。
		expect(
			kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED, true)),
		).toBe(true);
		expect(kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED))).toBe(
			false,
		);
		expect(
			kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED)),
		).toBe(false);
		expect(
			kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED, true)),
		).toBe(true);
		expect(kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED))).toBe(
			false,
		);
		expect(kycEditable(app(KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO))).toBe(
			false,
		);
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
		expect(kycFormKeyOfDraftPath("application.english_name")).toBe(
			"englishName",
		);
		expect(kycFormKeyOfDraftPath("application.business.rep_id_card_no")).toBe(
			"idCardNo",
		);
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
			lastRejection: { reason: "存摺不清楚", issues },
			outstandingIssues: issues,
		});

	it("沒有申請或沒有待處理的問題：什麼都不標", () => {
		const none = {
			form: {},
			slots: {},
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
					issue(KYCField.KYC_FIELD_REAL_NAME, "姓名已改過"),
					issue(KYCField.KYC_FIELD_ENGLISH_NAME, "英文姓名不符"),
				],
			},
			outstandingIssues: [
				issue(KYCField.KYC_FIELD_ENGLISH_NAME, "英文姓名不符"),
			],
		});
		expect(kycIssueMessages(application).form).toEqual({
			englishName: "英文姓名不符",
		});
	});

	it("欄位的說明對到表單欄位，同一處的多項說明接在一起", () => {
		const messages = kycIssueMessages(
			app(
				issue(KYCField.KYC_FIELD_BANK_CODE, "銀行代碼不存在"),
				issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, "與登記不符"),
				issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, "請重新確認"),
				issue(KYCField.KYC_FIELD_UNSPECIFIED, "不該出現"),
			),
		);
		expect(messages.form).toEqual({
			bankCode: "銀行代碼不存在",
			taxId: "與登記不符；請重新確認",
		});
	});

	it("單張證件的說明（不論有沒有指到那一張）對到證件位置", () => {
		const messages = kycIssueMessages(
			app(
				issue(KYCField.KYC_FIELD_ID_CARD_FRONT, "反光", "asset-front"),
				issue(KYCField.KYC_FIELD_ID_CARD_BACK, "缺少"),
				issue(KYCField.KYC_FIELD_BANK_BOOK, "過期", "asset-book"),
			),
		);
		expect(messages.slots).toEqual({
			idCardFront: "反光",
			idCardBack: "缺少",
			bankBook: "過期",
		});
		expect(messages.form).toEqual({});
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
