import type {
	Case,
	Report,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Store } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { addressText } from "@/app/admin/merchants/[id]/store-detail";
import { formatDateTimeShort } from "@/lib/format";
import { timeText } from "./report-view";

// 食安案件的轉報資料（.md，由 client 組）：交給主管機關的文件，以案件為單位。GetFDAExport 只回結果不是未違規、內容不足或濫報的通報（伺服器的 Supporting 規則），
// 這裡原樣列出、不再過濾。不放檢舉人的聯絡方式（需要聯絡檢舉人時由營運人員另外提供），佐證只寫份數、另附，不放後台連結。
// 這份文件照主管機關慣用的「通報」用詞，不跟買家面的「檢舉」。

/** K10：轉報資料的開頭一句（rb R5「以平台名義轉報」、3cd D′8）。 */
export const PLATFORM_REFERRAL_TEXT =
	"本轉報由 Hawkr 平台以平台名義提出；通報人的聯絡方式不在本檔。";
const MISSING_BY_REPORTER = "通報人沒有提供";
const NONE = "無";

/** 多行文字放進清單項目時，後面的行縮排兩格才屬於同一項。 */
function listItemText(text: string): string {
	return text.replace(/\r?\n/gu, "\n  ");
}

function orNone(value: string): string {
	return value.trim() === "" ? NONE : value;
}

export interface FdaExportData {
	case?: Pick<Case, "id" | "openTime" | "closeTime">;
	reports: readonly Report[];
	store?: Pick<
		Store,
		| "storeName"
		| "storeCode"
		| "contactName"
		| "contactPhone"
		| "address"
		| "approvedFoodRegistration"
	>;
}

/** 轉報資料的檔名。 */
export function fdaExportFileName(caseId: string): string {
	return `fda-referral-case-${caseId}.md`;
}

/** 組轉報資料的 Markdown。時間一律台北時間，產生時間由呼叫端傳入（now）。 */
export function buildFdaExport(data: FdaExportData, now: Date): string {
	const reports = data.reports;
	const store = data.store;
	const lines: string[] = [
		`# 食安通報轉報資料（案件 ${data.case?.id ?? ""}）`,
		"",
		PLATFORM_REFERRAL_TEXT,
		"",
		"## 1. 時間",
		"",
		`- 開案時間：${timeText(data.case?.openTime)}`,
		`- 結案時間：${data.case?.closeTime ? timeText(data.case.closeTime) : "還沒結案"}`,
		"",
		"## 2. 被通報的店家",
		"",
		`- 店名：${orNone(store?.storeName ?? "")}`,
		`- 店號：${orNone(store?.storeCode ?? "")}`,
		`- 聯絡人：${orNone(store?.contactName ?? "")}`,
		`- 聯絡電話：${orNone(store?.contactPhone ?? "")}`,
		`- 地址：${store?.address ? orNone(addressText(store.address)) : NONE}`,
		`- 食品業者登錄字號：${orNone(store?.approvedFoodRegistration?.registrationNumber ?? "")}`,
		"",
		`## 3. 通報（${reports.length} 筆）`,
	];
	reports.forEach((report, i) => {
		lines.push(
			"",
			`### 通報 ${i + 1}（${timeText(report.createTime)}）`,
			"",
			`- 商品 ID：${report.productId || MISSING_BY_REPORTER}`,
			`- 訂單編號：${report.orderNo || MISSING_BY_REPORTER}`,
			"",
			"內容：",
			"",
			report.description,
		);
		if (report.supplements.length > 0) {
			lines.push("", "補充：", "");
			for (const supplement of report.supplements) {
				lines.push(
					`- ${timeText(supplement.createTime)}：${listItemText(supplement.description)}`,
				);
			}
		}
		lines.push(
			"",
			`佐證：${report.evidence.length > 0 ? `${report.evidence.length} 份，另附` : NONE}`,
		);
	});
	lines.push(
		"",
		"---",
		"",
		`本檔由後台於 ${formatDateTimeShort(now)} 產生，供轉報主管機關使用。`,
		"",
	);
	return lines.join("\n");
}
