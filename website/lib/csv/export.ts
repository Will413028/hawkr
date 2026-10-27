export function buildCsvContent(rows: string[][]): string {
	return rows
		.map((row) =>
			row.map((cell) => `"${String(cell).replace(/"/g, '""')}"`).join(","),
		)
		.join("\n");
}

/** 觸發瀏覽器下載一個 CSV 檔案。BOM 前綴讓 Excel 正確辨識 UTF-8 編碼。 */
export function downloadCsv(filename: string, rows: string[][]): void {
	const csv = buildCsvContent(rows);
	const blob = new Blob([`﻿${csv}`], { type: "text/csv;charset=utf-8" });
	const url = URL.createObjectURL(blob);
	const a = document.createElement("a");
	a.href = url;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	document.body.removeChild(a);
	URL.revokeObjectURL(url);
}
