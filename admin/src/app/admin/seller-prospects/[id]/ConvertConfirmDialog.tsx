"use client";

import {
	ContactKind,
	type Prospect,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import type { KYCStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { kycStatusText } from "@/lib/kyc-status";
import { primaryContactValue } from "@/lib/seller-prospects";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 歸因綁定的治理確認框（詳情頁的 `ConvertPanel.tsx` 與統編對帳頁共用）。
 *
 * 對話框骨架與 a11y 行為由共用的 `Dialog` 提供；`ConvertPanel.tsx` 本身沒有對話框。
 *
 * 🔴 歸因是全案不可逆性最高的動作之一：它決定一家店的前身歸屬，沒有反向操作
 * （封存／還原互為逆動作，這個沒有）。所以這裡的保護比一般「確定嗎」重：並列
 * 顯示雙邊資料讓 admin 實際核對，而不是只給一句確認文字＋按鈕。
 *
 * ## 店家側的 props 只取兩個呼叫端各自有的欄位
 *
 * 詳情頁的賣家搜尋（`ConvertPanel.tsx`）有商店代號與聯絡電話、沒有認證公司名稱與認證狀態；統編對帳列
 * （`reconciliation/page.tsx`）反過來。`ConvertStoreSummary` 的這些欄位都是可選的，這支對話框對缺席的欄位
 * 一律顯示明確的「查無」說明，不留空白讓人誤以為兩邊都沒有、也不假裝有。業者側直接收 `Prospect`；
 * 對帳頁的清單不帶聯絡方式，所以那邊先 `GetProspect`，讀到之前 `prospect` 是 null，不能確認。
 */
export interface ConvertStoreSummary {
	storeId: string;
	storeName: string;
	/** Task 7 的賣家搜尋提供；Task 9 的對帳列沒有這些欄位時可省略。 */
	storeCode?: string;
	contactPhone?: string;
	/** Task 9 的對帳列提供（認證申請中的公司名稱）；Task 7 沒有這個欄位時可省略。 */
	certificationCompanyName?: string;
	certificationStatus?: KYCStatus;
}

export function ConvertConfirmDialog({
	prospect,
	store,
	prospectLoading = false,
	prospectLoadFailed = false,
	onRetryProspect,
	submitting,
	notice,
	onConfirm,
	onClose,
}: {
	prospect: Prospect | null;
	store: ConvertStoreSummary;
	/** 業者還在讀（含重讀）時停用確認。 */
	prospectLoading?: boolean;
	/** 業者讀取失敗；有 `onRetryProspect` 時顯示重試鍵。 */
	prospectLoadFailed?: boolean;
	onRetryProspect?: () => void;
	submitting: boolean;
	notice: GovernanceNotice | null;
	/** 送出（呼叫端已經知道要打哪個 prospect／storeId，這裡不需要參數）。 */
	onConfirm: () => void;
	onClose: () => void;
}) {
	// 沒有反向操作，所以比一般治理框多一道明文勾選——同
	// `ProviderRevokeDialog.tsx`（撤銷 AMEGO 連線，同樣是唯一終態）的形狀。
	const [acknowledged, setAcknowledged] = useState(false);

	const descId = "adm-prospect-convert-description";

	const hasStoreContact =
		store.storeCode !== undefined || store.contactPhone !== undefined;

	return (
		<Dialog
			title="確定要將這筆站外業者歸因綁定到這家店嗎？"
			closeLabel="關閉歸因綁定視窗"
			describedBy={descId}
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button
						variant="danger"
						onClick={onConfirm}
						loading={submitting}
						disabled={!acknowledged || !prospect || prospectLoading}
					>
						{submitting ? "送出中…" : "確認歸因綁定"}
					</Button>
				</>
			}
		>
			<p id={descId} className="adm-operation-field-note">
				送出後這筆站外業者會被永久標記為已入駐這家店，沒有反向操作；之後改由
				商家自行維護資料。請仔細核對下方兩側是否真的是同一家業者。
			</p>

			<div className="adm-operation-identity-grid">
				<div className="adm-operation-identity-card">
					<span className="adm-operation-identity-label">
						<MaterialSymbol name="person_search" /> 站外業者（本筆）
					</span>
					{prospect ? (
						<>
							<strong>{prospect.displayName || "（未命名）"}</strong>
							<span>統一編號 · {prospect.taxId || "未填寫"}</span>
							<span>
								信箱 ·{" "}
								{primaryContactValue(prospect.contacts, ContactKind.EMAIL) ||
									"未填寫"}
							</span>
							<span>
								電話 ·{" "}
								{primaryContactValue(prospect.contacts, ContactKind.PHONE) ||
									"未填寫"}
							</span>
						</>
					) : prospectLoadFailed ? (
						<>
							<span>無法讀取業者資料。</span>
							{onRetryProspect ? (
								<Button
									variant="outline"
									size="sm"
									type="button"
									onClick={onRetryProspect}
								>
									重試
								</Button>
							) : null}
						</>
					) : (
						<span aria-live="polite">正在讀取業者資料…</span>
					)}
				</div>
				<div className="adm-operation-identity-card adm-operation-identity-card--target">
					<span className="adm-operation-identity-label">
						<MaterialSymbol name="storefront" /> 選定的商家
					</span>
					<strong>{store.storeName || "（未命名）"}</strong>
					<span>Store ID · {store.storeId}</span>
					{/* 兩邊呼叫端目前都拿不到 store 的統一編號（賣家搜尋端點沒有這個
							    欄位、對帳列也只給認證公司名稱），誠實標示查無，不留空白。 */}
					<span>統一編號 · 此處查無資料（此來源未提供統一編號）</span>
					{hasStoreContact ? (
						<>
							<span>商店代號 · {store.storeCode || "未設定"}</span>
							<span>電話 · {store.contactPhone || "未填寫"}</span>
						</>
					) : (
						<span>此來源未提供聯絡方式資料</span>
					)}
					{store.certificationCompanyName !== undefined && (
						<span>
							認證公司名稱 · {store.certificationCompanyName || "未填寫"}
						</span>
					)}
					{store.certificationStatus !== undefined && (
						<span>認證狀態 · {kycStatusText(store.certificationStatus)}</span>
					)}
				</div>
			</div>

			<label
				htmlFor="adm-convert-ack"
				style={{
					display: "flex",
					alignItems: "flex-start",
					gap: 8,
					marginTop: 16,
					cursor: "pointer",
					fontSize: 14,
					color: "var(--hk-ink)",
				}}
			>
				<input
					id="adm-convert-ack"
					type="checkbox"
					checked={acknowledged}
					onChange={(event) => setAcknowledged(event.target.checked)}
					disabled={submitting}
					style={{
						flex: "0 0 auto",
						width: "auto",
						minWidth: 0,
						margin: "3px 0 0",
						accentColor: "var(--hk-primary-strong)",
					}}
				/>
				<span>
					我已核對上方雙方的名稱／統編／聯絡方式，確認這確實是同一家業者，
					且了解歸因綁定沒有反向操作。
				</span>
			</label>

			<GovernanceNoticeBanner notice={notice} />
		</Dialog>
	);
}
