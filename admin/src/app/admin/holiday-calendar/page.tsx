"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { Button, Dialog, StatusBadge } from "@/components/UI";
import { api } from "@/services/api";
import "@/styles/pages/HolidayCalendar.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// ==================== API 契約（設計書 24 §4.3/4.4；Gateway shipping.go）====================
// 數字欄位一律 string（Proto → HTTP JSON 慣例），前端顯示 / 比較時轉數字。

interface CoverageYear {
	year: string; // 年份
	count: string; // 該年假日筆數
	maxHolidayDate: string; // 該年最大假日日期（YYYY-MM-DD）
}

interface CoverageData {
	coverageDays: string; // 距耗盡天數（與 #898 監測 metric 同源）；"-1" = 無資料/耗盡
	years: CoverageYear[];
}

interface CoverageResp {
	code: number;
	message: string;
	data: CoverageData;
}

interface ImportData {
	year: string;
	importedCount: string;
	deletedCount: string;
}

interface ImportResp {
	code: number;
	message: string;
	data: ImportData;
}

// ==================== 本地 CSV 解析（鏡射後端 HolidayService.parseCsv / applyHolidays）====================
// 目的：預覽筆數必須與後端實際 insert 數一致（宣告年份過濾 + 依日期去重），
// 才能在二次確認時如實顯示「將寫入 M 筆」。解析規則逐條對齊後端：
//   - 跳過第一行（header）與空行 / 以逗號開頭的行
//   - 欄位以逗號切分，col0=名稱、col1=Start Date（yyyy/M/d）
//   - 名稱 / 日期任一為空即略過；移除名稱前綴 BOM
//   - 日期以 yyyy/M/d 解析，失敗即略過

interface PreviewRow {
	name: string;
	iso: string; // YYYY-MM-DD
	year: number;
}

interface ParsedPreview {
	totalRows: number; // 全部可解析列（含跨年）
	yearCounts: { year: number; count: number }[]; // 各年去重後筆數，依筆數降序
}

/** 嚴格解析 yyyy/M/d，回傳 ISO 字串與年份；非法日期回 null。 */
function parseYmd(s: string): { iso: string; year: number } | null {
	const m = s.match(/^(\d{1,4})\/(\d{1,2})\/(\d{1,2})$/);
	if (!m) return null;
	const y = Number(m[1]);
	const mo = Number(m[2]);
	const d = Number(m[3]);
	const dt = new Date(y, mo - 1, d);
	// 反查避免 2026/13/40 這類被 Date 進位吃掉的非法值
	if (
		dt.getFullYear() !== y ||
		dt.getMonth() !== mo - 1 ||
		dt.getDate() !== d
	) {
		return null;
	}
	const iso = `${String(y).padStart(4, "0")}-${String(mo).padStart(2, "0")}-${String(d).padStart(2, "0")}`;
	return { iso, year: y };
}

function parseCsvRows(text: string): PreviewRow[] {
	const rows: PreviewRow[] = [];
	// 去除全檔前綴 BOM，再依 CRLF / CR / LF 斷行（對齊後端 BufferedReader.readLine）
	const lines = text.replace(/^﻿/, "").split(/\r\n|\r|\n/);
	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		if (i === 0) continue; // 跳過 header
		if (line.trim() === "" || line.startsWith(",")) continue;
		const cols = line.split(",");
		if (cols.length < 2) continue;
		let subject = cols[0].trim();
		const startDate = cols[1].trim();
		if (subject === "" || startDate === "") continue;
		if (subject.startsWith("﻿")) subject = subject.slice(1);
		const parsed = parseYmd(startDate);
		if (!parsed) continue;
		rows.push({ name: subject, iso: parsed.iso, year: parsed.year });
	}
	return rows;
}

/** 由原始列彙整出各年去重後筆數（= 後端各年 insert 數），依筆數降序。 */
function buildPreview(rows: PreviewRow[]): ParsedPreview {
	const uniqPerYear = new Map<number, Set<string>>();
	for (const r of rows) {
		if (!uniqPerYear.has(r.year)) uniqPerYear.set(r.year, new Set());
		uniqPerYear.get(r.year)?.add(r.iso);
	}
	const yearCounts = [...uniqPerYear.entries()]
		.map(([year, set]) => ({ year, count: set.size }))
		.sort((a, b) => b.count - a.count || b.year - a.year);
	return { totalRows: rows.length, yearCounts };
}

/** 宣告年份的去重列（依日期），供筆數 / 日期範圍 / 樣本使用。 */
function declaredYearRows(rows: PreviewRow[], year: number): PreviewRow[] {
	const seen = new Set<string>();
	const out: PreviewRow[] = [];
	for (const r of rows) {
		if (r.year !== year || seen.has(r.iso)) continue;
		seen.add(r.iso);
		out.push(r);
	}
	out.sort((a, b) => a.iso.localeCompare(b.iso));
	return out;
}

// ==================== 覆蓋現況色標（呼應 #898）====================
// coverageDays 距耗盡天數：<7 紅（danger）/ <60 黃（warning）/ 否 綠（success）；
// "-1"（或負值）表無資料 / 已耗盡 → 灰。

function coverageBadge(coverageDays: string): {
	variant: "success" | "warning" | "danger" | "gray";
	label: string;
} {
	const days = Number(coverageDays);
	if (coverageDays === "-1" || Number.isNaN(days) || days < 0) {
		return { variant: "gray", label: "無資料" };
	}
	if (days < 7) return { variant: "danger", label: `距耗盡 ${days} 天` };
	if (days < 60) return { variant: "warning", label: `距耗盡 ${days} 天` };
	return { variant: "success", label: `距耗盡 ${days} 天` };
}

export default function HolidayCalendarPage() {
	const queryClient = useQueryClient();
	const fileInputRef = useRef<HTMLInputElement>(null);

	// 上傳 / 解析狀態
	const [fileName, setFileName] = useState("");
	const [csvText, setCsvText] = useState("");
	const [rows, setRows] = useState<PreviewRow[]>([]);
	const [preview, setPreview] = useState<ParsedPreview | null>(null);
	const [declaredYear, setDeclaredYear] = useState(""); // 宣告 / 辨識年份（字串）
	const [parseError, setParseError] = useState("");
	const [dragging, setDragging] = useState(false);

	// 二次確認 / 結果
	const [showConfirm, setShowConfirm] = useState(false);
	const [importResult, setImportResult] = useState<ImportData | null>(null);

	// 覆蓋現況
	const {
		data: coverage,
		isLoading: coverageLoading,
		refetch: refetchCoverage,
	} = useQuery({
		queryKey: ["holidays", "coverage"],
		queryFn: async () => {
			const res = await api.get<CoverageResp>(
				"/api/v1/admin/holidays/coverage",
			);
			return res?.data ?? { coverageDays: "-1", years: [] };
		},
	});

	// 匯入（POST import_content，方案 A：JSON {year, csvContent}，零 multipart）
	const importMutation = useMutation({
		mutationFn: async (payload: { year: string; csvContent: string }) => {
			const res = await api.post<ImportResp>(
				"/api/v1/admin/holidays/import_content",
				payload,
			);
			return res?.data ?? null;
		},
		onSuccess: (data) => {
			if (data) setImportResult(data);
			setShowConfirm(false);
			resetUpload();
			void queryClient.invalidateQueries({
				queryKey: ["holidays", "coverage"],
			});
		},
	});
	const importing = importMutation.isPending;

	const resetUpload = () => {
		setFileName("");
		setCsvText("");
		setRows([]);
		setPreview(null);
		setDeclaredYear("");
		setParseError("");
		if (fileInputRef.current) fileInputRef.current.value = "";
	};

	// 讀檔 → 本地解析 → 建立預覽 + 預設宣告年份（取筆數最多的年份）
	const handleFile = (file: File) => {
		setParseError("");
		setImportResult(null);
		const reader = new FileReader();
		reader.onload = () => {
			const text = String(reader.result ?? "");
			const parsedRows = parseCsvRows(text);
			if (parsedRows.length === 0) {
				setFileName(file.name);
				setCsvText("");
				setRows([]);
				setPreview(null);
				setDeclaredYear("");
				setParseError(
					"CSV 無有效資料，請確認為政府行事曆（Google 行事曆匯出）格式。",
				);
				return;
			}
			const built = buildPreview(parsedRows);
			setFileName(file.name);
			setCsvText(text);
			setRows(parsedRows);
			setPreview(built);
			setDeclaredYear(String(built.yearCounts[0].year));
		};
		reader.onerror = () => setParseError("讀取檔案失敗，請重試。");
		reader.readAsText(file);
	};

	const handleDrop = (e: React.DragEvent) => {
		e.preventDefault();
		setDragging(false);
		const file = e.dataTransfer.files?.[0];
		if (file) handleFile(file);
	};

	// 宣告年份的去重列（記憶化不必要，資料量小）
	const yearNum = declaredYear ? Number(declaredYear) : Number.NaN;
	const yearRows = preview ? declaredYearRows(rows, yearNum) : [];
	const writeCount = yearRows.length;
	const dateRange =
		yearRows.length > 0
			? `${yearRows[0].iso} ~ ${yearRows[yearRows.length - 1].iso}`
			: "-";

	// 該宣告年份於覆蓋現況中的現有筆數（= 將刪除數）
	const existingCount =
		coverage?.years.find((y) => y.year === declaredYear)?.count ?? "0";

	// 「確認匯入」：先 GET coverage 取最新現有筆數，再開二次確認 dialog
	const handleRequestImport = async () => {
		if (!preview || !declaredYear || writeCount === 0) return;
		importMutation.reset();
		await refetchCoverage();
		setShowConfirm(true);
	};

	const handleConfirmImport = () => {
		if (!csvText || !declaredYear || importing) return;
		importMutation.mutate({ year: declaredYear, csvContent: csvText });
	};

	const badge = coverage ? coverageBadge(coverage.coverageDays) : null;

	return (
		<div className="hcal-page">
			{/* Page Header */}
			<div className="page-header">
				<div>
					<h1 className="page-title">假日行事曆管理</h1>
					<p className="page-subtitle">
						上傳政府行事曆 CSV 補齊假日資料，並檢視各年度覆蓋現況
					</p>
				</div>
			</div>

			{/* ==================== 上傳與匯入 ==================== */}
			<div className="hcal-card">
				<h2 className="hcal-card-title">
					<MaterialSymbol name="cloud_upload" />
					上傳政府行事曆 CSV
				</h2>
				<p className="hcal-card-desc">
					支援行政院人事行政總處 Google
					行事曆匯出格式（第一欄為名稱、第二欄為日期
					yyyy/M/d）。系統會先本地解析預覽，確認後才覆寫該年度假日資料。
				</p>

				{/* 隱藏檔案輸入 + dashed dropzone（reuse ai-styles 視覺） */}
				<input
					ref={fileInputRef}
					type="file"
					accept=".csv,text/csv"
					style={{ display: "none" }}
					onChange={(e) => {
						const file = e.target.files?.[0];
						if (file) handleFile(file);
						e.target.value = "";
					}}
				/>
				<button
					type="button"
					className={`btn-reset hcal-dropzone ${dragging ? "dragging" : ""}`}
					onClick={() => fileInputRef.current?.click()}
					onDragOver={(e) => {
						e.preventDefault();
						setDragging(true);
					}}
					onDragLeave={() => setDragging(false)}
					onDrop={handleDrop}
				>
					<MaterialSymbol name="upload_file" />
					{fileName ? (
						<span className="hcal-file-name">{fileName}</span>
					) : (
						"點擊或拖曳上傳 CSV 檔案"
					)}
					<div className="hcal-dropzone-hint">僅接受 .csv 檔</div>
				</button>

				{parseError && (
					<div className="hcal-parse-error">
						<MaterialSymbol name="error" />
						{parseError}
					</div>
				)}

				{/* 本地解析預覽 */}
				{preview && (
					<div className="hcal-preview">
						<div className="hcal-year-field">
							<label htmlFor="hcal-year">辨識 / 宣告年份</label>
							<select
								id="hcal-year"
								className="hcal-year-select"
								value={declaredYear}
								onChange={(e) => setDeclaredYear(e.target.value)}
							>
								{preview.yearCounts.map((yc) => (
									<option key={yc.year} value={String(yc.year)}>
										{yc.year} 年（{yc.count} 筆）
									</option>
								))}
							</select>
						</div>

						<div className="hcal-preview-grid">
							<div className="hcal-preview-item">
								<span className="hcal-preview-label">檔案總解析筆數</span>
								<span className="hcal-preview-value">{preview.totalRows}</span>
							</div>
							<div className="hcal-preview-item">
								<span className="hcal-preview-label">
									{declaredYear} 年將寫入
								</span>
								<span className="hcal-preview-value accent">{writeCount}</span>
							</div>
							<div className="hcal-preview-item">
								<span className="hcal-preview-label">日期範圍</span>
								<span
									className="hcal-preview-value"
									style={{ fontSize: "14px", fontWeight: 500 }}
								>
									{dateRange}
								</span>
							</div>
						</div>

						{yearRows.length > 0 && (
							<>
								<div className="hcal-sample-title">
									資料樣本（前 {Math.min(5, yearRows.length)} 筆）
								</div>
								<div className="hcal-sample-list">
									{yearRows.slice(0, 5).map((r) => (
										<div className="hcal-sample-item" key={r.iso}>
											<span className="hcal-sample-date">{r.iso}</span>
											<span>{r.name}</span>
										</div>
									))}
								</div>
							</>
						)}

						<div className="hcal-actions">
							<Button
								variant="primary"
								onClick={handleRequestImport}
								disabled={writeCount === 0}
								icon={<MaterialSymbol name="event_available" />}
							>
								匯入 {declaredYear} 年假日
							</Button>
						</div>
					</div>
				)}

				{/* 結果回饋 */}
				{importResult && (
					<div className="hcal-result success">
						<MaterialSymbol name="check_circle" />
						{importResult.year} 年假日匯入完成：寫入{" "}
						{importResult.importedCount} 筆、刪除舊資料{" "}
						{importResult.deletedCount} 筆。
					</div>
				)}
			</div>

			{/* ==================== 各年度覆蓋現況 ==================== */}
			<div className="hcal-card">
				<h2 className="hcal-card-title">
					<MaterialSymbol name="calendar_month" />
					各年度覆蓋現況
				</h2>
				<p className="hcal-card-desc">
					距耗盡天數為全域指標（與 #898 監測告警同源公式）：越接近 0
					代表可用假日資料即將用罄，需盡快補匯入次年資料。
				</p>

				{badge && (
					<div className="hcal-coverage-summary">
						<span>假日資料距耗盡天數：</span>
						<StatusBadge variant={badge.variant}>{badge.label}</StatusBadge>
					</div>
				)}

				<div className="hcal-table-container">
					<table className="hcal-table">
						<thead>
							<tr>
								<th>年份</th>
								<th>假日筆數</th>
								<th>該年最後假日</th>
							</tr>
						</thead>
						<tbody>
							{coverageLoading ? (
								<tr>
									<td colSpan={3} className="hcal-empty">
										<MaterialSymbol name="progress_activity" />
										載入中...
									</td>
								</tr>
							) : !coverage || coverage.years.length === 0 ? (
								<tr>
									<td colSpan={3} className="hcal-empty">
										<MaterialSymbol name="event_busy" />
										尚無任何假日資料，請於上方上傳政府行事曆 CSV。
									</td>
								</tr>
							) : (
								[...coverage.years]
									.sort((a, b) => Number(b.year) - Number(a.year))
									.map((y) => (
										<tr key={y.year}>
											<td className="hcal-year-cell">{y.year}</td>
											<td>{y.count}</td>
											<td>{y.maxHolidayDate || "-"}</td>
										</tr>
									))
							)}
						</tbody>
					</table>
				</div>
			</div>

			{/* ==================== 二次確認對話框（reuse merchants modal + a11y）==================== */}
			{showConfirm && preview && (
				<Dialog
					title={
						<span
							style={{
								display: "flex",
								alignItems: "center",
								gap: "8px",
							}}
						>
							<MaterialSymbol
								name="error"
								style={{ color: "var(--warning)", fontSize: "24px" }}
							/>
							確認匯入 {declaredYear} 年假日
						</span>
					}
					maxWidth="500px"
					onClose={() => setShowConfirm(false)}
					busy={importing}
					footer={
						<>
							<Button
								variant="outline"
								onClick={() => setShowConfirm(false)}
								disabled={importing}
							>
								取消
							</Button>
							<Button
								variant="primary"
								onClick={handleConfirmImport}
								disabled={importing}
								loading={importing}
								icon={
									!importing ? (
										<MaterialSymbol name="event_available" />
									) : undefined
								}
							>
								確認匯入
							</Button>
						</>
					}
				>
					<div className="hcal-confirm-stats">
						<div className="hcal-confirm-stat">
							<div className="num del">{existingCount}</div>
							<div className="cap">將刪除（既有）</div>
						</div>
						<div className="hcal-confirm-stat">
							<div className="num add">{writeCount}</div>
							<div className="cap">將寫入（本次）</div>
						</div>
					</div>
					<div className="hcal-confirm-warn">
						<MaterialSymbol name="warning" />
						<span>
							此操作會先刪除 {declaredYear} 年既有假日資料（{existingCount}{" "}
							筆），再寫入本次解析的 {writeCount} 筆，等同覆寫整年度資料。
							請確認宣告年份與檔案內容相符後再執行。
						</span>
					</div>
					{importMutation.isError && (
						<div className="hcal-modal-error">
							<MaterialSymbol name="cancel" />
							{importMutation.error instanceof Error
								? importMutation.error.message
								: "匯入失敗，請重試。"}
						</div>
					)}
				</Dialog>
			)}
		</div>
	);
}
