import { getEnv } from "@/lib/config/env";

/**
 * 代行連結失效／無法開始代行（§4.5 失敗行為表）。
 *
 * `/adm/enter` 兌換失敗時導到這裡。⚠️ **不得自動重試**：交接碼是單次的，重試
 * 必然再失敗。⚠️ **不得**把 gateway 的 message 顯示出來（message 不透傳）。
 *
 * 未知／已兌換／逾時／工作階段已不可用四種成因在後端共用同一個碼（9282），
 * 那是刻意的 —— 分開等於給攻擊者一支「這把 code 存在嗎」的探測器。因此這個
 * 畫面也只分「連結失效」與「服務暫時無法使用」兩種。
 */

const COPY: Record<string, { title: string; body: string }> = {
	handoff_invalid: {
		title: "此代行連結已失效",
		body: "代行連結只能使用一次，且有效期很短。請回到 ADM 商家詳情重新建立代行工作階段。",
	},
	upstream: {
		title: "目前無法開始代行",
		body: "代行營運服務暫時無法使用。請回到 ADM 商家詳情稍後重試；這把連結已經作廢，不要重新整理本頁。",
	},
};

export default async function DelegatedUnavailablePage({
	searchParams,
}: {
	searchParams: Promise<{ reason?: string }>;
}) {
	const { reason } = await searchParams;
	const copy = COPY[reason ?? ""] ?? COPY.handoff_invalid;
	const admOrigin = getEnv().admOrigin;

	return (
		<div className="adm-blocked">
			<div className="adm-blocked__card" role="alert">
				<h1 className="adm-blocked__title">{copy.title}</h1>
				<p className="adm-blocked__body">{copy.body}</p>
				{/* `data-hkui-button`：實心底的白字會被 dark 的
				    `body a:not([data-hkui-button])` 換成 `--color-ink-200`（見 A4 不變式）。 */}
				<a
					data-hkui-button=""
					className="adm-blocked__link"
					href={`${admOrigin}/admin/merchants`}
				>
					← 返回 ADM
				</a>
			</div>
		</div>
	);
}
