"use client";

import * as React from "react";
import DelegatedSellerDataProvider from "../state/DelegatedSellerDataProvider";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import { canRenderDelegatedWorkspace } from "../state/session";
import AdmRail from "./AdmRail";
import AdmTopNav from "./AdmTopNav";
import DelegatedSellerSidebar from "./DelegatedSellerSidebar";
import OperationBlockedState from "./OperationBlockedState";
import OperationContextBar from "./OperationContextBar";
import OperationEndDialog from "./OperationEndDialog";

/**
 * 代行工作區的外框（mockup §4.4.1）：
 *
 * ```
 * [AdmRail]  [主區]
 *             ├─ 頂部白色導覽
 *             └─ .seller-shell（賣家中心既有 grid）
 *                  ├─ 內側欄（240）      └─ .seller-main
 *                                            ├─ OperationContextBar
 *                                            └─ 頁面內容
 * ```
 *
 * ⚠️ ADM 窄欄包在 `.seller-shell` **之外**，`.seller-shell` 的 grid 一格都不動 ——
 * 那個 grid 是 prod 賣家頁面在用的。沿用它同時把 `seller-center.css` 的整套深色
 * 覆寫一起帶進來，代行工作區的深色模式因此零額外工作。
 *
 * 工作階段不可用（expired／revoked／end_unknown／target_mismatch／invalid）時
 * **不渲染 children**：那些頁面會發代行請求，而此刻每一個都必然被拒。
 */
export default function DelegatedWorkspaceFrame({
	storeId,
	children,
}: {
	storeId: string;
	children: React.ReactNode;
}) {
	const { state, loading, returnUrl, endSession, ending } =
		useDelegatedSession();
	const [endDialogOpen, setEndDialogOpen] = React.useState(false);

	if (loading) {
		return (
			<div className="adm-blocked" aria-busy="true">
				<div className="adm-blocked__card">
					<p className="adm-blocked__body" role="status">
						正在確認代行工作階段…
					</p>
				</div>
			</div>
		);
	}

	if (!canRenderDelegatedWorkspace(state)) {
		return (
			<OperationBlockedState
				state={state}
				returnUrl={returnUrl}
				onRetryEnd={endSession}
				retryingEnd={ending}
			/>
		);
	}

	return (
		<div className="adm-workspace">
			<AdmRail />
			<div className="adm-workspace__main">
				<AdmTopNav storeId={storeId} />
				<div className="seller-shell" data-hk-seller-workspace-ready>
					<DelegatedSellerSidebar
						storeId={storeId}
						onRequestEnd={() => setEndDialogOpen(true)}
					/>
					<main className="seller-main">
						<OperationContextBar onRequestEnd={() => setEndDialogOpen(true)} />
						{/* 資料層的注入點包在頁面外面：賣家中心的元件（本工作區的內容全部
						    來自它們）一律經 `useSellerDataScope()` 取得 repo／快取鍵／
						    啟用條件，沒有 provider 時拿到的是賣家路徑那一份。 */}
						<DelegatedSellerDataProvider storeId={storeId}>
							{children}
						</DelegatedSellerDataProvider>
					</main>
				</div>
			</div>
			<OperationEndDialog
				open={endDialogOpen}
				onClose={() => setEndDialogOpen(false)}
			/>
		</div>
	);
}
