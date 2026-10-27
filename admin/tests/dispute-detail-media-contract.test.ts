import assert from "node:assert/strict";
import test from "node:test";
import {
	type DisputeDetail,
	selectAppealSubmittedAt,
	selectDisputePhotoKeys,
} from "../src/lib/disputes/detail";

/**
 * `GET /api/v1/admin/after_sales/{case_no}` 的媒體欄位契約。
 *
 * 兩份 fixture 是 gateway 的**實打回應原文**（2026-08-05 重新擷取，
 * `curl -H "Authorization: Bearer <admin JWT>" \
 *   http://127.0.0.1:19900/api/v1/admin/after_sales/{A674611149,A882981464}`）。
 * 打的是本機建置的 gateway 經 SSH 通道接 dev cluster 的 aftersales-service —— 因為
 * 頭像欄位收斂（`buyerAvatarKey`/`buyerAvatarUrl` → `avatarKey`/`avatarUrl`，49 §6.6
 * 扁平成對）的 backend commit 當時尚未部署到 test-api。除這兩欄外，回應與前一版
 * 擷取逐欄一致。
 * 不是手寫的期望值 —— 手寫 wire shape 正是這個 bug 的成因：爭議詳情頁自己宣告了
 * `evidence: { url, thumbnailUrl }`、`items: { productName, unitPrice }`、
 * `caseDetail.appealSubmittedAt`、`timeline: { desc }` 四組 gateway 從來沒送過的欄位，
 * TypeScript 全程無異議，畫面就一路空框 / 印 "undefined"（hawkr-website#430 A2）。
 *
 * 因此本檔的斷言分兩種，缺一不可：
 *   正向 — 選取器對實打回應取得到值（照片非空、金額轉得出數字）
 *   反向 — 舊欄位名在 wire 上**確實不存在**（否則「兩個名字都讀」的相容層照樣會通過）
 */

// 申訴案：appeal_state=10，2 張買家舉證（8601），無賣家驗貨照。
const APPEAL_CASE = {
	adminContext: {
		buyerAppealReason: "不同意賣家拒絕理由",
		buyerAppealDescription: "媒體契約驗證：申訴案，請平台檢視買家舉證照片",
		sellerRejectReason: "9104",
		sellerRejectMessage: "商品狀態正常，拒絕退款（媒體契約驗證）",
		buyerName: "測試買家",
		avatarKey: "public/defaults/buyer_avatar.png",
		avatarUrl:
			"https://test-media.example.com/i/original/public/defaults/buyer_avatar.png",
		sellerName: "測試",
		logoKey: "public/defaults/seller_logo.png",
		logoUrl:
			"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
	},
	arbitration: {
		state: "10",
		timeline: [
			{
				eventType: "APPEAL_SUBMITTED",
				timestamp: "2026-08-04T08:04:51.011503Z",
				label: "申訴提交",
				description:
					"買家提交申訴：不同意賣家拒絕理由；媒體契約驗證：申訴案，請平台檢視買家舉證照片",
			},
		],
	},
	caseDetail: {
		partnerId: "",
		returnShipmentNo: "",
		buyerId: "7489877415879899167",
		storeId: "7487768729113814280",
		status: "60",
		reason: "5001",
		description: "媒體契約驗證：申訴案前置，商品瑕疵附舉證照片",
		refundableAmount: "225",
		validationNo: "",
		returnShippingMethod: "",
		shipTradeNo: "",
		orderNo: "O869122935",
		caseNo: "A674611149",
		createdAt: "2026-08-04T08:04:21.597836Z",
		closedAt: "",
		appealState: "10",
		odno: "",
		refundTaskStatus: "",
		refundFailureMessage: "",
		estimatedRefundDate: "",
		bankDate: "",
		autoEscalated: false,
		processDeadlineHours: 72,
		resolutions: [
			{
				resolutionType: "5",
				refundAmount: "",
				couponAmount: "",
				couponMinSpend: "",
				couponMessage: "",
				rejectReason: "9104",
				rejectMessage: "商品狀態正常，拒絕退款（媒體契約驗證）",
				negotiationReason: "",
				createdAt: "2026-08-04T08:04:30.301474Z",
			},
		],
		shipmentPhotoKeys: null,
		shipmentPhotoUrls: null,
		evidence: [
			{
				assetId: "872389489879040650",
				imageKey:
					"public/aftersale/913700001/20260804/ae31d968-606a-47d2-8f7c-95ca750e9d77.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/aftersale/913700001/20260804/ae31d968-606a-47d2-8f7c-95ca750e9d77.png",
				evidenceType: "8601",
				createdAt: "2026-08-04T08:04:22.633861Z",
			},
			{
				assetId: "872389608871447057",
				imageKey:
					"public/aftersale/913700001/20260804/2d1cb378-1e4c-4fe9-810a-d51010a18ed7.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/aftersale/913700001/20260804/2d1cb378-1e4c-4fe9-810a-d51010a18ed7.png",
				evidenceType: "8601",
				createdAt: "2026-08-04T08:04:51.003604Z",
			},
		],
		items: [
			{
				orderItemId: "871950682890093666",
				skuId: "335",
				skuName: "單一規格",
				quantity: "1",
				originalPrice: "100",
				actualPayAmount: "100",
				imageKey: "",
				imageUrl: "",
			},
		],
		needRefundPage: false,
		canAppeal: false,
	},
} as unknown as DisputeDetail;

// 售後案：無申訴，1 張買家舉證（8601）+ 2 張賣家驗貨照（8602）。
const INSPECTION_CASE = {
	adminContext: {
		buyerAppealReason: "",
		buyerAppealDescription: "",
		sellerRejectReason: "",
		sellerRejectMessage: "",
		buyerName: "測試買家",
		avatarKey: "public/defaults/buyer_avatar.png",
		avatarUrl:
			"https://test-media.example.com/i/original/public/defaults/buyer_avatar.png",
		sellerName: "測試",
		logoKey: "public/defaults/seller_logo.png",
		logoUrl:
			"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
	},
	arbitration: {
		state: "0",
		timeline: [],
	},
	caseDetail: {
		partnerId: "",
		returnShipmentNo: "",
		buyerId: "7489760880179860732",
		storeId: "7487768729113814280",
		status: "40",
		reason: "5001",
		description:
			"商品瑕疵測試 - audit split verify run2\n媒體契約驗證：買家舉證補件（8601）",
		refundableAmount: "225",
		validationNo: "",
		returnShippingMethod: "3001",
		shipTradeNo: "",
		orderNo: "O271105429",
		caseNo: "A882981464",
		createdAt: "2026-08-03T03:01:45.564091Z",
		closedAt: "",
		appealState: "0",
		odno: "",
		refundTaskStatus: "30",
		refundFailureMessage: "",
		estimatedRefundDate: "",
		bankDate: "",
		processDeadline: "2026-08-06T03:01:45.563309Z",
		autoEscalated: false,
		processDeadlineHours: 72,
		returnDeliveredAt: "2026-08-04T08:03:51.929432Z",
		resolutions: [
			{
				resolutionType: "1",
				refundAmount: "225",
				couponAmount: "",
				couponMinSpend: "",
				couponMessage: "",
				rejectReason: "",
				rejectMessage: "",
				negotiationReason:
					"同意退貨退款（媒體契約驗證，DB 補建：宅配退貨建單在 dev 沙盒失敗）",
				createdAt: "2026-08-04T08:03:51.929432Z",
			},
			{
				resolutionType: "6",
				refundAmount: "180",
				couponAmount: "",
				couponMinSpend: "",
				couponMessage: "",
				rejectReason: "",
				rejectMessage: "",
				negotiationReason: "",
				createdAt: "2026-08-04T08:04:02.326674Z",
				deductionReason: "商品外包裝破損、配件缺件，扣除 45 元",
				inspectionNote: "驗貨照片如附（媒體契約驗證）",
			},
		],
		shipmentPhotoKeys: null,
		shipmentPhotoUrls: null,
		evidence: [
			{
				assetId: "872389174119252693",
				imageKey:
					"public/aftersale/745405000/20260804/762fd921-53dc-417b-9e35-55c971c2e2fe.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/aftersale/745405000/20260804/762fd921-53dc-417b-9e35-55c971c2e2fe.png",
				evidenceType: "8601",
				createdAt: "2026-08-04T08:03:07.363959Z",
			},
			{
				assetId: "872389401500862998",
				imageKey:
					"public/aftersale/2670447993/20260804/e1f287ae-aa73-4969-9adc-b6d31d96d606.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/aftersale/2670447993/20260804/e1f287ae-aa73-4969-9adc-b6d31d96d606.png",
				evidenceType: "8602",
				createdAt: "2026-08-04T08:04:02.311928Z",
			},
			{
				assetId: "872389404604648437",
				imageKey:
					"public/aftersale/2670447993/20260804/fb84738d-59e8-4ced-a0e7-81496616f477.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/aftersale/2670447993/20260804/fb84738d-59e8-4ced-a0e7-81496616f477.png",
				evidenceType: "8602",
				createdAt: "2026-08-04T08:04:02.316542Z",
			},
		],
		items: [
			{
				orderItemId: "871950810292079207",
				skuId: "336",
				skuName: "測試規格",
				quantity: "1",
				originalPrice: "100",
				actualPayAmount: "100",
				imageKey: "",
				imageUrl: "",
			},
		],
		needRefundPage: false,
		canAppeal: false,
	},
} as unknown as DisputeDetail;

test("wire 的 evidence 只有 imageKey/imageUrl —— 舊介面宣告的 url/thumbnailUrl 不存在", () => {
	const evidence = APPEAL_CASE.caseDetail.evidence;
	assert.equal(evidence.length, 2);
	for (const e of evidence) {
		assert.ok("imageKey" in e, "缺 imageKey（SSOT objectKey）");
		assert.ok("imageUrl" in e);
		assert.equal("url" in e, false, "wire 竟然有 url —— 舊欄位復活了");
		assert.equal("thumbnailUrl" in e, false);
		assert.ok(e.imageKey.startsWith("public/aftersale/"));
	}
});

test("買家舉證照片取得到 objectKey（修前這裡是兩個 undefined，畫面兩個空框）", () => {
	const submittedAt = selectAppealSubmittedAt(APPEAL_CASE.arbitration);
	const keys = selectDisputePhotoKeys(APPEAL_CASE.caseDetail, submittedAt);
	const all = [...keys.buyerOriginal, ...keys.buyerAppeal];
	assert.deepEqual(
		all,
		APPEAL_CASE.caseDetail.evidence.map((e) => e.imageKey),
		"分流後的 key 必須等於 wire 上的 imageKey 全集",
	);
	for (const key of all) {
		assert.equal(typeof key, "string");
		assert.notEqual(key, "", '空 key 會渲染成 src="" 的破圖框');
	}
	assert.deepEqual(keys.sellerInspection, []);
	assert.deepEqual(keys.sellerShipment, []);
});

test("8601/8602 分流：買家舉證與賣家驗貨照不會混在同一區塊", () => {
	const keys = selectDisputePhotoKeys(
		INSPECTION_CASE.caseDetail,
		selectAppealSubmittedAt(INSPECTION_CASE.arbitration),
	);
	const evidence = INSPECTION_CASE.caseDetail.evidence;
	assert.deepEqual(
		keys.buyerOriginal,
		evidence.filter((e) => e.evidenceType === "8601").map((e) => e.imageKey),
	);
	assert.deepEqual(
		keys.sellerInspection,
		evidence.filter((e) => e.evidenceType === "8602").map((e) => e.imageKey),
	);
	assert.equal(keys.buyerOriginal.length, 1);
	assert.equal(keys.sellerInspection.length, 2);
});

test("沒申訴的案件不會生出「申訴追加證據」區塊", () => {
	const submittedAt = selectAppealSubmittedAt(INSPECTION_CASE.arbitration);
	assert.equal(submittedAt, "", "timeline 為空時申訴時間必須是空字串");
	const keys = selectDisputePhotoKeys(INSPECTION_CASE.caseDetail, submittedAt);
	assert.deepEqual(keys.buyerAppeal, []);
});

test("申訴時間只能從 arbitration.timeline 取 —— caseDetail 沒有這個欄位", () => {
	assert.equal(
		"appealSubmittedAt" in APPEAL_CASE.caseDetail,
		false,
		"caseDetail 竟然有 appealSubmittedAt —— 那頁面該直接讀它",
	);
	assert.equal(
		selectAppealSubmittedAt(APPEAL_CASE.arbitration),
		"2026-08-04T08:04:51.011503Z",
	);
	// 沒有 timeline / arbitration 時不炸，回空字串讓畫面顯示 "-"。
	assert.equal(selectAppealSubmittedAt(undefined), "");
});

test("時間軸說明欄位叫 description，不叫 desc", () => {
	const entry = APPEAL_CASE.arbitration.timeline[0];
	assert.equal(entry.eventType, "APPEAL_SUBMITTED");
	assert.ok(entry.description.length > 0);
	assert.equal("desc" in entry, false);
});

test("售後明細有 skuName/actualPayAmount，沒有 productName/unitPrice", () => {
	const item = APPEAL_CASE.caseDetail.items[0];
	assert.equal("productName" in item, false);
	assert.equal(
		"unitPrice" in item,
		false,
		"unitPrice 不存在 —— 讀它就會印出 undefined",
	);
	assert.equal(item.skuName, "單一規格");
	assert.equal(Number(item.actualPayAmount), 100);
	assert.ok(Number.isFinite(Number(item.originalPrice)));
	assert.ok(item.orderItemId.length > 0, "列表 key 用 orderItemId，必須有值");
});

test("買家身分鍵是 caseDetail.buyerId，不是案件編號", () => {
	const { buyerId, caseNo } = APPEAL_CASE.caseDetail;
	assert.match(buyerId, /^\d+$/, "buyerId 是 surrogate 數字字串（設計書 44）");
	assert.match(caseNo, /^A\d+$/);
	assert.notEqual(buyerId, caseNo);
});

test("頭像/logo 有成對的 Key 欄位可讀（避免拿 /i/original/ URL 再剝回 key）", () => {
	const ctx = APPEAL_CASE.adminContext;
	assert.ok(ctx.avatarKey.length > 0);
	assert.ok(ctx.logoKey.length > 0, "商家 logo 的 key 欄位就叫 logoKey");
	assert.ok(ctx.avatarUrl.includes(`/i/original/${ctx.avatarKey}`));
	assert.ok(ctx.logoUrl.includes(`/i/original/${ctx.logoKey}`));
});

// 反向：頭像欄位收斂（49 §6.6）後，帶主體前綴的舊名在 wire 上必須真的消失。
// 少了這條，把頁面改回讀 buyerAvatarKey 仍會全綠（型別是我們自己宣告的），
// 而畫面上的買家頭像會是空框 —— 正是本檔開頭那個 bug 的形狀。
test("adminContext 的頭像只有 avatarKey/avatarUrl —— buyerAvatar* 舊名不存在", () => {
	for (const ctx of [APPEAL_CASE.adminContext, INSPECTION_CASE.adminContext]) {
		assert.ok("avatarKey" in ctx, "缺 avatarKey（SSOT objectKey）");
		assert.ok("avatarUrl" in ctx);
		assert.equal(
			"buyerAvatarKey" in ctx,
			false,
			"wire 竟然有 buyerAvatarKey —— 舊欄位復活了",
		);
		assert.equal("buyerAvatarUrl" in ctx, false);
		// 同物件裡的賣家 logo 維持 logoKey/logoUrl，收斂不該把兩個主體撞在一起。
		assert.notEqual(ctx.avatarKey, ctx.logoKey);
	}
});

test("出貨照片讀 shipmentPhotoKeys（SSOT）；缺值時回空陣列而非 null", () => {
	assert.equal(APPEAL_CASE.caseDetail.shipmentPhotoKeys, null);
	const keys = selectDisputePhotoKeys(APPEAL_CASE.caseDetail, "");
	assert.deepEqual(keys.sellerShipment, []);
});

test("沒有 objectKey 的佐證列被略過，不會渲染成空 src", () => {
	const broken = {
		...APPEAL_CASE.caseDetail,
		evidence: [
			{ ...APPEAL_CASE.caseDetail.evidence[0], imageKey: "" },
			APPEAL_CASE.caseDetail.evidence[1],
		],
	};
	const keys = selectDisputePhotoKeys(broken, "");
	assert.deepEqual(keys.buyerOriginal, [
		APPEAL_CASE.caseDetail.evidence[1].imageKey,
	]);
});
