"use client";

import { create } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { createClient } from "@connectrpc/connect";
import {
	createConnectQueryKey,
	useQuery,
	useTransport,
} from "@connectrpc/connect-query";
import { fieldMessage, submitError } from "@hawkr/api/connect/submit";
import {
	type AppVersionGate,
	AppVersionGateSchema,
	ContentAdminService,
} from "@hawkr/api/gen/hawkr/admin/content/v1alpha1/content_pb";
import { AppType } from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import { useQueryClient } from "@tanstack/react-query";
import type React from "react";
import { useCallback, useMemo, useState } from "react";
import {
	ConflictRow,
	FieldErrorText,
	VersionConflictNotice,
} from "@/components/connect/FormFeedback";
import { Dialog, useToast } from "@/components/UI";
import { errorMessage } from "@/lib/error-text";
import { formatDateTime } from "@/lib/format";
import { unknownText } from "@/lib/ui-text";

const APP_TYPE_TEXT: Record<Exclude<AppType, AppType.UNSPECIFIED>, string> = {
	[AppType.BUYER]: "買家",
	[AppType.SELLER]: "賣家",
};

/** App 種類的顯示文字；UNSPECIFIED 與不認得的值是「未知」。 */
const appTypeText = (appType: AppType): string =>
	(APP_TYPE_TEXT as Partial<Record<AppType, string>>)[appType] ?? unknownText();

// 每種 App 一組門檻（App 只發 iOS，沒有平台維度；只有編輯，無刪除）。
const FIXED_APP_TYPES: AppType[] = [AppType.BUYER, AppType.SELLER];

/** 修改時列進 update_mask 的欄位：表單上的四個欄位。 */
const FORM_FIELDS = [
	"minimum_version",
	"store_url",
	"notice_title",
	"notice_body",
];

/**
 * 送出前只擋必填：最低版本的格式、商店網址的開頭由伺服器檢查，錯誤標在欄位上（規則只在伺服器一處）。
 */
function validate(form: { minimumVersion: string }): string | null {
	return form.minimumVersion.trim() ? null : "最低版本必填";
}

const emptyGate = (appType: AppType): AppVersionGate =>
	create(AppVersionGateSchema, { appType });

const sameCombo = (a: AppVersionGate, b: AppVersionGate) =>
	a.appType === b.appType;

const comboLabel = (appType: AppType) => `${appTypeText(appType)} App`;

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

const AppVersionGateManagement: React.FC = () => {
	const queryClient = useQueryClient();
	const transport = useTransport();
	const client = useMemo(
		() => createClient(ContentAdminService, transport),
		[transport],
	);

	const [editing, setEditing] = useState<AppVersionGate | null>(null);
	const [confirming, setConfirming] = useState(false);
	const [saving, setSaving] = useState(false);
	const [formError, setFormError] = useState("");
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
	/** 版本衝突時伺服器上的最新值，給使用者對照。 */
	const [latest, setLatest] = useState<AppVersionGate | null>(null);
	const { showToast } = useToast();

	// 各組設定（display read）。
	const {
		data,
		isLoading,
		error: listError,
		refetch,
	} = useQuery(ContentAdminService.method.listAppVersionGates, {});
	const gates = data?.appVersionGates ?? [];

	// 後端初始為空表（§7.6 ①：查無設定＝不強更），買家、賣家兩列一律顯示、缺的以空值補位。
	const rows: AppVersionGate[] = FIXED_APP_TYPES.map(
		(appType) => gates.find((g) => g.appType === appType) ?? emptyGate(appType),
	);

	const closeEdit = useCallback(() => {
		if (saving) return;
		setEditing(null);
		setConfirming(false);
		setFormError("");
		setFieldErrors({});
		setLatest(null);
	}, [saving]);

	const closeConfirm = useCallback(() => {
		if (saving) return;
		setConfirming(false);
	}, [saving]);

	const handleFieldChange = (
		field: "minimumVersion" | "storeUrl" | "noticeTitle" | "noticeBody",
		value: string,
	) => {
		setEditing((prev) => (prev ? { ...prev, [field]: value } : prev));
	};

	// 送出前二次確認（設計書 §7.6 ⑥）：填錯會鎖死全體，所以驗證過了也不直接送出，先開確認視窗。
	const handleRequestSave = () => {
		if (!editing) return;
		const err = validate(editing);
		if (err) {
			setFormError(err);
			return;
		}
		setFormError("");
		setFieldErrors({});
		setConfirming(true);
	};

	const handleConfirmedSave = async () => {
		if (!editing) return;
		setSaving(true);
		const gate = {
			appType: editing.appType,
			minimumVersion: editing.minimumVersion.trim(),
			storeUrl: editing.storeUrl.trim(),
			noticeTitle: editing.noticeTitle,
			noticeBody: editing.noticeBody,
		};
		// 還沒有設定的一組用 Create；已經有的用 Update，帶讀到的 etag。
		const method = editing.etag
			? ContentAdminService.method.updateAppVersionGate
			: ContentAdminService.method.createAppVersionGate;
		try {
			if (editing.etag) {
				await client.updateAppVersionGate({
					appVersionGate: { ...gate, etag: editing.etag },
					updateMask: fieldMask(AppVersionGateSchema, FORM_FIELDS),
				});
			} else {
				await client.createAppVersionGate({ appVersionGate: gate });
			}
			// 列表交給 invalidate 重抓，不手動 patch 第二份 state
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: ContentAdminService.method.listAppVersionGates,
					cardinality: "finite",
				}),
			});
			setConfirming(false);
			setEditing(null);
			showToast(`已更新 ${comboLabel(editing.appType)} 的最低版本`);
		} catch (err: unknown) {
			setConfirming(false);
			const failure = submitError(err, defaultErrorTextZhTW, method);
			if (failure.kind === "conflict") {
				// 別人先改過或先建立了：保留輸入，讀最新的值給使用者對照；確認視窗的現值也跟著更新。
				const reloaded = await refetch();
				if (reloaded.error) {
					setFormError(errorMessage(reloaded.error));
				} else {
					setLatest(
						reloaded.data?.appVersionGates.find((g) => sameCombo(g, editing)) ??
							null,
					);
				}
			} else if (failure.kind === "fields") {
				setFieldErrors(failure.fields);
				setFormError(failure.message ?? "");
			} else {
				showToast(`儲存失敗：${failure.message}`, "error");
			}
		} finally {
			setSaving(false);
		}
	};

	const errorOf = (field: string) =>
		fieldMessage(fieldErrors, `app_version_gate.${field}`);

	return (
		<div>
			{/* Page Header */}
			<div className="page-header">
				<div>
					<div className="page-title">App 版本管理</div>
					<div className="page-subtitle">
						買家／賣家 iOS App 共 <span className="count">2</span>{" "}
						組強制更新設定
					</div>
				</div>
			</div>

			{/* 風險說明（常駐，不是只有送出時才提醒） */}
			<div
				style={{
					padding: "12px 16px",
					borderRadius: 8,
					background: "#fffbeb",
					border: "1px solid #fcd34d",
					color: "#92400e",
					fontSize: 13,
					lineHeight: 1.7,
					marginBottom: 16,
				}}
			>
				<MaterialSymbol name="warning" style={{ marginRight: 6 }} />
				<strong>最低版本一儲存即刻生效</strong>：所有版本低於它的 App
				會立即被擋在更新畫面，且<strong>使用者無法自行解除</strong>。
				請務必填寫「商店上已經可以下載到」的版本；填入尚未發布的版本會鎖死全體使用者，唯一救法是回到此頁改回正確值。
				<br />
				留空最低版本＝該組尚未設定，不會觸發強制更新。
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
					載入失敗：{errorMessage(listError)}
				</div>
			)}

			{/* Table */}
			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>App</th>
							<th>最低版本</th>
							<th>商店連結</th>
							<th>提示標題</th>
							<th>最後更新</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{isLoading ? (
							<tr>
								<td colSpan={6} style={{ textAlign: "center", padding: 40 }}>
									載入中…
								</td>
							</tr>
						) : (
							rows.map((row) => (
								<tr key={row.appType}>
									<td>
										<div className="cell-main">{appTypeText(row.appType)}</div>
									</td>
									<td>
										{row.minimumVersion ? (
											<code
												style={{
													background: "#f3f4f6",
													padding: "2px 6px",
													borderRadius: 4,
													fontSize: 13,
												}}
											>
												{row.minimumVersion}
											</code>
										) : (
											<span style={{ color: "#9ca3af" }}>未設定（不強更）</span>
										)}
									</td>
									<td
										style={{
											maxWidth: 240,
											overflow: "hidden",
											textOverflow: "ellipsis",
											whiteSpace: "nowrap",
										}}
										title={row.storeUrl}
									>
										{row.storeUrl || (
											<span style={{ color: "#9ca3af" }}>
												未設定（App 隱藏按鈕）
											</span>
										)}
									</td>
									<td>
										{row.noticeTitle || (
											<span style={{ color: "#9ca3af" }}>—</span>
										)}
									</td>
									<td style={{ whiteSpace: "nowrap" }}>
										{row.updateTime
											? formatDateTime(timestampDate(row.updateTime))
											: "—"}
									</td>
									<td style={{ whiteSpace: "nowrap" }}>
										<button
											type="button"
											className="btn-reset action-link"
											onClick={() => {
												setFormError("");
												setFieldErrors({});
												setLatest(null);
												setEditing({ ...row });
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
			{editing && !confirming && (
				<Dialog
					maxWidth={520}
					title="編輯強制更新設定"
					onClose={closeEdit}
					busy={saving}
					footer={
						<>
							<button type="button" className="btn" onClick={closeEdit}>
								取消
							</button>
							<button
								type="button"
								className="btn btn-primary"
								onClick={handleRequestSave}
								disabled={latest !== null}
							>
								<MaterialSymbol name="save" /> 儲存
							</button>
						</>
					}
				>
					<div style={{ fontSize: 13, color: "#6b7280", marginBottom: 20 }}>
						{comboLabel(editing.appType)}
					</div>

					<div style={{ marginBottom: 16 }}>
						<label htmlFor="min-version" style={labelStyle}>
							最低版本（三段數字，例如 1.0.3）
						</label>
						<input
							id="min-version"
							type="text"
							value={editing.minimumVersion}
							onChange={(e) =>
								handleFieldChange("minimumVersion", e.target.value)
							}
							placeholder="1.0.3"
							style={{ ...inputStyle, fontFamily: "monospace" }}
						/>
						<FieldErrorText message={errorOf("minimum_version")} />
					</div>

					<div style={{ marginBottom: 16 }}>
						<label htmlFor="store-url" style={labelStyle}>
							商店連結（可留空；留空時 App 隱藏「前往更新」按鈕）
						</label>
						<input
							id="store-url"
							type="text"
							value={editing.storeUrl}
							onChange={(e) => handleFieldChange("storeUrl", e.target.value)}
							placeholder="https://apps.apple.com/... 或 itms-apps://..."
							style={{ ...inputStyle, fontFamily: "monospace" }}
						/>
						<FieldErrorText message={errorOf("store_url")} />
					</div>

					<div style={{ marginBottom: 16 }}>
						<label htmlFor="notice-title" style={labelStyle}>
							提示標題
						</label>
						<input
							id="notice-title"
							type="text"
							value={editing.noticeTitle}
							onChange={(e) => handleFieldChange("noticeTitle", e.target.value)}
							placeholder="請更新至最新版本"
							style={inputStyle}
						/>
						<FieldErrorText message={errorOf("notice_title")} />
					</div>

					<div style={{ marginBottom: 20 }}>
						<label htmlFor="notice-body" style={labelStyle}>
							提示內容
						</label>
						<textarea
							id="notice-body"
							value={editing.noticeBody}
							onChange={(e) => handleFieldChange("noticeBody", e.target.value)}
							placeholder="此版本已停止支援，請前往商店更新後繼續使用。"
							style={{
								...inputStyle,
								minHeight: 96,
								resize: "vertical",
								lineHeight: 1.6,
							}}
						/>
						<FieldErrorText message={errorOf("notice_body")} />
					</div>

					{latest && (
						<div style={{ marginBottom: 16 }}>
							<VersionConflictNotice
								title="這組設定已被其他人修改"
								busy={saving}
								onOverwrite={() => {
									// 帶最新的 etag 再走一次確認：確認視窗的現值已是最新的。
									setEditing((prev) =>
										prev ? { ...prev, etag: latest.etag } : prev,
									);
									setLatest(null);
									setFormError("");
									setConfirming(true);
								}}
								onUseLatest={() => {
									setEditing({ ...latest });
									setLatest(null);
								}}
							>
								<ConflictRow
									label="最低版本"
									value={latest.minimumVersion || "未設定"}
								/>
								<ConflictRow
									label="商店連結"
									value={latest.storeUrl || "未設定"}
								/>
								<ConflictRow
									label="提示標題"
									value={latest.noticeTitle || "—"}
								/>
								<ConflictRow
									label="提示內容"
									value={latest.noticeBody || "—"}
								/>
							</VersionConflictNotice>
						</div>
					)}

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

			{/* ─── 二次確認 Modal（設計書 §7.6 ⑥ 唯一防護）─────────────────── */}
			{editing && confirming && (
				<Dialog
					maxWidth={600}
					tone="warning"
					busy={saving}
					title={
						<>
							<MaterialSymbol name="warning" style={{ marginRight: 8 }} />
							確認送出：此值會立即擋住所有低於它的 App
						</>
					}
					onClose={closeConfirm}
					footer={
						<>
							<button
								type="button"
								className="btn"
								onClick={closeConfirm}
								disabled={saving}
							>
								返回修改
							</button>
							<button
								type="button"
								className="btn btn-primary"
								onClick={handleConfirmedSave}
								disabled={saving}
								style={{ opacity: saving ? 0.5 : 1 }}
							>
								{saving ? "儲存中…" : "我已確認，送出"}
							</button>
						</>
					}
				>
					<div
						style={{
							fontSize: 13,
							lineHeight: 1.8,
							color: "#374151",
							marginBottom: 16,
						}}
					>
						你正在把 <strong>{comboLabel(editing.appType)}</strong>{" "}
						的最低版本設為{" "}
						<code
							style={{
								background: "#fef3c7",
								padding: "2px 6px",
								borderRadius: 4,
							}}
						>
							{editing.minimumVersion.trim()}
						</code>
						。儲存後<strong>立即生效</strong>，所有低於此版本的 App
						會被擋在更新畫面且無法自行解除。
						<br />
						若這個版本商店上還沒有，使用者會<strong>無法自救</strong>
						（下載不到） —— 送出前請先確認商店上確實已經可以下載到該版本。
					</div>

					<div
						style={{
							fontSize: 13,
							fontWeight: 600,
							marginBottom: 8,
							color: "#374151",
						}}
					>
						目前各組現值（供比對）
					</div>
					<table
						className="data-table"
						style={{ marginBottom: 20, fontSize: 13 }}
					>
						<thead>
							<tr>
								<th>組合</th>
								<th>目前最低版本</th>
								<th>送出後</th>
							</tr>
						</thead>
						<tbody>
							{rows.map((row) => {
								const isTarget = sameCombo(row, editing);
								return (
									<tr key={`confirm-${row.appType}`}>
										<td style={{ fontWeight: isTarget ? 600 : 400 }}>
											{comboLabel(row.appType)}
											{isTarget && (
												<span
													style={{
														marginLeft: 6,
														color: "#b45309",
														fontSize: 12,
													}}
												>
													（本次變更）
												</span>
											)}
										</td>
										<td style={{ fontFamily: "monospace" }}>
											{row.minimumVersion || "未設定"}
										</td>
										<td
											style={{
												fontFamily: "monospace",
												color: isTarget ? "#b45309" : "#9ca3af",
												fontWeight: isTarget ? 600 : 400,
											}}
										>
											{isTarget
												? editing.minimumVersion.trim()
												: row.minimumVersion || "未設定"}
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</Dialog>
			)}
		</div>
	);
};

export default AppVersionGateManagement;
