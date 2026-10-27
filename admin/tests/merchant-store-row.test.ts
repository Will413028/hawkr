import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	type StoreListItem,
	StoreListItemSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCFlowStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	rowBadge,
	rowLocation,
	rowState,
} from "../src/app/admin/merchants/store-row";

/**
 * 商家清單一列的顯示判斷：徽章與按鈕顏色由 kyc_flow_status、suspended 與店家的生命週期推出；代行入口能不能用不在這裡（伺服器的 delegationUnavailabilities）。
 */

function item(
	flow: KYCFlowStatus,
	opts: {
		suspended?: boolean;
		status?: StoreStatus;
		townCode?: string;
	} = {},
): StoreListItem {
	return create(StoreListItemSchema, {
		kycFlowStatus: flow,
		suspended: opts.suspended ?? false,
		store: {
			id: "1",
			status: opts.status ?? StoreStatus.ACTIVE,
			address: opts.townCode ? { townCode: opts.townCode } : undefined,
		},
	});
}

const F = KYCFlowStatus;

test("isActive 只看店家生命週期、與商家認證流程無關（只用於按鈕與徽章的顏色；代行能不能用是伺服器的 delegationUnavailabilities）", () => {
	for (const flow of Object.values(F).filter(
		(v): v is KYCFlowStatus => typeof v === "number",
	)) {
		assert.equal(
			rowState(item(flow)).isActive,
			true,
			`${F[flow]}：營運中的店家不因審核進度而不能代行`,
		);
	}
	for (const status of [StoreStatus.PENDING_DELETION, StoreStatus.CLOSED]) {
		assert.equal(
			rowState(item(F.KYC_FLOW_STATUS_APPROVED, { status })).isActive,
			false,
			StoreStatus[status],
		);
	}
});

test("審核中與被退回的分類", () => {
	for (const flow of [
		F.KYC_FLOW_STATUS_SUBMITTED,
		F.KYC_FLOW_STATUS_BINDING,
		F.KYC_FLOW_STATUS_BIND_FAILED,
		F.KYC_FLOW_STATUS_FORWARDED,
		F.KYC_FLOW_STATUS_UNDER_REVIEW,
		F.KYC_FLOW_STATUS_PENDING_INFO,
	]) {
		const s = rowState(item(flow));
		assert.equal(s.isPending, true, F[flow]);
		assert.equal(s.isRejected, false, F[flow]);
	}
	for (const flow of [
		F.KYC_FLOW_STATUS_REJECTED,
		F.KYC_FLOW_STATUS_PAYUNI_REJECTED,
	]) {
		const s = rowState(item(flow));
		assert.equal(s.isRejected, true, F[flow]);
		assert.equal(s.isPending, false, F[flow]);
	}
});

test("徽章：已關閉與停權優先於認證流程", () => {
	assert.deepEqual(
		rowBadge(item(F.KYC_FLOW_STATUS_APPROVED, { status: StoreStatus.CLOSED })),
		{ label: "已關閉", variant: "gray" },
	);
	assert.deepEqual(
		rowBadge(item(F.KYC_FLOW_STATUS_APPROVED, { suspended: true })),
		{ label: "停權中", variant: "danger" },
	);
	assert.equal(
		rowBadge(item(F.KYC_FLOW_STATUS_SUBMITTED, { suspended: true })).label,
		"停權中",
		"停權優先於認證流程",
	);
	// 已關閉優先於停權。
	assert.equal(
		rowBadge(
			item(F.KYC_FLOW_STATUS_APPROVED, {
				suspended: true,
				status: StoreStatus.CLOSED,
			}),
		).label,
		"已關閉",
	);
	assert.equal(rowBadge(item(F.KYC_FLOW_STATUS_APPROVED)).label, "已核准");
	assert.equal(
		rowBadge(item(F.KYC_FLOW_STATUS_BIND_FAILED)).label,
		"待重新綁定",
	);
	assert.equal(
		rowBadge(item(F.KYC_FLOW_STATUS_UNDER_REVIEW)).label,
		"PayUni 審核中",
	);
	// 沒有申請：後台員工受眾的「尚未申請」（共用目錄），灰色。
	assert.deepEqual(rowBadge(item(F.KYC_FLOW_STATUS_NOT_STARTED)), {
		label: "尚未申請",
		variant: "gray",
	});
	// UNSPECIFIED 與不認得的值不是有效狀態：「未知」，不編造成未啟用。
	for (const flow of [F.KYC_FLOW_STATUS_UNSPECIFIED, 99 as KYCFlowStatus]) {
		assert.deepEqual(rowBadge(item(flow)), { label: "未知", variant: "gray" });
	}
});

test("地點由 town_code 對照縣市與鄉鎮市區；沒有或不認得時是 -", () => {
	assert.equal(rowLocation(item(F.KYC_FLOW_STATUS_APPROVED)), "-");
	assert.equal(
		rowLocation(item(F.KYC_FLOW_STATUS_APPROVED, { townCode: "00000000" })),
		"-",
	);
	const known = rowLocation(
		item(F.KYC_FLOW_STATUS_APPROVED, { townCode: "63000010" }),
	);
	assert.match(known, /^臺北市.+區$/u);
});
