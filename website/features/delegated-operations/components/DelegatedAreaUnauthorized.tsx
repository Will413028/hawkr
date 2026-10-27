import Link from "@/components/shared/Link";

/**
 * 「不在本次代行授權範圍」空狀態 —— 代行工作區**唯一**的授權不足畫面（§6.1.14／
 * 定案第 3 條）。
 *
 * 為什麼收斂成一支元件：這段畫面原本在三個地方各抄一份（`[...area]` catch-all、
 * 訂單頁、上架新商品頁），而**沒有抄到的那幾條路徑各自長出了別的表現**，且每一種
 * 都在回答一個操作者沒問的問題：
 * - 動態列表（缺 `posts.read`）：`共 0 則動態` ＋「發布第一篇」CTA ⇒ 宣稱**這家店
 *   沒發過文**（實際 11 篇未刪）。平台管理員會據此下判斷，這是最貴的一種假話。
 * - 商店設定（缺 `storefront.read`）：`無法載入商店資料` ⇒ 宣稱**系統壞了**，操作者
 *   會去重試、去找工程，而重試一萬次也不會有資料。
 *
 * 三條路徑三套標準的根因是「這段畫面沒有主人」。有了主人之後，新的面要嘛用它、
 * 要嘛在 review 時被看見。
 *
 * ⚠️ 這個畫面**不得發出任何後端請求**（含「打一支必然 403 的端點來探測」）：那會在
 * `admin_operation_audit_logs` 製造一批 `*_READ` ＋ `FAILED` 的列，把「有人試圖跨店
 * 讀取」這個安全訊號淹掉，而稽核表看起來完全正常、沒有任何東西會報錯。
 */
export default function DelegatedAreaUnauthorized({
	reason,
	backHref,
	backLabel,
}: {
	/**
	 * 為什麼看不到。
	 *
	 * 一律講「本次工作階段沒有核發什麼」，**不得**講成「沒有資料」或「載入失敗」——
	 * 那兩句都是在替目標店家的實際狀態作證，而這個畫面恰恰是「我們沒讀過、不知道」。
	 */
	reason: string;
	backHref?: string;
	backLabel?: string;
}) {
	return (
		<section className="adm-panel">
			<h1 className="adm-panel__title">不在本次代行授權範圍</h1>
			<p className="adm-panel__hint">{reason}</p>
			{backHref ? (
				<Link
					href={backHref}
					// 理由同 `OperationBlockedState`：`.adm-blocked__link` 的實心底白字
					// 會被 dark 的 `body a:not([data-hkui-button])` 換成 `--color-ink-200`。
					data-hkui-button=""
					className="adm-blocked__link"
				>
					{backLabel ?? "返回"}
				</Link>
			) : null}
		</section>
	);
}
