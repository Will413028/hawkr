"use client";

import { useQuery } from "@tanstack/react-query";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import {
	type DelegatedEndSummary,
	fetchDelegatedEndSummary,
} from "../state/session-client";

/**
 * 結束確認 ＋ 結束摘要綠卡（mockup §4.11.3、L1032）。
 *
 * ⚠️ 綠卡與四列摘要的唯一資料來源是 `GET /adm/api/session/summary`。**不是**
 * `DELETE current`（它的 `data` 恰為 `{"ended": true}`，一個數字都沒有），也不是
 * `{sessionId}/summary`（那支掛 `requireAdmin()`，www host 打不到）。
 *
 * ⚠️ 拿不到摘要時**不得**把「未執行任何出貨或財務操作」寫死：那句話的價值來自
 * `fulfillmentActionCount`／`financeActionCount` 恆為 `"0"` 這個結構性保證，寫死
 * 之後它就從 P0 事故訊號退化成永遠顯示綠色的裝飾文案。
 *
 * ⚠️ 「稽核狀態」同理由 `pendingAuditCount` 決定，且**與綠卡不同源、不得連動**：
 * 一次 PENDING 殘留不該把一句永遠為真的安全宣稱變成紅色。
 */
export default function OperationEndDialog({
	open,
	onClose,
}: {
	open: boolean;
	onClose: () => void;
}) {
	const { ending, endError, endSession } = useDelegatedSession();
	const summaryQuery = useQuery({
		queryKey: ["delegated", "session", "summary"],
		queryFn: ({ signal }) => fetchDelegatedEndSummary(signal),
		enabled: open,
		retry: false,
		staleTime: 0,
	});
	const summary = summaryQuery.data ?? null;

	return (
		<Dialog
			open={open}
			onClose={() => {
				if (!ending) onClose();
			}}
			size="sm"
			title="結束本次代行？"
			isDismissable={!ending}
			footer={
				<>
					<Button
						type="button"
						variant="bordered"
						className="adm-quiet-button"
						onClick={onClose}
						disabled={ending}
					>
						返回繼續
					</Button>
					<Button
						type="button"
						color="primary"
						onClick={endSession}
						disabled={ending}
						isLoading={ending}
						loadingLabel="正在確認結束"
					>
						結束並返回 ADM
					</Button>
				</>
			}
		>
			<div className="flex flex-col gap-3">
				{summaryQuery.isLoading ? (
					<p className="text-[0.8125rem] text-foreground-soft">
						正在取得本次代行摘要…
					</p>
				) : summary ? (
					<EndSummaryRows summary={summary} />
				) : (
					<p className="text-[0.8125rem] text-foreground-soft">
						目前無法取得本次代行摘要，結束動作不受影響；稍後可在 ADM
						的操作紀錄查詢。
					</p>
				)}

				{summary ? <SafetyCard summary={summary} /> : null}

				{/* 清的是這個瀏覽器記憶體裡的代行快取（`removeQueries(["delegated"])`），
					不是伺服器端的任何東西 —— 文案要講得出清的是什麼、在哪裡。 */}
				<p className="text-center text-[0.75rem] text-foreground-soft">
					結束後將清除本機暫存的商店資料並返回 ADM 商家詳情。
				</p>

				{endError ? (
					<p className="adm-end-dialog__error" role="alert">
						結束結果尚未獲得伺服器明確證明，代行寫入維持封鎖。請重試結束，或返回
						ADM。
					</p>
				) : null}
			</div>
		</Dialog>
	);
}

function EndSummaryRows({ summary }: { summary: DelegatedEndSummary }) {
	const durationMinutes = Math.max(
		0,
		Math.round(Number(summary.durationSeconds) / 60),
	);
	const pending = summary.pendingAuditCount;
	return (
		<>
			<dl className="flex flex-col gap-2 rounded-hk-md bg-default-50 p-4 text-[0.8125rem]">
				<Row label="工作時長" value={`${durationMinutes} 分鐘`} />
				{/* `operationCount` 的後端述詞是 `count(*) FILTER (WHERE result='SUCCEEDED')`
				    —— 它**只數成功**。這一列曾經叫「總操作數」，於是一個做了 4 件事、3 件
				    失敗（其中 2 件高風險）的工作階段，在結束摘要上是「1 項操作」，而 ADM 的
				    同一個欄位標的是「成功操作」：兩邊對同一份資料給出兩種說法，而操作者只會
				    看到其中一邊。標籤與 ADM `SessionAuditConsole` 對齊，口徑由下面那行講明。 */}
				<Row label="成功操作" value={`${summary.operationCount} 項操作`} />
				<Row
					label="稽核狀態"
					value={
						pending === "0" ? "均已寫入稽核紀錄" : `${pending} 筆結果未確認`
					}
				/>
				{/* 語意是「本次至少有一列已發出通知事件」，不是每一筆都通知、更不是賣家已讀。
				    「賣家已通知」會被讀成前者。與 ADM 同一列的文案對齊。 */}
				<Row
					label="賣家通知"
					value={
						summary.notifiedSeller ? "本次已發送賣家通知" : "本次未發送通知"
					}
				/>
			</dl>
			<p className="text-[0.75rem] text-foreground-soft">
				「成功操作」<strong>只含已成功</strong>
				的紀錄，失敗不計入。完整的成功／失敗／結果未確認三態，請看 ADM
				的「代行紀錄」。
			</p>
		</>
	);
}

function SafetyCard({ summary }: { summary: DelegatedEndSummary }) {
	const clean =
		summary.fulfillmentActionCount === "0" &&
		summary.financeActionCount === "0";
	return clean ? (
		<p className="adm-end-dialog__safe">未執行任何出貨或財務操作</p>
	) : (
		<p className="adm-end-dialog__alert" role="alert">
			偵測到出貨或財務操作（{summary.fulfillmentActionCount} /{" "}
			{summary.financeActionCount}）。代行不具備這些能力，請立即回報平台工程。
		</p>
	);
}

function Row({ label, value }: { label: string; value: string }) {
	return (
		<div className="flex items-center justify-between gap-4">
			<dt className="text-foreground-soft">{label}</dt>
			<dd className="font-medium text-foreground">{value}</dd>
		</div>
	);
}
