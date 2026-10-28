"use client";

/**
 * 商品詳情 Layout（client）— 把 PDP 的 JSX 主體抽成可共用的 layout。
 *
 * 設計目的：消除「server PDP」與「seller 預覽 PDP」雙份維護的問題。
 * server PDP（fetch 驅動）與 seller 預覽 PDP（import_data adapter 驅動）
 * 都把資料展平後傳本 layout 渲染。
 *
 * 預覽模式（previewMode=true）下：
 *   - 不渲染 <ProductViewTracker>（不對預覽 item 打 view event）
 *   - 不渲染 <ProductBackButton>（caller 負責 header 區塊）
 *   - <ProductSellerIntro> 與正式 PDP 走同一條判定：sellerStoreId 有值
 *     就顯示真實賣家卡（caller 把當下登入賣家 storeId 帶進來），缺值就
 *     不顯示。**不再注入「我的賣場」placeholder**。
 *   - 配送業者 icons 與 shippingNote 在 enabledCarriers/shippingNote 為空
 *     時自然省略（與正式 PDP 行為一致），不塞假 placeholder
 */

import type { ReactNode } from "react";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import SellerSidebarIcon from "@/components/shared/SellerSidebarIcon";
import { Stars } from "@/components/ui/Stars";
import ProductBackButton from "@/features/product/components/ProductBackButton";
import ProductGallery from "@/features/product/components/ProductGallery";
import ProductPurchaseBox, {
	type PurchaseBoxSku,
} from "@/features/product/components/ProductPurchaseBox";
import ProductReviewsSection from "@/features/product/components/ProductReviewsSection";
import ProductSellerIntro from "@/features/product/components/ProductSellerIntro";
import { ProductViewTracker } from "@/features/product/ProductViewTracker";
import { ossImageUrl, resolveCdnUrl } from "@/lib/media/cdn";
import { parseRating } from "@/lib/products/rating";
import { sanitizeBlockStyle } from "@/lib/products/seller-repo";

export interface ProductDetailLayoutProps {
	product: Record<string, unknown>;
	seller: unknown;
	sellerStats: unknown;
	productSeller: unknown;
	reviews: ProductReview[];
	reviewTotal: number;
	purchasable: boolean;
	/** Map: 同一 block 物件參考 → OSS image 探到的 dims；caller 預先算好。 */
	descriptionImageDims: Map<
		Record<string, unknown>,
		{ width: number; height: number } | null
	>;
	thumbnails: string[];
	skus: PurchaseBoxSku[];
	enabledCarriers: ReadonlyArray<{
		method: string;
		label: string;
	}>;
	shippingNote: string;
	categoryName: string;
	categoryTrail: string[];
	sellerStoreId: string;
	purchaseUnit: string;
	productId: string;
	descriptionBlocks: Array<Record<string, unknown>>;
	previewMode?: boolean;
	/**
	 * Streamed seller card (live PDP). When provided it replaces the inline
	 * <ProductSellerIntro>, letting the seller + stats fetches resolve behind a
	 * <Suspense> boundary instead of blocking the page render. The preview path
	 * omits it and renders inline from its already-resolved seller props.
	 */
	sellerIntroSlot?: ReactNode;
	/**
	 * Streamed reviews section (live PDP). When provided it replaces the inline
	 * <ProductReviewsSection>, keeping the reviews fetch off the critical path.
	 * The preview path omits it and renders inline from `reviews`/`reviewTotal`.
	 */
	reviewsSlot?: ReactNode;
}

export interface ProductReview {
	id?: number | string;
	reviewId?: number | string;
	author?: string;
	avatar?: string;
	rating?: unknown;
	text?: string;
	content?: string;
	photos?: unknown;
	date?: string;
	createdAt?: string;
	created_at?: number;
}

function textOrEmpty(value: unknown): string {
	return typeof value === "string" ? value.trim() : "";
}

function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function descriptionImageRaw(block: Record<string, unknown>): string {
	// description.blocks[] 的圖片欄是 imageKey/imageUrl（backend#2235 起與
	// gallery 同名）。這裡與 deriveDetailLayoutProps 的同名函式必須一致。
	return textOrEmpty(block.imageUrl) || textOrEmpty(block.imageKey);
}

const HEX6 = /^#[0-9a-fA-F]{6}$/;
const HTTP_URL = /^https?:\/\//i;

/** 把 inline runs（02 §3.6）渲染成 span / a，逐 run 套粗體 / 斜體 / 底線 / 顏色 / 連結。 */
function renderDescRuns(runsRaw: unknown, keyBase: string): React.ReactNode {
	if (!Array.isArray(runsRaw)) return null;
	return runsRaw.map((r, i) => {
		const run = asRecord(r);
		const text = typeof run.text === "string" ? run.text : "";
		if (!text) return null;
		const style: React.CSSProperties = {};
		if (run.bold === true) style.fontWeight = 700;
		if (run.italic === true) style.fontStyle = "italic";
		if (run.underline === true) style.textDecoration = "underline";
		if (typeof run.color === "string" && HEX6.test(run.color)) {
			style.color = run.color;
		}
		const key = `${keyBase}-r${i}`;
		if (typeof run.href === "string" && HTTP_URL.test(run.href)) {
			return (
				<a
					key={key}
					href={run.href}
					target="_blank"
					rel="noopener noreferrer nofollow"
					className="text-hk-primary underline"
					style={style}
				>
					{text}
				</a>
			);
		}
		return (
			<span key={key} style={style}>
				{text}
			</span>
		);
	});
}

/** 渲染 table block（tableJson）。使用者核准橫向捲動：寬表格外層 overflow-x-auto。 */
function DescriptionTable({
	tableJson,
	keyBase,
}: {
	tableJson: unknown;
	keyBase: string;
}) {
	if (typeof tableJson !== "string") return null;
	let model: { withHeadings?: boolean; rows?: unknown } | null = null;
	try {
		model = JSON.parse(tableJson);
	} catch {
		return null;
	}
	const rows = Array.isArray(model?.rows) ? (model.rows as unknown[]) : [];
	if (rows.length === 0) return null;
	const withHeadings = model?.withHeadings === true;
	return (
		<div className="overflow-x-auto">
			<table className="w-full border-collapse text-[0.95rem]">
				<tbody>
					{rows.map((row, ri) => {
						const cells = Array.isArray(row) ? (row as unknown[]) : [];
						const isHead = withHeadings && ri === 0;
						return (
							// biome-ignore lint/suspicious/noArrayIndexKey: table rows are positional
							<tr key={`${keyBase}-row${ri}`}>
								{cells.map((cell, ci) => {
									const cellRecord = asRecord(cell);
									const cellRuns = cellRecord.runs;
									const textAlign =
										cellRecord.textAlign === "center" ||
										cellRecord.textAlign === "right"
											? cellRecord.textAlign
											: undefined;
									const cls = `border px-3 py-2 align-top break-words ${
										isHead ? "font-bold text-hk-heading" : ""
									}`;
									const cellKey = `${keyBase}-c${ri}-${ci}`;
									return isHead ? (
										<th
											key={cellKey}
											className={cls}
											scope="col"
											style={textAlign ? { textAlign } : undefined}
										>
											{renderDescRuns(cellRuns, cellKey)}
										</th>
									) : (
										<td
											key={cellKey}
											className={cls}
											style={textAlign ? { textAlign } : undefined}
										>
											{renderDescRuns(cellRuns, cellKey)}
										</td>
									);
								})}
							</tr>
						);
					})}
				</tbody>
			</table>
		</div>
	);
}

/** 渲染 list block（listJson）。有序 → 帶序號 <ol>，無序 → 帶項目符號 <ul>；
 *  每個 item 的 runs 透過 renderDescRuns 渲染（同 table cell）。 */
function DescriptionList({
	listJson,
	keyBase,
}: {
	listJson: unknown;
	keyBase: string;
}) {
	if (typeof listJson !== "string") return null;
	let model: {
		ordered?: boolean;
		items?: unknown;
		textAlign?: unknown;
	} | null = null;
	try {
		model = JSON.parse(listJson);
	} catch {
		return null;
	}
	const items = Array.isArray(model?.items) ? (model.items as unknown[]) : [];
	if (items.length === 0) return null;
	const ordered = model?.ordered === true;
	const textAlign =
		model?.textAlign === "center" || model?.textAlign === "right"
			? model.textAlign
			: undefined;
	const cls = `${ordered ? "list-decimal" : "list-disc"} ${
		textAlign ? "list-inside pl-0" : "pl-6"
	} space-y-1 break-words`;
	const lis = items.map((item, ii) => {
		const itemRuns = asRecord(item).runs;
		const itemKey = `${keyBase}-li${ii}`;
		return <li key={itemKey}>{renderDescRuns(itemRuns, itemKey)}</li>;
	});
	return ordered ? (
		<ol className={cls} style={textAlign ? { textAlign } : undefined}>
			{lis}
		</ol>
	) : (
		<ul className={cls} style={textAlign ? { textAlign } : undefined}>
			{lis}
		</ul>
	);
}

/**
 * 從 product payload 抽 gallery 上方的 hashtag chips。canonical 欄位是
 * `hashtags`（match 商城卡片 normaliseHashtags）；seller side 也可能用 `tags`。
 */
function extractGalleryTags(product: Record<string, unknown>): string[] {
	const candidates: unknown[] = [product.hashtags, product.tags];
	for (const raw of candidates) {
		if (!Array.isArray(raw)) continue;
		const tags = raw
			.map((t) => {
				if (typeof t === "string") return t;
				if (t && typeof t === "object") {
					const rec = t as Record<string, unknown>;
					return textOrEmpty(rec.name ?? rec.label ?? rec.tag ?? rec.value);
				}
				return "";
			})
			.map((s) => s.trim())
			.filter(Boolean);
		if (tags.length > 0) return tags;
	}
	return [];
}

export default function ProductDetailLayout({
	product,
	seller,
	sellerStats,
	productSeller,
	reviews,
	reviewTotal,
	purchasable,
	descriptionImageDims,
	thumbnails,
	skus,
	enabledCarriers,
	shippingNote,
	categoryName,
	categoryTrail,
	sellerStoreId,
	purchaseUnit,
	productId,
	descriptionBlocks,
	previewMode = false,
	sellerIntroSlot,
	reviewsSlot,
}: ProductDetailLayoutProps) {
	const productName = textOrEmpty(product.name) || "—";
	const productOrigin =
		textOrEmpty(product.origin) ||
		textOrEmpty(asRecord(product.seller).address) ||
		"台灣";
	const sellerAddress = textOrEmpty(asRecord(product.seller).address) || "台灣";
	const sellerNameForCart =
		textOrEmpty(asRecord(product.seller).name) || "Hawkr 選品";
	const firstDescContent = textOrEmpty(descriptionBlocks[0]?.content);
	const productImage = thumbnails[0];

	return (
		<section
			className="product-detail-layout max-w-[1440px] mx-auto pt-[var(--page-start-gap,6px)] px-6 pb-[60px] max-md:pt-[var(--page-start-gap,16px)] max-md:px-4 max-md:pb-10"
			data-hk-page-content-ready={previewMode ? undefined : "product-detail"}
			data-hk-product-detail-id={previewMode ? undefined : productId}
		>
			{previewMode ? null : <ProductViewTracker productId={productId} />}
			<div className="mt-3.5 mb-[26px] flex items-center justify-between gap-4 max-md:mt-0 max-md:mb-3 max-md:gap-3">
				{previewMode ? null : (
					<div className="shrink-0">
						<ProductBackButton />
					</div>
				)}
				<nav
					className="ml-auto flex min-w-0 flex-1 items-center justify-end flex-wrap gap-2.5 min-h-8 text-hk-muted text-[1.0625rem] font-semibold leading-[1.4] max-md:gap-2 max-md:text-sm"
					aria-label="商品路徑"
				>
					{categoryTrail.map((name, index) => {
						const isLeaf = index === categoryTrail.length - 1;
						return (
							// biome-ignore lint/suspicious/noArrayIndexKey: index = category level
							<span key={`crumb-${index}-${name}`} className="contents">
								{index > 0 ? (
									<span
										aria-hidden="true"
										className="text-hk-muted/50 font-medium"
									>
										/
									</span>
								) : null}
								{previewMode ? (
									<span
										aria-current={isLeaf ? "page" : undefined}
										className={
											isLeaf ? "text-hk-heading font-bold" : "text-hk-body"
										}
									>
										{name}
									</span>
								) : (
									<Link
										href={`/market/products?categories=${encodeURIComponent(name)}`}
										aria-current={isLeaf ? "page" : undefined}
										className={
											isLeaf
												? "text-hk-heading font-bold no-underline transition-colors duration-[160ms] ease-in-out hover:text-hk-primary"
												: "text-hk-body no-underline transition-colors duration-[160ms] ease-in-out hover:text-hk-primary"
										}
									>
										{name}
									</Link>
								)}
							</span>
						);
					})}
				</nav>
			</div>
			<div className="product-detail-layout__card public-section-surface rounded-[36px] p-10 border-[3px] border-hk-border-soft mb-8 max-lg:p-7 max-lg:rounded-[28px] max-md:p-5 max-md:rounded-hk-xl max-md:border-2 glass-card">
				<div className="product-detail-layout__hero grid grid-cols-2 gap-12 items-start max-lg:grid-cols-1 max-lg:gap-8">
					<ProductGallery
						thumbnails={thumbnails}
						productName={productName}
						tags={extractGalleryTags(product)}
					/>

					<div className="flex flex-col gap-3">
						<div className="product-detail-layout__identity grid grid-cols-[minmax(0,1fr)_minmax(190px,230px)] gap-5 items-start max-md:grid-cols-1 max-md:gap-4">
							<div className="min-w-0">
								<h1 className="[font-family:var(--font-primary)] text-4xl font-bold text-hk-heading m-0 leading-[1.3] max-lg:text-[1.75rem] max-md:text-2xl max-sm:text-xl">
									{productName}
								</h1>
								<div className="flex gap-2">
									<span className="text-[0.8125rem] text-hk-muted">
										商品分類 / {categoryName}
									</span>
								</div>
							</div>
							{sellerStoreId
								? // 預覽 & 正式 PDP 共用同一條 path — 預覽端只要把當下登入賣家
									// 的 storeId 填進 product 就會走真實賣家卡，缺資料就不顯示。
									// 正式 PDP 改傳 sellerIntroSlot（<Suspense> 串流），把 seller +
									// stats fetch 移出關鍵路徑；預覽端不帶 slot，內嵌即時渲染。
									(sellerIntroSlot ?? (
										<ProductSellerIntro
											sellerId={sellerStoreId}
											seller={seller}
											sellerStats={sellerStats}
											productSeller={productSeller}
										/>
									))
								: null}
						</div>
						<div className="text-sm text-hk-body font-medium">
							產地：{productOrigin}
						</div>

						<div className="flex flex-col gap-2 my-1">
							<div className="flex items-center gap-2 text-sm text-hk-body">
								<svg
									width="16"
									height="16"
									viewBox="0 0 24 24"
									fill="none"
									stroke="var(--action-primary)"
									strokeWidth="2"
									aria-hidden="true"
									focusable="false"
								>
									<path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0118 0z" />
									<circle cx="12" cy="10" r="3" />
								</svg>
								<span>{sellerAddress}</span>
							</div>
							<div className="flex items-center flex-wrap gap-2 text-sm text-hk-body">
								<SellerSidebarIcon
									name="truck"
									size={16}
									className="shrink-0 text-[var(--action-primary)]"
									focusable="false"
								/>
								{enabledCarriers.map((c) => (
									<span
										key={c.method}
										className="inline-flex items-center justify-center h-6 px-1.5 flex-shrink-0 bg-white border border-divider rounded-[6px] overflow-hidden text-xs font-semibold leading-none text-hk-ink-950 dark:bg-white dark:border-[#f5dea5]/32"
										title={c.label}
									>
										{c.label}
									</span>
								))}
							</div>
							{shippingNote ? (
								<div className="flex items-center gap-2 text-sm text-hk-body">
									<svg
										width="16"
										height="16"
										viewBox="0 0 24 24"
										fill="none"
										stroke="var(--action-primary)"
										strokeWidth="2"
										aria-hidden="true"
										focusable="false"
									>
										<rect x="1" y="3" width="15" height="13" />
										<polygon points="23 7 16 12 16 4 23 7" />
									</svg>
									<span>{shippingNote}</span>
								</div>
							) : null}
						</div>

						<div className="flex items-center gap-2.5">
							{(() => {
								const r = parseRating(product.rating);
								if (r == null) {
									return (
										<span className="text-sm text-hk-muted">尚無評價</span>
									);
								}
								return (
									<button
										type="button"
										onClick={() => {
											document
												.getElementById("product-reviews")
												?.scrollIntoView({
													behavior: "smooth",
													block: "start",
												});
										}}
										className="group flex items-center gap-2.5 m-0 p-0 bg-transparent border-0 appearance-none cursor-pointer rounded focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-hk-primary"
										aria-label={`查看 ${reviewTotal} 則商品評價`}
									>
										<div className="flex gap-0.5">
											<Stars count={Math.round(r)} />
										</div>
										<span className="text-sm text-hk-muted group-hover:underline">
											{r.toFixed(1)} ({reviewTotal}則評價)
										</span>
									</button>
								);
							})()}
						</div>

						<p className="text-sm text-hk-body leading-[1.6] m-0">
							{firstDescContent || textOrEmpty(product.origin) || ""}
						</p>

						<ProductPurchaseBox
							item={{
								id: productId,
								name: productName,
								image: productImage,
								sellerName: sellerNameForCart,
								href: previewMode
									? `#preview-${productId}`
									: `/products/${productId}`,
							}}
							skus={skus}
							unit={purchaseUnit}
							purchasable={purchasable}
							publicationState={textOrEmpty(product.publicationState)}
							initialRestockSubscribed={Boolean(
								(product as { isRestockSubscribed?: boolean })
									.isRestockSubscribed,
							)}
							previewMode={previewMode}
						/>
					</div>
				</div>
			</div>

			{descriptionBlocks.length ? (
				<div className="mb-8">
					<h2 className="text-[1.375rem] font-bold text-hk-heading m-0 mb-4">
						詳細說明
					</h2>
					<div className="public-section-surface bg-white/85 backdrop-blur-[12px] border-[1.5px] border-white/60 rounded-hk-xl py-7 px-8 max-md:p-5 max-md:rounded-2xl dark:bg-white/4 dark:backdrop-blur-[12px] dark:backdrop-saturate-[120%] dark:border-[#f5dea5]/14">
						{descriptionBlocks.map((block, index) => {
							const blockKey = [
								"desc",
								block.type || "block",
								block.id ||
									block.imageUrl ||
									block.imageKey ||
									String(block.content || "").slice(0, 24) ||
									"item",
								index,
							].join("-");
							if (block.type === "table") {
								return (
									<div key={blockKey} className={index > 0 ? "mt-[14px]" : ""}>
										<DescriptionTable
											tableJson={block.tableJson}
											keyBase={blockKey}
										/>
									</div>
								);
							}
							if (block.type === "list") {
								return (
									<div key={blockKey} className={index > 0 ? "mt-[14px]" : ""}>
										<DescriptionList
											listJson={block.listJson}
											keyBase={blockKey}
										/>
									</div>
								);
							}
							if (block.type === "text") {
								const blockStyle = sanitizeBlockStyle(block.style);
								const runs = block.runs;
								return (
									<p
										key={blockKey}
										className={`whitespace-pre-wrap break-words ${index > 0 ? "mt-[14px]" : ""}`}
										style={blockStyle}
									>
										{Array.isArray(runs) && runs.length > 0
											? renderDescRuns(runs, blockKey)
											: textOrEmpty(block.content)}
									</p>
								);
							}
							if (block.type === "image") {
								const rawSrc = descriptionImageRaw(block);
								if (!rawSrc) return null;
								const alt = textOrEmpty(block.alt) || productName;
								const dims = descriptionImageDims.get(block);
								if (dims) {
									return (
										<AppImage
											key={blockKey}
											src={resolveCdnUrl(rawSrc)}
											alt={alt}
											width={dims.width}
											height={dims.height}
											sizes="(max-width: 768px) 100vw, 700px"
											loading="eager"
											className="product-detail-layout__description-image block h-auto w-full max-w-full"
										/>
									);
								}
								// dims 無法解析 → 直接走原 <img> + 縮圖 preset。載入前用
								// 4:3 保留空間；`auto` 會在圖片 metadata 可用後改採原始比例，
								// 讓直式說明圖完整鋪滿內容寬度，不被橫式 placeholder 留白。
								return (
									// biome-ignore lint/performance/noImgElement: description image without resolvable dimensions
									<img
										key={blockKey}
										src={ossImageUrl(rawSrc, "productDetail")}
										alt={alt}
										width={800}
										height={600}
										loading="eager"
										className="product-detail-layout__description-image block h-auto w-full max-w-full"
										style={{
											display: "block",
											width: "100%",
											maxWidth: "100%",
											height: "auto",
											aspectRatio: "auto 4 / 3",
										}}
									/>
								);
							}
							return null;
						})}
					</div>
				</div>
			) : null}

			{reviewsSlot ?? (
				<ProductReviewsSection reviews={reviews} reviewTotal={reviewTotal} />
			)}
		</section>
	);
}
