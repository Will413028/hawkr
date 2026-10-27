"use client";

import * as React from "react";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";

/**
 * 等待 `authWaiting` 期間的阻擋層。
 *
 * 刻意「不可關閉」：這是身分確認的交接點（等待 → 動作發動，或由
 * LoginRequiredModal 接手），中途關掉只會讓使用者對著一個什麼都不會發生的
 * 頁面。但「不可關閉」不等於「可以不擋」——舊版只畫了一層不透明 backdrop，
 * 後方所有元素仍可 Tab、可點、可捲，畫面被遮住卻還在接受操作。
 *
 * 改建在共用 `Dialog` 上：inert 掉 `.page-wrapper`、鎖住頁面捲動、把焦點收進
 * 面板，全部沿用同一份實作；`isDismissable={false}` 保住不可關閉的設計意圖
 * （Escape 與 backdrop 點擊都不會關）。
 */
const WATCHDOG_MS = 15000;

export default function AuthGateWaitModal({ open }: { open: boolean }) {
	const [stalled, setStalled] = React.useState(false);

	React.useEffect(() => {
		if (!open) {
			setStalled(false);
			return;
		}
		// `authWaiting` 若因為請求卡住／例外而不翻轉，這個 overlay 會永久蓋住
		// 整個 app 且沒有任何出口。15 秒後給一條自救路徑（重新整理），而不是
		// 讓使用者只能關掉分頁。
		const timer = window.setTimeout(() => setStalled(true), WATCHDOG_MS);
		return () => window.clearTimeout(timer);
	}, [open]);

	return (
		<Dialog
			open={open}
			// Never called: dismissal is off. Kept because the prop is required.
			onClose={() => {}}
			isDismissable={false}
			showCloseButton={false}
			// 純等待型 overlay：不搶焦點，讓接手的 LoginRequiredModal 記到的
			// 「原焦點」是使用者真正點的那顆按鈕，關閉後才回得去（否則會回到
			// 這個即將卸載的面板，最後掉到 body）。
			autoFocus={false}
			size="sm"
			aria-busy={!stalled}
			className="w-[min(232px,100%)] border-[color:var(--color-neutral-panel)] bg-[linear-gradient(180deg,var(--hkui-content1),var(--hkui-content2))] text-hk-body shadow-[0_28px_70px_rgb(15_33_15_/_0.24),0_1px_0_rgb(255_255_255_/_0.8)_inset]"
			bodyClassName="flex flex-col items-center gap-3 px-6 py-6"
		>
			<div className="h-16 w-full">
				<BrandLogoLoader size="fit" />
			</div>
			{/* polite live region：等待訊息只宣告一次，watchdog 文案接著補上。 */}
			<div
				className="grid justify-items-center gap-1"
				role="status"
				aria-live="polite"
			>
				<p className="m-0 text-sm font-semibold text-hk-heading">
					確認登入狀態中
				</p>
				<p className="m-0 text-xs text-hk-muted">
					{stalled ? "花的時間比預期久，可以重新整理再試" : "馬上就好，請稍候"}
				</p>
			</div>
			{stalled ? (
				<Button
					variant="bordered"
					size="sm"
					onClick={() => window.location.reload()}
				>
					重新整理
				</Button>
			) : null}
		</Dialog>
	);
}
