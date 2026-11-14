import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	type StoreListItem,
	StoreListItemSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	KYCStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	kycBadge,
	rowBadge,
	rowLocation,
	rowState,
} from "../src/app/admin/merchants/store-row";

/**
 * 商家清單一列的顯示判斷：徽章與按鈕顏色由 kyc_status（沒有申請時不設）、suspended 與店家的生命週期推出；代行入口能不能用不在這裡（伺服器的 delegationUnavailabilities）。
 */

function item(
	status: KYCStatus | undefined,
	opts: {
		suspended?: boolean;
		status?: StoreStatus;
		townCode?: string;
	} = {},
): StoreListItem {
	return create(StoreListItemSchema, {
		kycStatus: status,
		suspended: opts.suspended ?? false,
		store: {
			id: "1",
			status: opts.status ?? StoreStatus.ACTIVE,
			address: opts.townCode ? { townCode: opts.townCode } : undefined,
		},
	});
}

const K = KYCStatus;
const ALL_KYC_STATUSES = Object.values(K).filter(
	(v): v is KYCStatus => typeof v === "number",
);

test("isActive 只看店家生命週期、與申請狀態無關（只用於按鈕與徽章的顏色；代行能不能用是伺服器的 delegationUnavailabilities）", () => {
	for (const status of [undefined, ...ALL_KYC_STATUSES]) {
		assert.equal(
			rowState(item(status)).isActive,
			true,
			`${status === undefined ? "沒有申請" : K[status]}：營運中的店家不因審核進度而不能代行`,
		);
	}
	for (const status of [StoreStatus.PENDING_DELETION, StoreStatus.CLOSED]) {
		assert.equal(
			rowState(item(K.KYC_STATUS_APPROVED, { status })).isActive,
			false,
			StoreStatus[status],
		);
	}
});

test("審核中與被退回的分類", () => {
	const pending = rowState(item(K.KYC_STATUS_SUBMITTED));
	assert.equal(pending.isPending, true);
	assert.equal(pending.isRejected, false);
	const rejected = rowState(item(K.KYC_STATUS_REJECTED));
	assert.equal(rejected.isRejected, true);
	assert.equal(rejected.isPending, false);
	for (const status of [
		undefined,
		K.KYC_STATUS_DRAFT,
		K.KYC_STATUS_APPROVED,
		K.KYC_STATUS_CANCELLED,
		K.KYC_STATUS_UNSPECIFIED,
	]) {
		const s = rowState(item(status));
		assert.equal(s.isPending, false, String(status));
		assert.equal(s.isRejected, false, String(status));
	}
});

test("徽章：已關閉與停權優先於申請狀態", () => {
	assert.deepEqual(
		rowBadge(item(K.KYC_STATUS_APPROVED, { status: StoreStatus.CLOSED })),
		{ label: "已關閉", variant: "gray" },
	);
	assert.deepEqual(rowBadge(item(K.KYC_STATUS_APPROVED, { suspended: true })), {
		label: "停權中",
		variant: "danger",
	});
	assert.equal(
		rowBadge(item(K.KYC_STATUS_SUBMITTED, { suspended: true })).label,
		"停權中",
		"停權優先於申請狀態",
	);
	// 已關閉優先於停權。
	assert.equal(
		rowBadge(
			item(K.KYC_STATUS_APPROVED, {
				suspended: true,
				status: StoreStatus.CLOSED,
			}),
		).label,
		"已關閉",
	);
});

test("徽章：有申請時文字取自共用目錄（後台員工受眾），顏色是 admin 的呈現", () => {
	assert.deepEqual(rowBadge(item(K.KYC_STATUS_APPROVED)), {
		label: "已核准",
		variant: "success",
	});
	assert.deepEqual(rowBadge(item(K.KYC_STATUS_SUBMITTED)), {
		label: "待審核",
		variant: "warning",
	});
	assert.deepEqual(rowBadge(item(K.KYC_STATUS_REJECTED)), {
		label: "已退回",
		variant: "danger",
	});
	assert.deepEqual(rowBadge(item(K.KYC_STATUS_DRAFT)), {
		label: "填寫中",
		variant: "gray",
	});
});

test("徽章：沒有申請（kyc_status 不帶值）顯示 admin 自己的「尚未申請」，灰色", () => {
	assert.deepEqual(rowBadge(item(undefined)), {
		label: "尚未申請",
		variant: "gray",
	});
	assert.deepEqual(kycBadge(undefined), { label: "尚未申請", variant: "gray" });
});

test("徽章：UNSPECIFIED 與不認得的值不是有效狀態，是「未知」，不當成沒有申請", () => {
	for (const status of [K.KYC_STATUS_UNSPECIFIED, 99 as KYCStatus]) {
		assert.deepEqual(rowBadge(item(status)), {
			label: "未知",
			variant: "gray",
		});
	}
});

test("地點由 town_code 對照縣市與鄉鎮市區；沒有或不認得時是 -", () => {
	assert.equal(rowLocation(item(KYCStatus.KYC_STATUS_APPROVED)), "-");
	assert.equal(
		rowLocation(item(KYCStatus.KYC_STATUS_APPROVED, { townCode: "00000000" })),
		"-",
	);
	const known = rowLocation(
		item(KYCStatus.KYC_STATUS_APPROVED, { townCode: "63000010" }),
	);
	assert.match(known, /^臺北市.+區$/u);
});
