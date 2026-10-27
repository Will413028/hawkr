"use client";

import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "./GovernanceNotice";

/**
 * 平台強制把一件商品下架。
 *
 * ⚠️ 這裡沒有理由欄位，是刻意的：後端 `PUT /admin/products/{id}/status` 的
 * `UpdateProductStatusRequest` 只有 `status` 一個欄位（gateway `admin_product.go`），
 * 收一個沒有人接收的理由等於在畫面上假裝有治理紀錄。要留理由就得先擴充那支端點。
 */

interface ProductForceUnpublishDialogProps {
	productName: string;
	productId: string;
	/** 商品目前狀態的顯示名稱；呼叫端一律用 `codeLabel(PRODUCT_STATUS_LABEL, status)` 取得 */
	statusLabel: string;
	onClose: () => void;
	/** 送出下架；reject 時對話框留在原地並就地顯示原因 */
	onConfirm: () => Promise<void>;
}

export default function ProductForceUnpublishDialog({
	productName,
	productId,
	statusLabel,
	onClose,
	onConfirm,
}: ProductForceUnpublishDialogProps) {
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleConfirm = async () => {
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm();
		} catch (caught) {
			setError(
				governanceErrorText(
					caught,
					"下架失敗，商品狀態沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="確定要強制下架這件商品嗎？"
			titleIcon="visibility_off"
			description="下架即時生效：買家端看不到也買不到這件商品，已成立的訂單不受影響。賣家仍可自行重新上架，要永久阻止販售請改用停權。"
			closeLabel="關閉強制下架視窗"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button
						variant="ghost"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						返回
					</Button>
					<Button variant="danger" onClick={handleConfirm} loading={submitting}>
						{submitting ? "正在下架" : "強制下架"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>下架對象</span>
				<strong>{productName}</strong>
				<span>商品 ID · {productId}</span>
				<span>目前狀態 · {statusLabel}</span>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
