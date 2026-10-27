import { create } from "@bufbuild/protobuf";
import { ImageSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { expect } from "storybook/test";
import BannerCarousel from "@/components/shared/banner/BannerCarousel";
import BannerLoadingCard from "@/components/shared/banner/BannerLoadingCard";
import BannerLoadingSkeleton from "@/components/shared/banner/BannerLoadingSkeleton";
import { ViewTransitions } from "@/components/shared/view-transitions";
import { CouponSelectionSheet } from "@/features/cart/components/CouponSelectionSheet";
import HomeCompactSearchExperience from "@/features/home/components/HomeCompactSearchExperience";
import { HomeSearchPanelSkeleton } from "@/features/home/components/HomeSearchSkeletons";
import { homeSearchQueryKeys } from "@/features/home/components/useHomeSearchQueries";
import {
	BuyerInvoiceSkeletonCards,
	BuyerInvoiceSkeletonRows,
} from "@/features/invoices/components/BuyerInvoiceListSkeleton";
import {
	InvoiceComplaintLoadingBody,
	InvoiceLoadingLines,
	ORDER_INVOICE_HEADER_CLASS,
} from "@/features/invoices/components/InvoiceSectionLoading";
import { ChatDrawerThreadBoundary } from "@/features/messages/components/ChatDrawerThread";
import { AssistantEntrySkeleton } from "@/features/procurement-assistant/AssistantEntryLayout";
import AssistantEntryLink from "@/features/procurement-assistant/AssistantEntryLink";
import ProductReviewsSection from "@/features/product/components/ProductReviewsSection";
import ProductReviewsSkeleton from "@/features/product/components/ProductReviewsSkeleton";
import ProductSellerIntro from "@/features/product/components/ProductSellerIntro";
import { AuthProvider } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import "@/styles/features/cart.css";
import "@/styles/components/sheet.css";

const IMAGE =
	"data:image/svg+xml," +
	encodeURIComponent(
		'<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="900"><rect width="1600" height="900" fill="#fdf1e1"/></svg>',
	);
const reviews = [0, 1].map((index) => ({
	id: index,
	author: "預覽買家",
	date: "2026/09/13",
	rating: 5,
	text: "元件預覽評價，用來檢查文字、頭像與評分在各種寬度下的位置。",
}));

const meta = {
	title: "Web/Feedback/InlineLoading",
	parameters: {
		layout: "padded",
		docs: {
			description: {
				component:
					"正式行內載入元件：商品評價、搜尋、banner、優惠券及發票。預覽只使用固定資料，不呼叫交易服務。內容行數、評價照片與 banner 原圖比例屬於資料決定的高度。",
			},
		},
	},
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Reviews: Story = {
	render: () => (
		<div className="mx-auto max-w-3xl">
			<div data-review-loading>
				<ProductReviewsSkeleton />
			</div>
			<div data-review-loaded>
				<ProductReviewsSection reviews={reviews} reviewTotal={2} />
			</div>
		</div>
	),
	play: async ({ canvas, canvasElement }) => {
		await expect(canvas.getByRole("status")).toHaveAttribute(
			"aria-busy",
			"true",
		);
		const loading = canvasElement.querySelector(
			"[data-review-loading]",
		) as HTMLElement;
		const loaded = canvasElement.querySelector(
			"[data-review-loaded]",
		) as HTMLElement;
		const pendingCard = loading.querySelector(
			".public-section-surface",
		) as HTMLElement;
		const readyCard = loaded.querySelector(
			".public-section-surface",
		) as HTMLElement;
		await expect(
			Math.abs(
				pendingCard.getBoundingClientRect().width -
					readyCard.getBoundingClientRect().width,
			),
		).toBeLessThan(1);
		await expect(
			Math.abs(
				pendingCard.getBoundingClientRect().top -
					loading.getBoundingClientRect().top -
					(readyCard.getBoundingClientRect().top -
						loaded.getBoundingClientRect().top),
			),
		).toBeLessThan(1);
	},
};

function BannerPreview() {
	const [client] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	return (
		<QueryClientProvider client={client}>
			<div className="mx-auto grid max-w-[420px] gap-6">
				<BannerLoadingSkeleton />
				<BannerLoadingCard />
				<BannerCarousel
					size="compact"
					heading="最新消息"
					intervalMs={0}
					initialBanners={[
						{
							id: "preview-banner",
							title: "元件預覽",
							previewImage: create(ImageSchema, { uriTemplate: IMAGE }),
						},
					]}
				/>
			</div>
		</QueryClientProvider>
	);
}
export const Banners: Story = { render: () => <BannerPreview /> };

function SearchPreview({ loading }: { loading: boolean }) {
	const panelRef = useRef<HTMLDivElement>(null);
	const [client] = useState(() => {
		const value = new QueryClient({
			defaultOptions: { queries: { retry: false, staleTime: Infinity } },
		});
		value.setQueryData(queryKeys.productHashtags.popular({ limit: 12 }), []);
		if (!loading) {
			value.setQueryData(homeSearchQueryKeys.products("預覽"), {
				records: [0, 1, 2].map((id) => ({
					id: String(id),
					name: "預覽商品",
					imageKey: IMAGE,
					sellerName: "預覽店家",
					price: "NT$ 100",
				})),
				total: 3,
			});
			value.setQueryData(homeSearchQueryKeys.suggestions("預覽"), [
				{ keyword: "預覽商品" },
			]);
		}
		return value;
	});
	return (
		<QueryClientProvider client={client}>
			<ViewTransitions>
				<AuthProvider>
					<HomeCompactSearchExperience
						open
						query="預覽"
						requestQuery="預覽"
						coords={{
							top: 16,
							left: 16,
							width: Math.min(window.innerWidth - 32, 720),
							maxHeight: window.innerHeight - 32,
						}}
						panelRef={panelRef}
						onDismiss={() => {}}
						onNavigate={() => {}}
					/>
				</AuthProvider>
			</ViewTransitions>
		</QueryClientProvider>
	);
}
function offlineSearch() {
	const original = globalThis.fetch;
	globalThis.fetch = (() => new Promise<Response>(() => {})) as typeof fetch;
	return () => {
		globalThis.fetch = original;
	};
}
export const SearchProductsLoading: Story = {
	beforeEach: offlineSearch,
	render: () => <SearchPreview loading />,
};
export const SearchProductsLoaded: Story = {
	beforeEach: offlineSearch,
	render: () => <SearchPreview loading={false} />,
};
export const SearchChunkLoading: Story = {
	render: () => (
		<div className="max-w-[720px] rounded-hk-lg border border-hk-border-soft bg-hk-surface-card">
			<HomeSearchPanelSkeleton hasQuery />
		</div>
	),
};
export const InvoiceList: Story = {
	render: () => (
		<div className="mx-auto max-w-4xl">
			<div className="hidden overflow-x-auto md:block">
				<table className="w-full border-collapse text-sm">
					<caption className="sr-only">發票載入預覽</caption>
					<thead>
						<tr>
							{["開立日期", "發票號碼", "開立人", "類型", "金額", "狀態"].map(
								(label) => (
									<th key={label} className="px-3 py-2.5 text-left text-xs">
										{label}
									</th>
								),
							)}
						</tr>
					</thead>
					<tbody>
						<BuyerInvoiceSkeletonRows />
					</tbody>
				</table>
			</div>
			<ul className="m-0 flex list-none flex-col gap-3 p-0 md:hidden">
				<BuyerInvoiceSkeletonCards />
			</ul>
		</div>
	),
};

function CouponPreview({ loading }: { loading: boolean }) {
	return (
		<CouponSelectionSheet
			open
			coupons={
				loading
					? []
					: [0, 1, 2].map((id) => ({
							id,
							templateId: `preview-${id}`,
							name: "預覽優惠券",
							kind: "cash",
							estimatedDiscount: 100,
							discountAmount: 100,
							thresholdAmount: 1000,
							expires_at: Date.UTC(2099, 11, 31),
						}))
			}
			selectedTemplateId={null}
			orderAmount={1500}
			shippingFee={0}
			loading={loading}
			onSelect={() => {}}
			onClear={() => {}}
			onClose={() => {}}
			onRetry={() => {}}
		/>
	);
}
export const CouponsLoading: Story = {
	render: () => <CouponPreview loading />,
};
export const CouponsLoaded: Story = {
	render: () => <CouponPreview loading={false} />,
};

export const ChatDrawer: Story = {
	render: () => (
		<div className="seller-message-chat-sheet flex h-[600px] max-h-[calc(100dvh-32px)] flex-col">
			<ChatDrawerThreadBoundary
				targetId="preview-peer"
				resource={{
					peek: () => null,
					load: () => new Promise(() => {}),
					prewarm: () => {},
				}}
			/>
		</div>
	),
};

function AgentEntryPreview() {
	const [client] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	const query = "預覽商品與規格比較，這是一段會在手機換行的已知搜尋內容";
	return (
		<QueryClientProvider client={client}>
			<ViewTransitions>
				<AuthProvider>
					<div className="mx-auto grid max-w-[720px] gap-6">
						<div data-agent-entry-loading>
							<AssistantEntrySkeleton query={query} />
						</div>
						<div data-agent-entry-loaded>
							<AssistantEntryLink query={query} />
						</div>
					</div>
				</AuthProvider>
			</ViewTransitions>
		</QueryClientProvider>
	);
}
export const SearchAgentEntry: Story = {
	beforeEach: offlineSearch,
	render: () => <AgentEntryPreview />,
	play: async ({ canvasElement }) => {
		const loading = canvasElement.querySelector(
			"[data-agent-entry-loading] > div",
		) as HTMLElement;
		const loaded = canvasElement.querySelector(
			"[data-agent-entry-loaded] > a",
		) as HTMLElement;
		await expect(
			Math.abs(
				loading.getBoundingClientRect().height -
					loaded.getBoundingClientRect().height,
			),
		).toBeLessThan(1);
	},
};
export const SellerStats: Story = {
	render: () => (
		<div className="mx-auto grid max-w-[480px] gap-6">
			<ProductSellerIntro
				previewMode
				pending
				sellerId="preview-seller"
				productSeller={{ name: "預覽商家", address: "預覽所在地" }}
			/>
			<div aria-hidden="true">
				<ProductSellerIntro
					previewMode
					sellerId="preview-seller"
					productSeller={{ name: "預覽商家", address: "預覽所在地" }}
				/>
			</div>
		</div>
	),
};
export const InvoiceSections: Story = {
	render: () => (
		<div className="mx-auto grid max-w-3xl gap-6">
			<section className="bg-content1 border border-divider rounded-hkui-lg p-5">
				<div className={ORDER_INVOICE_HEADER_CLASS}>
					<h2 className="m-0 text-base font-semibold">發票資訊載入預覽</h2>
					<span className="h-6 w-20 animate-pulse rounded-full bg-content2 motion-reduce:animate-none" />
				</div>
				<InvoiceLoadingLines />
			</section>
			<section className="mt-4 border-t border-divider pt-4">
				<h3 className="m-0 mb-3 text-sm font-semibold">發票申訴載入預覽</h3>
				<InvoiceComplaintLoadingBody />
			</section>
		</div>
	),
};
