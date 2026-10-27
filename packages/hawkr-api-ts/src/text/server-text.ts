/**
 * 伺服器驗證文字長度的規則：先以 Go `strings.TrimSpace` 修剪，再以 rune（Unicode code point）計長
 * （例如 `backend/internal/platform/auditreason`）。client 用同一組規則計數與阻擋，送出的也是修剪後的文字。
 * Dart 端的孿生是 `packages/hawkr_api/lib/go_text.dart`，兩邊的公開名稱與行為一致，改一邊要同步另一邊。
 */

/**
 * Go `unicode.IsSpace` 認定的空白：Latin-1 的 `\t \n \v \f \r`、空白、U+0085、U+00A0，
 * 加上 Unicode Zs 與 U+2028、U+2029。不含 U+FEFF（JS 的 `String.prototype.trim` 會修剪它，Go 不會）。
 */
function isGoSpace(codePoint: number): boolean {
	if (codePoint <= 0xff) {
		return (
			(codePoint >= 0x09 && codePoint <= 0x0d) ||
			codePoint === 0x20 ||
			codePoint === 0x85 ||
			codePoint === 0xa0
		);
	}
	return (
		codePoint === 0x1680 ||
		(codePoint >= 0x2000 && codePoint <= 0x200a) ||
		codePoint === 0x2028 ||
		codePoint === 0x2029 ||
		codePoint === 0x202f ||
		codePoint === 0x205f ||
		codePoint === 0x3000
	);
}

/** 與 Go `strings.TrimSpace` 相同的修剪（兩端）。 */
export function trimGoSpace(text: string): string {
	const points = Array.from(text);
	let start = 0;
	let end = points.length;
	while (start < end && isGoSpace(points[start].codePointAt(0) ?? 0)) start++;
	while (end > start && isGoSpace(points[end - 1].codePointAt(0) ?? 0)) end--;
	return points.slice(start, end).join("");
}

/** 伺服器的字數規則：先修剪（`trimGoSpace`），再以 rune（Unicode code point，surrogate pair 算一個）計。 */
export function goTrimmedRuneCount(text: string): number {
	let count = 0;
	for (const _ of trimGoSpace(text)) count++;
	return count;
}
