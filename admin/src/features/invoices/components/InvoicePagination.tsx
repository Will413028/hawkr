import { Button } from "@/components/UI";

/**
 * 發票佇列頁共用的分頁鍵。
 *
 * 形狀照 ADM 既有的 `anomalies/page.tsx`（最乾淨的那一份），但**收在 feature 之內**：
 * ADM 全站已經有至少三份同名同責的 `Pagination` 私有副本，發票這一波的四個佇列頁
 * （A02／A04／A05／A07）不再各抄一份變成第四到第七份。
 *
 * 抽到 `src/components/UI/` 屬「新增全站共用元件」，58A §B-3.1.7 要求先與使用者確認
 * ⇒ 本波停在 feature 層。
 */
export default function InvoicePagination({
	page,
	total,
	pageSize,
	onPage,
}: {
	page: number;
	total: number;
	pageSize: number;
	onPage: (p: number) => void;
}) {
	const totalPages = Math.max(1, Math.ceil(total / pageSize));
	if (totalPages <= 1) return null;
	return (
		<div className="invoice-pagination">
			<Button
				variant="outline"
				size="sm"
				disabled={page === 1}
				onClick={() => onPage(Math.max(1, page - 1))}
			>
				上一頁
			</Button>
			<span>
				第 {page} / {totalPages} 頁，共 {total} 筆
			</span>
			<Button
				variant="outline"
				size="sm"
				disabled={page >= totalPages}
				onClick={() => onPage(Math.min(totalPages, page + 1))}
			>
				下一頁
			</Button>
		</div>
	);
}
