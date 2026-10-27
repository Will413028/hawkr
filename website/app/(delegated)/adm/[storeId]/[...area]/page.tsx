import DelegatedAreaUnauthorized from "@/features/delegated-operations/components/DelegatedAreaUnauthorized";

/**
 * 「渲染但不可操作」的模組（§6.1.14／定案第 3 條）。
 *
 * 賣家中心側欄的每一項在代行工作區都看得到，**可操作的那一份宣告在
 * `features/delegated-operations/nav-policy.ts` 的 `DELEGATED_AREA_CAPABILITY`**
 * （各自有具名路由）；表以外的一律落到這一支空狀態頁。
 *
 * ⚠️ 這裡刻意**不再抄一份未開放模組的清單**：抄一份就會與 `nav-policy` 各說各話，
 * 而分岔的症狀是「宣告可操作的模組點進去卻是空狀態」（總覽曾經如此），沒有任何
 * 東西會報錯。兩份對得起來由 `tests/delegated-workspace-faces.test.ts` 以「可操作
 * 段 ↔ 具名路由目錄」雙向比對。
 *
 * 用 catch-all 而不是逐一建檔，是為了讓賣家中心之後新增的模組**自動**落在
 * 「渲染＋空狀態」這個預設分支 —— 已接線的面各自有具名路由，Next 的具名段永遠
 * 優先於 catch-all，所以它接不到已開放的頁面。
 *
 * 畫面本體由 `DelegatedAreaUnauthorized` 承擔（同一份文案的唯一主人）：這裡曾經是
 * 三份手抄副本之一，而沒抄到的路徑各自長出了會說謊的畫面。
 */
export default function DelegatedAreaUnavailablePage() {
	return (
		<DelegatedAreaUnauthorized reason="這個模組本期未開放代行操作。需要調整時請聯繫賣家本人，或回到 ADM 以平台治理流程處理。" />
	);
}
