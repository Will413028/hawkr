"use client";

import * as React from "react";
import styles from "@/styles/features/procurement-rfq-demo/rfq-demo.module.css";
import {
	confirmRfqSnapshot,
	DemoRequirementIncompleteError,
	loadDemoContext,
	runPricePreview,
} from "./load";
import type {
	DemoBuyerOffer,
	DemoContext,
	DemoSellerResponse,
	DemoStep,
	SellerDecision,
} from "./types";

const STEPS: ReadonlyArray<{ slug: DemoStep; label: string; actor: string }> = [
	{ slug: "buyer-offer", label: "買家出價", actor: "買家" },
	{ slug: "seller-inbox", label: "賣家決定", actor: "賣家" },
	{ slug: "buyer-confirm", label: "買家確認", actor: "買家" },
];

function money(value: number): string {
	return `NT$ ${value.toLocaleString("zh-TW")}`;
}

/** 基準是「每 basisQuantity basisUnit」；GRAM 是每 1000 公克，不能省略。 */
function basisLabel(quantity: string, unit: string): string {
	return quantity === "1" ? `每 1 ${unit}` : `每 ${quantity} ${unit}`;
}

const CANDIDATE_REASON_COPY: Record<string, string> = {
	NO_CANDIDATE_SEARCH_RUN: "這份需求還沒跑過候選搜尋，所以沒有收件名單。",
	NO_SELECTED_CANDIDATE: "候選搜尋跑過了，但買家一家都還沒勾選。",
	CANDIDATE_FETCH_FAILED: "讀不到那一輪候選搜尋的結果。",
};

function Field({ label, value }: { label: string; value: string | null }) {
	return (
		<div className={styles.field}>
			<dt>{label}</dt>
			<dd className={value ? undefined : styles.fieldEmpty}>
				{value ?? "未填"}
			</dd>
		</div>
	);
}

export default function BuyerOfferDemo({ requestId }: { requestId: string }) {
	const [context, setContext] = React.useState<DemoContext | null>(null);
	const [loadError, setLoadError] = React.useState<string | null>(null);
	const [step, setStep] = React.useState<DemoStep>("buyer-offer");

	const [idealPrice, setIdealPrice] = React.useState("");
	const [minPrice, setMinPrice] = React.useState("");
	const [maxPrice, setMaxPrice] = React.useState("");

	const [submitting, setSubmitting] = React.useState(false);
	const [submitError, setSubmitError] = React.useState<string | null>(null);
	const [offer, setOffer] = React.useState<DemoBuyerOffer | null>(null);
	const [sellerResponse, setSellerResponse] =
		React.useState<DemoSellerResponse | null>(null);
	const [settled, setSettled] = React.useState(false);

	React.useEffect(() => {
		let cancelled = false;
		void (async () => {
			try {
				const loaded = await loadDemoContext(requestId);
				if (cancelled) return;
				setContext(loaded);
				if (loaded.requirement.budgetPerUnit !== null) {
					setIdealPrice(String(loaded.requirement.budgetPerUnit));
				} else if (loaded.market) {
					setIdealPrice(String(loaded.market.midpoint));
				}
				if (loaded.market) {
					setMinPrice(String(loaded.market.lower));
					setMaxPrice(String(loaded.market.upper));
				}
			} catch (err) {
				if (cancelled) return;
				setLoadError(
					err instanceof DemoRequirementIncompleteError
						? `這份需求還缺 ${err.missing.join("、")}，請先回對話補齊再回來。`
						: "載入不到這份需求。確認 request_id 是這個帳號的，且對話還在。",
				);
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [requestId]);

	const priceError = React.useMemo(() => {
		const ideal = Number(idealPrice);
		const min = Number(minPrice);
		const max = Number(maxPrice);
		if (!idealPrice || !minPrice || !maxPrice) return "三個價格都要填。";
		if (!Number.isInteger(ideal) || ideal <= 0) return "理想價要是正整數。";
		if (!Number.isInteger(min) || min <= 0) return "可接受下限要是正整數。";
		if (!Number.isInteger(max) || max <= 0) return "可接受上限要是正整數。";
		if (min > max) return "下限不能高於上限。";
		if (ideal < min || ideal > max) return "理想價要落在可接受區間內。";
		return null;
	}, [idealPrice, minPrice, maxPrice]);

	if (loadError) {
		return (
			<main className={styles.shell}>
				<p className={styles.error}>{loadError}</p>
			</main>
		);
	}
	if (!context) {
		return (
			<main className={styles.shell}>
				<p className={styles.loading}>載入需求、收件名單與市場行情…</p>
			</main>
		);
	}

	const {
		requirement,
		candidates,
		candidatesUnavailableReason,
		market,
		quoteTotalRange,
		marketUnavailableReason,
	} = context;
	const quantityLabel =
		requirement.quantity !== null && requirement.unit
			? `${requirement.quantity} ${requirement.unit}`
			: null;

	/**
	 * 定稿：用買家填的區間重跑一次試算（伺服器會原樣保留買家金額、只補基準），
	 * 再對那一份試算做 confirm。confirm 通過才算真的定稿——它是伺服器側的 CAS，
	 * 不是前端說了算。
	 */
	async function submitOffer() {
		if (priceError || submitting) return;
		setSubmitting(true);
		setSubmitError(null);
		try {
			const outcome = await runPricePreview(requirement, {
				lower: Number(minPrice),
				upper: Number(maxPrice),
			});
			if (!outcome.preview) {
				setSubmitError(
					`伺服器沒有回一份可確認的試算（${outcome.reason ?? "UNKNOWN"}）。`,
				);
				return;
			}
			const confirmed = await confirmRfqSnapshot(requirement, outcome.preview);
			setContext((prev) =>
				prev
					? {
							...prev,
							market: outcome.market,
							quoteTotalRange: outcome.quoteTotalRange,
							preview: outcome.preview,
							marketUnavailableReason: outcome.reason,
						}
					: prev,
			);
			setOffer({
				idealUnitPrice: Number(idealPrice),
				minUnitPrice: Number(minPrice),
				maxUnitPrice: Number(maxPrice),
				confirmedSnapshotId: confirmed.confirmedSnapshotId,
				snapshotPayloadHash: confirmed.payloadHash,
				confirmedAt: new Date().toISOString(),
			});
			setSellerResponse(null);
			setStep("seller-inbox");
		} catch {
			setSubmitError(
				"定稿失敗。多半是這份需求版本已經變了，或這次試算過期了（15 分鐘）。重新整理再試。",
			);
		} finally {
			setSubmitting(false);
		}
	}

	function respond(decision: SellerDecision, note: string) {
		setSellerResponse({
			decision,
			counterUnitPrice:
				decision === "disputed" && offer
					? Math.round(offer.maxUnitPrice * 1.08)
					: null,
			note,
			respondedAt: new Date().toISOString(),
		});
		setStep("buyer-confirm");
	}

	return (
		<main className={styles.shell}>
			<header className={styles.head}>
				<div>
					<p className={styles.eyebrow}>Hawkr Agent · 商務線 demo</p>
					<h1>買家出價 → 賣家決定 → 買家確認</h1>
				</div>
				<p className={styles.flag}>
					買家那半全部是真的：需求、收件名單、Pricebook 行情，出價也真的凍結成
					confirmed snapshot。賣家那半是本機狀態，不會送出、不會通知任何人。
				</p>
			</header>

			<ol className={styles.steps}>
				{STEPS.map((entry, index) => {
					const current = entry.slug === step;
					const done = STEPS.findIndex((s) => s.slug === step) > index;
					return (
						<li
							key={entry.slug}
							className={current ? styles.stepCurrent : undefined}
							data-done={done ? "true" : undefined}
						>
							<span className={styles.stepIndex}>{index + 1}</span>
							<span>
								<strong>{entry.label}</strong>
								<small>{entry.actor}</small>
							</span>
						</li>
					);
				})}
			</ol>

			<section className={styles.panel}>
				<h2>這次的需求</h2>
				<p className={styles.panelNote}>
					以下六欄全部來自 request {requirement.requestId} 的 snapshot，demo
					不補值。
				</p>
				<dl className={styles.fields}>
					<Field label="品項" value={requirement.item} />
					<Field label="數量" value={quantityLabel} />
					<Field label="包裝" value={requirement.packaging} />
					<Field label="配送城市" value={requirement.deliveryCity} />
					<Field label="到貨日期" value={requirement.deliveryDate} />
					<Field
						label="釐清輪填的單價預算"
						value={
							requirement.budgetPerUnit !== null
								? money(requirement.budgetPerUnit)
								: null
						}
					/>
				</dl>
			</section>

			<section className={styles.panel}>
				<h2>收件名單</h2>
				{candidates.length > 0 ? (
					<>
						<p className={styles.panelNote}>
							買家已勾選的 {candidates.length}{" "}
							家，取自最新一輪候選搜尋。候選不是供貨承諾。
						</p>
						<ul className={styles.recipients}>
							{candidates.map((candidate) => (
								<li key={candidate.candidateId}>
									<strong>
										{candidate.sellerName ?? candidate.displayName}
									</strong>
									<span>
										{candidate.lane === "PLATFORM" ? "平台賣家" : "外部業者"}
									</span>
									<small>
										符合 {candidate.matchedFactCount} 項 · 待確認{" "}
										{candidate.unverifiableFactCount} 項
									</small>
								</li>
							))}
						</ul>
					</>
				) : (
					<p className={styles.panelNote}>
						{CANDIDATE_REASON_COPY[candidatesUnavailableReason ?? ""] ??
							"目前沒有收件名單。"}
					</p>
				)}
			</section>

			<section className={styles.panel}>
				<h2>市場行情</h2>
				{market ? (
					<>
						<p className={styles.marketRange}>
							{money(market.lower)} – {money(market.upper)}
							<span className={styles.marketMid}>
								中位 {money(market.midpoint)}
							</span>
						</p>
						<p className={styles.panelNote}>
							{market.regionName} ·{" "}
							{basisLabel(market.basisQuantity, market.basisUnit)} · 資料截至{" "}
							{market.sourceDataAsOf ?? "未提供"} · 新鮮度{" "}
							{market.freshnessStatus}
						</p>
						<p className={styles.panelNote}>
							這是區域行情，不屬於任何一家賣家，也不是報價。
						</p>
					</>
				) : (
					<p className={styles.panelNote}>
						這個品項與城市目前沒有可用行情（
						{marketUnavailableReason ?? "UNAVAILABLE"}）。買家仍可自行出價，demo
						不代為推估數字。
					</p>
				)}
			</section>

			{quoteTotalRange ? (
				<section className={styles.panel}>
					<h2>詢價總額區間</h2>
					<p className={styles.marketRange}>
						{money(quoteTotalRange.lower)} – {money(quoteTotalRange.upper)}
					</p>
					<p className={styles.panelNote}>
						{quoteTotalRange.quantity} {quoteTotalRange.quantityUnit} ×
						本次單價範圍，由伺服器計算。前端不自己乘——單價的基準是「每 N
						單位」，GRAM 品項自己乘會差 1000 倍。
					</p>
				</section>
			) : null}

			{step === "buyer-offer" ? (
				<section className={styles.panel}>
					<h2>買家出價</h2>
					<p className={styles.panelNote}>
						理想價是你希望成交的單價；可接受區間是你願意讓步的範圍。定稿時區間會送進伺服器並被凍結進
						confirmed snapshot。
					</p>
					<div className={styles.priceGrid}>
						<label>
							<span>可接受下限</span>
							<input
								inputMode="numeric"
								value={minPrice}
								onChange={(event) => setMinPrice(event.target.value.trim())}
							/>
						</label>
						<label>
							<span>理想價</span>
							<input
								inputMode="numeric"
								value={idealPrice}
								onChange={(event) => setIdealPrice(event.target.value.trim())}
							/>
						</label>
						<label>
							<span>可接受上限</span>
							<input
								inputMode="numeric"
								value={maxPrice}
								onChange={(event) => setMaxPrice(event.target.value.trim())}
							/>
						</label>
					</div>
					<p className={styles.boundary}>
						品質期待這一欄本 demo 不提供。後端的 qualityKnowledgeState
						目前在三個生產檔案裡寫死成 BUYER_QUALITY_UNKNOWN，全 repo
						沒有第二個值也沒有買家輸入來源——放一個選了不會生效的選單比不放更誤導。
					</p>
					{priceError ? <p className={styles.error}>{priceError}</p> : null}
					{submitError ? <p className={styles.error}>{submitError}</p> : null}
					<button
						type="button"
						className={styles.demoCta}
						disabled={priceError !== null || submitting}
						onClick={() => void submitOffer()}
					>
						{submitting ? "定稿中…" : "定稿並送給賣家"}
					</button>
					<p className={styles.boundary}>
						按下去會真的呼叫 rfq-confirmations，在伺服器上凍結一份 confirmed
						snapshot。它只鎖定你自己的條件，不會通知任何賣家——發邀請是
						quote-batches 的事，本 demo 不呼叫。
					</p>
				</section>
			) : null}

			{step === "seller-inbox" && offer ? (
				<section className={styles.panel}>
					<h2>賣家收件匣</h2>
					<p className={styles.panelNote}>
						賣家看到的是買家已定稿的條件，詢價單本身唯讀。
					</p>
					<dl className={styles.fields}>
						<Field label="品項" value={requirement.item} />
						<Field label="數量" value={quantityLabel} />
						<Field label="理想價" value={money(offer.idealUnitPrice)} />
						<Field
							label="可接受區間"
							value={`${money(offer.minUnitPrice)} – ${money(offer.maxUnitPrice)}`}
						/>
						<Field label="到貨日期" value={requirement.deliveryDate} />
						<Field label="條件版本" value={offer.confirmedSnapshotId} />
					</dl>
					<p className={styles.panelNote}>
						payload hash {offer.snapshotPayloadHash.slice(0, 16)}… ——
						賣家報價時要對它做 CAS，證明看到的是這一版。
					</p>
					<div className={styles.sellerActions}>
						<button
							type="button"
							className={styles.demoCta}
							onClick={() =>
								respond("accepted", "接受買家的理想價與交貨條件。")
							}
						>
							接受這個價格
						</button>
						<button
							type="button"
							className={styles.demoCtaMuted}
							onClick={() =>
								respond("disputed", "數量可承接，但單價需要上調。")
							}
						>
							提出異議
						</button>
						<button
							type="button"
							className={styles.demoCtaMuted}
							onClick={() => respond("declined", "這個交期無法配合。")}
						>
							婉拒
						</button>
					</div>
				</section>
			) : null}

			{step === "buyer-confirm" && offer && sellerResponse ? (
				<section className={styles.panel}>
					<h2>買家確認</h2>
					{sellerResponse.decision === "accepted" ? (
						<p className={styles.accepted}>
							賣家接受了 {money(offer.idealUnitPrice)} 的理想價。
						</p>
					) : null}
					{sellerResponse.decision === "disputed" &&
					sellerResponse.counterUnitPrice !== null ? (
						<p className={styles.disputed}>
							賣家提出異議，替代單價 {money(sellerResponse.counterUnitPrice)}
							，高於你的可接受上限。
						</p>
					) : null}
					{sellerResponse.decision === "declined" ? (
						<p className={styles.declined}>賣家婉拒了這張詢價單。</p>
					) : null}
					<p className={styles.panelNote}>{sellerResponse.note}</p>

					{settled ? (
						<p className={styles.boundary}>
							demo 到此為止。真實流程接下來會原子寫入 acceptance ＋ 成交價 ＋
							outbox，再由 worker 投遞冪等建單命令給 order-service
							建立唯一一張待付款訂單——那一段還沒實作（#2538），而且訂單怎麼落到
							order_items 仍待拍板。
						</p>
					) : (
						<button
							type="button"
							className={styles.demoCta}
							disabled={sellerResponse.decision !== "accepted"}
							onClick={() => setSettled(true)}
						>
							{sellerResponse.decision === "accepted"
								? "確認成交"
								: "賣家未接受，無法確認"}
						</button>
					)}

					<button
						type="button"
						className={styles.demoCtaLink}
						onClick={() => {
							setOffer(null);
							setSellerResponse(null);
							setSettled(false);
							setStep("buyer-offer");
						}}
					>
						重新出價
					</button>
				</section>
			) : null}
		</main>
	);
}
