import { notFound } from "next/navigation";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { AI_LISTING_ASSISTANT_ENABLED } from "@/features/seller/components/AiAssistantPanel";

export const metadata = { title: "AI 工具 — 賣家中心" };

const TOOLS: Array<{ label: string; desc: string; where: string }> = [
	{
		label: "AI 商品文案（上架小幫手）",
		desc: "輸入關鍵字或丟一張照片，自動生成商品名稱、產地、描述、標籤與規格。",
		where: "新增商品頁右上角的「AI 助手」按鈕",
	},
	{
		label: "AI 圖片美化",
		desc: "7 種預設風格（新鮮自然、白底極淨、高級禮盒…）或自訂提示詞，一鍵美化農產品照片。",
		where: "上架商品上傳圖片，或發佈貼文選圖時的「AI 美化」",
	},
];

function SparkleIcon() {
	return <AppIcon name="sparkle" size={28} />;
}

export default function Page() {
	if (!AI_LISTING_ASSISTANT_ENABLED) {
		notFound();
	}

	return (
		<div className="seller-ai">
			<header className="seller-ai__head">
				<div>
					<h1>AI 工具</h1>
					<p>商品文案生成、圖片美化，協助你更快上架。</p>
				</div>
				<span className="seller-ai__pill">已上線</span>
			</header>

			<div className="seller-card seller-ai__hero">
				<div className="seller-ai__hero-icon" aria-hidden="true">
					<SparkleIcon />
				</div>
				<div>
					<h2>用 AI 把上架時間從半小時砍到 3 分鐘</h2>
					<p>
						商品文案與圖片美化都已上線。到
						<Link href="/seller-center/products/new"> 新增商品 </Link>
						頁就能同時用到：右上角「AI 助手」生成文案，上傳圖片時用「AI
						美化」修圖；發佈貼文選圖時也能美化。
					</p>
				</div>
			</div>

			<section className="seller-card">
				<div className="seller-card__head">
					<h2>可用的 AI 工具</h2>
				</div>
				<ul className="seller-ai__plan">
					{TOOLS.map((item) => (
						<li key={item.label}>
							<div>
								<strong>{item.label}</strong>
								<small>{item.desc}</small>
								<small>使用位置：{item.where}</small>
							</div>
						</li>
					))}
				</ul>
			</section>

			<div className="seller-ai__actions">
				<Button href="/seller-center/products/new">前往新增商品</Button>
				<Button
					href="/seller-center/posts/new"
					variant="bordered"
					color="default"
				>
					發佈貼文
				</Button>
			</div>
		</div>
	);
}
