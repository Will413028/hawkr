"use client";

import * as React from "react";
import { Dialog, Dropdown } from "@/components/ui";
import Button from "@/components/ui/Button";
import {
	SELLER_CANCEL_REASON_LABEL,
	SellerCancelReason,
	type SellerCancelReasonValue,
} from "@/lib/codemeta/enums";
import { useResettableError } from "@/lib/hooks/useResettableError";
import { cancelSellerOrder } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";

const CANCEL_REASON_ITEMS: ReadonlyArray<{ value: string; label: string }> = [
	SellerCancelReason.OUT_OF_STOCK,
	SellerCancelReason.QUALITY_ISSUE,
	SellerCancelReason.FORCE_MAJEURE,
	SellerCancelReason.OTHER,
].map((value) => ({ value, label: SELLER_CANCEL_REASON_LABEL[value] }));

/**
 * 賣家取消訂單對話框 — 列表頁 kebab 與訂單詳情抽屜共用。後端
 * seller_cancellations endpoint 只接受 待出貨(paid/preparing) 狀態，呼叫端
 * 需先以 `canCancelOrder(order)` 把關，取消成功後系統會自動退款給買家。
 */
export default function CancelOrderDialog({
	order,
	onClose,
	onDone,
}: {
	order: Order;
	onClose: () => void;
	onDone: () => void | Promise<void>;
}) {
	const [reason, setReason] = React.useState<string>(
		SellerCancelReason.OUT_OF_STOCK,
	);
	const [remark, setRemark] = React.useState("");
	const [busy, setBusy] = React.useState(false);
	// 同一個 Dialog instance 在賣家中心切換不同訂單 reuse 時(父層用 portal
	// 維持 mount),order.orderNo 變化代表是新訂單的取消行為,resetKey 確保
	// 前次失敗訊息不殘留。
	const [error, setError] = useResettableError(order.orderNo);

	const submit = async () => {
		if (busy) return;
		setBusy(true);
		setError(null);
		try {
			await cancelSellerOrder(order.orderNo, {
				reason: reason as SellerCancelReasonValue,
				remark: remark.trim() || undefined,
			});
			await onDone();
		} catch (e) {
			setError(e instanceof Error ? e.message : "取消失敗，請稍後再試。");
			setBusy(false);
		}
	};

	return (
		<Dialog
			open
			onClose={onClose}
			size="sm"
			title="取消訂單"
			description={`訂單 ${order.orderNo}，取消後系統會自動啟動退款給買家。`}
			isDismissable={!busy}
			showCloseButton={!busy}
			footer={
				<>
					<Button
						variant="bordered"
						color="default"
						onClick={onClose}
						disabled={busy}
					>
						返回
					</Button>
					<Button
						color="danger"
						onClick={() => void submit()}
						disabled={busy}
						isLoading={busy}
						loadingLabel="取消訂單中…"
					>
						確認取消
					</Button>
				</>
			}
		>
			<div className="seller-orders__cancel-form">
				<Dropdown
					label="取消原因"
					value={reason}
					onChange={setReason}
					items={CANCEL_REASON_ITEMS}
					isRequired
				/>
				<label className="seller-orders__cancel-remark">
					<span>備註（選填）</span>
					<textarea
						value={remark}
						onChange={(e) => setRemark(e.target.value)}
						rows={3}
						maxLength={200}
						placeholder="補充說明，買家可能會看到。"
						disabled={busy}
					/>
				</label>
				{error && (
					<p className="seller-orders__cancel-error" role="alert">
						{error}
					</p>
				)}
			</div>
		</Dialog>
	);
}

/** 賣家可取消的狀態：待出貨(paid/preparing)。後端只接受 status 10/20。 */
export function canCancelOrder(order: Pick<Order, "status">): boolean {
	return order.status === "paid" || order.status === "preparing";
}
