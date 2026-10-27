import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	type Measure,
	MeasureSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	ClosureSchema,
	StoreCloser,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PostalAddressSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/postal_address_pb";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import {
	MeasureBasis,
	MeasureKind,
	MeasureState,
	RiskGround,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	accountEmailOf,
	addressText,
	closerLabel,
	DELETE_ACCOUNT_WARNING_TEXT,
	deletionOutcome,
	END_HINT_TEXT,
	END_SUCCESS_NEUTRAL_TEXT,
	END_SUCCESS_STILL_SUSPENDED_TEXT,
	endSuccessText,
	LIFT_SUCCESS_TEXT,
	lifecycleBadge,
	MEASURE_CREATED_TEXT,
	REASON_MAX_LENGTH,
	SUSPEND_WARNING_TEXT,
	validateReason,
} from "../src/app/admin/merchants/[id]/store-detail";

/**
 * 商家詳情頁的顯示判斷與輸入驗證（store-detail.ts）。畫面只把這裡的結果接上去，這裡釘的是：哪個狀態隱藏哪個操作、
 * 停權原因的文字窮舉 enum、違規停權去哪裡、送出前擋下哪些輸入。契約本身由 proto 與後端測試守。
 */

function measure(patch: MessageInitShape<typeof MeasureSchema> = {}): Measure {
	return create(MeasureSchema, {
		id: "m1",
		storeId: "100",
		accountId: "5",
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.RISK,
		riskGround: RiskGround.FRAUD,
		statement: "疑似詐欺",
		state: MeasureState.ACTIVE,
		etag: "v1",
		canEnd: true,
		...patch,
	});
}

// 動作的可用性（建立措施、結束、核准、退回、刪帳號）都來自伺服器的旗標，store-detail.ts 沒有推導它們；接線在下面的原始碼斷言與各自的純邏輯測試。

test("生命週期徽章：已關閉、等待刪除有徽章，營運中沒有", () => {
	assert.deepEqual(lifecycleBadge(StoreStatus.CLOSED), {
		label: "已關閉",
		variant: "gray",
	});
	assert.deepEqual(lifecycleBadge(StoreStatus.PENDING_DELETION), {
		label: "等待刪除",
		variant: "warning",
	});
	assert.equal(lifecycleBadge(StoreStatus.ACTIVE), undefined);
});

test("關店的原因：賣家自己、終止合作（附措施 ID）、隨帳號刪除；沒有紀錄、終止卻沒有措施 ID 回 -", () => {
	const closure = (patch: MessageInitShape<typeof ClosureSchema>) =>
		create(ClosureSchema, { closer: StoreCloser.UNSPECIFIED, ...patch });
	assert.equal(
		closerLabel(closure({ closer: StoreCloser.ACCOUNT_OWNER })),
		"賣家自己關店",
	);
	assert.equal(
		closerLabel(closure({ closer: StoreCloser.TERMINATION, measureId: "77" })),
		"終止合作（措施 77）",
	);
	assert.equal(closerLabel(closure({ closer: StoreCloser.TERMINATION })), "-");
	assert.equal(
		closerLabel(closure({ closer: StoreCloser.ACCOUNT_DELETION })),
		"隨帳號刪除",
	);
	assert.equal(closerLabel(undefined), "-");
});

test("地址：縣市＋鄉鎮市區＋明細；沒有地址回 -", () => {
	const address = create(PostalAddressSchema, {
		townCode: "63000050",
		addressDetail: "重慶南路一段 1 號",
	});
	assert.equal(addressText(address), "臺北市中正區重慶南路一段 1 號");
	assert.equal(addressText(undefined), "-");
	assert.equal(addressText(create(PostalAddressSchema, {})), "-");
});

// ---------------------------------------------------------------------------
// 理由與文案

test("關店與刪帳號的理由：去掉空白後必填，上限 1000 字", () => {
	assert.equal(validateReason("   ", "請填寫關店理由。"), "請填寫關店理由。");
	assert.equal(validateReason(" 違約 ", "x"), undefined);
	assert.equal(validateReason("字".repeat(REASON_MAX_LENGTH), "x"), undefined);
	assert.ok(validateReason("字".repeat(REASON_MAX_LENGTH + 1), "x"));
});

test("刪除帳號的兩種結果文案照 D4 第 3 點；UNSPECIFIED 不是結果，不能宣告成功", () => {
	assert.deepEqual(
		deletionOutcome(DeleteAccountResponse_DeletionState.DELETED),
		{
			title: "已刪除這個帳號",
			text: "帳號與其賣家、買家角色都已移除，這一頁的資料不再有效。",
		},
	);
	assert.equal(
		deletionOutcome(DeleteAccountResponse_DeletionState.PENDING_ORDERS)?.text,
		"已受理。帳號有未完成的訂單，完成後會自動刪除；期間賣家只能處理訂單與售後。",
	);
	assert.equal(
		deletionOutcome(DeleteAccountResponse_DeletionState.UNSPECIFIED),
		undefined,
	);
});

test("D4 的原文：停權警告、解除、關店與刪帳號的說明文字不被改動；實作者補的句子沿用原文的部分", () => {
	assert.equal(
		SUSPEND_WARNING_TEXT,
		"停權後買家看不到這家店與它的商品，未付款訂單會被取消，優惠券會停用。",
	);
	assert.ok(
		MEASURE_CREATED_TEXT.endsWith(
			"買家已看不到這家店；未付款訂單與優惠券會陸續取消、停用。",
		),
	);
	assert.ok(
		END_HINT_TEXT.startsWith(
			"結束後店家符合公開條件時商品會恢復顯示，優惠券會自動恢復；已取消的訂單不會恢復。",
		),
	);
	assert.equal(
		LIFT_SUCCESS_TEXT,
		"已解除停權。店家符合公開條件時商品會恢復顯示，優惠券會自動恢復；已取消的訂單不會恢復。",
	);
	assert.equal(
		DELETE_ACCOUNT_WARNING_TEXT,
		"刪除整個帳號（買家與店家）。沒有未完成的訂單時立即刪除，否則等最後一筆訂單完成後自動刪除。無法復原。",
	);
});

test("詳情頁的建立措施按鈕由伺服器的 can_create_measure 決定、終止合作按鈕由 terminationAvailability 決定（都不看 store.status），刪帳號看帳號讀得到，而且閘直接包住自己的按鈕", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	const gates: [string, RegExp][] = [
		[
			"建立風險措施",
			/detail\.canCreateMeasure &&\s*\(\s*<button[^>]*btn-suspend-seller/u,
		],
		[
			"終止合作",
			/termination\.canCreate &&\s*\(\s*<button[^>]*btn-close-store/u,
		],
		[
			"刪除帳號",
			/accountEmail\.value\.state === "ready" &&\s*\(\s*<button[^>]*btn-delete-account/u,
		],
	];
	for (const [label, pattern] of gates) {
		assert.match(page, pattern, `${label} 沒有以伺服器的旗標為閘`);
	}
	assert.match(
		page,
		/const termination = terminationAvailability\(detail\);/u,
		"終止合作的入口必須來自 GetStore 的回應",
	);
	assert.match(
		page,
		/storeId=\{store\.id\}[\s\S]{0,80}?options=\{termination\.options\}/u,
		"對話框只列伺服器給的事由",
	);
});

test("結束措施後的成功訊息：重讀後帳號仍有生效中的措施就照實說，沒有就用 D4 第 2 點的原文", () => {
	assert.equal(endSuccessText([]), LIFT_SUCCESS_TEXT);
	assert.equal(
		endSuccessText([measure({ id: "m2" })]),
		END_SUCCESS_STILL_SUSPENDED_TEXT,
	);
	assert.doesNotMatch(
		END_SUCCESS_STILL_SUSPENDED_TEXT,
		/符合公開條件時商品會恢復顯示/u,
		"還有其他措施時不得沿用 D4 的「符合公開條件時商品會恢復顯示」",
	);
});

test("登入信箱的狀態：查不到帳號是 notFound；其他錯誤與「成功卻沒有帳號」都是 error（可重試），不當成空信箱", () => {
	assert.deepEqual(accountEmailOf({ error: null, data: undefined }), {
		state: "loading",
	});
	assert.deepEqual(accountEmailOf({ error: null, data: {} }), {
		state: "error",
	});
	assert.deepEqual(
		accountEmailOf({
			error: null,
			data: { account: { email: "owner@example.com" } },
		}),
		{ state: "ready", email: "owner@example.com" },
	);
	assert.deepEqual(
		accountEmailOf({ error: null, data: { account: { email: "" } } }),
		{ state: "ready", email: "" },
	);
	assert.deepEqual(
		accountEmailOf({
			error: new ConnectError("找不到", Code.NotFound),
			data: undefined,
		}),
		{ state: "notFound" },
	);
	assert.deepEqual(
		accountEmailOf({
			error: new ConnectError("暫時無法讀取", Code.Unavailable),
			data: undefined,
		}),
		{ state: "error" },
	);
});

test("中性的結束訊息不宣告恢復公開或其他措施的狀態", () => {
	assert.doesNotMatch(END_SUCCESS_NEUTRAL_TEXT, /恢復|符合公開條件|違規/u);
});

test("結束的接線：送讀到的措施 id 與 etag；版本衝突重讀店家；成功後重讀並依剩下的措施決定句子，重讀失敗時說中性的成功訊息", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /const handleEndMeasure = async \(measure: Measure\)/u);
	assert.match(
		page,
		/endMeasureMutation\.mutateAsync\(\{\s*id: measure\.id,\s*etag: measure\.etag\s*\}\)/u,
	);
	assert.match(page, /\(\) => storeQuery\.refetch\(\),/u);
	assert.match(
		page,
		/const after = await storeQuery\.refetch\(\);[\s\S]{0,200}?after\.isError \|\| !after\.data[\s\S]{0,120}?END_SUCCESS_NEUTRAL_TEXT[\s\S]{0,80}?endSuccessText\(after\.data\.activeMeasures\)/u,
	);
});

test("結束遇到版本衝突：關掉對話框並在頁首說明（店家已重讀），其他失敗丟回對話框就地顯示", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(
		page,
		/result\.kind === "conflict"\) \{\s*setEndTargetId\(null\);\s*setNotice\(\{ tone: "error", text: result\.text \}\);\s*return;\s*\}\s*throw new Error\(result\.text\);/u,
	);
});

test("狀態區的接線：每個措施一張卡（結束入口只看措施自己的 can_end，不看店家是否關閉）；結束對話框只對仍然 can_end 的最新那一筆開", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /const activeMeasures = detail\.activeMeasures;/u);
	assert.match(
		page,
		/const endTarget = activeMeasures\.find\(\s*\(m\) => m\.id === endTargetId && m\.canEnd,\s*\);/u,
	);
	assert.doesNotMatch(page, /storeActions|canEndMeasure/u);
	assert.match(
		page,
		/<ActiveMeasureCards[\s\S]{0,200}?active=\{activeMeasures\}[\s\S]{0,300}?onEnd=\{\(measure\) => openDialog\(\(\) => setEndTargetId\(measure\.id\)\)\}/u,
	);
	assert.match(
		page,
		/\{endTarget && \([\s\S]{0,80}?<EndMeasureDialog[\s\S]{0,200}?onConfirm=\{\(\) => handleEndMeasure\(endTarget\)\}/u,
	);
});

test("建立的接線：CreateMeasure 的請求原樣送出（request_id 由對話框決定），成功後說明；歷史卡讀帳號範圍", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /createMeasureMutation\.mutateAsync\(request\)/u);
	assert.match(page, /<CreateRiskMeasureDialog\s+storeId=\{store\.id\}/u);
	assert.match(page, /<CreateTerminationDialog\s+storeId=\{store\.id\}/u);
	// 關聯終止改在措施詳情頁建立（/admin/measures/[id]）：目標是該列帳號的店家，來源措施帶入。
	const measurePage = await readFile(
		"src/app/admin/measures/[id]/page.tsx",
		"utf8",
	);
	assert.match(
		measurePage,
		/<CreateTerminationDialog\s+storeId=\{linkedTarget\.account\.storeId\}[\s\S]{0,160}?sourceMeasureId=\{linkedTarget\.sourceMeasureId\}/u,
		"關聯終止的目標是該列帳號的店家，來源措施帶入",
	);
	assert.doesNotMatch(
		page,
		/MeasureDetailDialog|setLinkedTarget/u,
		"商家詳情不再有措施詳情對話框",
	);
	assert.match(
		page,
		/terminationCreatedText\(res\.measure\)/u,
		"成功提示的生效時間用伺服器回的 period.start_time",
	);
	assert.doesNotMatch(page, /closeStore|CloseStore|canCloseStore/u);
	assert.match(
		page,
		/<MeasureHistoryCard\s+accountId=\{store\.accountId\}/u,
		"歷史是帳號範圍；換店由外層 MerchantDetail 的 key 整頁重掛，page token 不跨帳號沿用（內層不再用同一個 key，見 merchant-detail-no-duplicate-keys）",
	);
	assert.doesNotMatch(
		page,
		/onOpenDetail/u,
		"歷史列的「查看」是連到 /admin/measures/[id] 的連結，不再由頁面開對話框",
	);
});

test("登入信箱：頁首與刪帳號確認框共用 GetAccount 的查詢；找不到帳號時兩處都說明", async () => {
	const [page, dialog, hook] = await Promise.all([
		readFile("src/app/admin/merchants/[id]/page.tsx", "utf8"),
		readFile(
			"src/app/admin/merchants/[id]/SellerAccountDeleteDialog.tsx",
			"utf8",
		),
		readFile("src/app/admin/merchants/[id]/use-store-detail.ts", "utf8"),
	]);
	assert.match(page, /useAccountEmail\(store\?\.accountId\)/u);
	assert.match(page, /state === "notFound"[\s\S]{0,200}?找不到登入帳號/u);
	assert.match(dialog, /state === "notFound"[\s\S]{0,120}?找不到登入帳號/u);
	assert.match(hook, /IdentityAdminService\.method\.getAccount/u);
	assert.match(
		hook,
		/accountEmailOf\(query\)/u,
		"狀態由 accountEmailOf 決定，成功卻沒有帳號不得當成空信箱",
	);
});

test("核准與退回：按鈕由 reviewActions（只讀申請上伺服器的旗標）決定，不能核准時只顯示伺服器的原因，而且各自只包住自己的按鈕", async () => {
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /const reviewable = reviewActions\(application\);/u);
	assert.doesNotMatch(page, /storeGates|storeActions|reviewBlockedReason\b/u);
	assert.match(
		page,
		/const blockedLabel = reviewBlockedLabel\(application\);/u,
		"原因只來自伺服器（reviewBlockedLabel 讀 approve_unavailabilities 的第一個原因）",
	);
	assert.match(
		page,
		/\{blockedLabel &&\s*\(\s*<span className="kyc-review-blocked"/u,
		"不能審核時顯示伺服器的第一個原因，不留沒有功能的控制",
	);
	assert.match(
		page,
		/\{reviewable\.canReject && \(\s*<button[\s\S]{0,120}?btn-reject-figma/u,
		"退回按鈕必須由 reviewable.canReject 直接包住",
	);
	assert.match(
		page,
		/\{reviewable\.canApprove && \([\s\S]{0,400}?btn-approve-figma/u,
		"通過審核按鈕必須由 reviewable.canApprove 直接包住",
	);
});
