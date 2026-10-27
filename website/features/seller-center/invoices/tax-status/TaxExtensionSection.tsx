"use client";

/**
 * S02｜申請延長審核期限（端點 20）。
 *
 * 🔴 **能不能申請只看後端的 `canRequestExtension`**，不自己算 `extensionCount`：
 * 後端的條件不只「延長過幾次」，還包含期限剩餘天數。自己算會在兩種方向上出錯，
 * 而且都不會轉紅——多開一顆必定失敗的鍵，或少開一顆賣家其實按得動的鍵，
 * 後者的代價是**資格逾期失效**。
 *
 * ⚠️ 60 天是**平台的資料補件期限**，不是法定開票寬限期。這一區的文案不得暗示
 * 「延長了就可以晚點開發票」。
 */

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Textarea } from "@/components/ui/Input";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useRequestSellerTaxExtension } from "@/lib/hooks/useSellerInvoiceRemedy";
import type { SellerTaxProfile } from "@/lib/invoices/seller-types";

const REASON_MAX = 200;

export function TaxExtensionSection({
	profile,
}: {
	profile: SellerTaxProfile;
}) {
	const ext = useRequestSellerTaxExtension(profile.version);
	const [open, setOpen] = React.useState(false);
	const [reason, setReason] = React.useState("");

	React.useEffect(() => {
		if (ext.succeeded) setOpen(false);
	}, [ext.succeeded]);

	const pending = profile.pendingExtensionRequest;

	// 🔴 只看後端旗標——見檔頭。
	if (!profile.canRequestExtension) return null;

	return (
		<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
			<strong className="text-sm font-bold">需要更多時間準備文件？</strong>
			{/*
			 * 🔴 已送出的申請。**這一塊是賣家判斷「剛才那一按有沒有送出去」的唯一依據**：
			 * 端點 20 只寫一列稽核歷程、不碰 seller_tax_profiles 的任何一欄 ⇒ 在
			 * pendingExtensionRequest 出現之前（2026-09-05），按下去 extensionCount 與
			 * deadlineAt 都不變、canRequestExtension 仍是 true，畫面與按之前一模一樣。
			 *
			 * ⚠️ 有申請在等時**不停用按鈕**：後端沒有擋重複申請，而賣家補一個更完整的
			 * 理由是合理的（平台看的是最新那一筆）。停用會把「我想改理由」變成做不到。
			 */}
			{pending ? (
				<div
					data-testid="tax-extension-pending"
					className="m-0 rounded-hkui-md border border-info-200 bg-info-50 p-3 text-xs leading-relaxed text-info-700"
				>
					<strong className="font-bold">
						{`已於 ${formatTaipeiDate(pending.requestedAt)} 送出申請，等待平台審核。`}
					</strong>
					{pending.reason ? (
						<p className="m-0 mt-1 whitespace-pre-wrap break-words">
							{`你填的原因：${pending.reason}`}
						</p>
					) : null}
				</div>
			) : null}
			<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
				{profile.deadlineAt
					? `目前的補件期限是 ${formatTaipeiDate(profile.deadlineAt)}。可以申請延長一次。`
					: "可以申請延長一次補件期限。"}
			</p>
			<p className="m-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
				{
					"延長的是平台的資料補件期限，不是法定開票寬限期。期間仍應依法開立與交付發票。"
				}
			</p>
			<div className="mt-1">
				<Button size="lg" variant="bordered" onClick={() => setOpen(true)}>
					申請延長期限
				</Button>
			</div>

			<Dialog
				open={open}
				onClose={() => setOpen(false)}
				size="sm"
				title="申請延長期限"
				footer={
					<div className="flex justify-end gap-2">
						<Button
							variant="light"
							size="sm"
							onClick={() => setOpen(false)}
							disabled={ext.requesting}
						>
							取消
						</Button>
						<Button
							variant="solid"
							size="sm"
							isLoading={ext.requesting}
							loadingLabel="送出中…"
							disabled={!reason.trim()}
							onClick={() => ext.request(reason.trim())}
						>
							送出申請
						</Button>
					</div>
				}
			>
				<div className="flex flex-col gap-2">
					<Textarea
						label="申請原因"
						description={`${reason.length} / ${REASON_MAX}　平台會依原因決定是否核准。`}
						maxLength={REASON_MAX}
						rows={4}
						value={reason}
						onChange={(e) => setReason(e.target.value)}
					/>
					{ext.error ? (
						<p
							role="alert"
							className="text-[0.6875rem] leading-relaxed text-danger-700"
						>
							{"送出失敗。可能已經延長過或期限狀態已變更，請重新整理後再試。"}
						</p>
					) : null}
				</div>
			</Dialog>
		</section>
	);
}
