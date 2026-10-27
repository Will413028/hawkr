import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import Button from "@/components/ui/Button";
import { Card, CardBody, CardHeader } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Input } from "@/components/ui/Input";
import { FlutterCatalog } from "./apps/FlutterCatalog";

function Comparison({
	component = "buttons",
}: {
	component?: "buttons" | "inputs" | "chips" | "cards";
}) {
	return (
		<main>
			<h1>三端元件對照</h1>
			<p>
				比較相同的元件層級、互動狀態與品牌色；App 區塊直接執行 Flutter 元件。
			</p>
			<div className="catalog-grid">
				<section>
					<h2 style={{ fontSize: 18 }}>Web · {component}</h2>
					<div className="catalog-stack">
						{component === "buttons" && (
							<>
								<Button size="lg" fullWidth>
									加入購物車
								</Button>
								<Button variant="bordered" size="lg" fullWidth>
									繼續選購
								</Button>
								<Button isLoading loadingLabel="加入中…" size="lg" fullWidth>
									加入購物車
								</Button>
							</>
						)}
						{component === "inputs" && (
							<>
								<Input label="商品名稱" placeholder="花蓮小農香米" />
								<Input label="收件人姓名" floatingLabel defaultValue="林小農" />
								<Input label="停用欄位" placeholder="系統商品編號" disabled />
								<Input label="商品名稱" errorMessage="請輸入商品名稱" />
								<Input
									label="唯讀商品名稱"
									defaultValue="花蓮小農香米"
									readOnly
								/>
							</>
						)}
						{component === "chips" && (
							<div className="catalog-row">
								<Chip color="primary" variant="flat">
									冷藏配送
								</Chip>
								<Chip color="success" variant="dot">
									已完成
								</Chip>
								<Chip color="warning" variant="dot">
									待確認
								</Chip>
							</div>
						)}
						{component === "cards" && (
							<Card>
								<CardHeader>
									<strong>花蓮小農香米</strong>
								</CardHeader>
								<CardBody>每日餐桌上的安心選擇。</CardBody>
							</Card>
						)}
					</div>
				</section>
				<FlutterCatalog app="buyer" component={component} />
				<FlutterCatalog app="seller" component={component} />
			</div>
		</main>
	);
}
const meta = {
	title: "Cross Platform/Comparison",
	component: Comparison,
	args: { component: "buttons" },
	tags: ["!test"],
	parameters: { layout: "padded" },
} satisfies Meta<typeof Comparison>;
export default meta;
export const Overview: StoryObj<typeof meta> = {};
export const Inputs: StoryObj<typeof meta> = { args: { component: "inputs" } };
export const Chips: StoryObj<typeof meta> = { args: { component: "chips" } };
export const Cards: StoryObj<typeof meta> = { args: { component: "cards" } };
