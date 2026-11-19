"use client";

import Button from "@/components/ui/Button";
import type { ProductLoadStatus } from "../hooks/useProductForm";

/** 編輯草稿讀不到時取代表單：沒有可編輯的欄位，也就送不出更新。 */
export default function ProductLoadFailure({
	status,
	onRetry,
}: {
	status: Extract<ProductLoadStatus, "failed" | "notFound">;
	onRetry: () => void;
}) {
	if (status === "notFound") {
		return (
			<div
				className="grid min-h-64 place-items-center px-6 py-12 text-center"
				role="status"
			>
				<div className="max-w-sm">
					<h3 className="m-0 text-lg font-bold text-hk-heading">
						這頁不在貨架上
					</h3>
					<p className="mb-0 mt-2 text-sm leading-6 text-hk-muted">
						可能是連結過期，或商品已經下架。
					</p>
				</div>
			</div>
		);
	}
	return (
		<div
			className="grid min-h-64 place-items-center px-6 py-12 text-center"
			role="alert"
		>
			<div className="max-w-sm">
				<h3 className="m-0 text-lg font-bold text-hk-heading">
					商品資料載入失敗
				</h3>
				<p className="mb-5 mt-2 text-sm leading-6 text-hk-muted">
					請確認網路連線後再試一次。
				</p>
				<Button type="button" color="primary" onClick={onRetry}>
					重新載入
				</Button>
			</div>
		</div>
	);
}
