"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { deriveDetailLayoutProps } from "@/features/product/deriveDetailLayoutProps";
import ProductDetailLayout from "@/features/product/ProductDetailLayout";
import { PRODUCT_STATUS_LABEL } from "@/lib/codemeta/enums";
import { type MyStore, storeImageUrls } from "@/lib/seller/store";
import { townLocation } from "@/lib/seller/store-form";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import DelegatedAreaUnauthorized from "../components/DelegatedAreaUnauthorized";
import {
	classifyDelegatedLoadFailure,
	type DelegatedLoadFailureKind,
	delegatedProductLoadFailureCopy,
} from "../load-failure";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import { delegatedPreviewProduct } from "./preview-props";
import {
	createDelegatedProductEditRepo,
	type DelegatedProductDetail,
} from "./repo";

/**
 * 代行工作區・商品預覽（56 號 §2.2 out-of-scope 第 13 列）。
 *
 * **它回答的問題只有一個：這個商品上架之後，買家會看到什麼。**
 *
 * ⚠️ 三件事是設計書定版明令的，不要「順手改成比較省事的做法」：
 *
 * 1. **不新增預覽端點、不改打 `/api/v1/public/*`**。資料就是代行自己的
 *    `GET .../products/{productId}`（`products.read`），與編輯頁同一支。改打公開路徑
 *    看起來最省事，但**草稿在公開路徑上根本查不到**（`publish_status = 4402` 過濾），
 *    預覽會靜默空白 —— 而草稿正是最需要預覽的那一批。
 * 2. **不重用 `features/seller-center/preview/**`**。那是**店面**預覽不是商品預覽，
 *    而且整條讀的是公開來源。
 * 3. **買家視角在工作區內自繪**：重用買家 PDP 的 `ProductDetailLayout`（`previewMode`），
 *    但所有通往公開頁的出口都留在工作區內。這一頁不是「開一個新分頁去看買家頁」，
 *    那個做法對草稿是 404、對已上架的則會把代行工作階段的上下文丟掉。
 *
 * **賣家卡刻意走 `sellerIntroSlot` 自繪**：`ProductSellerIntro` 的整張卡是一個連到
 * `/sellers/{storeCode}` 的 `<Link>`，還掛著一顆 `SellerChatButton`（要 www 登入態，
 * 代行的瀏覽器沒有）。照原樣用會在工作區裡放兩個出口，其中一個還是死的。這裡改成
 * 同樣的身分資訊、但不可點 —— 買家看到的**內容**一致，只是預覽不導航。
 */
export default function DelegatedProductPreviewPage({
	productId,
}: {
	productId: string;
}) {
	const scope = useSellerDataScope();
	const { summary, hasCapability } = useDelegatedSession();
	const storeId = scope.ownerStoreId ?? summary?.target.storeId ?? "";
	const canRead = hasCapability("products.read");
	const productsHref = `/adm/${encodeURIComponent(storeId)}/products`;
	const editHref = `${productsHref}/${encodeURIComponent(productId)}/edit`;
	const repo = React.useMemo(
		() => createDelegatedProductEditRepo(storeId),
		[storeId],
	);

	const [detail, setDetail] = React.useState<DelegatedProductDetail | null>(
		null,
	);
	const [storefront, setStorefront] = React.useState<MyStore | null>(null);
	const [loading, setLoading] = React.useState(true);
	const [loadFailure, setLoadFailure] =
		React.useState<DelegatedLoadFailureKind | null>(null);

	const load = React.useCallback(async () => {
		setLoading(true);
		setLoadFailure(null);
		try {
			setDetail(await repo.detail(productId));
		} catch (e) {
			setLoadFailure(classifyDelegatedLoadFailure(e));
		} finally {
			setLoading(false);
		}
	}, [productId, repo]);

	React.useEffect(() => {
		void load();
	}, [load]);

	/**
	 * 商店資訊（賣家卡）。**失敗不擋預覽**：`storefront.read` 可以不核發，那時整張卡
	 * 不畫，而商品本體仍然預覽得了 —— 把它當成必要條件會讓「只核發商品權限」的工作
	 * 階段完全看不到預覽。
	 */
	React.useEffect(() => {
		if (!storeId || !hasCapability("storefront.read")) return;
		let cancelled = false;
		void (async () => {
			try {
				const self = await scope.repos.storefront.fetchMyStore();
				if (!cancelled) setStorefront(self);
			} catch {
				/* 賣家卡不畫；商品本體照樣預覽 */
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [hasCapability, scope.repos.storefront, storeId]);

	if (!canRead) {
		return (
			<DelegatedAreaUnauthorized
				reason="本次工作階段沒有核發商品讀取權限，因此看不到這個商品的買家視角預覽。需要處理請返回 ADM 重新建立工作階段。"
				backHref={productsHref}
				backLabel="返回商品管理"
			/>
		);
	}

	if (loading) {
		return (
			<section className="adm-panel" aria-busy="true">
				<p className="adm-panel__hint" role="status">
					正在載入預覽…
				</p>
			</section>
		);
	}

	if (!detail) {
		if (loadFailure === "denied") {
			return (
				<DelegatedAreaUnauthorized
					reason="本次工作階段沒有核發商品讀取權限，因此看不到這個商品的內容。需要處理請返回 ADM 重新建立工作階段。"
					backHref={productsHref}
					backLabel="返回商品管理"
				/>
			);
		}
		const failure = delegatedProductLoadFailureCopy(loadFailure ?? "temporary");
		return (
			<section className="adm-panel">
				<h1 className="adm-panel__title">{failure.title}</h1>
				<p className="adm-panel__hint">{failure.hint}</p>
				{failure.retryable ? (
					<button
						type="button"
						className="adm-blocked__link cursor-pointer border-0"
						onClick={() => void load()}
					>
						重新載入
					</button>
				) : null}
				<Link
					href={productsHref}
					data-hkui-button=""
					className="adm-blocked__link"
				>
					返回商品管理
				</Link>
			</section>
		);
	}

	const seller = previewSellerView(storefront);
	const derived = deriveDetailLayoutProps({
		product: delegatedPreviewProduct(detail, storeId),
		seller,
		sellerStats: null,
		// 代行 DTO 沒有評論，也不該為了預覽去打買家端的評論 API：預覽的對象常常是還沒
		// 上架、因此不可能有評論的商品。缺席時 layout 顯示「尚無評價」。
		reviewsResult: null,
		// 不 probe 圖片尺寸：layout 對 image block 走 fallback <img>，與正式 PDP 取不到
		// dims 時行為一致。
		descriptionImageDimEntries: [],
	});
	const statusLabel =
		detail.statusLabel || PRODUCT_STATUS_LABEL[detail.status] || detail.status;

	return (
		<SellerPageShell
			className="adm-product-preview"
			title="買家視角預覽"
			subtitle={`商品管理 / ${detail.name || "未命名商品"}（目前狀態：${statusLabel}）`}
			actions={
				<Link href={editHref} className="seller-orders__btn">
					<AppIcon name="edit" size={16} />
					返回編輯
				</Link>
			}
		>
			{/* 預覽與「已上架」是兩件事。草稿／未啟用的商品在買家端根本沒有這一頁，
			    不說清楚的話這個畫面本身就是一句「它已經上架了」。 */}
			<p className="adm-panel__hint" role="note">
				{
					"這是依商品目前已儲存的內容算出來的買家視角。買家實際看得到這一頁，要等商品上架之後；評價與評分來自買家端，預覽一律顯示「尚無評價」。"
				}
			</p>
			<ProductDetailLayout
				previewMode
				productId={detail.productId || productId}
				{...derived}
				sellerIntroSlot={
					derived.sellerStoreId ? <PreviewSellerCard seller={seller} /> : null
				}
			/>
		</SellerPageShell>
	);
}

interface PreviewSellerView {
	storeId: string;
	storeName: string;
	description: string;
	prize: string;
	contactPhone: string;
	city: string;
	district: string;
	addressDetail: string;
	websiteUrl: string;
	logoUrl: string | undefined;
	coverUrl: string | undefined;
}

/**
 * 代行店面（`GetMyStoreResponse`）→ 商品頁 layout 讀的賣家欄位。
 * 圖片是 **objectKey**（代行 DTO 只回 key，§25），而 `Avatar` 內部走 `ossImageUrl`，對 key 與
 * URL 都成立。
 */
function previewSellerView(my: MyStore | null): PreviewSellerView | null {
	const store = my?.store;
	if (!store) return null;
	const location = townLocation(store.address?.townCode ?? "");
	const images = storeImageUrls(store);
	return {
		storeId: store.id,
		storeName: store.storeName,
		description: store.description,
		prize: store.prize,
		contactPhone: store.contactPhone,
		city: location.county,
		district: location.town,
		addressDetail: store.address?.addressDetail ?? "",
		websiteUrl: store.websiteUrl,
		logoUrl: images.logoUrl,
		coverUrl: images.coverUrl,
	};
}

/** 賣家卡（不可點）。 */
function PreviewSellerCard({ seller }: { seller: PreviewSellerView | null }) {
	const name = seller?.storeName ?? "";
	const location = [seller?.city, seller?.district].filter(Boolean).join(" ");
	return (
		<aside
			className="flex flex-col items-end gap-3 justify-self-end w-full max-w-[230px] text-right max-md:max-w-none max-md:pt-3.5 max-md:border-t max-md:border-hk-border-soft/75"
			aria-label="商家資訊"
		>
			<div className="flex gap-3 items-center justify-end w-full">
				<div className="min-w-0">
					{location ? (
						<span className="block mb-0.5 overflow-hidden text-hk-muted text-xs leading-[1.4] text-ellipsis whitespace-nowrap">
							{location}
						</span>
					) : null}
					<strong className="block overflow-hidden text-hk-heading text-base font-extrabold leading-[1.35] text-ellipsis whitespace-nowrap max-md:text-[0.9375rem]">
						{name || "Hawkr 商家"}
					</strong>
				</div>
				<div className="grid flex-shrink-0 w-16 h-16 overflow-hidden place-items-center bg-hk-cream border-2 border-hk-border-soft rounded-full max-md:w-[58px] max-md:h-[58px]">
					<Avatar
						src={seller?.logoUrl}
						name={name || "商"}
						alt={name}
						entity="seller"
						loading="eager"
						fallback="empty"
					/>
				</div>
			</div>
			{/* 買家頁在這裡有一顆「聯絡賣家」。預覽不畫它而不是畫成 disabled：代行者
			    不是買家，那顆按鈕在這個工作階段裡沒有任何合法按法。 */}
		</aside>
	);
}
