"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type {
	PayUniRegistration,
	Store,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PayUniBindStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import type { ReactNode } from "react";
import { Button, StatusBadge } from "@/components/UI";
import { formatDateTimeShort } from "@/lib/format";
import { flowBadge } from "../store-row";
import {
	type PayUniState,
	payUniProblemFields,
	rebindPresentation,
} from "./kyc-application";
import { addressText } from "./store-detail";

// 商家詳情的 PayUni 卡（GetPayUniRegistration，K9、B18）與聯絡資訊卡（讀店家自己的欄位）。
// RefreshPayUniStatus 與觀測紀錄不做 UI（B18）。

function PayUniShell({ children }: { children: ReactNode }) {
	return (
		<div className="payment-status-card">
			<div className="payment-left">
				<span className="payuni-logo-text">PayUni</span>
				<div className="payment-info">{children}</div>
			</div>
		</div>
	);
}

/**
 * PayUni 狀態卡：徽章是伺服器給的 PayUni 階段（flow_status）；「重新綁定」與不能重新綁定的原因都讀伺服器（can_rebind、rebind_unavailabilities）；「最後同步」是
 * 目前狀態的事件時間。第一次核准前沒有註冊（not_found）：顯示尚未註冊，只在申請送出過時才畫這張卡（hasApplication）。
 */
export function PayUniStatusCard({
	state,
	hasApplication,
	rebinding,
	onRebind,
	onRetry,
}: {
	state: PayUniState;
	hasApplication: boolean;
	rebinding: boolean;
	onRebind: () => void;
	onRetry: () => void;
}) {
	if (state.state === "notFound") {
		if (!hasApplication) return null;
		return (
			<PayUniShell>
				<div className="payment-label">金流商店審核</div>
				<div className="payment-desc">
					尚未向 PayUni 註冊（通過身分認證後才會註冊）。
				</div>
			</PayUniShell>
		);
	}
	if (state.state === "loading") {
		return (
			<PayUniShell>
				<div className="payment-label">金流商店審核</div>
				<div className="payment-desc" role="status">
					PayUni 狀態載入中…
				</div>
			</PayUniShell>
		);
	}
	if (state.state === "error") {
		return (
			<PayUniShell>
				<div className="payment-label">金流商店審核</div>
				<div className="payment-desc" role="alert">
					PayUni 狀態讀取失敗
				</div>
				<div>
					<Button variant="outline" onClick={onRetry}>
						重試
					</Button>
				</div>
			</PayUniShell>
		);
	}
	const registration = state.registration;
	const badge = flowBadge(registration.flowStatus);
	// PayUni 寫的審核意見（外部資料原文，伺服器已去掉前後空白）：直接顯示。
	const problem = (registration.statusCodes?.reason ?? "").trim();
	const rebind = rebindPresentation(registration);
	const problemFields = payUniProblemFields(registration);
	const failed = registration.bindStatus === PayUniBindStatus.FAILED;
	return (
		<PayUniShell>
			<div className="payment-label">
				金流商店審核
				<StatusBadge variant={badge.variant}>{badge.label}</StatusBadge>
			</div>
			{failed && (
				<div className="payment-desc payuni-reason-text">
					綁定失敗
					{registration.bindErrorCode
						? `（錯誤碼 ${registration.bindErrorCode}）`
						: ""}
					，已嘗試 {registration.bindAttempts} 次。
				</div>
			)}
			{problem && (
				<div className="payment-desc payuni-reason-text">{problem}</div>
			)}
			{problemFields.length > 0 && (
				<div className="payment-desc payuni-reason-text">
					需補資料：{problemFields.join("、")}
				</div>
			)}
			{rebind.reason && (
				<div className="payment-desc payuni-reason-text">{rebind.reason}</div>
			)}
			{registration.merId && (
				<div className="payment-merchant-id">
					金流商家編號：{registration.merId}
				</div>
			)}
			{registration.statusTime && (
				<div className="payment-desc payuni-sync-meta">
					最後同步：
					{formatDateTimeShort(timestampDate(registration.statusTime))}
				</div>
			)}
			{rebind.canRebind && (
				<div>
					<Button
						variant="primary"
						onClick={onRebind}
						loading={rebinding}
						disabled={rebinding}
					>
						重新綁定
					</Button>
				</div>
			)}
		</PayUniShell>
	);
}

/** 實作者補：聯絡資訊卡的標題與說明（讀的是店家目前的欄位，不是 PayUni 收到的值，K9）。 */
export const PAYUNI_CONTACT_TITLE = "店家目前的聯絡資料";
export const PAYUNI_CONTACT_HINT =
	"PayUni 註冊用的是店家的這些欄位；這裡顯示店家現在的值，不一定是 PayUni 已經收到的值。";

/** 聯絡資訊的欄位（店家自己的：聯絡人、手機、電話、客服信箱、地址）；全空時整張卡不渲染。 */
export function payUniContactFields(
	store: Store,
): { label: string; value: string }[] {
	return [
		{ label: "聯絡人姓名", value: store.contactName },
		{ label: "聯絡手機", value: store.contactMobile },
		{ label: "聯絡電話", value: store.contactPhone },
		{ label: "Email", value: store.supportEmail },
		{
			label: "地址",
			value: store.address ? addressText(store.address) : "",
		},
	].map((f) => ({ label: f.label, value: f.value === "-" ? "" : f.value }));
}

export function PayUniContactCard({
	store,
	registration,
}: {
	store: Store;
	/** 有註冊才畫：這張卡是送給 PayUni 的聯絡資訊。 */
	registration: PayUniRegistration | undefined;
}) {
	if (!registration) return null;
	const fields = payUniContactFields(store);
	if (fields.every((f) => !f.value)) return null;
	return (
		<div className="detail-card payuni-contact-card">
			<div className="section-title">
				<MaterialSymbol name="contacts" />
				<span>{PAYUNI_CONTACT_TITLE}</span>
			</div>
			<div className="card-body">
				<p className="payment-desc">{PAYUNI_CONTACT_HINT}</p>
				<div className="info-grid">
					{fields.map((f) => (
						<div className="info-field" key={f.label}>
							<div className="field-label">{f.label}</div>
							<div className="field-value">{f.value || "-"}</div>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}
