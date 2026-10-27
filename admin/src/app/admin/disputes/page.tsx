"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import type React from "react";
import { useMemo, useState } from "react";
import StatCard from "@/components/UI/StatCard";
import StatusBadge, { type BadgeVariant } from "@/components/UI/StatusBadge";
import { AFTER_SALE_APPEAL_STATE_LABEL, codeLabel } from "@/lib/codemeta";
import { formatDateTime as formatTaipeiDateTimeShared } from "@/lib/format";
import { api } from "@/services/api";

// appeal_state 篩選值：AfterSaleAppealState value（10=已申訴 / 30=已裁決）＋ all=不限。
// 請求參數與回應欄位一律是 Value，前端不做 Name↔Value 轉換。
type AppealStateFilter = "10" | "30" | "all";

interface AppealSummary {
	caseNo: string;
	orderNo: string;
	buyerId: string;
	buyerName: string;
	avatarUrl: string;
	storeId: string;
	sellerName: string;
	logoUrl: string;
	appealState: string;
	appealSubmittedAt: string;
	lastRejectedAt: string;
	refundableAmount: string;
}

interface AppealListResponse {
	code: number;
	message: string;
	data: {
		records: AppealSummary[];
		total: string;
		page: string;
		pageSize: string;
	};
}

// 標籤從 AFTER_SALE_APPEAL_STATE_LABEL 查，不在這裡另抄一份。
//
// 以前這裡把 10 這個碼硬寫成「待裁決」，而同一頁的列表欄位走 codeLabel 顯示同一個碼值時是
// 「已申訴」——**同一個狀態在同一個畫面上有兩個名字**。那正是手抄鏡像會產生的病：
// 兩處各自演化，而且沒有任何東西會發現它們不一致。
const TAB_LABELS: Record<AppealStateFilter, string> = {
	"10": codeLabel(AFTER_SALE_APPEAL_STATE_LABEL, "10"),
	"30": codeLabel(AFTER_SALE_APPEAL_STATE_LABEL, "30"),
	all: "全部",
};

// appealState（AfterSaleAppealState value）→ badge 樣式。
// label 一律走集中常數 AFTER_SALE_APPEAL_STATE_LABEL，這裡只管顏色。
//
// 值域要收齊（`tests/codemeta-no-second-copy.test.ts` 會擋）：以碼值為鍵的呈現層對照
// 少一個成員，那個成員就會靜靜落進 `|| "gray"`，沒有任何訊號。0（NONE＝未申訴）在這一頁
// 理論上不會出現——這裡是申訴列表——但「不會出現」與「出現了長什麼樣」是兩件事，
// 明寫成灰色比留給 fallback 決定誠實。
const STATE_BADGE_VARIANT: Record<string, BadgeVariant> = {
	"0": "gray",
	"10": "danger",
	"20": "success",
	"30": "gray",
};

// 時間一律走 lib/format 的台北釘死版（backend#3053 的稽查）：本地這份用
// toLocaleString 會跟著看的人的瀏覽器時區跑，而管理員不一定在台灣。
const formatDateTime = formatTaipeiDateTimeShared;

/**
 * 幣別前綴是 `NT$ `，不是裸 `$`。
 *
 * 這一頁在裁決要退多少錢，而 `$225` 讀者無從判斷是新台幣還是美元。
 *
 * ⚠️ 不要照抄「其他頁都已經是 NT$」這種說法去跳過清查 —— 本批修的當下，
 * `products/page.tsx` 的 `formatPriceRange` 與 `disputes/[id]` 的退款金額輸入框
 * 都還是裸 `$`（同批一起改掉了）。裸 `$` 沒有任何機械保證擋著，新頁面隨時會再長出來。
 * ⚠️ 全 ADM 有三份同名同責的 `formatAmount`（本檔、`disputes/[id]`、`anomalies`），
 * 這次只把值對齊、沒有合併 —— 合併會動到三頁的匯入，不在本次範圍。
 */
function formatAmount(raw: string): string {
	const n = Number(raw);
	if (!Number.isFinite(n)) return raw || "-";
	return `NT$ ${n.toLocaleString()}`;
}

export default function DisputeList() {
	const router = useRouter();
	const [filter, setFilter] = useState<AppealStateFilter>("10");
	const [keyword, setKeyword] = useState("");
	const [page, setPage] = useState(1);
	const pageSize = 20;

	const { data, isLoading: loading } = useQuery({
		queryKey: ["disputes", "appeals", { filter, page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				appeal_state: filter,
				page: String(page),
				page_size: String(pageSize),
			});
			const res = await api.get<AppealListResponse>(
				`/api/v1/admin/after_sales/appeals?${params}`,
			);
			if (res?.code === 0 && res.data) {
				return {
					records: res.data.records || [],
					total: parseInt(res.data.total, 10) || 0,
				};
			}
			return { records: [] as AppealSummary[], total: 0 };
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;

	const filtered = useMemo(() => {
		if (!keyword.trim()) return records;
		const kw = keyword.trim().toLowerCase();
		return records.filter(
			(r) =>
				r.caseNo.toLowerCase().includes(kw) ||
				r.orderNo.toLowerCase().includes(kw) ||
				r.buyerId.includes(kw) ||
				r.storeId.includes(kw) ||
				(r.buyerName || "").toLowerCase().includes(kw) ||
				(r.sellerName || "").toLowerCase().includes(kw),
		);
	}, [records, keyword]);

	// 比對的是 AfterSaleAppealState value：10=已申訴 / 20=平台調查中 / 30=已裁決。
	const stats = useMemo(() => {
		const pending = records.filter((r) => r.appealState === "10").length;
		const processing = records.filter((r) => r.appealState === "20").length;
		const closed = records.filter((r) => r.appealState === "30").length;
		return { pending, processing, closed };
	}, [records]);

	const handleFilter = (tab: AppealStateFilter) => {
		setFilter(tab);
		setPage(1);
	};

	const totalPages = Math.ceil(total / pageSize);

	return (
		<div>
			<div className="page-header">
				<div>
					<div className="page-title">售後爭議管理</div>
					<div className="page-subtitle">需處理的買家申訴案件列表</div>
				</div>
			</div>

			<div className="stats-row">
				<StatCard
					label="待裁決案件"
					value={stats.pending}
					icon="schedule"
					variant="danger"
				/>
				<StatCard
					label="處理中案件"
					value={stats.processing}
					icon="progress_activity"
					variant="warning"
				/>
				<StatCard
					label="已裁決案件"
					value={stats.closed}
					icon="check_circle"
					variant="success"
				/>
				<StatCard
					label="本頁案件數"
					value={total}
					icon="list_alt"
					variant="default"
				/>
			</div>

			<div style={{ display: "flex", gap: 8, marginBottom: 16 }}>
				{(["10", "30", "all"] as AppealStateFilter[]).map((tab) => (
					<button
						type="button"
						key={tab}
						onClick={() => handleFilter(tab)}
						style={{
							padding: "8px 16px",
							borderRadius: 8,
							border: "1px solid",
							borderColor:
								filter === tab ? "var(--admin-primary-strong)" : "#dadada",
							background:
								filter === tab ? "var(--admin-primary-strong)" : "white",
							color: filter === tab ? "white" : "#374151",
							fontWeight: 600,
							cursor: "pointer",
						}}
					>
						{TAB_LABELS[tab]}
					</button>
				))}
			</div>

			<div style={{ marginBottom: 16 }}>
				<div
					style={{
						position: "relative",
						background: "white",
						border: "1px solid #dadada",
						borderRadius: 10,
						padding: "10px 16px",
						display: "flex",
						alignItems: "center",
						gap: 8,
					}}
				>
					<MaterialSymbol name="search" style={{ color: "#9ca3af" }} />
					<input
						type="text"
						placeholder="搜尋案件編號 / 訂單編號 / 買賣家名稱 / ID..."
						value={keyword}
						onChange={(e) => setKeyword(e.target.value)}
						style={{
							flex: 1,
							border: "none",
							outline: "none",
							fontSize: 14,
							background: "transparent",
						}}
					/>
				</div>
			</div>

			<div className="table-container">
				<table style={{ width: "100%", borderCollapse: "collapse" }}>
					<thead
						style={{ background: "#f9fafb", borderBottom: "1px solid #e5e7eb" }}
					>
						<tr>
							<th style={thStyle}>案件編號</th>
							<th style={thStyle}>訂單編號</th>
							<th style={thStyle}>買 / 賣家</th>
							<th style={thStyle}>可退金額</th>
							<th style={thStyle}>申訴時間</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>操作</th>
						</tr>
					</thead>
					<tbody>
						{loading ? (
							<tr>
								<td
									colSpan={7}
									style={{ textAlign: "center", padding: 40, color: "#9ca3af" }}
								>
									載入中...
								</td>
							</tr>
						) : filtered.length === 0 ? (
							<tr>
								<td
									colSpan={7}
									style={{ textAlign: "center", padding: 40, color: "#9ca3af" }}
								>
									暫無資料
								</td>
							</tr>
						) : (
							filtered.map((r) => {
								const badgeVariant: BadgeVariant =
									STATE_BADGE_VARIANT[r.appealState] || "gray";
								return (
									<tr
										key={r.caseNo}
										style={{ borderBottom: "1px solid #f3f4f6" }}
									>
										<td style={tdStyle}>
											<span style={{ fontWeight: 600, color: "#101828" }}>
												{r.caseNo}
											</span>
										</td>
										<td style={tdStyle}>{r.orderNo}</td>
										<td style={tdStyle}>
											<div
												style={{
													display: "flex",
													alignItems: "center",
													gap: 6,
												}}
											>
												<span
													style={{
														fontSize: 12,
														padding: "1px 6px",
														border: "1px solid #2b7fff",
														borderRadius: 4,
														color: "#2b7fff",
													}}
												>
													買
												</span>
												<span style={{ color: "#374151" }}>
													{r.buyerName || r.buyerId}
												</span>
											</div>
											<div
												style={{
													display: "flex",
													alignItems: "center",
													gap: 6,
													marginTop: 4,
												}}
											>
												<span
													style={{
														fontSize: 12,
														padding: "1px 6px",
														border: "1px solid var(--admin-primary-strong)",
														borderRadius: 4,
														color: "var(--admin-primary-strong)",
													}}
												>
													賣
												</span>
												<span style={{ color: "#374151" }}>
													{r.sellerName || r.storeId}
												</span>
											</div>
										</td>
										<td style={tdStyle}>{formatAmount(r.refundableAmount)}</td>
										<td style={tdStyle}>
											{formatDateTime(r.appealSubmittedAt)}
										</td>
										<td style={tdStyle}>
											<StatusBadge variant={badgeVariant}>
												{codeLabel(
													AFTER_SALE_APPEAL_STATE_LABEL,
													r.appealState,
												)}
											</StatusBadge>
										</td>
										<td style={tdStyle}>
											<button
												type="button"
												onClick={() =>
													router.push(
														`/admin/disputes/${encodeURIComponent(r.caseNo)}`,
													)
												}
												style={{
													background: "var(--admin-primary-strong)",
													color: "white",
													border: "none",
													borderRadius: 8,
													padding: "6px 14px",
													fontSize: 14,
													fontWeight: 600,
													cursor: "pointer",
												}}
											>
												查看詳情
											</button>
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>

			{totalPages > 1 && (
				<div
					style={{
						display: "flex",
						justifyContent: "center",
						alignItems: "center",
						gap: 16,
						marginTop: 16,
					}}
				>
					<button
						type="button"
						onClick={() => setPage((p) => Math.max(1, p - 1))}
						disabled={page === 1}
						style={pageBtnStyle(page === 1)}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span style={{ color: "#6b7280" }}>
						第 {page} / {totalPages} 頁
					</span>
					<button
						type="button"
						onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
						disabled={page >= totalPages}
						style={pageBtnStyle(page >= totalPages)}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
}

const thStyle: React.CSSProperties = {
	padding: "12px 16px",
	textAlign: "left",
	fontSize: 14,
	fontWeight: 600,
	color: "#374151",
};

const tdStyle: React.CSSProperties = {
	padding: "16px",
	fontSize: 14,
	color: "#374151",
	verticalAlign: "middle",
};

function pageBtnStyle(disabled: boolean): React.CSSProperties {
	return {
		background: "white",
		border: "1px solid #dadada",
		borderRadius: 8,
		padding: "6px 14px",
		fontSize: 14,
		fontWeight: 600,
		color: disabled ? "#9ca3af" : "#374151",
		cursor: disabled ? "not-allowed" : "pointer",
		opacity: disabled ? 0.6 : 1,
	};
}
