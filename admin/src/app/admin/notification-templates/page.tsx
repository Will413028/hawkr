"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type React from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Dialog, useToast } from "@/components/UI";
import {
	type BroadcastOutcome,
	readBroadcastOutcome,
} from "@/lib/broadcast-result";
import { api } from "@/services/api";

// ─── Types ──────────────────────────────────────────────────────────────────

// FieldSpec describes how each FCM data key is resolved (matches backend domain.FieldSpec).
// Exactly one of const / field should be set.
//   - { const: "ORDER" } — literal value written verbatim
//   - { field: "orderNo" } — looked up from MQ event payload
type FieldSpec = { const?: string; field?: string };

interface NotificationTemplate {
	id: number;
	exchange: string;
	routingKey: string;
	eventType: string;
	targetUserType: string;
	targetUserField: string;
	category: string;
	titleTemplate: string;
	bodyTemplate: string;
	bodyFallback: string;
	dataType: string;
	dataFields: Record<string, FieldSpec>;
	schemaUrl: string;
	referenceIdTemplate: string;
	enabled: boolean;
	updatedBy: number;
	createdAt: string;
	updatedAt: string;
}

interface ListResponse {
	code: number;
	data: NotificationTemplate[];
}

interface GetResponse {
	code: number;
	data: NotificationTemplate;
}

/** 渲染預覽的 `data` 內容（Gateway `AdminPreviewTemplate`，六個欄位全收在 data 之下）。 */
interface PreviewResult {
	renderedTitle: string;
	renderedBody: string;
	renderedData: Record<string, string>;
	renderedSchemaUrl: string;
	renderedReferenceId: string;
	missingFields: string[];
}

interface PreviewResponse {
	code: number;
	data: PreviewResult;
}

// Available event types (from pkg/mq/events.go — must match backend MQ routing keys)
const EVENT_TYPE_OPTIONS = [
	{ value: "order.paid", label: "訂單付款成功 (order.paid)" },
	{ value: "order.shipped", label: "訂單已出貨 (order.shipped)" },
	{ value: "order.completed", label: "訂單已完成 (order.completed)" },
	{ value: "order.cancelled", label: "訂單已取消 (order.cancelled)" },
	{
		value: "order.refund.requested",
		label: "退貨申請 (order.refund.requested)",
	},
	{ value: "shipment.delivered", label: "包裹到貨 (shipment.delivered)" },
	{ value: "refund.completed", label: "退款完成 (refund.completed)" },
	{ value: "payout.completed", label: "款項撥付 (payout.completed)" },
	{
		value: "merchant.verification.completed",
		label: "認證審核 (merchant.verification.completed)",
	},
	{ value: "post.created", label: "新貼文 (post.created)" },
	{ value: "comment.created", label: "新留言 (comment.created)" },
	{ value: "follow.added", label: "新追蹤者 (follow.added)" },
	{ value: "coupon.issued", label: "優惠券發放 (coupon.issued)" },
	{
		value: "notification.announcement",
		label: "系統公告 (notification.announcement)",
	},
	{ value: "return.shipped", label: "退貨配送中 (return.shipped)" },
	{ value: "product.favorited", label: "收藏商品 (product.favorited)" },
	{ value: "product.viewed", label: "瀏覽商品 (product.viewed)" },
	{ value: "post.liked", label: "貼文按讚 (post.liked)" },
	{ value: "comment.replied", label: "留言回覆 (comment.replied)" },
	{ value: "comment.liked", label: "留言按讚 (comment.liked)" },
];

// Display in table: Chinese name + MQ routing key
const EVENT_TYPE_DISPLAY: Record<string, string> = {
	"order.paid": "訂單付款成功 (MQ: order.paid)",
	"order.placed": "新訂單（賣家）(MQ: order.paid)",
	"order.shipped.buyer": "出貨通知（買家）(MQ: order.shipped)",
	"order.shipped.merchant": "出貨通知（賣家）(MQ: order.shipped)",
	"order.completed.buyer": "訂單完成（買家）(MQ: order.completed)",
	"order.completed.merchant": "訂單完成（賣家）(MQ: order.completed)",
	"order.cancelled.buyer": "訂單取消（買家）(MQ: order.cancelled)",
	"order.cancelled.merchant": "訂單取消（賣家）(MQ: order.cancelled)",
	"order.refund.requested.buyer":
		"退貨申請（買家）(MQ: order.refund.requested)",
	"order.refund.requested.merchant":
		"退貨申請（賣家）(MQ: order.refund.requested)",
	"shipment.delivered": "包裹到貨 (MQ: shipment.delivered)",
	"refund.completed": "退款完成 (MQ: refund.completed)",
	"payout.completed": "款項撥付 (MQ: payout.completed)",
	"merchant.verification.completed":
		"認證審核 (MQ: merchant.verification.completed)",
	"post.created": "新貼文 (MQ: post.created)",
	"comment.created": "新留言 (MQ: comment.created)",
	"follow.added": "新追蹤者 (MQ: follow.added)",
	"coupon.issued": "優惠券發放 (MQ: coupon.issued)",
	"notification.announcement": "系統公告 (MQ: notification.announcement)",
	"return.shipped": "退貨配送中 (MQ: return.shipped)",
	"product.favorited": "收藏商品 (MQ: product.favorited)",
	"product.viewed": "瀏覽商品 (MQ: product.viewed)",
	"post.liked": "貼文按讚 (MQ: post.liked)",
	"comment.replied.seller": "貼文被回覆（賣家）(MQ: comment.replied)",
	"comment.replied.buyer": "貼文留言被回覆（買家）(MQ: comment.replied)",
	"comment.liked": "留言按讚 (MQ: comment.liked)",
};

// Display name for category in table
const CATEGORY_DISPLAY: Record<string, string> = {
	order_update: "訂單更新",
	refund_update: "退款更新",
	system: "系統通知",
	chat: "聊天通知",
	follower: "追蹤通知",
	content: "內容通知",
};

// 目標用戶類型的顯示名稱（值本身是後端契約，照原樣印出來仍要看得懂）
const USER_TYPE_DISPLAY: Record<string, string> = {
	buyer: "buyer（買家）",
	seller: "seller（賣家）",
};

// ─── Main Component ──────────────────────────────────────────────────────────

const TemplateList: React.FC = () => {
	const queryClient = useQueryClient();
	const [filterEventType, setFilterEventType] = useState("");
	const [filterUserType, setFilterUserType] = useState("");
	const [filterEnabled, setFilterEnabled] = useState("");
	const [filterTitle, setFilterTitle] = useState("");

	// Detail drawer state。模板內容唯讀：唯一寫入路徑是 Atlas migration
	// （issue #1176 SSOT 政策），Gateway 的 POST / PUT 兩支 route 已隨之移除。
	const [drawerOpen, setDrawerOpen] = useState(false);
	const [viewingTemplate, setViewingTemplate] = useState<
		Partial<NotificationTemplate>
	>({});
	const [openSection, setOpenSection] = useState<string[]>([
		"notification",
		"fcm",
		"mq",
	]);
	const [dataFieldRows, setDataFieldRows] = useState<
		{ id: number; key: string; type: "field" | "const"; value: string }[]
	>([]);
	const rowIdRef = useRef(0);

	// Test Modal state
	const [testModalOpen, setTestModalOpen] = useState(false);
	const [testTemplateId, setTestTemplateId] = useState(0);
	const [samplePayload, setSamplePayload] = useState("{}");
	const [targetAccountId, setTargetAccountId] = useState("");
	const [targetUserType, setTargetUserType] = useState("");
	const [previewResult, setPreviewResult] = useState<PreviewResult | null>(
		null,
	);
	const [previewing, setPreviewing] = useState(false);
	const [testSending, setTestSending] = useState(false);

	// Broadcast Modal state
	const [broadcastOpen, setBroadcastOpen] = useState(false);
	const [broadcastTitle, setBroadcastTitle] = useState("");
	const [broadcastBody, setBroadcastBody] = useState("");
	const [broadcastTarget, setBroadcastTarget] = useState("all");
	const [broadcasting, setBroadcasting] = useState(false);
	const [broadcastResult, setBroadcastResult] =
		useState<BroadcastOutcome | null>(null);

	// Reload state
	const [reloading, setReloading] = useState(false);
	const [, setReloadMsg] = useState("");

	// Dialog refs (focus trap / restore)

	const { showToast } = useToast();

	const {
		data: templates = [],
		isLoading: loading,
		error: templatesError,
	} = useQuery({
		queryKey: [
			"notification-templates",
			"list",
			{
				eventType: filterEventType,
				userType: filterUserType,
				enabled: filterEnabled,
			},
		],
		queryFn: async () => {
			const params = new URLSearchParams();
			if (filterEventType) params.set("eventType", filterEventType);
			if (filterUserType) params.set("targetUserType", filterUserType);
			if (filterEnabled) params.set("enabled", filterEnabled);
			const res = await api.get<ListResponse>(
				`/api/v1/admin/notification-templates?${params}`,
			);
			return res?.data ?? [];
		},
	});
	const invalidateTemplates = () =>
		queryClient.invalidateQueries({ queryKey: ["notification-templates"] });

	useEffect(() => {
		if (templatesError) {
			showToast(
				`載入失敗：${templatesError instanceof Error ? templatesError.message : "未知錯誤"}`,
				"error",
			);
		}
	}, [templatesError, showToast]);

	// ─── Toggle Enabled ──────────────────────────────────────────────────────

	const handleToggleEnabled = async (tmpl: NotificationTemplate) => {
		try {
			await api.patch(
				`/api/v1/admin/notification-templates/${tmpl.id}/enabled`,
				{
					enabled: !tmpl.enabled,
				},
			);
			invalidateTemplates();
			showToast(`已${!tmpl.enabled ? "啟用" : "停用"} ${tmpl.eventType}`);
		} catch (err: unknown) {
			showToast(
				`操作失敗：${err instanceof Error ? err.message : "未知錯誤"}`,
				"error",
			);
		}
	};

	// ─── Reload Cache ────────────────────────────────────────────────────────

	const handleReload = async () => {
		setReloading(true);
		setReloadMsg("");
		try {
			// reloadedCount 在 data 之下且是 **string**（10 檔 §4：數字對外一律字串）。
			const res = await api.post<{
				code: number;
				data: { reloadedCount: string };
			}>("/api/v1/admin/notification-templates/reload", {});
			const n = res?.data?.reloadedCount ?? "0";
			setReloadMsg(`已重載 ${n} 筆模板`);
			showToast(`已重載 ${n} 筆模板`);
		} catch (err: unknown) {
			showToast(
				`重載失敗：${err instanceof Error ? err.message : "未知錯誤"}`,
				"error",
			);
		} finally {
			setReloading(false);
		}
	};

	// ─── Broadcast ────────────────────────────────────────────────────────

	const openBroadcastModal = () => {
		setBroadcastTitle("");
		setBroadcastBody("");
		setBroadcastTarget("all");
		setBroadcastResult(null);
		setBroadcastOpen(true);
	};

	const handleBroadcast = async () => {
		if (!broadcastTitle.trim() || !broadcastBody.trim()) {
			showToast("請填入標題和內容", "error");
			return;
		}
		setBroadcasting(true);
		try {
			// broadcastId / recipientCount 在 data 之下，recipientCount 是字串。
			// 送出後不關視窗 —— 公告 ID 要留在畫面上才抄得到，收件人數為 0 也
			// 才有機會被看見（3 秒的 toast 兩者都留不住）。
			const res = await api.post<{
				code: number;
				data?: { broadcastId?: string; recipientCount?: string };
			}>("/api/v1/admin/notifications/broadcast", {
				title: broadcastTitle,
				body: broadcastBody,
				targetUserType: broadcastTarget,
			});
			const outcome = readBroadcastOutcome(res?.data);
			setBroadcastResult(outcome);
			if (outcome.kind === "delivered") {
				showToast(`公告已送出，${outcome.count} 位使用者收到`);
			} else if (outcome.kind === "empty") {
				showToast("公告已建立，但收件人數為 0：沒有人收到", "warning");
			} else {
				showToast("公告已送出，但後端沒有回報收件人數", "warning");
			}
		} catch (err: unknown) {
			showToast(
				`發送失敗：${err instanceof Error ? err.message : "未知錯誤"}`,
				"error",
			);
		} finally {
			setBroadcasting(false);
		}
	};

	// ─── Detail Drawer ───────────────────────────────────────────────────────

	const openDetailDrawer = async (id: number) => {
		try {
			const res = await api.get<GetResponse>(
				`/api/v1/admin/notification-templates/${id}`,
			);
			if (res?.data) {
				const rows = Object.entries(res.data.dataFields || {}).map(([k, v]) => {
					const id = rowIdRef.current++;
					if (v && typeof v === "object") {
						if (typeof v.const === "string") {
							return { id, key: k, type: "const" as const, value: v.const };
						}
						if (typeof v.field === "string") {
							return { id, key: k, type: "field" as const, value: v.field };
						}
					}
					// Defensive fallback for legacy string-form data (shouldn't happen post-migration).
					return {
						id,
						key: k,
						type: "field" as const,
						value: typeof v === "string" ? v : "",
					};
				});
				setDataFieldRows(rows);
				setViewingTemplate(res.data);
				setDrawerOpen(true);
			}
		} catch (err: unknown) {
			showToast(
				`載入失敗：${err instanceof Error ? err.message : "未知錯誤"}`,
				"error",
			);
		}
	};

	const closeDrawer = () => setDrawerOpen(false);

	const toggleSection = (section: string) => {
		setOpenSection((prev) =>
			prev.includes(section)
				? prev.filter((s) => s !== section)
				: [...prev, section],
		);
	};

	// ─── Test Modal ──────────────────────────────────────────────────────────

	const buildSamplePayload = (tmpl: NotificationTemplate): string => {
		const payload: Record<string, unknown> = {};
		// target user field
		if (tmpl.targetUserField) {
			payload[tmpl.targetUserField] = 12345;
		}
		// extract #{field} placeholders from title + body templates
		const placeholderRe = /#\{(\w+)\}/g;
		const templates = `${tmpl.titleTemplate} ${tmpl.bodyTemplate} ${tmpl.referenceIdTemplate || ""}`;
		let match: RegExpExecArray | null = placeholderRe.exec(templates);
		while (match !== null) {
			const field = match[1];
			if (!(field in payload)) {
				payload[field] = `sample_${field}`;
			}
			match = placeholderRe.exec(templates);
		}
		// dataFields field-references (sample value for each referenced payload field).
		// Const specs don't need samples — they don't read from payload.
		if (tmpl.dataFields) {
			for (const spec of Object.values(tmpl.dataFields)) {
				const payloadField = spec?.field;
				if (payloadField && !(payloadField in payload)) {
					payload[payloadField] = `sample_${payloadField}`;
				}
			}
		}
		return JSON.stringify(payload, null, 2);
	};

	const openTestModal = (tmpl: NotificationTemplate) => {
		const payload = buildSamplePayload(tmpl);
		setTestTemplateId(tmpl.id);
		setTargetUserType(tmpl.targetUserType);
		setSamplePayload(payload);
		setPreviewResult(null);
		setTargetAccountId("");
		setTestModalOpen(true);
		triggerPreview(payload, tmpl.id);
	};

	const previewTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);

	const triggerPreview = useCallback((payload: string, templateId: number) => {
		if (previewTimerRef.current) clearTimeout(previewTimerRef.current);
		try {
			JSON.parse(payload);
		} catch {
			return;
		} // skip invalid JSON
		previewTimerRef.current = setTimeout(async () => {
			setPreviewing(true);
			try {
				const res = await api.post<PreviewResponse>(
					`/api/v1/admin/notification-templates/${templateId}/preview`,
					{ samplePayload: payload },
				);
				if (res?.data) setPreviewResult(res.data);
			} catch {
				/* silent */
			} finally {
				setPreviewing(false);
			}
		}, 400);
	}, []);

	const handleTestSend = async () => {
		if (!targetAccountId) {
			showToast("請填入目標帳號 ID", "error");
			return;
		}
		setTestSending(true);
		try {
			await api.post(
				`/api/v1/admin/notification-templates/${testTemplateId}/test-send`,
				{
					samplePayload: samplePayload,
					targetAccountId: Number(targetAccountId),
					targetUserType: targetUserType,
				},
			);
			showToast("測試推播已發送");
		} catch (err: unknown) {
			showToast(
				`發送失敗：${err instanceof Error ? err.message : "未知錯誤"}`,
				"error",
			);
		} finally {
			setTestSending(false);
		}
	};

	// ─── Render ──────────────────────────────────────────────────────────────

	return (
		<div>
			{/* Page Header */}
			<div className="page-header">
				<div>
					<div className="page-title">通知模板管理</div>
					<div className="page-subtitle">
						共{" "}
						<span className="count">
							{filterTitle
								? templates.filter((t) =>
										t.titleTemplate
											.toLowerCase()
											.includes(filterTitle.toLowerCase()),
									).length
								: templates.length}
						</span>{" "}
						個模板
					</div>
				</div>
				<div className="header-actions" style={{ gap: 8, display: "flex" }}>
					<button
						type="button"
						className="btn"
						onClick={handleReload}
						disabled={reloading}
					>
						<MaterialSymbol
							name="refresh"
							className={reloading ? "icon-spin" : undefined}
						/>
						{reloading ? " 重載中…" : " 重載快取"}
					</button>
					<button
						type="button"
						className="btn"
						style={{
							background: "var(--admin-info)",
							color: "#fff",
							border: "none",
						}}
						onClick={openBroadcastModal}
					>
						<MaterialSymbol name="campaign" /> 發送系統公告
					</button>
				</div>
			</div>

			{/* Filter Bar */}
			<div className="filter-bar">
				<select
					value={filterEventType}
					onChange={(e) => setFilterEventType(e.target.value)}
					style={{
						padding: "6px 12px",
						borderRadius: 6,
						border: "1px solid #e5e7eb",
					}}
				>
					<option value="">全部事件類型</option>
					{EVENT_TYPE_OPTIONS.map((o) => (
						<option key={o.value} value={o.value}>
							{o.value}
						</option>
					))}
				</select>
				<select
					value={filterUserType}
					onChange={(e) => setFilterUserType(e.target.value)}
					style={{
						padding: "6px 12px",
						borderRadius: 6,
						border: "1px solid #e5e7eb",
					}}
				>
					<option value="">全部用戶類型</option>
					<option value="buyer">buyer</option>
					<option value="seller">seller（賣家）</option>
				</select>
				<select
					value={filterEnabled}
					onChange={(e) => setFilterEnabled(e.target.value)}
					style={{
						padding: "6px 12px",
						borderRadius: 6,
						border: "1px solid #e5e7eb",
					}}
				>
					<option value="">全部狀態</option>
					<option value="true">啟用</option>
					<option value="false">停用</option>
				</select>
				<input
					type="text"
					placeholder="搜尋標題…"
					value={filterTitle}
					onChange={(e) => setFilterTitle(e.target.value)}
					style={{
						padding: "6px 12px",
						borderRadius: 6,
						border: "1px solid #e5e7eb",
						width: 180,
					}}
				/>
			</div>

			{/* Table */}
			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>事件類型 (MQ Event)</th>
							<th>用戶類型</th>
							<th>標題模板</th>
							<th>用戶偏好分類</th>
							<th>啟用</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{loading ? (
							<tr>
								<td colSpan={6} style={{ textAlign: "center", padding: 40 }}>
									載入中…
								</td>
							</tr>
						) : templates.filter(
								(t) =>
									!filterTitle ||
									t.titleTemplate
										.toLowerCase()
										.includes(filterTitle.toLowerCase()),
							).length === 0 ? (
							<tr>
								<td colSpan={6} style={{ textAlign: "center", padding: 40 }}>
									無資料
								</td>
							</tr>
						) : (
							templates
								.filter(
									(t) =>
										!filterTitle ||
										t.titleTemplate
											.toLowerCase()
											.includes(filterTitle.toLowerCase()),
								)
								.map((t) => (
									<tr key={t.id}>
										<td>
											<div className="cell-main">
												{EVENT_TYPE_DISPLAY[t.eventType] ?? t.eventType}
											</div>
										</td>
										<td>
											<span
												className={`status-badge ${t.targetUserType === "buyer" ? "approved" : "pending"}`}
											>
												{t.targetUserType === "seller"
													? "seller"
													: t.targetUserType}
											</span>
										</td>
										<td>
											<div className="cell-main" title={t.titleTemplate}>
												{t.titleTemplate.length > 30
													? `${t.titleTemplate.slice(0, 30)}…`
													: t.titleTemplate}
											</div>
										</td>
										<td>
											<div className="cell-main">
												{CATEGORY_DISPLAY[t.category] ?? t.category}
											</div>
										</td>
										<td style={{ textAlign: "center" }}>
											<input
												type="checkbox"
												checked={t.enabled}
												onChange={() => handleToggleEnabled(t)}
												style={{ width: 16, height: 16, cursor: "pointer" }}
											/>
										</td>
										<td style={{ whiteSpace: "nowrap" }}>
											<button
												type="button"
												className="btn-reset action-link"
												onClick={() => openDetailDrawer(t.id)}
											>
												<MaterialSymbol name="visibility" /> 檢視
											</button>
											<span style={{ margin: "0 6px", color: "#e5e7eb" }}>
												|
											</span>
											<button
												type="button"
												className="btn-reset action-link"
												style={{ color: "var(--admin-info)" }}
												onClick={() => openTestModal(t)}
											>
												<MaterialSymbol name="science" /> 測試
											</button>
										</td>
									</tr>
								))
						)}
					</tbody>
				</table>
			</div>

			{/* ─── Edit Drawer ─────────────────────────────────────────────── */}
			{drawerOpen && (
				<Dialog
					variant="side"
					title="檢視通知模板"
					onClose={closeDrawer}
					footer={
						<button type="button" className="btn" onClick={closeDrawer}>
							關閉
						</button>
					}
				>
					<div
						style={{
							marginBottom: 12,
							padding: "10px 14px",
							background: "#eff6ff",
							borderRadius: 8,
							border: "1px solid #bfdbfe",
							fontSize: 13,
							color: "#1e40af",
						}}
					>
						<MaterialSymbol name="info" /> 模板內容唯讀：新增與修改的唯一入口是
						Atlas migration（issue #1176），Gateway 沒有對應的寫入
						API。啟用狀態可在列表切換。
					</div>

					{/* Section: 通知內容 */}
					<DrawerSection
						title="通知內容"
						open={openSection.includes("notification")}
						onToggle={() => toggleSection("notification")}
					>
						<PreviewField
							label="Event Type"
							value={
								viewingTemplate.eventType
									? (EVENT_TYPE_DISPLAY[viewingTemplate.eventType] ??
										viewingTemplate.eventType)
									: ""
							}
						/>
						<PreviewField
							label="目標用戶類型"
							value={
								viewingTemplate.targetUserType
									? (USER_TYPE_DISPLAY[viewingTemplate.targetUserType] ??
										viewingTemplate.targetUserType)
									: ""
							}
						/>
						<PreviewField
							label="目標用戶欄位"
							value={viewingTemplate.targetUserField ?? ""}
						/>
						<PreviewField
							label="分類"
							value={
								viewingTemplate.category
									? (CATEGORY_DISPLAY[viewingTemplate.category] ??
										viewingTemplate.category)
									: ""
							}
						/>
						<PreviewField
							label="標題模板"
							value={viewingTemplate.titleTemplate ?? ""}
						/>
						<PreviewField
							label="內文模板"
							value={viewingTemplate.bodyTemplate ?? ""}
						/>
						<PreviewField
							label="內文 Fallback"
							value={viewingTemplate.bodyFallback ?? ""}
						/>
						<PreviewField
							label="Reference ID 模板"
							value={viewingTemplate.referenceIdTemplate ?? ""}
						/>
						<PreviewField
							label="啟用狀態"
							value={viewingTemplate.enabled ? "啟用" : "停用"}
						/>
					</DrawerSection>

					{/* Section: FCM Data */}
					<DrawerSection
						title="FCM Data"
						open={openSection.includes("fcm")}
						onToggle={() => toggleSection("fcm")}
					>
						<PreviewField
							label="Data Type"
							value={viewingTemplate.dataType ?? ""}
						/>
						<PreviewField
							label="Schema URL"
							value={viewingTemplate.schemaUrl ?? ""}
						/>
						<div style={{ marginBottom: 8 }}>
							<div style={{ fontSize: 12, color: "#6b7280", marginBottom: 2 }}>
								Data Fields
								<span style={{ marginLeft: 6, color: "#9ca3af" }}>
									FCM key → field（讀 payload 欄位）或 const（字串常數）
								</span>
							</div>
							{dataFieldRows.length === 0 ? (
								<div
									style={{
										padding: "6px 10px",
										background: "#fff",
										borderRadius: 6,
										border: "1px solid #e5e7eb",
										fontSize: 14,
										color: "#9ca3af",
									}}
								>
									（無）
								</div>
							) : (
								dataFieldRows.map((row) => (
									<div
										key={row.id}
										style={{
											display: "flex",
											alignItems: "center",
											gap: 8,
											padding: "6px 10px",
											marginBottom: 4,
											background: "#fff",
											borderRadius: 6,
											border: "1px solid #e5e7eb",
											fontSize: 13,
										}}
									>
										<code style={{ flex: 1, wordBreak: "break-all" }}>
											{row.key}
										</code>
										<span
											style={{
												flexShrink: 0,
												padding: "1px 8px",
												borderRadius: 999,
												fontSize: 11,
												background:
													row.type === "const" ? "#f3f4f6" : "#e0f2fe",
												color: row.type === "const" ? "#4b5563" : "#075985",
											}}
										>
											{row.type}
										</span>
										<code style={{ flex: 1, wordBreak: "break-all" }}>
											{row.value}
										</code>
									</div>
								))
							)}
						</div>
					</DrawerSection>

					{/* Section: MQ 進階設定 */}
					<DrawerSection
						title="MQ 進階設定"
						open={openSection.includes("mq")}
						onToggle={() => toggleSection("mq")}
					>
						<PreviewField
							label="Exchange"
							value={viewingTemplate.exchange ?? ""}
						/>
						<PreviewField
							label="Routing Key"
							value={viewingTemplate.routingKey ?? ""}
						/>
					</DrawerSection>
				</Dialog>
			)}

			{/* ─── Broadcast Modal ────────────────────────────────────────── */}
			{broadcastOpen && (
				<Dialog
					title={
						<span
							style={{
								display: "flex",
								alignItems: "center",
								gap: 8,
							}}
						>
							<MaterialSymbol
								name="campaign"
								style={{ color: "var(--admin-info)" }}
							/>
							發送系統公告
						</span>
					}
					maxWidth={500}
					onClose={() => setBroadcastOpen(false)}
					busy={broadcasting}
				>
					{broadcastResult ? (
						<BroadcastResultPanel
							outcome={broadcastResult}
							onClose={() => setBroadcastOpen(false)}
						/>
					) : (
						<>
							<FormRow label="公告標題 *">
								<input
									type="text"
									className="form-input"
									value={broadcastTitle}
									onChange={(e) => setBroadcastTitle(e.target.value)}
									placeholder="例：版本更新通知"
								/>
							</FormRow>
							<FormRow label="公告內容 *">
								<textarea
									className="form-input"
									rows={4}
									value={broadcastBody}
									onChange={(e) => setBroadcastBody(e.target.value)}
									placeholder="例：Hawkr v2.0 已上線，請更新至最新版本！"
									style={{ fontFamily: "inherit" }}
								/>
							</FormRow>
							<FormRow label="發送對象">
								<select
									className="form-input"
									value={broadcastTarget}
									onChange={(e) => setBroadcastTarget(e.target.value)}
								>
									<option value="all">全部用戶（買家 + 賣家）</option>
									<option value="buyer">僅買家</option>
									<option value="seller">僅賣家</option>
								</select>
							</FormRow>
							<div
								style={{
									marginTop: 16,
									padding: "10px 14px",
									background: "#fef3c7",
									borderRadius: 8,
									border: "1px solid #fbbf24",
									fontSize: 13,
									color: "#92400e",
								}}
							>
								<MaterialSymbol name="warning" /> 此操作會立即推播給
								{broadcastTarget === "all"
									? "所有用戶"
									: broadcastTarget === "buyer"
										? "所有買家"
										: "所有賣家"}
								，請確認內容無誤。
							</div>
							<div
								style={{
									marginTop: 16,
									display: "flex",
									gap: 10,
									justifyContent: "flex-end",
								}}
							>
								<button
									type="button"
									className="btn"
									onClick={() => setBroadcastOpen(false)}
									disabled={broadcasting}
								>
									取消
								</button>
								<button
									type="button"
									className="btn"
									style={{
										background: "var(--admin-info)",
										color: "#fff",
										border: "none",
									}}
									onClick={handleBroadcast}
									disabled={
										broadcasting ||
										!broadcastTitle.trim() ||
										!broadcastBody.trim()
									}
								>
									<MaterialSymbol name="send" />
									{broadcasting ? " 發送中…" : " 確認發送"}
								</button>
							</div>
						</>
					)}
				</Dialog>
			)}

			{/* ─── Test Modal ──────────────────────────────────────────────── */}
			{testModalOpen && (
				<Dialog
					title={
						<span
							style={{
								display: "flex",
								alignItems: "center",
								gap: 8,
							}}
						>
							<MaterialSymbol
								name="science"
								style={{ color: "var(--admin-info)" }}
							/>
							測試模板
						</span>
					}
					maxWidth={620}
					onClose={() => setTestModalOpen(false)}
					busy={testSending || previewing}
				>
					<div style={{ marginBottom: 16 }}>
						<label
							htmlFor="test-sample-payload"
							style={{
								display: "block",
								fontSize: 13,
								fontWeight: 600,
								marginBottom: 6,
							}}
						>
							Sample Payload（JSON）
						</label>
						<textarea
							id="test-sample-payload"
							rows={6}
							value={samplePayload}
							onChange={(e) => {
								setSamplePayload(e.target.value);
								triggerPreview(e.target.value, testTemplateId);
							}}
							style={{
								width: "100%",
								padding: "8px 12px",
								borderRadius: 6,
								border: "1px solid #e5e7eb",
								fontFamily: "monospace",
								fontSize: 13,
								boxSizing: "border-box",
							}}
							placeholder='{"buyerAccountId": 123, "orderNo": "ORD-001"}'
						/>
					</div>

					{previewResult && (
						<div
							style={{
								marginTop: 16,
								padding: 14,
								borderRadius: 8,
								background: "#f9fafb",
								border: "1px solid #e5e7eb",
							}}
						>
							<PreviewField label="標題" value={previewResult.renderedTitle} />
							<PreviewField label="內文" value={previewResult.renderedBody} />
							{previewResult.renderedSchemaUrl && (
								<PreviewField
									label="Schema URL"
									value={previewResult.renderedSchemaUrl}
								/>
							)}
							{previewResult.renderedReferenceId && (
								<PreviewField
									label="Reference ID"
									value={previewResult.renderedReferenceId}
								/>
							)}
							{Object.keys(previewResult.renderedData || {}).length > 0 && (
								<div style={{ marginTop: 8 }}>
									<div
										style={{
											fontSize: 12,
											color: "#6b7280",
											marginBottom: 4,
										}}
									>
										FCM Data
									</div>
									<pre
										style={{
											background: "#f3f4f6",
											padding: 8,
											borderRadius: 6,
											fontSize: 12,
											margin: 0,
											overflow: "auto",
										}}
									>
										{JSON.stringify(previewResult.renderedData, null, 2)}
									</pre>
								</div>
							)}
							{(previewResult.missingFields ?? []).length > 0 && (
								<div
									style={{
										marginTop: 10,
										padding: "8px 12px",
										background: "#fef3c7",
										borderRadius: 6,
										border: "1px solid #fbbf24",
									}}
								>
									<MaterialSymbol name="warning" style={{ color: "#d97706" }} />
									<span
										style={{
											fontSize: 13,
											color: "#92400e",
											marginLeft: 6,
										}}
									>
										缺少欄位：{previewResult.missingFields.join(", ")}
									</span>
								</div>
							)}
						</div>
					)}

					<div
						style={{
							marginTop: 20,
							paddingTop: 16,
							borderTop: "1px solid #e5e7eb",
						}}
					>
						<div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10 }}>
							發送測試推播
						</div>
						<div style={{ display: "flex", gap: 8, marginBottom: 8 }}>
							<div style={{ flex: 1 }}>
								<label
									htmlFor="test-target-account-id"
									style={{ fontSize: 12, color: "#6b7280" }}
								>
									目標帳號 ID
								</label>
								<input
									id="test-target-account-id"
									type="text"
									inputMode="numeric"
									className="form-input"
									value={targetAccountId}
									onChange={(e) =>
										setTargetAccountId(e.target.value.replace(/\D/g, ""))
									}
									placeholder="123456"
								/>
							</div>
							<div style={{ flex: 1 }}>
								<label
									htmlFor="test-target-user-type"
									style={{ fontSize: 12, color: "#6b7280" }}
								>
									用戶類型
								</label>
								<select
									id="test-target-user-type"
									className="form-input"
									value={targetUserType}
									onChange={(e) => setTargetUserType(e.target.value)}
								>
									<option value="buyer">buyer</option>
									<option value="seller">seller（賣家）</option>
								</select>
							</div>
						</div>
						<button
							type="button"
							className="btn btn-primary"
							onClick={handleTestSend}
							disabled={testSending || !targetAccountId}
						>
							<MaterialSymbol name="send" />
							{testSending ? " 發送中…" : " 發送測試推播"}
						</button>
					</div>
				</Dialog>
			)}
		</div>
	);
};

// ─── Helper Sub-components ────────────────────────────────────────────────────

const DrawerSection: React.FC<{
	title: string;
	open: boolean;
	onToggle: () => void;
	children: React.ReactNode;
}> = ({ title, open, onToggle, children }) => (
	<div
		style={{
			marginBottom: 8,
			border: "1px solid #e5e7eb",
			borderRadius: 8,
			overflow: "hidden",
		}}
	>
		<button
			type="button"
			onClick={onToggle}
			style={{
				width: "100%",
				padding: "10px 14px",
				background: open ? "#f9fafb" : "#fff",
				border: "none",
				cursor: "pointer",
				display: "flex",
				alignItems: "center",
				justifyContent: "space-between",
				fontWeight: 600,
				fontSize: 14,
			}}
		>
			<span>{title}</span>
			<MaterialSymbol
				name={open ? "keyboard_arrow_up" : "keyboard_arrow_down"}
			/>
		</button>
		{open && (
			<div style={{ padding: "12px 14px", borderTop: "1px solid #e5e7eb" }}>
				{children}
			</div>
		)}
	</div>
);

const FormRow: React.FC<{ label: string; children: React.ReactNode }> = ({
	label,
	children,
}) => (
	<div style={{ marginBottom: 12 }}>
		<span
			style={{
				display: "block",
				fontSize: 13,
				fontWeight: 600,
				marginBottom: 4,
				color: "#374151",
			}}
		>
			{label}
		</span>
		{children}
	</div>
);

const PreviewField: React.FC<{ label: string; value: string }> = ({
	label,
	value,
}) => (
	<div style={{ marginBottom: 8 }}>
		<div style={{ fontSize: 12, color: "#6b7280", marginBottom: 2 }}>
			{label}
		</div>
		<div
			style={{
				padding: "6px 10px",
				background: "#fff",
				borderRadius: 6,
				border: "1px solid #e5e7eb",
				fontSize: 14,
				whiteSpace: "pre-wrap",
				wordBreak: "break-word",
			}}
		>
			{value || <span style={{ color: "#9ca3af" }}>（空）</span>}
		</div>
	</div>
);

// 公告送出後的結果。收件人數 0 與「有送到」必須一眼分得出來 —— 收件人枚舉
// 整個壞掉時，這個數字是唯一看得出「推播打空」的訊號，長得像成功就等於沒有。
const BroadcastResultPanel: React.FC<{
	outcome: BroadcastOutcome;
	onClose: () => void;
}> = ({ outcome, onClose }) => {
	const ok = outcome.kind === "delivered";
	return (
		<div>
			<div
				style={{
					padding: "14px 16px",
					borderRadius: 8,
					background: ok ? "var(--color-status-success-soft)" : "#fef3c7",
					border: `1px solid ${ok ? "var(--color-status-success)" : "#fbbf24"}`,
					color: ok ? "var(--color-ink-700)" : "#92400e",
				}}
			>
				<div
					style={{
						display: "flex",
						alignItems: "center",
						gap: 8,
						fontWeight: 700,
						fontSize: 15,
					}}
				>
					<MaterialSymbol name={ok ? "check" : "warning"} />
					{outcome.kind === "delivered"
						? `公告已送達 ${outcome.count} 位使用者`
						: outcome.kind === "empty"
							? "收件人數為 0"
							: "後端未回報收件人數"}
				</div>
				{outcome.kind === "empty" && (
					<div style={{ marginTop: 6, fontSize: 13 }}>
						公告已建立，但沒有任何使用者收到。請確認發送對象是否有註冊使用者，若對象不該是空的，代表收件人查詢有異常。
					</div>
				)}
				{outcome.kind === "unreported" && (
					<div style={{ marginTop: 6, fontSize: 13 }}>
						公告已建立，但回應沒有帶回 recipientCount，無法確認實際送達幾人。
					</div>
				)}
			</div>

			{outcome.broadcastId && (
				<div style={{ marginTop: 12 }}>
					<div style={{ fontSize: 12, color: "#6b7280", marginBottom: 2 }}>
						公告 ID<span style={{ marginLeft: 6 }}>追查這則推播時用得到</span>
					</div>
					<code
						style={{
							display: "block",
							padding: "6px 10px",
							background: "#f9fafb",
							borderRadius: 6,
							border: "1px solid #e5e7eb",
							fontSize: 13,
							wordBreak: "break-all",
						}}
					>
						{outcome.broadcastId}
					</code>
				</div>
			)}

			<div
				style={{
					marginTop: 16,
					display: "flex",
					justifyContent: "flex-end",
				}}
			>
				<button type="button" className="btn" onClick={onClose}>
					關閉
				</button>
			</div>
		</div>
	);
};

export default TemplateList;
