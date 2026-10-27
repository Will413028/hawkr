import AnchorLink from "@/components/shared/AnchorLink";
import Link from "@/components/shared/Link";
import "@/styles/features/home.css";
import "@/styles/features/about.css";
import HomeProcessCards from "@/features/home/components/HomeProcessCards";
import { openhuninn } from "@/lib/fonts/openhuninn";
import { zenMaruGothic } from "@/lib/fonts/zenMaruGothic";
import AboutHeroMascot from "./components/AboutHeroMascot";
import BrandStoryTrailSection from "./components/BrandStoryTrailSection";
import HowItWorksDemoSection from "./components/HowItWorksDemoSection";

export default function AboutPage() {
	return (
		<main
			className={`hkh-page ${zenMaruGothic.variable} ${openhuninn.variable}`}
		>
			<section className="hero about-hero">
				<div className="hero-grid about-hero-grid">
					<div className="about-hero-content">
						<div className="hero-copy about-hero-heading">
							<div className="eyebrow">
								<span className="eb-dot" />
								認識 Hawkr · ABOUT
							</div>
							<h1 className="max-w-[14ch]">
								每日好食材,
								<br />
								<em>宅配送到家</em>
							</h1>
						</div>

						<div className="hero-copy about-hero-body">
							<p className="lede">
								{
									"「Hawkr」把在地小農、職人商家與冷鏈配送接在一起。從蔬菜水果、蛋奶肉品，到地方工夫食品 ——我們不種田、不下田，只是把好東西從產地誠實地搬到你家。"
								}
							</p>

							<div className="hero-ctas">
								<Link className="btn btn-primary" href="/market/products">
									開始逛市集 →
								</Link>
								<AnchorLink className="btn btn-ghost" href="#how-it-works-demo">
									看看怎麼運作
								</AnchorLink>
							</div>

							<ul className="hero-trust">
								<li>
									<i aria-hidden="true" />
									在地商家直送
								</li>
								<li>
									<i aria-hidden="true" />
									冷藏冷凍配送
								</li>
								<li>
									<i aria-hidden="true" />
									安心付款
								</li>
								<li>
									<i aria-hidden="true" />
									看得到產地故事
								</li>
							</ul>
						</div>
					</div>

					<AboutHeroMascot />
				</div>
			</section>

			<HomeProcessCards />
			<HowItWorksDemoSection />
			<BrandStoryTrailSection />
		</main>
	);
}
