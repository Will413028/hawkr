/**
 * 買家端的品相狀態顯示字。稿 P-01 頂部四格與 R-01 ①都用「買家尚未指定」——
 * 比賣家端的「尚未指定」多兩個字，因為買家看的是自己有沒有指定過。
 */
const LABELS: Record<string, string> = {
	BUYER_QUALITY_UNKNOWN: "買家尚未指定",
};

export function buyerQualityStateLabel(state: string): string {
	if (!state) return "";
	return LABELS[state] ?? state;
}
