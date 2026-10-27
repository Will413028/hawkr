import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import { SellerMobileDisclosureList } from "@/features/seller-center/components/SellerMobileDisclosureList";
import {
	SellerCouponsSkeleton,
	SellerCustomersSkeletonRows,
	SellerInventorySkeletonRows,
	SellerInvoiceDocumentsSkeletonRows,
	SellerInvoiceOverviewSkeleton,
	SellerMobileDisclosureSkeleton,
	SellerOrdersSkeletonRows,
	SellerPostsSkeleton,
	SellerProductsSkeletonRows,
} from "@/features/seller-center/components/SellerPageSkeletons";
import {
	SellerCardGridSkeleton,
	SellerStatRowSkeleton,
	SellerTableSkeletonRows,
} from "@/features/seller-center/components/SellerSkeletons";
import "@/styles/features/seller-center.css";

const meta = {
	title: "Web/Feedback/SellerSkeletons",
	parameters: {
		layout: "fullscreen",
		docs: {
			description: {
				component:
					"賣家骨架沿用正式表格、手機欄位卡、動態封面及折疊列的 CSS。商品表格須傳入實際 delegated / canPublish；表單使用 panel，避免方形商品圖片撐高。資料與對照列皆為固定 Storybook fixture，沒有真實交易。",
			},
		},
	},
	decorators: [
		(Story) => (
			<div
				className="seller-shell"
				style={{ display: "block", padding: 16, minWidth: 0 }}
			>
				<Story />
			</div>
		),
	],
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const productLabels = [
	"選取",
	"商品",
	"評分",
	"分類",
	"價格",
	"庫存",
	"收藏",
	"銷量",
	"上架時間",
	"狀態",
	"操作",
];
function ProductTable({
	delegated = false,
	canPublish = true,
}: {
	delegated?: boolean;
	canPublish?: boolean;
}) {
	const labels = delegated
		? [
				"選取",
				"商品 ID",
				"商品",
				"分類",
				"價格",
				"庫存",
				"銷量",
				"狀態",
				"操作",
			]
		: productLabels;
	return (
		<div className="seller-card seller-products__card">
			<div className="seller-orders__table-wrap">
				<table
					className={`seller-orders__table seller-products-table ${delegated ? "seller-products-table--delegated" : ""}`}
					data-mobile-layout="cards"
					aria-label="商品載入範例"
				>
					<thead>
						<tr>
							{labels
								.filter((label) => canPublish || label !== "選取")
								.map((label) => (
									<th
										key={label}
										className={
											label === "選取"
												? "seller-products-table__check"
												: undefined
										}
									>
										{label}
									</th>
								))}
						</tr>
					</thead>
					<tbody>
						<SellerProductsSkeletonRows
							rows={2}
							delegated={delegated}
							canPublish={canPublish}
						/>
					</tbody>
				</table>
			</div>
		</div>
	);
}
export const Products: Story = {
	render: () => <ProductTable />,
	play: async ({ canvasElement }) => {
		const table = canvasElement.querySelector("table");
		await expect(table?.querySelectorAll("thead th")).toHaveLength(11);
		await expect(
			table?.querySelectorAll("tbody tr:first-child td"),
		).toHaveLength(11);
		await expect(
			table?.querySelector(
				"tbody td[data-label='商品'] .seller-products-table__thumb",
			),
		).toBeTruthy();
	},
};
export const DelegatedReadOnlyProducts: Story = {
	render: () => <ProductTable delegated canPublish={false} />,
	play: async ({ canvasElement }) => {
		await expect(canvasElement.querySelectorAll("thead th")).toHaveLength(8);
		await expect(
			canvasElement.querySelectorAll("tbody tr:first-child td"),
		).toHaveLength(8);
		await expect(
			canvasElement.querySelector("tbody td[data-label='選取']"),
		).toBeNull();
	},
};
export const InventoryAndCustomers: Story = {
	render: () => (
		<div className="grid gap-5">
			<div className="seller-card seller-inventory__card">
				<div className="seller-orders__table-wrap">
					<table
						className="seller-inventory__table"
						data-mobile-layout="cards"
						aria-label="庫存載入範例"
					>
						<thead>
							<tr>
								{[
									"商品名稱",
									"當前庫存",
									"建議安全庫存",
									"週銷趨勢",
									"預估售罄",
									"狀態",
									"庫存價值",
									"操作",
								].map((label) => (
									<th key={label}>{label}</th>
								))}
							</tr>
						</thead>
						<tbody>
							<SellerInventorySkeletonRows rows={1} />
						</tbody>
					</table>
				</div>
			</div>
			<div className="seller-card seller-customers__card">
				<div className="seller-orders__table-wrap">
					<table
						className="seller-customers__table seller-customers__table--list"
						data-mobile-layout="cards"
						aria-label="客戶載入範例"
					>
						<thead>
							<tr>
								{["客戶", "標籤", "訂單", "累計消費", "最近購買", "操作"].map(
									(label) => (
										<th key={label}>{label}</th>
									),
								)}
							</tr>
						</thead>
						<tbody>
							<SellerCustomersSkeletonRows rows={1} />
						</tbody>
					</table>
				</div>
			</div>
		</div>
	),
	play: async ({ canvasElement }) => {
		for (const cell of canvasElement.querySelectorAll("tbody td"))
			await expect(cell).toHaveAttribute("data-label");
	},
};
export const OrdersAndInvoices: Story = {
	render: () => (
		<div className="grid gap-5">
			<div className="seller-orders__table-wrap">
				<table
					className="seller-orders__table seller-orders__table--orders"
					aria-label="訂單載入範例"
				>
					<thead>
						<tr>
							{[
								"選取",
								"訂單編號",
								"顧客資訊",
								"商品",
								"數量",
								"金額",
								"配送方式",
								"訂單狀態",
								"操作",
							].map((label) => (
								<th
									key={label}
									className={
										label === "選取" ? "seller-orders__col-select" : undefined
									}
								>
									{label}
								</th>
							))}
						</tr>
					</thead>
					<tbody>
						<SellerOrdersSkeletonRows rows={2} />
					</tbody>
				</table>
			</div>
			<div className="seller-orders__table-wrap">
				<table className="seller-table" aria-label="發票載入範例">
					<thead>
						<tr>
							{[
								"訂單編號",
								"付款時間",
								"買家類型",
								"開票方式",
								"金額",
								"發票狀態",
								"出貨狀態",
								"更新時間",
							].map((label) => (
								<th key={label}>{label}</th>
							))}
						</tr>
					</thead>
					<tbody>
						<SellerInvoiceDocumentsSkeletonRows rows={2} />
					</tbody>
				</table>
			</div>
		</div>
	),
};
export const PostsAndPhoneRows: Story = {
	render: () => (
		<div className="seller-posts">
			<div className="seller-mobile-disclosure-desktop">
				<SellerPostsSkeleton count={2} />
			</div>
			<SellerMobileDisclosureSkeleton count={2} />
			<SellerMobileDisclosureList
				items={[{ id: "reference" }]}
				getKey={(item) => item.id}
				ariaLabel="正式折疊列幾何對照"
				getSummary={() => ({
					label: "範例動態",
					leading: null,
					title: "本週新鮮食材",
					meta: "12",
					status: "已發布",
				})}
				renderDetails={() => <p>固定範例</p>}
			/>
		</div>
	),
	play: async ({ canvasElement }) => {
		const loading = canvasElement.querySelector(
			"[aria-busy='true'].seller-mobile-disclosure-list .seller-mobile-disclosure",
		);
		const reference = canvasElement.querySelector(
			"ul[aria-label='正式折疊列幾何對照'] .seller-mobile-disclosure",
		);
		if (loading && reference && loading.getBoundingClientRect().height > 0) {
			await expect(loading.getBoundingClientRect().height).toBe(
				reference.getBoundingClientRect().height,
			);
			await expect(loading.getBoundingClientRect().width).toBe(
				reference.getBoundingClientRect().width,
			);
		}
	},
};
export const Coupons: Story = {
	render: () => (
		<>
			<div className="seller-mobile-disclosure-desktop">
				<SellerCouponsSkeleton count={2} />
			</div>
			<SellerMobileDisclosureSkeleton count={2} />
		</>
	),
};
export const FormPanelsAndStatistics: Story = {
	render: () => (
		<div className="grid gap-5">
			<SellerCardGridSkeleton variant="panel" count={2} />
			<SellerStatRowSkeleton count={4} />
		</div>
	),
	play: async ({ canvasElement }) => {
		await expect(canvasElement.querySelector(".aspect-square")).toBeNull();
	},
};
export const Messages: Story = {
	render: () => (
		<div className="seller-card seller-messages__card">
			<div className="seller-orders__table-wrap">
				<table
					className="seller-orders__table seller-messages__table"
					data-mobile-layout="cards"
					aria-label="訊息載入範例"
				>
					<thead>
						<tr>
							{["客戶名稱", "下單時間", "購買紀錄", "上次回覆", "操作"].map(
								(label) => (
									<th key={label}>{label}</th>
								),
							)}
						</tr>
					</thead>
					<tbody>
						<SellerTableSkeletonRows
							columns={5}
							rows={2}
							labels={["客戶名稱", "下單時間", "購買紀錄", "上次回覆", "操作"]}
						/>
					</tbody>
				</table>
			</div>
		</div>
	),
};

export const InvoiceOverview: Story = {
	render: () => <SellerInvoiceOverviewSkeleton />,
};
