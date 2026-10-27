"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Button, Dialog } from "@/components/ui";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import styles from "@/styles/features/procurement-assistant/buyer-quote-compare.module.css";
import {
	type BuyerFacingSellerResponse,
	type BuyerQuoteAcceptanceCommand,
	type BuyerQuoteAcceptanceResponse,
	type BuyerQuoteFailure,
	type BuyerQuoteResult,
	formatTwdAmount,
	hasComparableUnitBasis,
	isAcceptableQuote,
	QUOTE_ACCEPTED_HEADLINE,
	type QuoteShortlistTarget,
	quoteAcceptabilityLabel,
	SELLER_QUOTE_RESPONSE_TYPE_LABEL,
	shortVersionLabel,
	TAX_INCLUSION_BASIS_LABEL,
} from "./buyer-quote-contract";

/**
 * 買家端「報價比較與接受」（issue #2538 A8 前半，稿 B-02）。
 *
 * 這支元件只做四件事：列出每個 immutable 版本並可橫向比較、呈現單一報價的
 * 明細、提供 exact-version 的接受 CTA 與確認流程、呈現接受之後的狀態。
 *
 * ⛔ 刻意**不**做的事，每一條都是契約而不是待辦：
 *   - 不自行判斷可否接受（不比 `validUntil`、不比版本號大小、不看 `status`）。
 *   - 不算、也不顯示任何「每單位價」——各家 `unitBasis` 可以不同，壓成一個數字
 *     就是把「不可直接比較」講成「可以直接比較」。
 *   - 不把不可接受的版本從清單移除：它們是稽核軌跡的一部分，只是按不下去。
 *   - 不在 acceptance 的 body 帶任何金額。
 *   - 不宣稱「已成交」「已下單」——ACK 不帶 `orderNo`，`CREATING_ORDER` 是買家
 *     看得到的中間狀態。
 *
 * props 走 `on*` 命令回呼（與 `RfqDraftFlow` 同型）而不是自己拿 hook：資料層在
 * `useBuyerQuoteComparison`，這裡保持可單獨掛載、可注入假回呼測試。
 */

export interface BuyerQuoteComparisonProps {
	responses: readonly BuyerFacingSellerResponse[];
	/**
	 * 已確認快照本身的 hash（`RfqConfirmation.payloadHash`），接受時原樣回送做
	 * CAS。空字串＝這個畫面沒有可用的比對權杖，所有接受一律 fail closed。
	 */
	rfqSnapshotHash: string;
	onToggleShortlist: (
		target: QuoteShortlistTarget,
		shortlisted: boolean,
	) => Promise<BuyerQuoteResult<boolean>>;
	onAcceptQuote: (
		command: BuyerQuoteAcceptanceCommand,
	) => Promise<BuyerQuoteResult<BuyerQuoteAcceptanceResponse>>;
	/** 一次送出動作一把；重試沿用同一把是本元件的責任。 */
	createIdempotencyKey: () => string;
	/** 報價已變動、快照已過期這類錯誤要能讓買家重新載入清單。 */
	onRefresh?: () => void;
}

type AcceptFlow =
	| { status: "idle" }
	| { status: "confirming"; response: BuyerFacingSellerResponse }
	| { status: "submitting"; response: BuyerFacingSellerResponse }
	| { status: "accepted"; ack: BuyerQuoteAcceptanceResponse }
	| {
			status: "failed";
			response: BuyerFacingSellerResponse;
			error: BuyerQuoteFailure;
	  };

const ORDER_PROVISIONING_COPY: Record<
	BuyerQuoteAcceptanceResponse["orderProvisioningState"],
	string
> = {
	// ⛔ 這三句都不得出現「成交」「已下單」。價格凍結 ≠ 訂單成立。
	CREATING_ORDER: "正在建立待付款訂單，完成後這裡會出現訂單編號與付款期限。",
	ORDER_CREATED: "待付款訂單已建立，請在期限前完成付款。",
	ORDER_FAILED:
		"待付款訂單沒有建立成功。成交價格仍然凍結，這筆接受不會消失；請重新整理查看最新狀態。",
};

function responseKey(
	response: Pick<
		BuyerFacingSellerResponse,
		"sellerResponseId" | "sellerResponseVersion"
	>,
): string {
	return `${response.sellerResponseId}:${response.sellerResponseVersion}`;
}

function quoteTarget(
	response: BuyerFacingSellerResponse,
): QuoteShortlistTarget {
	return {
		sellerResponseId: response.sellerResponseId,
		sellerResponseVersion: response.sellerResponseVersion,
	};
}

/** 時間欄位一律走台北固定偏移；空字串代表沒有這個事實，整格不顯示。 */
function taipeiOrDash(raw: string): string {
	const text = formatTaipeiDateTime(raw);
	return text ? `${text}（Asia/Taipei）` : "未提供";
}

function unitBasisText(response: BuyerFacingSellerResponse): string {
	const { unitBasis } = response.quote;
	return `每 ${unitBasis.quantity} ${unitBasis.unit}`;
}

function offeredQuantityText(response: BuyerFacingSellerResponse): string {
	return `${response.quote.offeredQuantity} ${response.quote.offeredQuantityUnit}`;
}

function QuoteDetail({ response }: { response: BuyerFacingSellerResponse }) {
	const { quote, seller } = response;
	return (
		<dl className={styles.detail}>
			<div>
				<dt>回價類型</dt>
				<dd>{SELLER_QUOTE_RESPONSE_TYPE_LABEL[quote.responseType]}</dd>
			</div>
			<div>
				<dt>賣家身分</dt>
				<dd>
					{seller.storeCode
						? `${seller.partyType}｜店鋪代碼 ${seller.storeCode}`
						: seller.partyType}
				</dd>
			</div>
			<div>
				<dt>實際品相</dt>
				<dd>{quote.offeredQuality.grade || "賣家未填"}</dd>
			</div>
			<div>
				<dt>驗貨說明</dt>
				<dd>{quote.offeredQuality.inspectionNotes || "無"}</dd>
			</div>
			<div>
				<dt>計價基準</dt>
				<dd>{unitBasisText(response)}</dd>
			</div>
			<div>
				<dt>供貨量</dt>
				<dd>{offeredQuantityText(response)}</dd>
			</div>
			<div>
				<dt>換算依據</dt>
				{/* 空字串＝賣家沒有給換算依據。⛔ 不要在這裡自己乘除補一個出來。 */}
				<dd>{quote.conversionBasis || "賣家未提供換算依據"}</dd>
			</div>
			<div>
				<dt>貨款</dt>
				<dd>{formatTwdAmount(quote.goodsAmount)}</dd>
			</div>
			<div>
				<dt>運費</dt>
				<dd>{formatTwdAmount(quote.shippingAmount)}</dd>
			</div>
			<div>
				<dt>稅額</dt>
				<dd>{formatTwdAmount(quote.taxAmount)}</dd>
			</div>
			<div>
				<dt>稅務口徑</dt>
				<dd>{TAX_INCLUSION_BASIS_LABEL[quote.taxInclusionBasis]}</dd>
			</div>
			<div>
				<dt>總額</dt>
				<dd>{`${formatTwdAmount(quote.totalAmount)} ${quote.currency}`}</dd>
			</div>
			<div>
				<dt>交期</dt>
				<dd>{`${quote.leadTimeDays} 天`}</dd>
			</div>
			<div>
				<dt>承諾到貨</dt>
				<dd>{taipeiOrDash(quote.promisedDeliveryAt)}</dd>
			</div>
			<div>
				<dt>有效至</dt>
				<dd>{taipeiOrDash(quote.validUntil)}</dd>
			</div>
			<div>
				<dt>條件備註</dt>
				<dd>{quote.termsNote || "無"}</dd>
			</div>
			<div>
				<dt>差異揭露</dt>
				<dd>{quote.deviationDisclosure || "無"}</dd>
			</div>
			<div>
				<dt>版本內容雜湊</dt>
				<dd className={styles.hash}>{response.payloadHash}</dd>
			</div>
		</dl>
	);
}

export default function BuyerQuoteComparison({
	responses,
	rfqSnapshotHash,
	onToggleShortlist,
	onAcceptQuote,
	createIdempotencyKey,
	onRefresh,
}: BuyerQuoteComparisonProps) {
	const [expanded, setExpanded] = React.useState<string | null>(null);
	const [flow, setFlow] = React.useState<AcceptFlow>({ status: "idle" });
	const [shortlistError, setShortlistError] =
		React.useState<BuyerQuoteFailure | null>(null);
	const [pendingShortlist, setPendingShortlist] = React.useState<string | null>(
		null,
	);
	// 同一個版本的重試必須沿用同一把冪等鍵，否則伺服器上會多出第二筆接受。
	// 換一個版本就是換一次送出動作，各自一把。
	const idempotencyKeys = React.useRef(new Map<string, string>());

	const accepted = flow.status === "accepted" ? flow.ack : null;
	// 有一筆接受成立之後這張詢價就鎖住了：伺服器對第二次會回
	// RFQ_CHECKOUT_ALREADY_ACTIVE／RFQ_ALREADY_AWARDED，畫面先誠實反映。
	const locked = accepted !== null;
	const comparable = hasComparableUnitBasis(responses);
	const missingSnapshotHash = rfqSnapshotHash === "";

	const toggleShortlist = React.useCallback(
		async (response: BuyerFacingSellerResponse) => {
			const key = responseKey(response);
			setPendingShortlist(key);
			setShortlistError(null);
			const result = await onToggleShortlist(
				quoteTarget(response),
				!response.shortlisted,
			);
			setPendingShortlist(null);
			if (!result.ok) setShortlistError(result.error);
		},
		[onToggleShortlist],
	);

	const submitAccept = React.useCallback(
		async (response: BuyerFacingSellerResponse) => {
			const key = responseKey(response);
			const idempotencyKey =
				idempotencyKeys.current.get(key) ?? createIdempotencyKey();
			idempotencyKeys.current.set(key, idempotencyKey);
			setFlow({ status: "submitting", response });
			const result = await onAcceptQuote({
				idempotencyKey,
				sellerResponseId: response.sellerResponseId,
				expectedSellerResponseVersion: response.sellerResponseVersion,
				expectedRfqSnapshotHash: rfqSnapshotHash,
			});
			if (result.ok) {
				setFlow({ status: "accepted", ack: result.data });
				return;
			}
			setFlow({ status: "failed", response, error: result.error });
		},
		[createIdempotencyKey, onAcceptQuote, rfqSnapshotHash],
	);

	const confirming =
		flow.status === "confirming" || flow.status === "submitting"
			? flow.response
			: null;

	return (
		<section className={styles.panel} aria-labelledby="buyer-quote-compare">
			<header className={styles.header}>
				<h2 id="buyer-quote-compare">{`收到 ${responses.length} 份正式回價`}</h2>
				<p>
					{
						"每一份都是賣家各自送出的不可變版本。你接受的是指定的那一個版本，不是這家賣家的最新報價。"
					}
				</p>
			</header>

			{accepted ? (
				<div className={styles.accepted} role="status">
					<strong>{QUOTE_ACCEPTED_HEADLINE}</strong>
					<p>{ORDER_PROVISIONING_COPY[accepted.orderProvisioningState]}</p>
					<dl>
						<div>
							<dt>接受的版本</dt>
							<dd>{shortVersionLabel(accepted)}</dd>
						</div>
						<div>
							<dt>凍結的成交價格</dt>
							<dd>
								{`${formatTwdAmount(accepted.acceptedDealPrice)} ${accepted.currency}`}
							</dd>
						</div>
						<div>
							<dt>接受時間</dt>
							<dd>{taipeiOrDash(accepted.acceptedAt)}</dd>
						</div>
						{accepted.orderNo ? (
							<div>
								<dt>訂單編號</dt>
								<dd>{accepted.orderNo}</dd>
							</div>
						) : null}
						{accepted.paymentDeadline ? (
							<div>
								<dt>付款期限</dt>
								<dd>{taipeiOrDash(accepted.paymentDeadline)}</dd>
							</div>
						) : null}
					</dl>
					{accepted.replayed ? (
						<p>{"這次是同一筆接受的重試結果，沒有建立第二筆。"}</p>
					) : null}
					{accepted.orderNo ? (
						<Button href={`/account/orders/${accepted.orderNo}`}>
							前往付款
						</Button>
					) : null}
				</div>
			) : null}

			{missingSnapshotHash ? (
				<div className={styles.notice} data-tone="warning" role="alert">
					<AppIcon name="warning" size={18} aria-hidden="true" />
					<div>
						<strong>目前無法接受任何版本</strong>
						<p>
							{
								"這個畫面沒有取得已確認快照的比對權杖，接受請求一定會被伺服器拒絕。請回到詢價流程重新確認快照。"
							}
						</p>
					</div>
				</div>
			) : null}

			{responses.length > 1 && !comparable ? (
				<div className={styles.notice} data-tone="info">
					<AppIcon name="info" size={18} aria-hidden="true" />
					<div>
						<strong>各家的計價基準不同，單價不可直接相比</strong>
						<p>
							{
								"請先看每一列的計價基準、供貨量單位與換算依據，再判斷金額代表的是不是同一件事。"
							}
						</p>
					</div>
				</div>
			) : null}

			{shortlistError ? (
				<div className={styles.notice} data-tone="warning" role="alert">
					<AppIcon name="warning" size={18} aria-hidden="true" />
					<div>
						<strong>比較清單沒有更新</strong>
						<p>{shortlistError.detail}</p>
					</div>
				</div>
			) : null}

			{flow.status === "failed" ? (
				<div className={styles.notice} data-tone="warning" role="alert">
					<AppIcon name="warning" size={18} aria-hidden="true" />
					<div>
						<strong>沒有接受這個報價版本</strong>
						<p>{flow.error.detail}</p>
						<div className={styles.noticeActions}>
							{flow.error.code === "RFQ_CHECKOUT_ALREADY_ACTIVE" ? (
								<Button
									size="sm"
									href={
										flow.error.orderNo
											? `/account/orders/${flow.error.orderNo}`
											: "/account/orders"
									}
								>
									前往付款
								</Button>
							) : null}
							{onRefresh ? (
								<Button size="sm" variant="bordered" onClick={onRefresh}>
									重新載入報價
								</Button>
							) : null}
						</div>
					</div>
				</div>
			) : null}

			{responses.length === 0 ? (
				<p className={styles.empty}>
					{
						"目前還沒有收到正式回價。已發布不代表賣家已經看到或已經報價，收到之後這裡會列出可以比較的版本。"
					}
				</p>
			) : (
				<div className={styles.tableWrap}>
					<table className={styles.table}>
						<caption className={styles.caption}>
							{"賣家正式回價比較。不可接受的版本仍保留在清單中供查閱。"}
						</caption>
						<thead>
							<tr>
								<th scope="col">賣家與版本</th>
								<th scope="col">計價基準</th>
								<th scope="col">供貨量</th>
								<th scope="col">總額</th>
								<th scope="col">交期</th>
								<th scope="col">有效至</th>
								<th scope="col">可接受性</th>
								<th scope="col">動作</th>
							</tr>
						</thead>
						<tbody>
							{responses.map((response) => {
								const key = responseKey(response);
								const acceptable = isAcceptableQuote(response.acceptability);
								const isExpanded = expanded === key;
								return (
									<React.Fragment key={key}>
										<tr
											data-shortlisted={response.shortlisted || undefined}
											data-acceptable={acceptable || undefined}
										>
											<td data-label="賣家與版本">
												<strong>{response.seller.displayName}</strong>
												<span>
													{`${shortVersionLabel(response)}｜送出於 ${taipeiOrDash(response.submittedAt)}`}
												</span>
												{response.outsideBuyerRequestedRange ? (
													<span className={styles.flag}>
														超出你的希望詢價區間
													</span>
												) : null}
											</td>
											<td data-label="計價基準">{unitBasisText(response)}</td>
											<td data-label="供貨量">
												{offeredQuantityText(response)}
											</td>
											<td data-label="總額">
												<strong>
													{formatTwdAmount(response.quote.totalAmount)}
												</strong>
												<span>
													{
														TAX_INCLUSION_BASIS_LABEL[
															response.quote.taxInclusionBasis
														]
													}
												</span>
											</td>
											<td data-label="交期">{`${response.quote.leadTimeDays} 天`}</td>
											<td data-label="有效至">
												{taipeiOrDash(response.quote.validUntil)}
											</td>
											<td data-label="可接受性">
												<span
													className={styles.acceptability}
													data-acceptable={acceptable || undefined}
												>
													{quoteAcceptabilityLabel(response.acceptability)}
												</span>
											</td>
											<td data-label="動作">
												<div className={styles.rowActions}>
													<Button
														size="sm"
														variant="bordered"
														color="default"
														aria-expanded={isExpanded}
														onClick={() => setExpanded(isExpanded ? null : key)}
													>
														{isExpanded ? "收合明細" : "查看明細"}
													</Button>
													<Button
														size="sm"
														variant={
															response.shortlisted ? "bordered" : "solid"
														}
														color={response.shortlisted ? "default" : "primary"}
														isLoading={pendingShortlist === key}
														loadingLabel="更新中"
														onClick={() => void toggleShortlist(response)}
													>
														{response.shortlisted
															? `將 ${response.seller.displayName} 移出比較`
															: `將 ${response.seller.displayName} 加入比較`}
													</Button>
													<Button
														size="sm"
														disabled={
															!acceptable || locked || missingSnapshotHash
														}
														onClick={() =>
															setFlow({ status: "confirming", response })
														}
													>
														接受這個版本
													</Button>
												</div>
											</td>
										</tr>
										{isExpanded ? (
											<tr className={styles.detailRow}>
												<td colSpan={8}>
													<QuoteDetail response={response} />
												</td>
											</tr>
										) : null}
									</React.Fragment>
								);
							})}
						</tbody>
					</table>
				</div>
			)}

			<Dialog
				open={confirming !== null}
				onClose={() => setFlow({ status: "idle" })}
				title="確認接受這個報價版本"
				size="md"
			>
				{confirming ? (
					<div className={styles.confirm}>
						<p>
							{
								"接受之後成交價格立刻凍結，且只對下面這一個版本生效；賣家之後送出的新版本不會自動取代它。"
							}
						</p>
						<dl>
							<div>
								<dt>賣家</dt>
								<dd>{confirming.seller.displayName}</dd>
							</div>
							<div>
								<dt>版本</dt>
								<dd>{shortVersionLabel(confirming)}</dd>
							</div>
							<div>
								<dt>總額</dt>
								<dd>
									{`${formatTwdAmount(confirming.quote.totalAmount)} ${confirming.quote.currency}`}
								</dd>
							</div>
							<div>
								<dt>計價基準</dt>
								<dd>{unitBasisText(confirming)}</dd>
							</div>
							<div>
								<dt>供貨量</dt>
								<dd>{offeredQuantityText(confirming)}</dd>
							</div>
						</dl>
						<p className={styles.confirmFootnote}>
							{
								"成交價格由伺服器從這一版取，接受請求不會夾帶任何金額。接受不等於付款，訂單要付款成功才成立。"
							}
						</p>
						<div className={styles.confirmActions}>
							<Button
								variant="bordered"
								color="default"
								onClick={() => setFlow({ status: "idle" })}
							>
								再想一下
							</Button>
							<Button
								isLoading={flow.status === "submitting"}
								loadingLabel="送出中"
								onClick={() => void submitAccept(confirming)}
							>
								確認接受這個版本
							</Button>
						</div>
					</div>
				) : null}
			</Dialog>
		</section>
	);
}
