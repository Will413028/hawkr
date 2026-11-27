"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Button } from "@/components/ui";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";
import styles from "@/styles/features/procurement-assistant/quote-batch-summary.module.css";
import {
	type BuyerQuoteAcceptanceResponse,
	formatTwdAmount,
} from "./buyer-quote-contract";
import type { RfqBuyerRequestedRange } from "./rfq-price-contract";
import {
	deriveRfqProgress,
	type RfqProgressStep,
} from "./runtime-snapshot-contract";
import type { ProcurementSlots, PublishedRfqPublication } from "./types";
import type { RfqDealTruthCounts } from "./useRfqRuntimeSnapshot";

interface QuoteBatchSummaryProps {
	/**
	 * 稿 R-02 ①的需求來源：品項、數量與配送。
	 *
	 * 這是**唯一**的需求來源。legacy `QuoteBatch` 已從這個面板下架（doc 64 §7：
	 * `quote-batches/preview|create|withdraw` 退場），缺席的欄位一律整列不顯示，
	 * 不從別處反推。
	 */
	slots?: ProcurementSlots;
	/**
	 * 伺服器算出的希望詢價區間。
	 *
	 * ⛔ 讀不到就整列不顯示。買家私有預算不得頂替：v6.0 §V.7 明文「不得自動等同
	 * buyerRequestedRange」，它也不進 seller-facing 投影。
	 */
	buyerRequestedRange?: RfqBuyerRequestedRange | null;
	/** runtime snapshot 裡的發布結果；null＝伺服器沒有發布事實。 */
	publication?: PublishedRfqPublication | null;
	/** 「目前交易真相」的筆數；`null` 欄位代表沒讀到，不是 0。 */
	counts?: RfqDealTruthCounts;
	/** 接受回執。這是 acceptedDealPrice 唯一的來源，null＝本畫面沒有成立證據。 */
	acceptance?: BuyerQuoteAcceptanceResponse | null;
	/** 前往 R-01 的不可變快照檢視。缺席時 CTA 停用而不是消失。 */
	onViewSnapshot?: () => void;
	/** 前往賣家回價比較表（`BuyerQuoteComparison`）。 */
	onCompareQuotes?: () => void;
}

const EMPTY_COUNTS: RfqDealTruthCounts = {
	responseCount: null,
	shortlistCount: null,
	error: null,
};

/** 一列快照事實；`value` 為空字串代表伺服器沒給，整列不渲染。 */
function Fact({ label, value }: { label: string; value: string }) {
	if (!value) return null;
	return (
		<div>
			<dt>{label}</dt>
			<dd>{value}</dd>
		</div>
	);
}

/**
 * 每一步右側的狀態字。
 *
 * `done` 但沒有時間戳時說「已完成」而不是留白：留白會跟「待進行」長得一樣。
 * 反過來也不拿別的步驟的時間頂替——後端目前不產 `CONFIRMED_RFQ` artifact，
 * 第一步就是沒有時間可寫。
 */
function stepMeta(step: RfqProgressStep): string {
	if (step.state === "failed") return "未建立成功";
	if (step.state === "current") return "進行中";
	if (step.state === "pending") return "待進行";
	return step.at ? formatTaipeiDateTime(step.at) : "已完成";
}

/** 訂單列：⛔ `CREATING_ORDER` 一律是「建立中」，不得寫成已成交或已下單（D3）。 */
function orderCellText(acceptance: BuyerQuoteAcceptanceResponse): string {
	if (acceptance.orderProvisioningState === "ORDER_CREATED")
		return acceptance.orderNo;
	if (acceptance.orderProvisioningState === "ORDER_FAILED") return "未建立成功";
	return "建立中";
}

/**
 * 詢價已建立／已發布視圖（稿 R-02 三區版面，issue #2538 A8 後半）。
 *
 * 三區的分工就是這個畫面存在的理由（v6.0 §V.11.4 Transaction truth）：
 *   ① 詢價快照——已經固定下來、賣家會看到的那份需求。
 *   ② 流程進度——六步驟各自的伺服器事實與時間戳。
 *   ③ 目前交易真相——**還沒成立的東西也要講出來**。
 *
 * ⛔ 三條不得違反的規則：
 *   1. 沒有回價就顯示 0 筆、成交價格「尚未成立」、訂單「尚未建立」。不得為了
 *      畫面好看隱藏空狀態，也不得換成樂觀文案——這張卡是 §V.11.4「Delivery
 *      truth」的視覺落地，藏掉空狀態等於把它拆掉。
 *   2. `orderProvisioningState = CREATING_ORDER` **不得**渲染成「已成交」或
 *      「已下單」。接受回執刻意不帶 `orderNo`，付款成功前不得宣稱正式成立。
 *   3. 讀不到的事實顯示「尚未取得」，**不是** 0。0 是一個結論，null 不是。
 *
 * 資料由呼叫端注入（同 `BuyerQuoteComparison` 的 props-only 慣例），資料層在
 * `useRfqRuntimeSnapshot`；這裡保持可單獨掛載、可用固定 fixture 測試。
 *
 * ⛔ 這個面板**不含撤回入口**。gateway 唯一的買家端撤回是 legacy 的
 * `quote-batches/{id}/withdraw`（doc 64 §7 明列退場），publication 側沒有對應的
 * 撤回端點。少一顆 CTA 是刻意的：畫一顆停用的按鈕等於宣稱這個功能還在。
 */
export default function QuoteBatchSummary({
	slots,
	buyerRequestedRange = null,
	publication = null,
	counts = EMPTY_COUNTS,
	acceptance = null,
	onViewSnapshot,
	onCompareQuotes,
}: QuoteBatchSummaryProps) {
	const steps = React.useMemo(
		() =>
			deriveRfqProgress({
				publication,
				responseCount: counts.responseCount,
				acceptance,
			}),
		[publication, counts.responseCount, acceptance],
	);

	const itemText = slots?.itemResolution?.displayValue || slots?.item || "";
	const quantityText = [
		slots?.quantity === undefined ? "" : String(slots.quantity),
		slots?.unit ?? "",
	]
		.filter(Boolean)
		.join(" ");
	const deliveryText = [
		slots?.deliveryCity ?? "",
		slots?.deliveryDate ? formatTaipeiDate(slots.deliveryDate) : "",
	]
		.filter(Boolean)
		.join("・");
	const rangeText = buyerRequestedRange
		? `${formatTwdAmount(buyerRequestedRange.lowerBoundAmount)}–${formatTwdAmount(
				buyerRequestedRange.upperBoundAmount,
			)} / ${buyerRequestedRange.basisQuantity} ${buyerRequestedRange.basisUnit}`
		: "";
	const inviteHeadline = publication
		? `已建立 1 組詢價・${publication.recipientCount} 筆邀請${
				quantityText ? `（${quantityText}）` : ""
			}`
		: "";
	const canCompare = Boolean(
		onCompareQuotes && counts.responseCount && counts.responseCount > 0,
	);

	return (
		<section
			className={styles.panel}
			aria-labelledby="quote-summary-title"
			data-testid="quote-batch-summary"
		>
			<header className={styles.head}>
				<h2 id="quote-summary-title">
					{publication ? "詢價已發布" : "詢價建立摘要"}
				</h2>

				{inviteHeadline ? <p>{inviteHeadline}</p> : null}
				<p className="assistant-quote-summary__delivery">
					通知已排入處理；不代表通知或聯絡已送達。
				</p>
				{publication ? null : (
					<p className="assistant-quote-summary__delivery">
						後端未提供不可變核准證據；此畫面不代表核准已完成。
					</p>
				)}
			</header>

			<div className={styles.columns}>
				<div className={styles.left}>
					<section
						className={styles.card}
						aria-labelledby="quote-summary-snapshot"
					>
						<h3 id="quote-summary-snapshot">詢價快照</h3>
						<dl className={styles.facts}>

							{publication ? (
								<div>
									<dt>結果版本</dt>
									<dd>
										{`v${publication.version}`}
										<span className={styles.immutable}>{"・不可變"}</span>
									</dd>
								</div>
							) : null}
							<Fact label="品項" value={itemText} />
							<Fact label="數量" value={quantityText} />
							<Fact label="配送" value={deliveryText} />
							<Fact label="希望詢價" value={rangeText} />

						</dl>
						<p className={styles.note}>
							<AppIcon name="info" size={17} aria-hidden="true" />
							<span>
								{
									"供應商回價時須提交實際品相、驗收條件、計價單位、商品／運費／稅額／總額與有效期限。"
								}
							</span>
						</p>
					</section>


					{publication ? (
						<section
							className={styles.card}
							aria-labelledby="quote-summary-invites"
						>
							<h3 id="quote-summary-invites">這次發布的收件人</h3>
							<dl className={styles.facts}>
								<Fact
									label="收件人"
									value={String(publication.recipientCount)}
								/>
								<Fact
									label="已排入處理"
									value={String(publication.queuedCount)}
								/>
								<Fact
									label="未排入處理"
									value={String(
										Math.max(
											0,
											publication.recipientCount - publication.queuedCount,
										),
									)}
								/>
							</dl>
							<p className="assistant-quote-summary__delivery">
								這三個數字是發布當下的收件人分布，不是投遞結果。
							</p>
						</section>
					) : null}

					<section
						className={styles.card}
						aria-labelledby="quote-summary-progress"
					>
						<h3 id="quote-summary-progress">流程進度</h3>
						<ol className={styles.steps}>
							{steps.map((step, index) => (
								<li
									key={step.key}
									className={styles.step}
									data-state={step.state}
									data-step={step.key}
									aria-current={step.state === "current" ? "step" : undefined}
								>
									<span className={styles.marker} aria-hidden="true">
										{index + 1}
									</span>
									<span className={styles.stepLabel}>{step.label}</span>
									<span className={styles.stepMeta}>{stepMeta(step)}</span>
								</li>
							))}
						</ol>
					</section>
				</div>

				<section className={styles.truth} aria-labelledby="quote-summary-truth">
					<h3 id="quote-summary-truth">目前交易真相</h3>
					<dl className={styles.truthList}>
						<div className={styles.truthRow}>
							<dt>已收到正式回價</dt>
							<dd data-empty={counts.responseCount === null ? "" : undefined}>
								{counts.responseCount === null
									? "尚未取得"
									: `${counts.responseCount} 筆`}
							</dd>
						</div>
						<div className={styles.truthRow}>
							<dt>shortlist</dt>
							<dd data-empty={counts.shortlistCount === null ? "" : undefined}>
								{counts.shortlistCount === null
									? "尚未取得"
									: `${counts.shortlistCount}`}
							</dd>
						</div>

						<div className={styles.truthRow}>
							<dt>成交價格</dt>
							<dd data-empty={acceptance ? undefined : ""}>
								{acceptance
									? formatTwdAmount(acceptance.acceptedDealPrice)
									: "尚未成立"}
							</dd>
						</div>
						<div className={styles.truthRow}>
							<dt>訂單</dt>
							<dd
								data-empty={
									acceptance &&
									acceptance.orderProvisioningState === "ORDER_CREATED"
										? undefined
										: ""
								}
							>
								{acceptance ? orderCellText(acceptance) : "尚未建立"}
							</dd>
						</div>
					</dl>
					<p className={styles.truthFootnote}>
						{
							"買家希望價格不是成交價格。只有買家接受指定 sellerResponseVersion 並收到伺服器回執後才形成 acceptedDealPrice；付款成功前不得視為正式成交。"
						}
					</p>
					<div className={styles.actions}>
						<Button
							fullWidth
							disabled={!onViewSnapshot || !publication}
							onClick={() => onViewSnapshot?.()}
							startContent={
								<AppIcon name="lock" size={17} aria-hidden="true" />
							}
						>
							檢視不可變 RFQ 快照
						</Button>
						<Button
							fullWidth
							variant="bordered"
							color="default"
							disabled={!canCompare}
							onClick={() => onCompareQuotes?.()}
						>
							比較供應商回價
						</Button>
					</div>
					{counts.error ? (
						<p className={styles.actionNote}>{counts.error.detail}</p>
					) : counts.responseCount === 0 ? (
						<p className={styles.actionNote}>
							{"目前沒有可比較的正式回價，收到之後這顆按鈕才會開啟。"}
						</p>
					) : null}
				</section>
			</div>
		</section>
	);
}
