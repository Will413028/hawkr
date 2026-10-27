import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	FoodRegistrationEndCause,
	FoodRegistrationInbox,
	FoodRegistrationSchema,
	FoodRegistrationState,
	FoodRegistrationSubjectSchema,
	KYCSellerType,
	StoreActionUnavailabilitySchema,
	StoreActionUnavailableReason,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { KYCStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	decideBlockedLabel,
	endCauseLabel,
	FADENBOOK_URL,
	LIST_TABS,
	listRequest,
	REVOKE_BUTTON_TEXT,
	REVOKE_DIALOG_TEXT,
	REVOKE_REASON_LABEL,
	STATE_LABEL,
	STATE_TABS,
	stateBadge,
	stateLabel,
	storeFoodRows,
	subjectRows,
} from "../src/lib/food-registration";
import { kycStatusText } from "../src/lib/kyc-status";

const request = (
	init: Parameters<typeof create<typeof FoodRegistrationSchema>>[1],
) => create(FoodRegistrationSchema, init);

test("收件匣與歷史的分頁涵蓋每個狀態，每個狀態都有文案與徽章", () => {
	assert.equal(STATE_TABS[0], FoodRegistrationState.PENDING);
	assert.equal(STATE_TABS.length, 5);
	for (const state of STATE_TABS) {
		assert.ok(STATE_LABEL[state]);
		assert.equal(stateBadge(state).label, STATE_LABEL[state]);
	}
	assert.equal(stateBadge(FoodRegistrationState.PENDING).variant, "warning");
	assert.equal(stateBadge(FoodRegistrationState.REJECTED).variant, "danger");
	// UNSPECIFIED 與不認得的值不是有效狀態：顯示共用目錄的「未知」，不編造狀態。
	for (const state of [
		FoodRegistrationState.UNSPECIFIED,
		99 as FoodRegistrationState,
	]) {
		assert.deepEqual(stateBadge(state), { label: "未知", variant: "gray" });
	}
});

test("KYC 狀態：沒有設（店家還沒有申請）顯示「尚未申請」，UNSPECIFIED 與不認得的值是「未知」，SUBMITTED 與 KYCFlowStatus 同為「待審核」", () => {
	assert.equal(kycStatusText(undefined), "尚未申請");
	assert.equal(kycStatusText(KYCStatus.KYC_STATUS_UNSPECIFIED), "未知");
	assert.equal(kycStatusText(99 as KYCStatus), "未知");
	assert.equal(kycStatusText(KYCStatus.KYC_STATUS_SUBMITTED), "待審核");
	assert.equal(kycStatusText(KYCStatus.KYC_STATUS_APPROVED), "已核准");
});

test("清單請求帶狀態、去掉前後空白的關鍵字與 page token", () => {
	const input = listRequest({
		state: FoodRegistrationState.REJECTED,
		inbox: FoodRegistrationInbox.UNSPECIFIED,
		keyword: "  A-123 ",
		pageToken: "tok",
		pageSize: 20,
	});
	assert.deepEqual(input, {
		state: FoodRegistrationState.REJECTED,
		inbox: FoodRegistrationInbox.UNSPECIFIED,
		query: "A-123",
		pageToken: "tok",
		pageSize: 20,
	});
});

test("主體：企業列公司名稱、統編與負責人，個人只列真實姓名，沒有申請時沒有列", () => {
	const company = request({
		subject: create(FoodRegistrationSubjectSchema, {
			sellerType: KYCSellerType.KYC_SELLER_TYPE_COMPANY,
			realName: "王負責",
			companyName: "好食股份有限公司",
			taxId: "22223223",
		}),
	});
	assert.deepEqual(subjectRows(company), [
		{ label: "公司名稱", value: "好食股份有限公司" },
		{ label: "統一編號", value: "22223223" },
		{ label: "負責人", value: "王負責" },
	]);
	const individual = request({
		subject: create(FoodRegistrationSubjectSchema, {
			sellerType: KYCSellerType.KYC_SELLER_TYPE_INDIVIDUAL,
			realName: "李翠綠",
		}),
	});
	assert.deepEqual(subjectRows(individual), [
		{ label: "真實姓名", value: "李翠綠" },
	]);
	assert.deepEqual(subjectRows(request({})), []);
});

// 伺服器真的會送的原因組合（model.FoodRegistrationUnavailableReasons）：句子與 reason 一起來，PAYUNI 之類不相干的值不會出現。
const U = StoreActionUnavailableReason;
const why = (reason: StoreActionUnavailableReason, message?: string) =>
	create(StoreActionUnavailabilitySchema, {
		reason,
		message: message ? { message, locale: "zh-TW" } : undefined,
	});
const KYC_NOT_APPROVED_TEXT = "店家的商家認證還沒核准，不能核准食品業者登錄。";
const STORE_CLOSED_TEXT = "店家已關閉，不能裁定食品業者登錄。";
const NOT_PENDING_TEXT = "這筆食品業者登錄已不在待審。";

test("不能裁定的句子只讀伺服器的原因，只顯示第一個：商家認證還沒核准、店家已關閉會顯示，不在待審（結構性）與可以裁定時什麼都不顯示", () => {
	// 待審、營運中、商家認證還沒核准：只擋核准。
	assert.equal(
		decideBlockedLabel(
			request({
				approveUnavailabilities: [
					why(U.KYC_NOT_APPROVED, KYC_NOT_APPROVED_TEXT),
				],
			}),
		),
		KYC_NOT_APPROVED_TEXT,
	);
	// 店家已關閉而且商家認證沒核准（真實的伺服器形狀）：核准列出兩個原因，只顯示第一個。
	assert.equal(
		decideBlockedLabel(
			request({
				approveUnavailabilities: [
					why(U.STORE_CLOSED, STORE_CLOSED_TEXT),
					why(U.KYC_NOT_APPROVED, KYC_NOT_APPROVED_TEXT),
				],
				rejectUnavailabilities: [why(U.STORE_CLOSED, STORE_CLOSED_TEXT)],
			}),
		),
		STORE_CLOSED_TEXT,
	);
	// 已不在待審（第一個原因是結構性的）：沒有入口，不說明，即使後面還有別的原因。
	assert.equal(
		decideBlockedLabel(
			request({
				approveUnavailabilities: [
					why(U.FOOD_REGISTRATION_NOT_PENDING, NOT_PENDING_TEXT),
					why(U.STORE_CLOSED, STORE_CLOSED_TEXT),
					why(U.KYC_NOT_APPROVED, KYC_NOT_APPROVED_TEXT),
				],
				rejectUnavailabilities: [
					why(U.FOOD_REGISTRATION_NOT_PENDING, NOT_PENDING_TEXT),
					why(U.STORE_CLOSED, STORE_CLOSED_TEXT),
				],
			}),
		),
		undefined,
	);
	// 可以裁定：兩份清單都是空的。
	assert.equal(
		decideBlockedLabel(request({ canApprove: true, canReject: true })),
		undefined,
	);
	// 旗標不是推導來源：伺服器說不能核准卻沒給原因時 client 不編句子。
	assert.equal(
		decideBlockedLabel(request({ canReject: true, canApprove: false })),
		undefined,
	);
});

test("裁定區：兩個動作都不能做而且有句子時只有標題與第一個原因（沒有備註欄、沒有鍵）；至少一個能做時維持原本的版面", async () => {
	const page = await readFile(
		"src/app/admin/food-registrations/[id]/page.tsx",
		"utf8",
	);
	assert.match(
		page,
		/const blockedLabel = decideBlockedLabel\(registration\);/u,
	);
	// 只有句子的區塊：從「兩個旗標都是 false 且有句子」的條件到完整版面的條件之間，沒有 textarea 與 Button。
	const startOnly = page.search(
		/!registration\.canApprove &&\s*!registration\.canReject &&\s*blockedLabel !== undefined/u,
	);
	const startFull = page.search(
		/\(registration\.canApprove \|\| registration\.canReject\) && \(/u,
	);
	assert.ok(startOnly > 0 && startFull > startOnly, "找不到兩個區塊");
	const only = page.slice(startOnly, startFull);
	assert.doesNotMatch(only, /<textarea|<Button|food-registration-note/u);
	assert.match(only, /\{blockedLabel\}/u);
	// 完整版面只在至少一個動作能做時畫出來，仍有備註欄、兩個鍵與（有句子時）那一句。
	const full = page.slice(startFull, startFull + 4000);
	assert.match(full, /food-registration-note/u);
	assert.match(full, /\{blockedLabel &&/u);
	assert.match(full, /核准[\s\S]*退回/u);
});

test("店家頁：核准的字號與待審的登錄各一列，待審列帶審核頁的 id", () => {
	assert.deepEqual(storeFoodRows({}), []);
	assert.deepEqual(
		storeFoodRows({
			approvedFoodRegistration: { registrationNumber: "A-1" },
			openFoodRegistration: {
				id: "req-1",
				state: FoodRegistrationState.PENDING,
				registrationNumber: "A-2",
			},
		}),
		[
			{ label: "食品業者登錄字號", value: "A-1" },
			{ label: "待審核的登錄", value: "A-2", registrationId: "req-1" },
		],
	);
	assert.equal(
		storeFoodRows({
			openFoodRegistration: {
				id: "req-2",
				state: FoodRegistrationState.REJECTED,
				registrationNumber: "A-3",
			},
		})[0]?.label,
		"已退回的登錄",
	);
});

test("審核頁：外部連結 rel noopener、不自己判斷能否核准、側欄有入口", async () => {
	assert.equal(FADENBOOK_URL, "https://fadenbook.fda.gov.tw/");
	const detail = await readFile(
		"src/app/admin/food-registrations/[id]/page.tsx",
		"utf8",
	);
	assert.match(detail, /rel="noopener noreferrer"/u);
	assert.match(detail, /registration\.canApprove/u);
	assert.doesNotMatch(detail, /KYC_STATUS_APPROVED/u);
	const sidebar = await readFile("src/components/Layout/Sidebar.tsx", "utf8");
	assert.match(sidebar, /\/admin\/food-registrations/u);
});

test("收件匣兩個分頁各帶自己的隊列與空狀態文字，歷史分頁不帶隊列", () => {
	const [decidable, awaiting, ...history] = LIST_TABS;
	assert.equal(decidable.label, "待審核");
	assert.equal(decidable.state, FoodRegistrationState.PENDING);
	assert.equal(decidable.inbox, FoodRegistrationInbox.DECIDABLE);
	assert.equal(decidable.emptyText, "沒有待審核的食品業者登錄");
	assert.equal(awaiting.label, "等商家認證");
	assert.equal(awaiting.state, FoodRegistrationState.PENDING);
	assert.equal(awaiting.inbox, FoodRegistrationInbox.AWAITING_KYC);
	assert.equal(awaiting.emptyText, "沒有等待商家認證的食品業者登錄");
	assert.deepEqual(
		history.map((tab) => tab.state),
		STATE_TABS.slice(1),
	);
	for (const tab of history) {
		assert.equal(tab.inbox, FoodRegistrationInbox.UNSPECIFIED);
		assert.equal(tab.label, stateLabel(tab.state));
	}
	assert.equal(new Set(LIST_TABS.map((tab) => tab.key)).size, LIST_TABS.length);
	for (const tab of [decidable, awaiting]) {
		const input = listRequest({
			state: tab.state,
			inbox: tab.inbox,
			keyword: "",
			pageToken: "",
			pageSize: 20,
		});
		assert.equal(input.inbox, tab.inbox);
	}
});

test("清單頁：副標與分頁來自 LIST_TABS，收件匣的請求帶分頁的隊列", async () => {
	const page = await readFile(
		"src/app/admin/food-registrations/page.tsx",
		"utf8",
	);
	assert.match(
		page,
		/「待審核」是店家已通過商家認證、可以裁定的件，「等商家認證」是還不能核准的件。/u,
	);
	assert.doesNotMatch(page, /實名認證已核准的件排在前面/u);
	assert.match(page, /inbox: tab\.inbox/u);
	assert.match(page, /tab\.emptyText/u);
});

test("撤銷：按鈕、對話框說明與欄位文案逐字，結束原因有文案，頁面只讀 can_revoke", async () => {
	assert.equal(REVOKE_BUTTON_TEXT, "撤銷登錄");
	assert.equal(
		REVOKE_DIALOG_TEXT,
		"撤銷後，店面與商品頁立即不再顯示這個字號，賣家需要重新送審。若屬冒用或不實登錄，請另外記錄違規認定。這家店若有同一字號的待審件，請一併退回。",
	);
	assert.equal(REVOKE_REASON_LABEL, "給賣家的理由（必填）");
	assert.equal(endCauseLabel(FoodRegistrationEndCause.REVOKED), "平台撤銷");
	assert.equal(
		endCauseLabel(FoodRegistrationEndCause.STORE_RETIRED),
		"商店已關閉",
	);
	assert.equal(endCauseLabel(FoodRegistrationEndCause.UNSPECIFIED), "未知");
	assert.equal(endCauseLabel(9999 as FoodRegistrationEndCause), "未知");
	const detail = await readFile(
		"src/app/admin/food-registrations/[id]/page.tsx",
		"utf8",
	);
	assert.match(detail, /registration\.canRevoke/u);
	assert.match(detail, /revokeFoodRegistration/u);
	// 撤銷不清掉核准時寫的內部註記：核准過的登錄上它標成「核准時的內部註記」。
	assert.match(detail, /核准時的內部註記/u);
	// 結束原因欄位的標籤是給賣家看的理由（Will 核定 2026-10-16），不是「撤銷理由」。
	assert.match(detail, /label="給賣家的理由"/u);
	assert.doesNotMatch(detail, /label="撤銷理由"/u);
	const dialog = await readFile(
		"src/app/admin/food-registrations/[id]/RevokeFoodRegistrationDialog.tsx",
		"utf8",
	);
	assert.match(dialog, /REVOKE_DIALOG_TEXT/u);
	assert.match(dialog, /REVOKE_REASON_LABEL/u);
});
