/** 把文字存成檔案讓瀏覽器下載（只能在瀏覽器裡呼叫）。 */
export function downloadTextFile(
	fileName: string,
	text: string,
	mimeType = "text/markdown;charset=utf-8",
): void {
	const url = URL.createObjectURL(new Blob([text], { type: mimeType }));
	const link = document.createElement("a");
	link.href = url;
	link.download = fileName;
	document.body.appendChild(link);
	link.click();
	link.remove();
	URL.revokeObjectURL(url);
}
