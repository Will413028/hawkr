"use client";

import AppIcon from "@/components/shared/AppIcon";
import {
	RFQ_TEMPERATURE_BAND_LABEL,
	RFQ_TEMPERATURE_UNKNOWN_COPY,
	RFQ_TEMPERATURE_UNKNOWN_DETAIL,
	RFQ_TEMPERATURE_UNKNOWN_FALLBACK,
} from "./rfq-guidance";
import type { RfqPricePreview } from "./rfq-price-contract";

// bps → 整數百分點。server 保證是 100 的倍數，這裡只做除法不做四捨五入的決策
// ——前端不得自行改變 server 給的精度。
function toPercent(bps: string): number {
	return Number(bps) / 100;
}

/**
 * 成交溫度計。兩個機率都是「市場行情相對買家希望區間的位置」估計，不是成交或
 * 付款機率——文案與計算都不得暗示後者。band 完全由 server 決定，這裡只負責顯示
 * server 給的 band，不自行由 bps 推導冷熱。
 *
 * **畫面上只有一個數字**：`probabilityUpperBps`，等價於「你的出價上限落在行情
 * 分布的第幾百分位」。`probabilityLowerBps`（市場價落在你的區間內）照收但不顯示
 * ——它對買家是**反指標**：出價 70–90、行情 52–74 時它只有 12%，而那正是買家最
 * 有利的情況。留在 wire 上是因為它會成為 `outcomeDefinitionVersion = 2` 的
 * feature（ADR 2026-09-09 的 Amendment，2026-09-10）。
 *
 * 稿 P-01 的量表軸**固定是 0%–100%**，刻度位置直接取 server 的寬鬆界。
 * 顏色掛在容器的 `data-band` 上、由 CSS 對應三個 token，因此「顏色跟著機率
 * 改變」的路徑是：買家調區間 → server 重算機率與 band → 新的 `data-band`。
 * ⛔ 軌道**不得**畫成 0/20/50/100 的漸層：那等於把 `BANDS_V1` 的門檻抄一份
 * 進前端，server 改 `bandVersion` 之後畫面會與它回的 band 靜默牴觸
 * （ADR 2026-09-09-deal-temperature-v1-coverage-not-outcome 的 invariant）。
 *
 * `preview` 可以是 null——買家還沒觸發過一次價格試算時就是這個狀態，這裡與
 * 舊版硬編區塊一樣一律渲染（不是等 preview 出現才顯示），只是改成資料驅動：
 * 沒有 preview 時用跟 UNKNOWN／缺 reasonCode 時相同的「未知」文案 fallback，
 * 不編造任何 reasonCode。
 */
export function DealTemperature({
	preview,
}: {
	preview: RfqPricePreview | null;
}) {
	const temp = preview?.dealTemperature ?? null;
	if (!temp || temp.status !== "AVAILABLE") {
		const detail = temp
			? (RFQ_TEMPERATURE_UNKNOWN_DETAIL[temp.reasonCode] ??
				RFQ_TEMPERATURE_UNKNOWN_FALLBACK)
			: RFQ_TEMPERATURE_UNKNOWN_FALLBACK;
		return (
			<div className="assistant-rfq-guidance__temperature">
				<div className="assistant-rfq-guidance__temperature-label">
					<span>
						<AppIcon name="activity" size={20} aria-hidden="true" />
					</span>
					<div>
						<strong>{RFQ_TEMPERATURE_UNKNOWN_COPY}</strong>
					</div>
				</div>
				<p>{detail}</p>
			</div>
		);
	}
	// 只讀寬鬆界。lower 刻意不解析——沒有任何顯示路徑用得到它。
	const position = toPercent(temp.probabilityUpperBps);
	return (
		<div
			className="assistant-rfq-guidance__temperature"
			data-band={temp.band}
			data-status="available"
		>
			<div className="assistant-rfq-guidance__temperature-head">
				<div className="assistant-rfq-guidance__temperature-label">
					<span>
						<AppIcon name="activity" size={20} aria-hidden="true" />
					</span>
					<div>
						<small>成交溫度計</small>
						<strong>{`${position}%`}</strong>
						<em>{`出價位置：${RFQ_TEMPERATURE_BAND_LABEL[temp.band]}`}</em>
					</div>
				</div>
				<p>
					{`行情有 ${position}% 落在你的出價上限之下。這是行情位置，不是成交或付款機率。`}
				</p>
			</div>
			{/* 量表是上面兩個數字的視覺重述，讀屏器讀 strong 與下面兩條說明就夠，
			    因此整塊 aria-hidden，避免把同一組數字唸三遍。 */}
			<div
				className="assistant-rfq-guidance__temperature-gauge"
				aria-hidden="true"
			>
				<span className="assistant-rfq-guidance__temperature-axis">0%</span>
				<span className="assistant-rfq-guidance__temperature-track">
					{/* 0 → 刻度的填滿：讀作「行情有這麼多落在你的上限之下」。 */}
					<span
						className="assistant-rfq-guidance__temperature-fill"
						style={{ left: "0%", width: `${position}%` }}
					/>
					<span
						className="assistant-rfq-guidance__temperature-point"
						style={{ left: `${position}%` }}
					/>
				</span>
				<span className="assistant-rfq-guidance__temperature-axis">100%</span>
			</div>
		</div>
	);
}
