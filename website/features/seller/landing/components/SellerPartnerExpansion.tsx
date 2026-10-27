import AppImage from "@/components/shared/AppImage";
import BrandLogo from "@/components/shared/BrandLogo";
import SellerSpotlightCard from "./SellerSpotlightCard";

const PARTNER_TYPES = [
	{
		index: "01",
		label: "產地小農",
		title: "把產地與產期說清楚",
		body: "有明確的產地、產期與供貨方式，希望多一個線上接單管道。",
		scene: "/placeholders/scene-irrigation.svg",
	},
	{
		index: "02",
		label: "市場與生鮮門市",
		title: "把熟悉的品項搬到線上",
		body: "商品已經賣得熟，想把規格、庫存與訂單整理在同一個地方。",
		scene: "/placeholders/scene-market.svg",
	},
	{
		index: "03",
		label: "食品品牌與加工職人",
		title: "讓買家看懂你的講究",
		body: "有清楚的成分、規格與保存方式，也有一段值得被認識的品牌故事。",
		scene: "/placeholders/scene-greenhouse.svg",
	},
] as const;

export default function SellerPartnerExpansion() {
	return (
		<section
			className="seller-fit seller-snap-section"
			aria-labelledby="seller-fit-title"
			data-seller-snap
		>
			<header className="sect-head sect-head--center seller-fit__head">
				<div>
					<div className="eyebrow eyebrow--hashtags">
						<span>#適合誰加入</span>
						<span>#好商品上架</span>
					</div>
					<h2 id="seller-fit-title">
						有好商品，<em>上架門檻高嗎？</em>
					</h2>
					<p>
						{
							"不用先準備一套完整網站。把商品照片、規格與供貨方式整理好，從商店資料開始，一步一步上架。"
						}
					</p>
				</div>
			</header>

			<div className="seller-fit__grid">
				{PARTNER_TYPES.map((partner) => (
					<SellerSpotlightCard className="seller-fit-card" key={partner.label}>
						<div className="seller-fit-card__art" aria-hidden="true">
							<AppImage
								className="seller-fit-card__scene"
								src={partner.scene}
								alt=""
								fill
								sizes="(max-width: 720px) 100vw, 33vw"
							/>
							<span className="seller-fit-card__mascot">
								<BrandLogo variant="mark" size="fill" />
							</span>
						</div>
						<div className="seller-fit-card__copy">
							<div className="seller-fit-card__meta">
								<span>{partner.index}</span>
								{partner.label}
							</div>
							<h3>{partner.title}</h3>
							<p>{partner.body}</p>
						</div>
					</SellerSpotlightCard>
				))}
			</div>
		</section>
	);
}
