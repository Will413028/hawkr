import { describe, expect, it } from "vitest";
import { renderWithIntl } from "@/tests/browser/render";
import { SellerMobileDisclosureList } from "./SellerMobileDisclosureList";
import {
	SellerCustomersSkeletonRows,
	SellerInventorySkeletonRows,
	SellerMobileDisclosureSkeleton,
	SellerPostsSkeleton,
	SellerProductsSkeletonRows,
} from "./SellerPageSkeletons";
import {
	SellerCardGridSkeleton,
	SellerStatRowSkeleton,
} from "./SellerSkeletons";
import "@/styles/features/seller-center.css";

function SellerShell({ children }: { children: React.ReactNode }) {
	return (
		<div
			className="seller-shell"
			style={{ display: "block", padding: 16, minWidth: 0 }}
		>
			{children}
		</div>
	);
}

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

describe("Seller skeletons", () => {
	it("renders the products skeleton with every column and a thumbnail", () => {
		const { container } = renderWithIntl(
			<SellerShell>
				<ProductTable />
			</SellerShell>,
		);
		const table = container.querySelector("table");
		expect(table?.querySelectorAll("thead th")).toHaveLength(11);
		expect(table?.querySelectorAll("tbody tr:first-child td")).toHaveLength(11);
		expect(
			table?.querySelector(
				"tbody td[data-label='商品'] .seller-products-table__thumb",
			),
		).toBeTruthy();
	});

	it("omits the select column for delegated read-only products", () => {
		const { container } = renderWithIntl(
			<SellerShell>
				<ProductTable delegated canPublish={false} />
			</SellerShell>,
		);
		expect(container.querySelectorAll("thead th")).toHaveLength(8);
		expect(container.querySelectorAll("tbody tr:first-child td")).toHaveLength(
			8,
		);
		expect(container.querySelector("tbody td[data-label='選取']")).toBeNull();
	});

	it("labels every inventory and customer skeleton cell", async () => {
		const { container } = renderWithIntl(
			<SellerShell>
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
										{[
											"客戶",
											"標籤",
											"訂單",
											"累計消費",
											"最近購買",
											"操作",
										].map((label) => (
											<th key={label}>{label}</th>
										))}
									</tr>
								</thead>
								<tbody>
									<SellerCustomersSkeletonRows rows={1} />
								</tbody>
							</table>
						</div>
					</div>
				</div>
			</SellerShell>,
		);
		const cells = container.querySelectorAll("tbody td");
		expect(cells.length).toBeGreaterThan(0);
		for (const cell of cells)
			await expect.element(cell as HTMLElement).toHaveAttribute("data-label");
	});

	it("keeps the phone disclosure skeleton the same size as the real row", () => {
		const { container } = renderWithIntl(
			<SellerShell>
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
			</SellerShell>,
		);
		const loading = container.querySelector(
			"[aria-busy='true'].seller-mobile-disclosure-list .seller-mobile-disclosure",
		);
		const reference = container.querySelector(
			"ul[aria-label='正式折疊列幾何對照'] .seller-mobile-disclosure",
		);
		expect(loading).not.toBeNull();
		expect(reference).not.toBeNull();
		if (!loading || !reference) return;
		expect(loading.getBoundingClientRect().height).toBeGreaterThan(0);
		expect(loading.getBoundingClientRect().height).toBe(
			reference.getBoundingClientRect().height,
		);
		expect(loading.getBoundingClientRect().width).toBe(
			reference.getBoundingClientRect().width,
		);
	});

	it("does not reserve square product images in form panels", () => {
		const { container } = renderWithIntl(
			<SellerShell>
				<div className="grid gap-5">
					<SellerCardGridSkeleton variant="panel" count={2} />
					<SellerStatRowSkeleton count={4} />
				</div>
			</SellerShell>,
		);
		expect(container.querySelector(".aspect-square")).toBeNull();
	});
});
