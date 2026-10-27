"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { createClient } from "@connectrpc/connect";
import {
	createConnectQueryKey,
	useQuery as useConnectQuery,
	useTransport,
} from "@connectrpc/connect-query";
import {
	BuyerAdminService,
	type MembershipPlan as BuyerMembershipPlan,
	MembershipPlanSchema,
} from "@hawkr/api/gen/hawkr/admin/buyer/v1alpha1/buyer_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQueryClient } from "@tanstack/react-query";
import type React from "react";
import { useCallback, useMemo, useState } from "react";
import { Dialog, useToast } from "@/components/UI";
import { formatDateTime } from "@/lib/format";

interface EditForm {
	planCode: string;
	etag: string;
	displayName: string;
	monthlyAnswerLimit: string;
	monthlyPriceTwd: string;
	isPurchasable: boolean;
}

const INT_RE = /^\d+$/;

/** 回傳第一個錯誤訊息；通過回 null。鏡射後端 CHECK constraint（doc 59 §5.1）。 */
function validate(form: EditForm): string | null {
	const limit = form.monthlyAnswerLimit.trim();
	if (!INT_RE.test(limit) || Number(limit) <= 0) {
		return "每期額度必須為正整數";
	}
	const price = form.monthlyPriceTwd.trim();
	if (!INT_RE.test(price)) {
		return "月費必須為 0 或正整數（新台幣元，免費方案填 0）";
	}
	return null;
}

const toEditForm = (plan: BuyerMembershipPlan): EditForm => ({
	planCode: plan.planCode,
	etag: plan.etag,
	displayName: plan.displayName,
	monthlyAnswerLimit: String(plan.monthlyAnswerLimit),
	monthlyPriceTwd: String(plan.monthlyPrice),
	isPurchasable: plan.purchasable,
});

const formatPrice = (twd: string) =>
	Number(twd) === 0 ? "免費" : `NT$ ${twd}`;

const inputStyle: React.CSSProperties = {
	width: "100%",
	padding: "8px 12px",
	borderRadius: 8,
	border: "1px solid #d1d5db",
	fontSize: 14,
	outline: "none",
};

const labelStyle: React.CSSProperties = {
	display: "block",
	fontSize: 13,
	fontWeight: 500,
	marginBottom: 6,
	color: "#374151",
};

// ─── Main Component ──────────────────────────────────────────────────────────

const MembershipPlanManagement: React.FC = () => {
	const queryClient = useQueryClient();
	const transport = useTransport();
	const buyer = useMemo(
		() => createClient(BuyerAdminService, transport),
		[transport],
	);

	const [editing, setEditing] = useState<EditForm | null>(null);
	const [saving, setSaving] = useState(false);
	const [formError, setFormError] = useState("");
	const { showToast } = useToast();

	const listQuery = useConnectQuery(
		BuyerAdminService.method.listMembershipPlans,
		{},
	);
	const plans = listQuery.data?.membershipPlans ?? [];
	const isLoading = listQuery.isLoading;
	const listError = listQuery.error;

	const closeEdit = useCallback(() => {
		if (saving) return;
		setEditing(null);
		setFormError("");
	}, [saving]);

	const handleFieldChange = <K extends keyof EditForm>(
		field: K,
		value: EditForm[K],
	) => {
		setEditing((prev) => (prev ? { ...prev, [field]: value } : prev));
	};

	const handleSave = async () => {
		if (!editing) return;
		const err = validate(editing);
		if (err) {
			setFormError(err);
			return;
		}
		setFormError("");
		setSaving(true);
		try {
			await buyer.updateMembershipPlan({
				membershipPlan: {
					planCode: editing.planCode,
					etag: editing.etag,
					monthlyAnswerLimit: Number(editing.monthlyAnswerLimit.trim()),
					monthlyPrice: BigInt(editing.monthlyPriceTwd.trim()),
					purchasable: editing.isPurchasable,
				},
				updateMask: fieldMask(MembershipPlanSchema, [
					"monthly_answer_limit",
					"monthly_price",
					"purchasable",
				]),
			});
			// 列表交給 invalidate 重抓，不手動 patch 第二份 state
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: BuyerAdminService.method.listMembershipPlans,
					cardinality: "finite",
				}),
			});
			showToast(`已更新「${editing.displayName}」方案`);
			setEditing(null);
		} catch (err: unknown) {
			showToast(
				`儲存失敗：${err instanceof Error ? err.message : "未知錯誤"}`,
				"error",
			);
		} finally {
			setSaving(false);
		}
	};

	return (
		<div>
			{/* Page Header */}
			<div className="page-header">
				<div>
					<div className="page-title">會員方案目錄</div>
					<div className="page-subtitle">
						編輯 free／basic／premium 三個方案的每期額度、月費與是否開放購買
					</div>
				</div>
			</div>

			{listError && (
				<div
					style={{
						padding: "12px 16px",
						borderRadius: 8,
						background: "#fef2f2",
						color: "#dc2626",
						fontSize: 14,
						marginBottom: 16,
					}}
				>
					<MaterialSymbol name="error" style={{ marginRight: 8 }} />
					載入失敗：
					{listError instanceof Error ? listError.message : "未知錯誤"}
				</div>
			)}

			{/* Table */}
			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>方案代碼</th>
							<th>名稱</th>
							<th>每期額度</th>
							<th>月費</th>
							<th>開放購買</th>
							<th>最後更新</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{isLoading ? (
							<tr>
								<td colSpan={7} style={{ textAlign: "center", padding: 40 }}>
									載入中…
								</td>
							</tr>
						) : plans.length === 0 ? (
							<tr>
								<td
									colSpan={7}
									style={{ textAlign: "center", padding: 40, color: "#9ca3af" }}
								>
									尚無方案資料
								</td>
							</tr>
						) : (
							plans.map((plan) => (
								<tr key={plan.planCode}>
									<td>
										<code
											style={{
												background: "#f3f4f6",
												padding: "2px 6px",
												borderRadius: 4,
												fontSize: 13,
											}}
										>
											{plan.planCode}
										</code>
									</td>
									<td>
										<div className="cell-main">{plan.displayName}</div>
									</td>
									<td>{plan.monthlyAnswerLimit} 次</td>
									<td>{formatPrice(String(plan.monthlyPrice))}</td>
									<td>
										{plan.purchasable ? (
											<span
												style={{
													padding: "2px 8px",
													borderRadius: 8,
													fontSize: 12,
													background: "var(--color-ink-100)",
													color: "var(--admin-primary-strong)",
												}}
											>
												開放
											</span>
										) : (
											<span
												style={{
													padding: "2px 8px",
													borderRadius: 8,
													fontSize: 12,
													background: "#f3f4f6",
													color: "#6b7280",
												}}
											>
												已下架
											</span>
										)}
									</td>
									<td style={{ whiteSpace: "nowrap" }}>
										{plan.updateTime
											? formatDateTime(timestampDate(plan.updateTime))
											: "—"}
									</td>
									<td style={{ whiteSpace: "nowrap" }}>
										<button
											type="button"
											className="btn-reset action-link"
											onClick={() => {
												setFormError("");
												setEditing(toEditForm(plan));
											}}
										>
											<MaterialSymbol name="edit" /> 編輯
										</button>
									</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>

			{/* ─── 編輯 Modal ──────────────────────────────────────────────── */}
			{editing && (
				<Dialog
					maxWidth={480}
					title={<>編輯方案：{editing.displayName}</>}
					onClose={closeEdit}
					busy={saving}
					footer={
						<>
							<button
								type="button"
								className="btn"
								onClick={closeEdit}
								disabled={saving}
							>
								取消
							</button>
							<button
								type="button"
								className="btn btn-primary"
								onClick={handleSave}
								disabled={saving}
								style={{ opacity: saving ? 0.5 : 1 }}
							>
								{saving ? "儲存中…" : "儲存"}
							</button>
						</>
					}
				>
					<div style={{ fontSize: 13, color: "#6b7280", marginBottom: 20 }}>
						方案代碼 <code>{editing.planCode}</code>（識別用，不可修改）
					</div>

					<div style={{ marginBottom: 16 }}>
						<label htmlFor="monthly-answer-limit" style={labelStyle}>
							每期額度（次）
						</label>
						<input
							id="monthly-answer-limit"
							type="text"
							inputMode="numeric"
							value={editing.monthlyAnswerLimit}
							onChange={(e) =>
								handleFieldChange("monthlyAnswerLimit", e.target.value)
							}
							placeholder="300"
							style={{ ...inputStyle, fontFamily: "monospace" }}
						/>
					</div>

					<div style={{ marginBottom: 16 }}>
						<label htmlFor="monthly-price-twd" style={labelStyle}>
							月費（新台幣元；免費方案填 0）
						</label>
						<input
							id="monthly-price-twd"
							type="text"
							inputMode="numeric"
							value={editing.monthlyPriceTwd}
							onChange={(e) =>
								handleFieldChange("monthlyPriceTwd", e.target.value)
							}
							placeholder="99"
							style={{ ...inputStyle, fontFamily: "monospace" }}
						/>
					</div>

					<div style={{ marginBottom: 20 }}>
						<label
							style={{
								display: "flex",
								alignItems: "center",
								gap: 8,
								cursor: "pointer",
							}}
						>
							<input
								type="checkbox"
								checked={editing.isPurchasable}
								onChange={(e) =>
									handleFieldChange("isPurchasable", e.target.checked)
								}
							/>
							<span style={{ fontSize: 14 }}>開放新指派／購買</span>
						</label>
						<div style={{ fontSize: 12, color: "#9ca3af", marginTop: 4 }}>
							關閉只影響新指派／購買通道，既有持有者不受影響（doc 59 §10
							D5）；admin 手動指派不受此欄位限制。
						</div>
					</div>

					{formError && (
						<div
							style={{
								padding: "10px 14px",
								borderRadius: 8,
								background: "#fef2f2",
								color: "#dc2626",
								fontSize: 13,
								marginBottom: 16,
							}}
						>
							<MaterialSymbol name="error" style={{ marginRight: 6 }} />
							{formError}
						</div>
					)}
				</Dialog>
			)}
		</div>
	);
};

export default MembershipPlanManagement;
