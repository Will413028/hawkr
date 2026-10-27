"use client";

import { notFound } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { deriveDetailLayoutProps } from "@/features/product/deriveDetailLayoutProps";
import ProductDetailLayout from "@/features/product/ProductDetailLayout";
import { fetchSeller, fetchSellerReviewStats } from "@/lib/api";
import { ApiClientError } from "@/lib/api/client";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { storeIdOf } from "@/lib/seller/store";
import { PreviewBanner } from "../components/PreviewBanner";
import { UI_TEXT } from "../i18n";
import { getImportItemPreview } from "../repo";
import type { ImportItemPreview } from "../types";

interface Props {
	jobId: string;
	itemId: string;
}

export function PreviewDetailPage({ jobId, itemId }: Props) {
	const toast = useNotificationToast();
	const { data: sellerSelf } = useSellerSelf();
	const [preview, setPreview] = React.useState<ImportItemPreview | null>(null);
	const [seller, setSeller] = React.useState<unknown>(null);
	const [sellerStats, setSellerStats] = React.useState<unknown>(null);
	const [loading, setLoading] = React.useState(true);
	const [missing, setMissing] = React.useState(false);

	const storeId = storeIdOf(sellerSelf);

	React.useEffect(() => {
		let cancelled = false;
		(async () => {
			try {
				const data = await getImportItemPreview(jobId, itemId);
				if (!cancelled) setPreview(data);
			} catch (e) {
				if (!cancelled) {
					if (e instanceof ApiClientError && e.status === 404) {
						setMissing(true);
						return;
					}
					toast.show({
						variant: "error",
						title: "載入預覽失敗",
						body: e instanceof Error ? e.message : "請稍後再試",
						durationMs: 5000,
					});
				}
			} finally {
				if (!cancelled) setLoading(false);
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [jobId, itemId, toast]);

	// 當下登入賣家資訊 — 預覽商品還沒 commit，沒有真實 storeId；用當下
	// 賣家身分當預覽展示對象，與真實上架後行為一致。
	React.useEffect(() => {
		if (!storeId) {
			setSeller(null);
			setSellerStats(null);
			return;
		}
		let cancelled = false;
		(async () => {
			const [s, st] = await Promise.all([
				fetchSeller(storeId).catch(() => null),
				fetchSellerReviewStats(storeId).catch(() => null),
			]);
			if (cancelled) return;
			setSeller(s);
			setSellerStats(st);
		})();
		return () => {
			cancelled = true;
		};
	}, [storeId]);

	if (loading) {
		return (
			<section className="max-w-[1440px] mx-auto px-6 pb-[60px] pt-6">
				<p className="py-16 text-center text-sm text-hk-muted">載入中…</p>
			</section>
		);
	}

	if (missing) notFound();

	if (!preview) {
		return (
			<section className="max-w-[1440px] mx-auto px-6 pb-[60px] pt-6">
				<p className="py-16 text-center text-sm text-hk-muted">
					找不到預覽商品
				</p>
			</section>
		);
	}

	// previewData 直接當 product 餵進 derive — 不再做形狀翻譯。額外把當下登入
	// 賣家 storeId 注入，讓 derive 取到 sellerStoreId，layout 才會渲染真實
	// 賣家卡（沒登入就不顯示，與正式 PDP 行為對齊）。
	//
	// Bug B 修：item 在 PARSING / ASSET_DOWNLOADING 階段 description 可能為空字串
	// 或缺欄位 — backend 用 JSON string 攜帶 ImportDescription，這裡 parse 並
	// 提供空 `{ blocks: [] }` default，避免 ProductDetailLayout 內 description?.blocks
	// 取值炸 error boundary。
	const product: Record<string, unknown> = {
		...(preview.previewData as unknown as Record<string, unknown>),
	};
	const rawDescription = preview.previewData?.description;
	if (typeof rawDescription === "string" && rawDescription.length > 0) {
		try {
			const parsed = JSON.parse(rawDescription);
			product.description =
				parsed && typeof parsed === "object" && Array.isArray(parsed.blocks)
					? parsed
					: { version: "1.0", blocks: [] };
		} catch {
			product.description = { version: "1.0", blocks: [] };
		}
	} else {
		product.description = { version: "1.0", blocks: [] };
	}
	if (storeId && !product.sellerAccountId && !product.storeId) {
		product.storeId = storeId;
	}

	const derived = deriveDetailLayoutProps({
		product,
		seller,
		sellerStats,
		// 預覽商品尚未上架 → 無評論。
		reviewsResult: null,
		// 預覽不 probe OSS 圖片尺寸，layout 對 image block 走 fallback <img>，
		// 與正式 PDP 取不到 dims 時行為一致。
		descriptionImageDimEntries: [],
	});

	return (
		<main className="max-w-[1440px] mx-auto pt-[var(--page-start-gap,6px)] px-6 pb-[60px] max-md:pt-[var(--page-start-gap,16px)] max-md:px-4 max-md:pb-10">
			{/* 輕量 header：取代 SellerPageShell（預覽頁掛在 (public) 不能用賣家中心殼）。 */}
			<header className="mt-2 mb-4 flex flex-wrap items-center justify-between gap-3 max-md:mt-0 max-md:mb-3">
				<div className="flex flex-col gap-1">
					<span className="text-xs font-semibold tracking-wider text-hk-primary uppercase">
						商城預覽 · 賣家批次匯入 #{jobId}
					</span>
					<span className="text-sm text-hk-muted">
						商品詳情預覽（完整 PDP layout）
					</span>
				</div>
				<div className="flex items-center gap-2">
					<Link
						href={`/seller-product-import-preview/${jobId}`}
						className="seller-page-header-cta inline-flex items-center gap-1.5 rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-3 py-1.5 text-xs text-hk-primary hover:bg-hk-surface-glass-hover"
					>
						<AppIcon name="arrow-left" size={14} />
						{UI_TEXT.preview.backToPreview}
					</Link>
					<button
						type="button"
						onClick={() => {
							if (typeof window !== "undefined") window.close();
						}}
						className="seller-page-header-cta inline-flex items-center gap-1.5 rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-3 py-1.5 text-xs text-hk-muted hover:bg-hk-surface-glass-hover"
					>
						關閉此分頁
					</button>
				</div>
			</header>

			<div className="mb-6">
				<PreviewBanner jobFilename="" variant="detail" />
			</div>

			<ProductDetailLayout
				previewMode
				productId={preview.importItemId}
				{...derived}
			/>

			{/* 回到列表預覽 */}
			<div className="mt-8 text-center">
				<Link
					href={`/seller-product-import-preview/${jobId}`}
					className="inline-flex items-center gap-1.5 rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-sm text-hk-primary hover:bg-hk-surface-glass-hover"
				>
					<AppIcon name="arrow-left" size={16} />
					返回列表預覽
				</Link>
			</div>
		</main>
	);
}
