/**
 * lib/after-sales/normalize.ts — wire → domain normalization. The backend
 * mixes camelCase and snake_case across the buyer/seller views, so these
 * tests pin the defensive coercion the UI gating relies on, in particular:
 *   - `appealState` (the platform-arbitration lock input — once non-empty,
 *     buyer appeal and seller resolve/confirm are gated off in the UI)
 *   - `refundProgress.needRefundPage` derived from refundTaskStatus "5"
 *   - buyer vs seller evidence splitting.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import {
	normalizeArbitration,
	normalizeCase,
	normalizeDetail,
} from "../lib/after-sales/normalize";

/* ===== normalizeCase ===== */

test("normalizeCase reads snake_case wire fields and coerces values to strings", () => {
	const c = normalizeCase({
		case_no: "AS001",
		order_no: "O001",
		buyerId: 42,
		buyer_name: "買家",
		status: 27,
		reason_code: 5001,
		reason_text: "商品損壞",
		refundable_amount: 350,
		return_shipment_no: "SHIP1",
		return_shipping_method: 3002,
		created_at: "2026-06-01T00:00:00Z",
		store_pickup_deadline_at: "2026-06-04T00:00:00Z",
	});

	assert.equal(c.caseNo, "AS001");
	assert.equal(c.orderNo, "O001");
	assert.equal(c.buyerId, "42");
	assert.equal(c.status, "27");
	assert.equal(c.reason, "5001");
	assert.equal(c.description, "商品損壞");
	assert.equal(c.refundableAmount, "350");
	assert.equal(c.returnShippingMethod, "3002");
	assert.equal(c.storePickupDeadlineAt, "2026-06-04T00:00:00Z");
});

test("normalizeCase prefers camelCase but falls through empty strings to snake_case", () => {
	const c = normalizeCase({
		caseNo: "",
		case_no: "AS002",
		orderNo: "O-camel",
		order_no: "O-snake",
		status: "0",
	});
	assert.equal(c.caseNo, "AS002");
	assert.equal(c.orderNo, "O-camel");
});

test("normalizeCase yields safe defaults for an empty wire object", () => {
	const c = normalizeCase({});
	assert.equal(c.caseNo, "");
	assert.equal(c.orderNo, "");
	assert.equal(c.status, "");
	assert.equal(c.refundableAmount, "");
	assert.equal(c.closedAt, undefined);
	assert.equal(c.storePickupDeadlineAt, undefined);
	// NONE / absent appealState stays undefined so list rows don't false-fold.
	assert.equal(c.appealState, undefined);
});

test("normalizeCase surfaces appeal_state from the list summary so rows can fold 申訴仲裁中", () => {
	const snake = normalizeCase({
		case_no: "AS001",
		status: "60",
		appeal_state: "10",
	});
	assert.equal(snake.appealState, "10");
	// camelCase key, same codemeta Value — the wire never carries the Name.
	const camel = normalizeCase({
		caseNo: "AS001",
		status: "0",
		appealState: "20",
	});
	assert.equal(camel.appealState, "20");
	// DEFAULT "0" on the wire must NOT be read as appealed.
	const none = normalizeCase({
		case_no: "AS001",
		status: "60",
		appeal_state: "0",
	});
	assert.equal(none.appealState, "0");
});

/* ===== normalizeDetail — appeal lock fields ===== */

test("normalizeDetail surfaces appeal_state — the arbitration lock the UI gates on", () => {
	const snake = normalizeDetail({
		case_no: "AS001",
		status: "60",
		appeal_state: "10",
		can_appeal: "false",
		appeal_deadline: "2026-06-10T00:00:00Z",
	});
	assert.equal(snake.appealState, "10");
	assert.equal(snake.canAppeal, false);
	assert.equal(snake.appealDeadline, "2026-06-10T00:00:00Z");

	const camel = normalizeDetail({ caseNo: "AS001", appealState: "10" });
	assert.equal(camel.appealState, "10");
});

test("normalizeDetail keeps an absent/empty appealState undefined so the lock stays off", () => {
	const absent = normalizeDetail({ case_no: "AS001", status: "60" });
	assert.equal(absent.appealState, undefined);
	const empty = normalizeDetail({
		case_no: "AS001",
		status: "60",
		appeal_state: "",
	});
	assert.equal(empty.appealState, undefined);
});

test("normalizeDetail coerces canAppeal/autoEscalated from string and numeric booleans", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		can_appeal: "1",
		auto_escalated: 0,
	});
	assert.equal(d.canAppeal, true);
	assert.equal(d.autoEscalated, false);
});

/* ===== normalizeDetail — refund progress ===== */

test("normalizeDetail derives needRefundPage from flat refundTaskStatus 5", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		status: "40",
		refundTaskStatus: "5",
	});
	assert.equal(d.refundProgress?.refundTaskStatus, "5");
	assert.equal(d.refundProgress?.needRefundPage, true);

	const processing = normalizeDetail({
		case_no: "AS001",
		status: "40",
		refund_task_status: "10",
		refund_failure_message: "",
	});
	assert.equal(processing.refundProgress?.refundTaskStatus, "10");
	assert.equal(processing.refundProgress?.needRefundPage, false);
});

test("normalizeDetail lets an explicit need_refund_page override the derived flag", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		refundTaskStatus: "5",
		need_refund_page: false,
	});
	assert.equal(d.refundProgress?.needRefundPage, false);
});

test("normalizeDetail merges a nested refundProgress block over flat fields", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		refundTaskStatus: "10",
		refundProgress: {
			refundTaskStatus: "30",
			refund_failure_message: "卡號錯誤",
		},
	});
	assert.equal(d.refundProgress?.refundTaskStatus, "30");
	assert.equal(d.refundProgress?.refundFailureMessage, "卡號錯誤");
});

/* ===== normalizeDetail — evidence, items, logistics ===== */

/**
 * Contract fixture — captured from the dev Gateway response for case
 * A882981464 (`GET /app/after_sales/A882981464`, 2026-08-05; the seller view
 * `GET /app/seller/after_sales/A882981464` returns an identical `evidence[]`).
 * Field names, types and values are as they came off the wire; only one long
 * free-text `negotiationReason` was shortened.
 * The point of pinning the real shape is `evidenceType`: it is a CodeMeta
 * AfterSaleEvidenceType NUMERIC code ("8601" 買家舉證 / "8602" 賣家驗貨照片),
 * NOT a "BUYER"/"SELLER" role literal — those never existed anywhere in the
 * chain, and comparing against them made 驗貨照片 render 0 張 forever (#430 A3).
 */
const CASE_DETAIL_WIRE = {
	caseNo: "A882981464",
	orderNo: "O271105429",
	buyerId: "7489760880179860732",
	status: "40",
	reason: "5001",
	refundableAmount: "225",
	returnShippingMethod: "3001",
	createdAt: "2026-08-03T03:01:45.564091Z",
	appealState: "0",
	refundTaskStatus: "30",
	resolutions: [
		{
			resolutionType: "1",
			refundAmount: "225",
			couponAmount: "",
			couponMinSpend: "",
			couponMessage: "",
			rejectReason: "",
			rejectMessage: "",
			negotiationReason: "同意退貨退款（媒體契約驗證）",
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
};

test("normalizeDetail splits the REAL wire evidence by numeric AfterSaleEvidenceType", () => {
	const d = normalizeDetail(CASE_DETAIL_WIRE);

	// 1 × 8601 → 買家舉證區；2 × 8602 → 驗貨照片區。Both non-empty is the whole
	// point: the pre-fix code compared against "SELLER" and left sellerPhotos [].
	// §231 (#430 D3-S5): the evidence entry is read from `imageKey` — the bare
	// objectKey SSOT — not from the Gateway-composed `imageUrl` twin. Pinning the
	// key here is what keeps a future revert from silently going back to the URL.
	assert.deepEqual(
		d.photos.map((p) => p.url),
		[
			"public/aftersale/745405000/20260804/762fd921-53dc-417b-9e35-55c971c2e2fe.png",
		],
	);
	assert.deepEqual(
		d.sellerPhotos.map((p) => p.url),
		[
			"public/aftersale/2670447993/20260804/e1f287ae-aa73-4969-9adc-b6d31d96d606.png",
			"public/aftersale/2670447993/20260804/fb84738d-59e8-4ced-a0e7-81496616f477.png",
		],
	);
});

test("normalizeDetail keeps the 驗貨扣款 resolution fields the real wire carries", () => {
	const d = normalizeDetail(CASE_DETAIL_WIRE);
	const inspection = d.resolutions[1];

	assert.equal(d.resolutions.length, 2);
	assert.equal(inspection.resolutionType, "6");
	// The wire has NO `actualRefundAmount` / `inspectionPhotos` on a resolution
	// (gateway ResolutionResponse) — the deducted amount IS `refundAmount`, and
	// the photos live in `evidence[]` as 8602.
	assert.equal(inspection.refundAmount, "180");
	assert.equal(
		inspection.deductionReason,
		"商品外包裝破損、配件缺件，扣除 45 元",
	);
	assert.equal(inspection.inspectionNote, "驗貨照片如附（媒體契約驗證）");
});

test("normalizeDetail treats untagged / unknown evidence as buyer evidence, never drops it", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		evidence: [
			{ imageKey: "public/a/tagged-8601.jpg", evidenceType: "8601" },
			{ imageKey: "public/a/untagged.jpg" },
			{ imageKey: "public/a/snake-8602.jpg", evidence_type: "8602" },
			{ evidenceType: "8601" }, // no imageKey → dropped
		],
	});
	assert.deepEqual(
		d.photos.map((p) => p.url),
		["public/a/tagged-8601.jpg", "public/a/untagged.jpg"],
	);
	assert.deepEqual(
		d.sellerPhotos.map((p) => p.url),
		["public/a/snake-8602.jpg"],
	);
});

test("normalizeDetail maps order items with skuName/price fallbacks", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		order_items: [
			{
				order_item_id: 7,
				sku_name: "A 規格 600g",
				quantity: "2",
				actual_pay_amount: "180",
				original_price: "200",
			},
		],
	});
	assert.equal(d.items.length, 1);
	assert.equal(d.items[0].orderItemId, "7");
	// No product name on the wire → falls back to the sku name.
	assert.equal(d.items[0].name, "A 規格 600g");
	assert.equal(d.items[0].quantity, 2);
	assert.equal(d.items[0].price, "180");
});

test("normalizeDetail builds return logistics from flat detail fields (status 28 failure text)", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		status: "28",
		return_shipping_method: "3002",
		return_shipment_no: "SHIP1",
		partner_id: "P1",
		odno: "OD1",
		validation_no: "V1",
		return_provider_message: "包裹逾期未取",
	});
	assert.deepEqual(d.returnLogistics, {
		method: "3002",
		shipmentNo: "SHIP1",
		partnerId: "P1",
		odno: "OD1",
		validationNo: "V1",
		shipmentStatus: "包裹逾期未取",
	});
	assert.equal(d.returnProviderMessage, "包裹逾期未取");
});

test("normalizeDetail prefers a nested returnLogistics object over flat fields", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		return_shipment_no: "FLAT",
		returnLogistics: { shipment_no: "NESTED", method: "3001" },
	});
	assert.equal(d.returnLogistics?.shipmentNo, "NESTED");
	assert.equal(d.returnLogistics?.method, "3001");
});

test("normalizeDetail maps resolutions from the decisions alias", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		decisions: [
			{
				resolution_type: 5,
				reject_reason: "4001",
				reject_message: "證據不足",
				created_at: "2026-06-02T00:00:00Z",
			},
		],
	});
	assert.equal(d.resolutions.length, 1);
	assert.equal(d.resolutions[0].resolutionType, "5");
	assert.equal(d.resolutions[0].rejectReason, "4001");
	assert.equal(d.resolutions[0].rejectMessage, "證據不足");
});

test("normalizeDetail maps compensation coupon and inspection deduction resolution fields", () => {
	const d = normalizeDetail({
		case_no: "AS001",
		decisions: [
			{
				resolution_type: 4,
				coupon_amount: "80",
				coupon_min_spend: "300",
				coupon_message: "售後補償",
				created_at: "2026-06-02T00:00:00Z",
			},
			{
				resolution_type: 6,
				refund_amount: "120",
				deduction_reason: "商品已拆封",
				inspection_note: "外盒破損，內容物正常",
			},
		],
	});

	assert.equal(d.resolutions.length, 2);
	assert.equal(d.resolutions[0].resolutionType, "4");
	assert.equal(d.resolutions[0].couponAmount, "80");
	assert.equal(d.resolutions[0].couponMinSpend, "300");
	assert.equal(d.resolutions[0].couponMessage, "售後補償");
	assert.equal(d.resolutions[1].resolutionType, "6");
	assert.equal(d.resolutions[1].refundAmount, "120");
	assert.equal(d.resolutions[1].deductionReason, "商品已拆封");
	assert.equal(d.resolutions[1].inspectionNote, "外盒破損，內容物正常");
});

/* ===== normalizeArbitration ===== */

test("normalizeArbitration maps state, decision and snake_case timeline events", () => {
	const arb = normalizeArbitration({
		state: 20,
		decision: {
			outcome: "PARTIAL_REFUND",
			reason: "部分責任歸屬賣家",
			refund_amount: "150",
			decided_at: "2026-06-08T00:00:00Z",
		},
		timeline: [
			{
				event_type: "APPEAL_SUBMITTED",
				label: "申訴提交",
				description: "買家提出申訴",
				timestamp: "2026-06-05T00:00:00Z",
			},
		],
	});
	assert.equal(arb.state, "20");
	assert.equal(arb.decision?.outcome, "PARTIAL_REFUND");
	assert.equal(arb.decision?.refundAmount, "150");
	assert.equal(arb.timeline.length, 1);
	assert.equal(arb.timeline[0].eventType, "APPEAL_SUBMITTED");
});

test("normalizeArbitration treats a missing or empty decision as undecided", () => {
	assert.equal(
		normalizeArbitration({ state: "10", timeline: [] }).decision,
		undefined,
	);
	assert.equal(
		normalizeArbitration({ state: "10", decision: {}, timeline: [] }).decision,
		undefined,
	);
});
