"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { Button, Dialog } from "@/components/ui";
import HawkrAgentMark from "@/features/procurement-assistant/HawkrAgentMark";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import styles from "@/styles/features/procurement-rfq-prototype/rfq-prototype.module.css";
import {
	EXTERNAL_VENDOR_BUYER_OFFER,
	RFQ_DEMO_SUMMARY,
	RFQ_PROTOTYPE_QUOTES,
	RFQ_PROTOTYPE_SCREENS,
	RFQ_PROTOTYPE_SUPPLIERS,
	rfqPrototypeScreenPath,
} from "./data";
import ExternalVendorPortal from "./ExternalVendorPortal";
import type {
	RfqPrototypeQuote,
	RfqPrototypeScreenDefinition,
	RfqPrototypeScreenSlug,
} from "./types";

const ROLE_LABEL = {
	buyer: "買家視角",
	seller: "平台賣家視角",
	vendor: "外部業者視角",
	finance: "財務視角",
} as const;

function money(value: number): string {
	return `NT$${new Intl.NumberFormat("zh-TW", {
		maximumFractionDigits: 0,
	}).format(value)}`;
}

function StageBadge({ screen }: { screen: RfqPrototypeScreenDefinition }) {
	return (
		<span
			className={styles.stageBadge}
			data-wave={screen.wave}
			data-role={screen.role}
		>
			{screen.wave === "wave-1" ? "Wave 1｜正式詢價" : "Later Wave｜目標畫面"}
		</span>
	);
}

function StatusBadge({
	children,
	tone = "neutral",
}: {
	children: React.ReactNode;
	tone?: "neutral" | "success" | "warning" | "danger" | "info";
}) {
	return (
		<span className={styles.statusBadge} data-tone={tone}>
			{children}
		</span>
	);
}

function ScreenNav({ current }: { current: RfqPrototypeScreenSlug }) {
	const waveOne = RFQ_PROTOTYPE_SCREENS.filter(
		(screen) => screen.wave === "wave-1",
	);
	const later = RFQ_PROTOTYPE_SCREENS.filter(
		(screen) => screen.wave === "later",
	);

	return (
		<nav className={styles.screenNav} aria-label="RFQ 原型畫面">
			<div className={styles.navGroup}>
				<strong>先打通正式詢價</strong>
				{waveOne.map((screen, index) => (
					<Link
						key={screen.slug}
						href={rfqPrototypeScreenPath(screen.slug)}
						className={styles.navItem}
						aria-current={current === screen.slug ? "page" : undefined}
					>
						<span>{index + 1}</span>
						{screen.label}
					</Link>
				))}
			</div>
			<div className={styles.navGroup}>
				<strong>v5 後續交易畫面</strong>
				{later.map((screen) => (
					<Link
						key={screen.slug}
						href={rfqPrototypeScreenPath(screen.slug)}
						className={styles.navItem}
						aria-current={current === screen.slug ? "page" : undefined}
					>
						<span>{screen.targetId}</span>
						{screen.label}
					</Link>
				))}
			</div>
		</nav>
	);
}

function RequestSummary() {
	return (
		<aside className={styles.requestSummary} aria-label="詢價摘要">
			<div className={styles.summaryHeading}>
				<AppIcon name="document" size={20} aria-hidden="true" />
				<div>
					<strong>{RFQ_DEMO_SUMMARY.title}</strong>
					<span>{RFQ_DEMO_SUMMARY.rfqNo}</span>
				</div>
			</div>
			<dl>
				<div>
					<dt>需求版本</dt>
					<dd>{RFQ_DEMO_SUMMARY.requestVersion}</dd>
				</div>
				<div>
					<dt>數量</dt>
					<dd>{RFQ_DEMO_SUMMARY.quantity}</dd>
				</div>
				<div>
					<dt>預算</dt>
					<dd>{RFQ_DEMO_SUMMARY.budget}</dd>
				</div>
				<div>
					<dt>交貨</dt>
					<dd>{RFQ_DEMO_SUMMARY.delivery}</dd>
				</div>
				<div>
					<dt>報價截止</dt>
					<dd>{RFQ_DEMO_SUMMARY.deadline}</dd>
				</div>
			</dl>
			<p>
				<AppIcon name="info" size={17} aria-hidden="true" />
				示意資料，不代表後端已有 RFQ、付款或訂單能力。
			</p>
		</aside>
	);
}

function Notice({
	tone = "info",
	icon = "info",
	children,
}: {
	tone?: "info" | "warning" | "danger" | "success";
	icon?: React.ComponentProps<typeof AppIcon>["name"];
	children: React.ReactNode;
}) {
	return (
		<div className={styles.notice} data-tone={tone} role="note">
			<AppIcon name={icon} size={20} aria-hidden="true" />
			<div>{children}</div>
		</div>
	);
}

function Section({
	title,
	description,
	action,
	children,
	className,
}: {
	title: string;
	description?: string;
	action?: React.ReactNode;
	children: React.ReactNode;
	className?: string;
}) {
	return (
		<section className={`${styles.section} ${className ?? ""}`}>
			<header className={styles.sectionHead}>
				<div>
					<h2>{title}</h2>
					{description ? <p>{description}</p> : null}
				</div>
				{action}
			</header>
			{children}
		</section>
	);
}

function CandidateHandoffView() {
	const [selected, setSelected] = React.useState(
		() =>
			new Set(
				RFQ_PROTOTYPE_SUPPLIERS.filter((item) => item.selected).map(
					(item) => item.id,
				),
			),
	);
	const selectedCount = selected.size;

	return (
		<div className={styles.stack}>
			<Notice>
				<strong>這一步只整理詢價收件人。</strong>
				候選是公開資料與平台資料的比對結果，不代表賣家已同意供貨。
			</Notice>
			<Section
				title="確認邀請名單"
				description="勾選會加入這次 RFQ；查看候選本身不會改變名單。"
				action={<StatusBadge tone="info">已選 {selectedCount} 家</StatusBadge>}
			>
				<div className={styles.supplierList}>
					{RFQ_PROTOTYPE_SUPPLIERS.map((supplier) => {
						const checked = selected.has(supplier.id);
						return (
							<label
								key={supplier.id}
								className={styles.supplierRow}
								data-selected={checked || undefined}
							>
								<input
									type="checkbox"
									checked={checked}
									onChange={() =>
										setSelected((current) => {
											const next = new Set(current);
											if (next.has(supplier.id)) next.delete(supplier.id);
											else next.add(supplier.id);
											return next;
										})
									}
								/>
								<span className={styles.supplierMark} aria-hidden="true">
									<AppIcon
										name={supplier.lane === "平台賣家" ? "bag" : "goods"}
										size={22}
									/>
								</span>
								<span className={styles.supplierMain}>
									<span className={styles.supplierTitle}>
										<strong>{supplier.name}</strong>
										<StatusBadge
											tone={
												supplier.lane === "平台賣家" ? "success" : "warning"
											}
										>
											{supplier.lane}
										</StatusBadge>
									</span>
									<small>
										比對 {supplier.matchScore} 分｜{supplier.status}
									</small>
									<span className={styles.factLine}>
										{supplier.facts.join(" · ")}
									</span>
								</span>
							</label>
						);
					})}
				</div>
				<div className={styles.sectionActions}>
					<p>建立 RFQ 後仍可在發布前移除收件人。</p>
					{selectedCount > 0 ? (
						<Button
							href={rfqPrototypeScreenPath("rfq-compose")}
							endContent={<AppIcon name="arrow-right" size={18} />}
						>
							用 {selectedCount} 家候選建立詢價單
						</Button>
					) : (
						<Button disabled>請先選擇候選</Button>
					)}
				</div>
			</Section>
		</div>
	);
}

function RfqComposeView() {
	const router = useObservedRouter("procurement_rfq_prototype");
	const [confirmOpen, setConfirmOpen] = React.useState(false);
	const [saving, setSaving] = React.useState(false);
	const [saved, setSaved] = React.useState(false);
	const [minimumUnitPrice, setMinimumUnitPrice] = React.useState(
		EXTERNAL_VENDOR_BUYER_OFFER.buyerMinimumUnitPrice,
	);
	const [maximumUnitPrice, setMaximumUnitPrice] = React.useState(
		EXTERNAL_VENDOR_BUYER_OFFER.buyerMaximumUnitPrice,
	);
	const [idealUnitPrice, setIdealUnitPrice] = React.useState(
		EXTERNAL_VENDOR_BUYER_OFFER.transactionUnitPrice,
	);
	const qualityTarget =
		idealUnitPrice >= 275
			? "品質優先"
			: idealUnitPrice <= 250
				? "價格優先"
				: "均衡品質";
	const qualityDescription =
		qualityTarget === "品質優先"
			? "優先完整產銷履歷、較高規格與較低替代比例。"
			: qualityTarget === "價格優先"
				? "接受更多當季替代品，但重量、食安與交期不得降低。"
				: "兼顧履歷來源、每箱重量與活動分裝品質。";
	const projectedTotal =
		idealUnitPrice * EXTERNAL_VENDOR_BUYER_OFFER.quantity +
		EXTERNAL_VENDOR_BUYER_OFFER.freight;

	const updateMinimumUnitPrice = (value: number) => {
		const next = Math.min(value, maximumUnitPrice - 5);
		setMinimumUnitPrice(next);
		setIdealUnitPrice((current) => Math.max(current, next));
	};

	const updateMaximumUnitPrice = (value: number) => {
		const next = Math.max(value, minimumUnitPrice + 5);
		setMaximumUnitPrice(next);
		setIdealUnitPrice((current) => Math.min(current, next));
	};

	const saveDraft = () => {
		setSaving(true);
		window.setTimeout(() => {
			setSaving(false);
			setSaved(true);
		}, 520);
	};

	return (
		<div className={styles.stack}>
			<Notice tone="warning" icon="warning">
				<strong>送出會固定需求、Pricebook 價格與收件名單。</strong>
				後續若改規格、價格區間或理想成交價，必須建立新版本，不能悄悄覆寫賣家收到的內容。
			</Notice>
			<form
				className={styles.formGrid}
				onSubmit={(event) => event.preventDefault()}
			>
				<Section
					title="需求與交貨條件"
					description="這些欄位會進入正式 RFQ 快照。"
				>
					<div className={styles.fields}>
						<label className={styles.fieldWide}>
							<span>詢價主旨</span>
							<input defaultValue={RFQ_DEMO_SUMMARY.title} />
						</label>
						<label>
							<span>需求數量</span>
							<input defaultValue="80" inputMode="numeric" />
						</label>
						<label>
							<span>單位</span>
							<select defaultValue="份">
								<option>份</option>
								<option>箱</option>
								<option>公斤</option>
							</select>
						</label>
						<label className={styles.fieldWide}>
							<span>品項與規格</span>
							<textarea defaultValue="葉菜、根莖與菇類共 8–10 種；每份適合 1 人午餐料理；若需同等級當季替代品，供應商必須在異議中逐項列明。" />
						</label>
						<label>
							<span>交貨日期</span>
							<input type="date" defaultValue="2026-08-15" />
						</label>
						<label>
							<span>最晚到貨時間</span>
							<input type="time" defaultValue="09:00" />
						</label>
						<label className={styles.fieldWide}>
							<span>交貨地點</span>
							<input defaultValue="台北市內湖區瑞光路 000 號｜1 樓收貨區" />
						</label>
					</div>
				</Section>
				<Section
					title="Pricebook 價格與品質範圍"
					description="Pricebook 先提供可成交邊界；你再設定願意接受的上下限與理想成交價。"
					action={<StatusBadge tone="success">基準日 2026/08/10</StatusBadge>}
					className={styles.pricePlannerSection}
				>
					<div className={styles.pricePlanner}>
						<div className={styles.priceBandSummary}>
							<div>
								<span>可接受單價區間</span>
								<strong>
									{money(minimumUnitPrice)}–{money(maximumUnitPrice)}／箱
								</strong>
							</div>
							<div data-primary>
								<span>理想成交價</span>
								<strong>{money(idealUnitPrice)}／箱</strong>
							</div>
							<div>
								<span>預估採購總額</span>
								<strong>{money(projectedTotal)}</strong>
							</div>
						</div>

						<div className={styles.rangeControls}>
							<label className={styles.rangeRow}>
								<span>
									<strong>最低可接受單價</strong>
									<output>{money(minimumUnitPrice)}</output>
								</span>
								<input
									type="range"
									aria-label="最低可接受單價"
									min={EXTERNAL_VENDOR_BUYER_OFFER.pricebookFloor}
									max={EXTERNAL_VENDOR_BUYER_OFFER.pricebookCeiling - 5}
									step={5}
									value={minimumUnitPrice}
									onChange={(event) =>
										updateMinimumUnitPrice(Number(event.currentTarget.value))
									}
								/>
								<small>低於這個價格，預期品質與履約風險可能不符合需求。</small>
							</label>
							<label className={styles.rangeRow}>
								<span>
									<strong>理想成交單價</strong>
									<output>{money(idealUnitPrice)}</output>
								</span>
								<input
									type="range"
									aria-label="理想成交單價"
									min={minimumUnitPrice}
									max={maximumUnitPrice}
									step={5}
									value={idealUnitPrice}
									onChange={(event) =>
										setIdealUnitPrice(Number(event.currentTarget.value))
									}
								/>
								<small>
									送給賣家的主要成交提案，仍保留上下限作為異議協商邊界。
								</small>
							</label>
							<label className={styles.rangeRow}>
								<span>
									<strong>最高可接受單價</strong>
									<output>{money(maximumUnitPrice)}</output>
								</span>
								<input
									type="range"
									aria-label="最高可接受單價"
									min={EXTERNAL_VENDOR_BUYER_OFFER.pricebookFloor + 5}
									max={EXTERNAL_VENDOR_BUYER_OFFER.pricebookCeiling}
									step={5}
									value={maximumUnitPrice}
									onChange={(event) =>
										updateMaximumUnitPrice(Number(event.currentTarget.value))
									}
								/>
								<small>
									若賣家超過這個單價，系統應要求買家重新確認，不可自動成交。
								</small>
							</label>
						</div>

						<div className={styles.qualityTarget} data-quality={qualityTarget}>
							<span className={styles.qualityIcon}>
								<AppIcon name="star" size={21} />
							</span>
							<span>
								<small>目前品質取向</small>
								<strong>{qualityTarget}</strong>
								<p>{qualityDescription}</p>
							</span>
							<div>
								<small>零售參考價</small>
								<strong>{money(idealUnitPrice * 2)}／箱</strong>
								<span>批發成交價 × 2</span>
							</div>
						</div>
					</div>
				</Section>
				<Section
					title="詢價單與回覆規則"
					description="讓每家供應商針對同一個已確認版本接受、提出異議或婉拒。"
				>
					<div className={styles.fields}>
						<label>
							<span>報價截止</span>
							<input type="datetime-local" defaultValue="2026-08-12T18:00" />
						</label>
						<label>
							<span>採購總預算上限</span>
							<input defaultValue="24000" inputMode="numeric" />
						</label>
						<label>
							<span>價格口徑</span>
							<select defaultValue="含稅、含運總價">
								<option>含稅、含運總價</option>
								<option>含稅、運費另列</option>
							</select>
						</label>
						<label>
							<span>報價有效期至少</span>
							<select defaultValue="24 小時">
								<option>24 小時</option>
								<option>48 小時</option>
							</select>
						</label>
						<div className={styles.fieldWide}>
							<span>附件</span>
							<span className={styles.uploadField}>
								<AppIcon name="upload" size={20} aria-hidden="true" />
								採購規格_v4.pdf｜248 KB
							</span>
						</div>
					</div>
				</Section>
			</form>
			<div className={styles.stickyActions}>
				<div>
					<strong>2 家平台賣家＋1 家外部業者</strong>
					<span>
						{saved
							? "價格與需求草稿已儲存於此裝置（示意）"
							: "尚未送出，不會通知賣家"}
					</span>
				</div>
				<Button
					variant="bordered"
					color="default"
					isLoading={saving}
					loadingLabel="儲存中…"
					onClick={saveDraft}
				>
					儲存草稿
				</Button>
				<Button
					endContent={<AppIcon name="send" size={18} />}
					onClick={() => setConfirmOpen(true)}
				>
					確認並送出給賣家
				</Button>
			</div>
			<Dialog
				open={confirmOpen}
				onClose={() => setConfirmOpen(false)}
				title="確認價格與品質後送出？"
				description="送出後，收件賣家會看到同一份需求 v4、價格上下限、理想成交價與品質期待；若要修改，必須建立新版本。"
				footer={
					<>
						<Button
							variant="bordered"
							color="default"
							onClick={() => setConfirmOpen(false)}
						>
							返回檢查
						</Button>
						<Button
							onClick={() => {
								setConfirmOpen(false);
								router.push(rfqPrototypeScreenPath("seller-inbox"));
							}}
						>
							確認並送出示意詢價單
						</Button>
					</>
				}
			>
				<ul className={styles.confirmList}>
					<li>
						<AppIcon name="check" size={18} />
						需求版本：v4
					</li>
					<li>
						<AppIcon name="check" size={18} />
						價格：{money(minimumUnitPrice)}–{money(maximumUnitPrice)}，理想{" "}
						{money(idealUnitPrice)}／箱
					</li>
					<li>
						<AppIcon name="check" size={18} />
						品質：{qualityTarget}｜零售參考 {money(idealUnitPrice * 2)}／箱
					</li>
					<li>
						<AppIcon name="check" size={18} />
						收件人：2 家平台賣家＋1 家外部業者
					</li>
					<li>
						<AppIcon name="check" size={18} />
						截止：8/12（三）18:00
					</li>
				</ul>
			</Dialog>
		</div>
	);
}

function SellerInboxView() {
	const [filter, setFilter] = React.useState("待確認");
	const rows = [
		{
			no: RFQ_DEMO_SUMMARY.rfqNo,
			buyer: "青田設計",
			title: RFQ_DEMO_SUMMARY.title,
			due: "剩 2 天 3 小時",
			status: "待確認",
			tone: "warning" as const,
		},
		{
			no: "RFQ-2026-0809-008",
			buyer: "禾光企劃",
			title: "中秋禮盒｜常溫果乾與堅果",
			due: "剩 18 小時",
			status: "異議草稿",
			tone: "info" as const,
		},
		{
			no: "RFQ-2026-0808-003",
			buyer: "木日工作室",
			title: "每週辦公室水果箱",
			due: "已回覆",
			status: "已接受",
			tone: "success" as const,
		},
	];
	const visible =
		filter === "全部" ? rows : rows.filter((row) => row.status === filter);

	return (
		<div className={styles.stack}>
			<div className={styles.summaryStrip}>
				<div>
					<span>待確認</span>
					<strong>3</strong>
				</div>
				<div>
					<span>異議草稿</span>
					<strong>1</strong>
				</div>
				<div>
					<span>今日截止</span>
					<strong>1</strong>
				</div>
				<div>
					<span>已回覆</span>
					<strong>8</strong>
				</div>
			</div>
			<Section
				title="採購詢價收件匣"
				description="買家已先確認價格與品質；供應商接受詢價仍不代表訂單已付款成立。"
				action={
					<label className={styles.compactSelect}>
						<span className="sr-only">篩選狀態</span>
						<select
							value={filter}
							onChange={(event) => setFilter(event.currentTarget.value)}
						>
							<option>待確認</option>
							<option>異議草稿</option>
							<option>已接受</option>
							<option>全部</option>
						</select>
					</label>
				}
			>
				<div className={styles.inboxList}>
					{visible.map((row) => (
						<article key={row.no} className={styles.inboxRow}>
							<div className={styles.rowIcon}>
								<AppIcon name="document" size={22} />
							</div>
							<div className={styles.inboxMain}>
								<div>
									<strong>{row.title}</strong>
									<StatusBadge tone={row.tone}>{row.status}</StatusBadge>
								</div>
								<span>
									{row.buyer}｜{row.no}
								</span>
								<small>{row.due}｜交貨地點：台北市</small>
							</div>
							{row.no === RFQ_DEMO_SUMMARY.rfqNo ? (
								<Button
									href={rfqPrototypeScreenPath("seller-response")}
									endContent={<AppIcon name="arrow-right" size={17} />}
								>
									查看並回覆
								</Button>
							) : (
								<Button variant="light" color="default">
									查看
								</Button>
							)}
						</article>
					))}
				</div>
			</Section>
		</div>
	);
}

function SellerResponseView() {
	const [confirmOpen, setConfirmOpen] = React.useState(false);
	const [submitted, setSubmitted] = React.useState(false);
	const [response, setResponse] = React.useState<
		"accept" | "adjust" | "decline"
	>("accept");
	const offer = EXTERNAL_VENDOR_BUYER_OFFER;
	const responseLabel =
		response === "accept"
			? "接受買家條件"
			: response === "adjust"
				? "提出條件異議"
				: "婉拒本次詢價";

	if (submitted) {
		return (
			<div className={styles.stack}>
				<div className={styles.resultPanel} data-tone="success">
					<span>
						<AppIcon name="check" size={30} />
					</span>
					<h2>{responseLabel}已送出</h2>
					<p>
						供應商回覆 v1 已在 8/10（一）18:04 固定；買家詢價單 v4
						仍保持唯讀。後續修改會建立新版本，不覆寫任何一方的原始紀錄。
					</p>
					<div>
						<StatusBadge tone="success">唯讀</StatusBadge>
						<StatusBadge tone="info">Supplier response v1</StatusBadge>
					</div>
					<Button
						href={rfqPrototypeScreenPath("quote-compare")}
						endContent={<AppIcon name="arrow-right" size={18} />}
					>
						切換到買家比較畫面
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className={styles.stack}>
			<Notice tone="warning" icon="lock">
				<strong>買家詢價單已定稿，供應商不能直接修改。</strong>
				你可以接受、提出獨立異議或婉拒；每次正式回覆都保留版本與操作者。
			</Notice>
			<div className={styles.twoColumn}>
				<Section
					title="買家已確認的採購條件"
					description="詢價單 v4｜確認於 8/10（一）15:20"
				>
					<div className={styles.requirementRows}>
						<div>
							<span>品項</span>
							<strong>葉菜、根莖與菇類 8–10 種</strong>
						</div>
						<div>
							<span>數量</span>
							<strong>{offer.quantity} 箱</strong>
						</div>
						<div>
							<span>交貨</span>
							<strong>8/15 09:00 前｜內湖</strong>
						</div>
						<div>
							<span>理想成交價</span>
							<strong>{money(offer.transactionUnitPrice)}／箱</strong>
						</div>
						<div>
							<span>可接受區間</span>
							<strong>
								{money(offer.buyerMinimumUnitPrice)}–
								{money(offer.buyerMaximumUnitPrice)}／箱
							</strong>
						</div>
						<div>
							<span>品質取向</span>
							<strong>{offer.qualityTarget}</strong>
						</div>
						<div>
							<span>採購總額</span>
							<strong>{money(offer.total)}（含稅、含運）</strong>
						</div>
					</div>
				</Section>
				<Section
					title="選擇回覆方式"
					description="回覆會另建版本，不會改動買家原始詢價單。"
				>
					<div className={styles.choiceList}>
						<label data-selected={response === "accept" || undefined}>
							<input
								type="radio"
								name="supplier-response"
								checked={response === "accept"}
								onChange={() => setResponse("accept")}
							/>
							<AppIcon name="check" size={22} />
							<span>
								<strong>接受全部條件</strong>
								<small>確認價格、品質、數量與交期皆可履行</small>
							</span>
						</label>
						<label data-selected={response === "adjust" || undefined}>
							<input
								type="radio"
								name="supplier-response"
								checked={response === "adjust"}
								onChange={() => setResponse("adjust")}
							/>
							<AppIcon name="edit" size={22} />
							<span>
								<strong>價格或條件需要調整</strong>
								<small>提出新的建議值與原因，等待買家確認</small>
							</span>
						</label>
						<label data-selected={response === "decline" || undefined}>
							<input
								type="radio"
								name="supplier-response"
								checked={response === "decline"}
								onChange={() => setResponse("decline")}
							/>
							<AppIcon name="close" size={22} />
							<span>
								<strong>無法承接</strong>
								<small>婉拒本次詢價並提供結構化原因</small>
							</span>
						</label>
					</div>
				</Section>
			</div>

			{response === "accept" ? (
				<Notice tone="success" icon="check">
					<strong>
						將接受理想成交價 {money(offer.transactionUnitPrice)}／箱。
					</strong>
					接受後等待買家付款；付款成功才會建立正式訂單並開放履約。
				</Notice>
			) : null}

			{response === "adjust" ? (
				<Section
					title="提出獨立異議"
					description="只填寫與買家版本不同的項目，原詢價單保持唯讀。"
				>
					<div className={styles.fields}>
						<label>
							<span>建議成交單價</span>
							<input defaultValue="275" inputMode="numeric" />
						</label>
						<label>
							<span>調整原因</span>
							<select defaultValue="品質規格成本">
								<option>品質規格成本</option>
								<option>交期與物流成本</option>
								<option>供貨量限制</option>
							</select>
						</label>
						<label className={styles.fieldWide}>
							<span>異議說明</span>
							<textarea defaultValue="可依原規格供應，但完整產銷履歷與指定時段冷藏配送的成本較高，建議調整為每箱 275 元。" />
						</label>
					</div>
				</Section>
			) : null}

			{response === "decline" ? (
				<Section
					title="婉拒原因"
					description="結構化原因可協助買家調整下一輪詢價。"
				>
					<div className={styles.fields}>
						<label>
							<span>主要原因</span>
							<select defaultValue="交期無法配合">
								<option>交期無法配合</option>
								<option>供貨量不足</option>
								<option>品質規格無法符合</option>
								<option>價格範圍無法接受</option>
							</select>
						</label>
						<label className={styles.fieldWide}>
							<span>補充說明（選填）</span>
							<textarea defaultValue="本週冷藏配送時段已滿，無法在指定時間前送達。" />
						</label>
					</div>
				</Section>
			) : null}
			<div className={styles.stickyActions}>
				<div>
					<strong>{responseLabel}</strong>
					<span>前端示意尚未送出，也不會通知買家</span>
				</div>
				<Button
					variant="bordered"
					color="default"
					href={rfqPrototypeScreenPath("seller-inbox")}
				>
					返回收件匣
				</Button>
				<Button onClick={() => setConfirmOpen(true)}>
					確認{responseLabel}
				</Button>
			</div>
			<Dialog
				open={confirmOpen}
				onClose={() => setConfirmOpen(false)}
				title={`確認${responseLabel}？`}
				description="送出後會建立不可直接覆寫的供應商回覆 v1，並保留對應的買家詢價單 v4。"
				footer={
					<>
						<Button
							variant="bordered"
							color="default"
							onClick={() => setConfirmOpen(false)}
						>
							返回檢查
						</Button>
						<Button
							onClick={() => {
								setConfirmOpen(false);
								setSubmitted(true);
							}}
						>
							確認送出回覆 v1
						</Button>
					</>
				}
			>
				<div className={styles.priceConfirm}>
					<span>{responseLabel}</span>
					<strong>
						{response === "adjust"
							? money(275)
							: money(offer.transactionUnitPrice)}
						／箱
					</strong>
					<small>
						{offer.quantity} 箱｜買家範圍 {money(offer.buyerMinimumUnitPrice)}–
						{money(offer.buyerMaximumUnitPrice)}｜{offer.deliveryAt}
					</small>
				</div>
			</Dialog>
		</div>
	);
}

function QuoteCompareView() {
	const [shortlisted, setShortlisted] = React.useState(
		() =>
			new Set(
				RFQ_PROTOTYPE_QUOTES.filter((quote) => quote.shortlisted).map(
					(quote) => quote.id,
				),
			),
	);
	const toggle = (quote: RfqPrototypeQuote) =>
		setShortlisted((current) => {
			const next = new Set(current);
			if (next.has(quote.id)) next.delete(quote.id);
			else next.add(quote.id);
			return next;
		});

	return (
		<div className={styles.stack}>
			<Notice>
				<strong>shortlist 只代表想進一步評估。</strong>
				不會建立訂單、不會通知付款，也不會讓 Agent 替你接受供應商異議。
			</Notice>
			<Section
				title="收到 3 份供應商正式回覆"
				description="比較接受狀態、調整價格、供貨能力與例外條件；原始買家詢價單保持唯讀。"
				action={
					<StatusBadge tone="info">shortlist {shortlisted.size}</StatusBadge>
				}
			>
				<div className={styles.compareWrap}>
					<table className={styles.compareTable}>
						<thead>
							<tr>
								<th scope="col">賣家／版本</th>
								<th scope="col">含運總額</th>
								<th scope="col">回覆單價</th>
								<th scope="col">供貨量</th>
								<th scope="col">交期</th>
								<th scope="col">稅與運費</th>
								<th scope="col">動作</th>
							</tr>
						</thead>
						<tbody>
							{RFQ_PROTOTYPE_QUOTES.map((quote) => {
								const isExternal = quote.supplier === "豐田產地直送";
								return (
									<tr
										key={quote.id}
										data-shortlisted={shortlisted.has(quote.id) || undefined}
									>
										<td>
											<strong>{quote.supplier}</strong>
											<span>
												{quote.version}｜{quote.submittedAt}
											</span>
											{isExternal ? (
												<StatusBadge tone="warning">資格待確認</StatusBadge>
											) : null}
										</td>
										<td>
											<strong>{money(quote.total)}</strong>
											<span>{quote.total > 24000 ? "高於預算" : "預算內"}</span>
										</td>
										<td>{money(quote.unitPrice)}</td>
										<td>{quote.availableQuantity} 份</td>
										<td>{quote.leadTime}</td>
										<td>
											{quote.tax}
											<br />
											{quote.freight}
										</td>
										<td>
											<Button
												size="sm"
												variant={
													shortlisted.has(quote.id) ? "bordered" : "solid"
												}
												color={
													shortlisted.has(quote.id) ? "default" : "primary"
												}
												onClick={() => toggle(quote)}
											>
												{shortlisted.has(quote.id) ? "移除" : "加入"}
											</Button>
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			</Section>
			<div className={styles.quoteCards}>
				{RFQ_PROTOTYPE_QUOTES.map((quote) => (
					<article
						key={quote.id}
						data-shortlisted={shortlisted.has(quote.id) || undefined}
					>
						<header>
							<div>
								<strong>{quote.supplier}</strong>
								<span>
									{quote.version}｜{quote.submittedAt}
								</span>
							</div>
							<strong>{money(quote.total)}</strong>
						</header>
						<dl>
							<div>
								<dt>回覆單價</dt>
								<dd>{money(quote.unitPrice)}</dd>
							</div>
							<div>
								<dt>供貨</dt>
								<dd>{quote.availableQuantity} 份</dd>
							</div>
							<div>
								<dt>交期</dt>
								<dd>{quote.leadTime}</dd>
							</div>
							<div>
								<dt>口徑</dt>
								<dd>
									{quote.tax}｜{quote.freight}
								</dd>
							</div>
						</dl>
						<ul>
							{quote.notes.map((note) => (
								<li key={note}>{note}</li>
							))}
						</ul>
						<Button
							fullWidth
							variant={shortlisted.has(quote.id) ? "bordered" : "solid"}
							color={shortlisted.has(quote.id) ? "default" : "primary"}
							onClick={() => toggle(quote)}
						>
							{shortlisted.has(quote.id)
								? "從 shortlist 移除"
								: "加入 shortlist"}
						</Button>
					</article>
				))}
			</div>
			<div className={styles.stopBoundary}>
				<AppIcon name="lock" size={24} />
				<div>
					<strong>Wave 1 在 shortlist 停止</strong>
					<p>
						要選定供應商回覆、付款與建立訂單，仍需 authoritative
						checkout、payment 與 order contracts。
					</p>
				</div>
				<Button href={rfqPrototypeScreenPath("checkout")} variant="bordered">
					查看後續目標畫面
				</Button>
			</div>
		</div>
	);
}

function CheckoutView() {
	const [method, setMethod] = React.useState("card");
	const [confirmed, setConfirmed] = React.useState(false);
	return (
		<div className={styles.stack}>
			<Notice tone="warning" icon="warning">
				<strong>Later Wave target-state。</strong>
				這個畫面只驗證資訊架構，不會建立付款意圖或訂單。
			</Notice>
			<div className={styles.checkoutGrid}>
				<div className={styles.stack}>
					<Section
						title="付款方式"
						description="同一時間只能有一個 active checkout。"
					>
						<div className={styles.choiceList}>
							<label data-selected={method === "card" || undefined}>
								<input
									type="radio"
									name="payment"
									value="card"
									checked={method === "card"}
									onChange={() => setMethod("card")}
								/>
								<AppIcon name="wallet" size={22} />
								<span>
									<strong>信用卡一次付清</strong>
									<small>付款成功後才建立正式訂單</small>
								</span>
							</label>
							<label data-selected={method === "atm" || undefined}>
								<input
									type="radio"
									name="payment"
									value="atm"
									checked={method === "atm"}
									onChange={() => setMethod("atm")}
								/>
								<AppIcon name="coin" size={22} />
								<span>
									<strong>ATM 虛擬帳號</strong>
									<small>取號後鎖定 2 小時，逾時需重新確認</small>
								</span>
							</label>
						</div>
					</Section>
					<Section
						title="發票與聯絡資料"
						description="資料在建立付款意圖前仍可返回修改。"
					>
						<div className={styles.fields}>
							<label>
								<span>發票類型</span>
								<select defaultValue="公司電子發票">
									<option>公司電子發票</option>
									<option>個人電子發票</option>
								</select>
							</label>
							<label>
								<span>統一編號</span>
								<input defaultValue="12345678" />
							</label>
							<label className={styles.fieldWide}>
								<span>收件信箱</span>
								<input type="email" defaultValue="procurement@example.test" />
							</label>
						</div>
					</Section>
				</div>
				<aside className={styles.orderSummary}>
					<h2>付款摘要</h2>
					<span>{RFQ_DEMO_SUMMARY.rfqNo}</span>
					<div>
						<span>山腳蔬果共同運銷社</span>
						<strong>Quote v1</strong>
					</div>
					<dl>
						<div>
							<dt>商品與服務</dt>
							<dd>{money(22880)}</dd>
						</div>
						<div>
							<dt>運費</dt>
							<dd>已含</dd>
						</div>
						<div>
							<dt>稅額</dt>
							<dd>已含</dd>
						</div>
						<div className={styles.orderTotal}>
							<dt>一次付清</dt>
							<dd>{money(22880)}</dd>
						</div>
					</dl>
					<label className={styles.consent}>
						<input
							type="checkbox"
							checked={confirmed}
							onChange={(event) => setConfirmed(event.currentTarget.checked)}
						/>
						<span>我已確認報價版本、交貨資訊與付款後才成立訂單。</span>
					</label>
					{method === "atm" && confirmed ? (
						<Button href={rfqPrototypeScreenPath("atm-payment")} fullWidth>
							取得 ATM 虛擬帳號
						</Button>
					) : (
						<Button fullWidth disabled={!confirmed}>
							前往一次付清
						</Button>
					)}
					<small>Target-state prototype｜不會送出真實付款</small>
				</aside>
			</div>
		</div>
	);
}

function AtmPaymentView() {
	return (
		<div className={styles.stack}>
			<Notice tone="warning" icon="clock">
				<strong>付款保留剩餘 01:42:18</strong>
				倒數不逐秒對讀屏軟體播報；到期後需重新確認報價與庫存。
			</Notice>
			<div className={styles.paymentPanel}>
				<div className={styles.paymentState}>
					<span>
						<AppIcon name="lock" size={28} />
					</span>
					<div>
						<StatusBadge tone="warning">等待 ATM 入帳</StatusBadge>
						<h2>訂單尚未成立</h2>
						<p>款項入帳成功後才會建立唯一訂單並通知賣家備貨。</p>
					</div>
				</div>
				<dl>
					<div>
						<dt>銀行代碼</dt>
						<dd>812</dd>
					</div>
					<div>
						<dt>虛擬帳號</dt>
						<dd>8120 0000 0000 1234</dd>
					</div>
					<div>
						<dt>應付金額</dt>
						<dd>{money(22880)}</dd>
					</div>
					<div>
						<dt>繳款期限</dt>
						<dd>2026/08/10 20:00</dd>
					</div>
				</dl>
				<div className={styles.paymentActions}>
					<Button
						variant="bordered"
						color="default"
						startContent={<AppIcon name="download" size={18} />}
					>
						下載繳款資訊
					</Button>
					<Button variant="light" color="default">
						我已完成轉帳
					</Button>
				</div>
			</div>
			<Notice tone="danger" icon="warning">
				<strong>晚到款不會硬套回過期報價。</strong>
				若銀行在期限後才通知入帳，系統進入退款處理並留下完整紀錄。
			</Notice>
		</div>
	);
}

function ExternalVendorView() {
	return (
		<div className={styles.stack}>
			<div className={styles.portalHead}>
				<div>
					<span className={styles.vendorLogo}>
						<AppIcon name="goods" size={24} />
					</span>
					<div>
						<strong>Hawkr 外部供應商 Portal</strong>
						<small>受邀詢價｜{RFQ_DEMO_SUMMARY.rfqNo}</small>
					</div>
				</div>
				<StatusBadge tone="warning">資格驗證未完成</StatusBadge>
			</div>
			<div className={styles.twoColumn}>
				<Section
					title="回覆這份邀請"
					description="可先儲存反報價，但完成資格驗證前不能進入正式交易。"
				>
					<div className={styles.requirementRows}>
						<div>
							<span>買家</span>
							<strong>青田設計</strong>
						</div>
						<div>
							<span>需求</span>
							<strong>{RFQ_DEMO_SUMMARY.title}</strong>
						</div>
						<div>
							<span>數量</span>
							<strong>80 份</strong>
						</div>
						<div>
							<span>截止</span>
							<strong>8/12 18:00</strong>
						</div>
					</div>
					<div className={styles.sectionActions}>
						<Button variant="bordered" color="default">
							不參與本次詢價
						</Button>
						<Button>建立反報價草稿</Button>
					</div>
				</Section>
				<Section
					title="交易資格"
					description="這些檢查是接受報價與付款前的必要條件。"
				>
					<ul className={styles.checklist}>
						<li data-done>
							<AppIcon name="check" size={19} />
							<span>
								<strong>受邀連結驗證</strong>
								<small>已確認邀請仍有效</small>
							</span>
						</li>
						<li data-done>
							<AppIcon name="check" size={19} />
							<span>
								<strong>平台帳號</strong>
								<small>已完成 Email 與手機驗證</small>
							</span>
						</li>
						<li>
							<AppIcon name="clock" size={19} />
							<span>
								<strong>KYC 與負責人身分</strong>
								<small>文件審核中，預計 1–2 個工作天</small>
							</span>
						</li>
						<li>
							<AppIcon name="lock" size={19} />
							<span>
								<strong>賣家與 PAYUNi 合作商店</strong>
								<small>待 KYC 完成後開放</small>
							</span>
						</li>
					</ul>
					<Button fullWidth disabled>
						完成資格後才能接受與付款
					</Button>
				</Section>
			</div>
		</div>
	);
}

function SellerOrderView() {
	return (
		<div className={styles.stack}>
			<div className={styles.orderHero}>
				<div>
					<StatusBadge tone="success">已付款｜待備貨</StatusBadge>
					<h2>訂單 HK202608150014</h2>
					<p>由「山腳蔬果共同運銷社」自行完成備貨、出貨與發票。</p>
				</div>
				<Button startContent={<AppIcon name="truck" size={18} />}>
					開始備貨
				</Button>
			</div>
			<div className={styles.threeColumn}>
				<Section title="履約狀態">
					<ul className={styles.timeline}>
						<li data-active>
							<span />
							<div>
								<strong>付款成功</strong>
								<small>8/12 16:42｜訂單正式成立</small>
							</div>
						</li>
						<li data-active>
							<span />
							<div>
								<strong>待賣家備貨</strong>
								<small>最晚 8/13 18:00 前確認</small>
							</div>
						</li>
						<li>
							<span />
							<div>
								<strong>安排冷藏配送</strong>
								<small>尚未開始</small>
							</div>
						</li>
						<li>
							<span />
							<div>
								<strong>買家收貨</strong>
								<small>尚未開始</small>
							</div>
						</li>
					</ul>
				</Section>
				<Section title="發票">
					<div className={styles.requirementRows}>
						<div>
							<span>發票狀態</span>
							<strong>待開立</strong>
						</div>
						<div>
							<span>買受人</span>
							<strong>範例設計股份有限公司</strong>
						</div>
						<div>
							<span>統編</span>
							<strong>12345678</strong>
						</div>
					</div>
					<Button variant="bordered" color="default" fullWidth>
						上傳發票
					</Button>
				</Section>
				<Section title="結算">
					<div className={styles.settlementAmount}>
						<span>預計可結算</span>
						<strong>{money(22193)}</strong>
						<small>平台服務費與退款風險期依正式契約計算</small>
					</div>
					<StatusBadge tone="warning">待買家收貨與風險期結束</StatusBadge>
				</Section>
			</div>
			<Notice>
				<strong>權限界線：</strong>Agent 與 ADM
				可讀取狀態並協助說明，但不能替賣家按下備貨、出貨或發票操作。
			</Notice>
		</div>
	);
}

function AftersalesView() {
	const [submitted, setSubmitted] = React.useState(false);
	return (
		<div className={styles.stack}>
			<div className={styles.twoColumn}>
				<Section
					title="回報收貨問題"
					description="送出後會建立售後案件並評估是否凍結待結算款項。"
				>
					{!submitted ? (
						<form
							className={styles.fields}
							onSubmit={(event) => {
								event.preventDefault();
								setSubmitted(true);
							}}
						>
							<label>
								<span>問題類型</span>
								<select defaultValue="數量短少">
									<option>數量短少</option>
									<option>品質不符</option>
									<option>配送延誤</option>
								</select>
							</label>
							<label>
								<span>受影響數量</span>
								<input defaultValue="6" inputMode="numeric" />
							</label>
							<label className={styles.fieldWide}>
								<span>情況說明</span>
								<textarea defaultValue="其中 6 份根莖品項數量不足，已在收貨現場完成清點。" />
							</label>
							<div className={styles.fieldWide}>
								<span>證據</span>
								<span className={styles.uploadField}>
									<AppIcon name="camera" size={20} />
									現場照片 3 張｜收貨清點表 1 份
								</span>
							</div>
							<div className={styles.fieldWide}>
								<Button type="submit">送出售後申請</Button>
							</div>
						</form>
					) : (
						<div className={styles.resultInline}>
							<AppIcon name="check" size={24} />
							<div>
								<strong>案件 AS202608150021 已建立</strong>
								<span>資料已送交賣家與客服；你仍可補充證據。</span>
							</div>
						</div>
					)}
				</Section>
				<Section
					title="款項與處理進度"
					description="狀態同時以文字、圖示與時間顯示。"
				>
					<div className={styles.freezePanel}>
						<div>
							<AppIcon name="lock" size={24} />
							<span>
								<strong>
									{submitted ? "部分款項評估凍結中" : "尚未建立凍結"}
								</strong>
								<small>
									{submitted
										? "最高 1,716 元｜等待初步判定"
										: "送出售後申請後才會評估"}
								</small>
							</span>
						</div>
					</div>
					<ul className={styles.timeline}>
						<li data-active>
							<span />
							<div>
								<strong>買家提出問題</strong>
								<small>{submitted ? "8/15 10:24" : "尚未送出"}</small>
							</div>
						</li>
						<li>
							<span />
							<div>
								<strong>賣家回覆</strong>
								<small>待處理</small>
							</div>
						</li>
						<li>
							<span />
							<div>
								<strong>平台判定</strong>
								<small>尚未開始</small>
							</div>
						</li>
						<li>
							<span />
							<div>
								<strong>退款或解除凍結</strong>
								<small>尚未開始</small>
							</div>
						</li>
					</ul>
				</Section>
			</div>
		</div>
	);
}

function FinanceView() {
	const [selected, setSelected] = React.useState("SET-2026-0818-019");
	const [reviewed, setReviewed] = React.useState(false);
	return (
		<div className={styles.stack}>
			<div className={styles.summaryStrip}>
				<div>
					<span>待初審</span>
					<strong>12</strong>
				</div>
				<div>
					<span>待覆審</span>
					<strong>5</strong>
				</div>
				<div>
					<span>今日可匯</span>
					<strong>{money(184320)}</strong>
				</div>
				<div>
					<span>已退回</span>
					<strong>2</strong>
				</div>
			</div>
			<div className={styles.financeGrid}>
				<Section title="匯款佇列" description="初審與覆審不可由同一人完成。">
					<div className={styles.financeQueue}>
						{[
							"SET-2026-0818-019",
							"SET-2026-0818-018",
							"SET-2026-0818-017",
						].map((id, index) => (
							<button
								key={id}
								type="button"
								data-selected={selected === id || undefined}
								onClick={() => setSelected(id)}
							>
								<span>
									<strong>{id}</strong>
									<small>
										{index === 0
											? "山腳蔬果共同運銷社"
											: index === 1
												? "青禾食材行"
												: "木日農產"}
									</small>
								</span>
								<span>
									<strong>
										{money(index === 0 ? 22193 : index === 1 ? 18420 : 31680)}
									</strong>
									<StatusBadge tone={index === 0 ? "warning" : "info"}>
										{index === 0 ? "待覆審" : "待初審"}
									</StatusBadge>
								</span>
							</button>
						))}
					</div>
				</Section>
				<Section
					title="覆審 SET-2026-0818-019"
					description="來源訂單、調整與初審結論並列，不能只看匯款總額。"
				>
					<div className={styles.auditRows}>
						<div>
							<span>賣家</span>
							<strong>山腳蔬果共同運銷社</strong>
						</div>
						<div>
							<span>來源訂單</span>
							<strong>HK202608150014</strong>
						</div>
						<div>
							<span>訂單實收</span>
							<strong>{money(22880)}</strong>
						</div>
						<div>
							<span>平台費與調整</span>
							<strong>-{money(687)}</strong>
						</div>
						<div>
							<span>應匯金額</span>
							<strong>{money(22193)}</strong>
						</div>
						<div>
							<span>初審</span>
							<strong>王○怡｜8/18 10:42</strong>
						</div>
					</div>
					<Notice
						tone={reviewed ? "success" : "info"}
						icon={reviewed ? "check" : "shield"}
					>
						{reviewed ? (
							<>
								<strong>覆審已核准。</strong>
								決策已寫入不可覆寫稽核紀錄，等待批次匯款。
							</>
						) : (
							<>
								<strong>你是覆審者。</strong>核准前請確認收貨、售後 freeze
								與發票狀態。
							</>
						)}
					</Notice>
					<div className={styles.sectionActions}>
						<Button variant="bordered" color="danger">
							退回初審
						</Button>
						<Button disabled={reviewed} onClick={() => setReviewed(true)}>
							{reviewed ? "已完成覆審" : "核准並加入匯款批次"}
						</Button>
					</div>
				</Section>
			</div>
		</div>
	);
}

function ScreenBody({ screen }: { screen: RfqPrototypeScreenSlug }) {
	switch (screen) {
		case "candidate-handoff":
			return <CandidateHandoffView />;
		case "rfq-compose":
			return <RfqComposeView />;
		case "seller-inbox":
			return <SellerInboxView />;
		case "seller-response":
			return <SellerResponseView />;
		case "quote-compare":
			return <QuoteCompareView />;
		case "checkout":
			return <CheckoutView />;
		case "atm-payment":
			return <AtmPaymentView />;
		case "external-vendor":
			return <ExternalVendorView />;
		case "seller-order":
			return <SellerOrderView />;
		case "aftersales":
			return <AftersalesView />;
		case "finance":
			return <FinanceView />;
	}
}

export default function RfqPrototype({
	screen,
}: {
	screen: RfqPrototypeScreenSlug;
}) {
	const definition = RFQ_PROTOTYPE_SCREENS.find((item) => item.slug === screen);
	if (!definition) return null;
	if (screen === "external-vendor") {
		return <ExternalVendorPortal step="invitation" />;
	}

	return (
		<div className={styles.prototype} data-screen={screen}>
			<header className={styles.topbar}>
				<div className={styles.brandBlock}>
					<HawkrAgentMark label="Hawkr Agent，AI 採購經理" />
					<div>
						<strong>Hawkr Agent v5.0</strong>
						<span>RFQ 交易流程前端實作</span>
					</div>
				</div>
				<div className={styles.prototypeFlag}>
					<AppIcon name="warning" size={18} />
					<span>
						<strong>Target-state prototype</strong>
						<small>示意資料｜未連 RFQ 後端</small>
					</span>
				</div>
				<Button
					href="/account/messages/assistant/demo"
					variant="bordered"
					color="default"
					size="sm"
					startContent={<AppIcon name="arrow-left" size={17} />}
				>
					返回 Agent Demo
				</Button>
			</header>
			<div className={styles.mobileNav}>
				<ScreenNav current={screen} />
			</div>
			<div className={styles.workspace}>
				<aside className={styles.sidebar}>
					<ScreenNav current={screen} />
				</aside>
				<main className={styles.main}>
					<header className={styles.pageHead}>
						<div>
							<div className={styles.pageMeta}>
								<StageBadge screen={definition} />
								{definition.targetId ? (
									<span>{definition.targetId}</span>
								) : null}
								<span>{ROLE_LABEL[definition.role]}</span>
							</div>
							<h1>{definition.title}</h1>
							<p>{definition.description}</p>
						</div>
					</header>
					<div className={styles.mobileSummary}>
						<RequestSummary />
					</div>
					<ScreenBody screen={screen} />
				</main>
				<div className={styles.summaryRail}>
					<RequestSummary />
				</div>
			</div>
		</div>
	);
}
