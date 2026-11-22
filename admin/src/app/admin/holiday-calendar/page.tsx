"use client";

import { useMutation, useQuery } from "@connectrpc/connect-query";
import { BizCalendarAdminService } from "@hawkr/api/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendar_pb";
import { formatDate } from "@hawkr/api/proto/date";
import { useRef, useState } from "react";
import { Button, Dialog, StatusBadge } from "@/components/UI";
import { connectErrorText, errorMessage } from "@/lib/error-text";
import { formatIsoDate } from "@/lib/format";
import "@/styles/pages/HolidayCalendar.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import {
	buildImportRequest,
	buildPreview,
	type CalendarPreview,
	type CalendarRow,
	coverageBadge,
	coverageDaysLeft,
	parseCalendarCsv,
	yearRowsOf,
} from "./holiday-calendar";

const IMPORT_FAILED_TEXT = "匯入失敗，請重試。";

interface ImportResult {
	year: number;
	dayCount: number;
	holidayCount: number;
	replacedDayCount: number;
}

export default function HolidayCalendarPage() {
	const fileInputRef = useRef<HTMLInputElement>(null);

	// 上傳 / 解析狀態
	const [fileName, setFileName] = useState("");
	const [csvText, setCsvText] = useState("");
	const [rows, setRows] = useState<CalendarRow[]>([]);
	const [preview, setPreview] = useState<CalendarPreview | null>(null);
	const [declaredYear, setDeclaredYear] = useState(""); // 宣告 / 辨識年份（字串）
	const [parseError, setParseError] = useState("");
	const [dragging, setDragging] = useState(false);

	// 二次確認 / 結果
	const [showConfirm, setShowConfirm] = useState(false);
	const [importResult, setImportResult] = useState<ImportResult | null>(null);

	// 覆蓋現況
	const coverageQuery = useQuery(
		BizCalendarAdminService.method.getHolidayCalendarCoverage,
		{},
	);
	const coverage = coverageQuery.data;
	const coverageLoading = coverageQuery.isLoading;

	const importMutation = useMutation(
		BizCalendarAdminService.method.importHolidayCalendar,
		{
			onSuccess: (res) => {
				if (res.year) {
					setImportResult({
						year: res.year.year,
						dayCount: res.year.dayCount,
						holidayCount: res.year.holidayCount,
						replacedDayCount: res.replacedDayCount,
					});
				}
				setShowConfirm(false);
				resetUpload();
				void coverageQuery.refetch();
			},
		},
	);
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
			const parsedRows = parseCalendarCsv(text);
			if (parsedRows.length === 0) {
				setFileName(file.name);
				setCsvText("");
				setRows([]);
				setPreview(null);
				setDeclaredYear("");
				setParseError("CSV 無有效資料，請確認為人事行政總處辦公日曆表格式。");
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

	// 宣告年份的列（資料量小，不必記憶化）
	const yearNum = declaredYear ? Number(declaredYear) : Number.NaN;
	const yearRows = preview ? yearRowsOf(rows, yearNum) : [];
	const writeCount = yearRows.length;
	const holidayRows = yearRows.filter((r) => r.holiday);
	const dateRange =
		yearRows.length > 0
			? `${yearRows[0].iso} ~ ${yearRows[yearRows.length - 1].iso}`
			: "-";

	// 該宣告年份於覆蓋現況中的現有天數（= 將取代的天數）
	const existingCount =
		coverage?.years.find((y) => y.year === yearNum)?.dayCount ?? 0;

	// 「確認匯入」：先讀最新覆蓋現況取得現有天數，再開二次確認 dialog
	const handleRequestImport = async () => {
		if (!preview || !declaredYear || writeCount === 0) return;
		importMutation.reset();
		await coverageQuery.refetch();
		setShowConfirm(true);
	};

	const handleConfirmImport = () => {
		if (!csvText || !declaredYear || importing) return;
		importMutation.mutate(buildImportRequest(declaredYear, csvText));
	};

	const badge = coverage
		? coverageBadge(
				coverage.coveredThrough
					? coverageDaysLeft(
							formatDate(coverage.coveredThrough),
							formatIsoDate(new Date()),
						)
					: -1,
			)
		: null;

	return (
		<div className="hcal-page">
			{/* Page Header */}
			<div className="page-header">
				<div>
					<h1 className="page-title">假日行事曆管理</h1>
					<p className="page-subtitle">
						上傳辦公日曆表 CSV 補齊假日資料，並檢視各年度覆蓋現況
					</p>
				</div>
			</div>

			{/* ==================== 上傳與匯入 ==================== */}
			<div className="hcal-card">
				<h2 className="hcal-card-title">
					<MaterialSymbol name="cloud_upload" />
					上傳辦公日曆表 CSV
				</h2>
				<p className="hcal-card-desc">
					支援行政院人事行政總處辦公日曆表逐日
					CSV（表頭「西元日期,星期,是否放假,備註」，日期 yyyymmdd，是否放假為 0
					或
					2，須涵蓋整年每一天）。系統會先本地解析預覽，確認後才覆寫該年度假日資料。
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
										{yc.year} 年（{yc.count} 列）
									</option>
								))}
							</select>
						</div>

						<div className="hcal-preview-grid">
							<div className="hcal-preview-item">
								<span className="hcal-preview-label">檔案總解析列數</span>
								<span className="hcal-preview-value">{preview.totalRows}</span>
							</div>
							<div className="hcal-preview-item">
								<span className="hcal-preview-label">
									{declaredYear} 年將寫入（天）
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

						{holidayRows.length > 0 && (
							<>
								<div className="hcal-sample-title">
									放假樣本（前 {Math.min(5, holidayRows.length)} 天）
								</div>
								<div className="hcal-sample-list">
									{holidayRows.slice(0, 5).map((r) => (
										<div className="hcal-sample-item" key={r.iso}>
											<span className="hcal-sample-date">{r.iso}</span>
											<span>{r.note}</span>
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
						{importResult.year} 年假日匯入完成：寫入 {importResult.dayCount}{" "}
						天（放假 {importResult.holidayCount} 天）、取代舊資料{" "}
						{importResult.replacedDayCount} 天。
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
					距耗盡天數為最後涵蓋日與今天的差距：越接近 0
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
								<th>天數</th>
								<th>放假天數</th>
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
							) : coverageQuery.error ? (
								<tr>
									<td colSpan={3} className="hcal-empty" role="alert">
										<MaterialSymbol name="error" />
										{errorMessage(coverageQuery.error)}
										<button
											type="button"
											className="btn btn-sm"
											onClick={() => void coverageQuery.refetch()}
										>
											重試
										</button>
									</td>
								</tr>
							) : !coverage || coverage.years.length === 0 ? (
								<tr>
									<td colSpan={3} className="hcal-empty">
										<MaterialSymbol name="event_busy" />
										尚無任何假日資料，請於上方上傳辦公日曆表 CSV。
									</td>
								</tr>
							) : (
								[...coverage.years]
									.sort((a, b) => b.year - a.year)
									.map((y) => (
										<tr key={y.year}>
											<td className="hcal-year-cell">{y.year}</td>
											<td>{y.dayCount}</td>
											<td>{y.holidayCount}</td>
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
							<div className="cap">將取代（既有天數）</div>
						</div>
						<div className="hcal-confirm-stat">
							<div className="num add">{writeCount}</div>
							<div className="cap">將寫入（本次天數）</div>
						</div>
					</div>
					<div className="hcal-confirm-warn">
						<MaterialSymbol name="warning" />
						<span>
							此操作會整年取代 {declaredYear} 年既有假日資料（{existingCount}{" "}
							天），改寫入本次解析的 {writeCount} 天。
							請確認宣告年份與檔案內容相符後再執行。
						</span>
					</div>
					{importMutation.isError && (
						<div className="hcal-modal-error">
							<MaterialSymbol name="cancel" />
							{connectErrorText(importMutation.error, IMPORT_FAILED_TEXT)}
						</div>
					)}
				</Dialog>
			)}
		</div>
	);
}
