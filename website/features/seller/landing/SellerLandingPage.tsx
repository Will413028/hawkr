import AnchorLink from "@/components/shared/AnchorLink";
import AppImage from "@/components/shared/AppImage";
import BrandLogo from "@/components/shared/BrandLogo";
import Link from "@/components/shared/Link";
import "@/styles/features/home.css";
import "@/styles/features/seller-feature-overview.css";
import "@/styles/features/seller-onboarding.css";
import "@/styles/features/seller-recruitment-scroll-world.css";
import "@/styles/features/seller-showcase.css";
import "@/styles/features/desktop-frame.css";
import { openhuninn } from "@/lib/fonts/openhuninn";
import { zenMaruGothic } from "@/lib/fonts/zenMaruGothic";
import SellerAnimations from "./components/SellerAnimations";
import SellerFeatureOverview from "./components/SellerFeatureOverview";
import SellerPartnerExpansion from "./components/SellerPartnerExpansion";
import SellerRecruitmentScrollWorld from "./components/SellerRecruitmentScrollWorld";
import SellerShowcaseCarousel from "./components/SellerShowcaseCarousel";
import SellerShowcaseImageWarmer from "./components/SellerShowcaseImageWarmer";

interface ProcessStep {
	title: string;
	body: string;
	visual: {
		src: string;
		alt: string;
	};
}

const STEPS: ProcessStep[] = [
	{
		title: "建立賣場資料",
		body: "填寫商家名稱、所在地、寄件地址與商店介紹，建立你的品牌賣場。",
		visual: {
			src: "/placeholders/web-screen.svg",
			alt: "Hawkr 建立賣場畫面，包含商家名稱、所在地與寄件地址欄位",
		},
	},
	{
		title: "上架你的商品",
		body: "上傳商品照片、填好價格與庫存，再完成適用的金流、物流設定與必要審核。",
		visual: {
			src: "/placeholders/web-screen.svg",
			alt: "Hawkr 賣家中心商品管理畫面，顯示商品、價格與庫存狀態",
		},
	},
	{
		title: "接單與出貨",
		body: "商品完成必要審核並開始販售後，可在賣家中心查看訂單、備貨與適用的出貨方式。",
		visual: {
			src: "/placeholders/web-screen.svg",
			alt: "Hawkr 賣家中心訂單管理畫面，顯示待出貨訂單與預約取貨操作",
		},
	},
];

const FAQS = [
	{
		question: "需要先準備好產品照、商品描述才能建立賣場嗎?",
		answer:
			"不用。你可以先建立賣場；商品正式販售前，仍需補齊商品資料並完成必要審核。若需要拍攝、文案、上架或方案協助，再聯絡合作顧問。",
		open: true,
	},
	{
		question: "合作方案與結算方式怎麼確認?",
		answer:
			"可先查看公開方案；建立賣場不代表方案或結算條件已生效。實際合作條件依雙方確認的合約與帳務資料為準，不確定時可聯絡合作顧問。",
	},
	{
		question: "物流和金流要自己處理嗎?",
		answer:
			"平台會協助串接合作金流與物流服務。實際可用的配送方式、費用、款項結算週期及入帳帳戶，依合作方案、審核結果與每期結算明細為準。",
	},
	{
		question: "我只有 5 樣商品,可以開店嗎?",
		answer:
			"可以先建立賣場並填寫商品資料。個別商品能否販售，仍依品類、資料完整度與審核結果確認；平台不保證曝光、成交或回購。",
	},
	{
		question: "出貨流程是什麼樣子?",
		answer:
			"顧客下單後，可在賣家中心查看訂單與適用的出貨方式。賣家仍需確認接單、備貨與出貨；可用物流與取件方式依申請、地區與啟用狀態為準。",
	},
	{
		question: "如果遇到客訴或退貨怎麼辦?",
		answer:
			"賣家可以經由售後處理系統管理售後案件。爭議處理流程透明，結果有紀錄，讓買賣雙方都有依據。",
	},
];

function CharSplit({ text }: { text: string }) {
	return (
		<>
			{Array.from(text).map((ch, i) => (
				<span
					className="char"
					// biome-ignore lint/suspicious/noArrayIndexKey: text is static
					key={i}
				>
					{ch}
				</span>
			))}
		</>
	);
}

function SpinBadge() {
	return (
		<div className="spin-badge" aria-hidden="true">
			<svg
				className="spin-badge__stack"
				viewBox="0 0 200 200"
				aria-hidden="true"
			>
				<circle className="ring-outer" cx="100" cy="100" r="100" />
				<circle className="ring-brand" cx="100" cy="100" r="96" />
				<circle className="ring-gap" cx="100" cy="100" r="65" />
				<circle className="ring-dashed" cx="100" cy="100" r="73" />
				<circle className="ring-center" cx="100" cy="100" r="59" />
			</svg>
			<svg
				className="spin-badge__ring"
				viewBox="0 0 200 200"
				aria-hidden="true"
			>
				<defs>
					<path
						id="sellerSpinTextPath"
						d="M 100,100 m -82,0 a 82,82 0 1,1 164,0 a 82,82 0 1,1 -164,0"
					/>
				</defs>
				<text>
					<textPath href="#sellerSpinTextPath" startOffset="0">
						★ 新註冊有保庇 · 賣東西有好運 · 新註冊有保庇 · 賣東西有好運
					</textPath>
				</text>
			</svg>
			<div className="spin-badge__center">
				<AppImage
					src="/placeholders/badge-character.svg"
					alt=""
					width={2048}
					height={2048}
					loading="eager"
				/>
			</div>
			<span className="spin-badge__tag">NEW</span>
		</div>
	);
}

export default function SellerLandingPage() {
	return [
		<SellerRecruitmentScrollWorld
			key="seller-scroll-world"
			className={`${zenMaruGothic.variable} ${openhuninn.variable}`}
		/>,
		<main
			key="seller-landing"
			className={`seller-onboarding hkh-page ${zenMaruGothic.variable} ${openhuninn.variable}`}
		>
			<SellerShowcaseImageWarmer />

			<section className="seller-hero seller-snap-section" data-seller-snap>
				<div className="seller-hero-inner">
					<div className="eyebrow eyebrow--hashtags">
						<span>#商家招募</span>
						<span>#Hawkr 賣家</span>
					</div>
					<h1>
						生鮮商品
						<br />
						也能被<em>萬眾矚目</em>嗎?
					</h1>
					<p className="lede">
						{
							"「Hawkr」是小農、鮮食與食品品牌的自助開店市集。先建立市集內品牌賣場，再依步驟補齊商品、金流與物流設定；完成必要審核後開始販售。"
						}
					</p>
					<div className="hero-ctas">
						<Link className="btn btn-primary" href="/seller/onboarding">
							開始建立賣場 →
						</Link>
						<Link className="btn btn-ghost" href="/seller/pricing">
							比較開店方案
						</Link>
						<AnchorLink className="btn btn-ghost" href="#how">
							看看怎麼開始
						</AnchorLink>
					</div>
				</div>
				<SpinBadge />
			</section>

			<SellerFeatureOverview />

			<SellerPartnerExpansion />

			<section
				className="seller-process seller-snap-section"
				id="how"
				data-seller-snap
			>
				<div className="process-glass">
					<header className="sect-head sect-head--center">
						<div>
							<div className="eyebrow eyebrow--hashtags">
								<span>#開店三步驟</span>
								<span>#自助開店</span>
							</div>
							<h2>
								三步驟,
								<br />
								完成<em>開店準備</em>
							</h2>
							<p>
								先建立賣場，再補齊商品與必要設定；完成審核後開始販售，遇到問題再請團隊協助。
							</p>
						</div>
					</header>

					<ol className="process-steps" aria-label="自助開店三步驟">
						{STEPS.map((step) => (
							<li className="process-step" key={step.title}>
								<h3>{step.title}</h3>
								<figure className="process-step__visual">
									{/* Eager, not lazy: this landing page materialises all of its
									    showcase imagery up front (see SellerShowcaseImageWarmer +
									    the eager carousel slides) so nothing loads only because
									    the user scrolled. process-apply.png is the one process
									    image not already warmed/eager elsewhere, so a lazy tag
									    made it fetch mid-scroll — a stability regression the
									    long-page E2E gate rejects. */}
									<AppImage
										className="process-step__image"
										src={step.visual.src}
										alt={step.visual.alt}
										width={1040}
										height={650}
										sizes="(max-width: 880px) 84vw, 30vw"
										loading="eager"
									/>
								</figure>
								<p>{step.body}</p>
							</li>
						))}
					</ol>
				</div>
			</section>

			<SellerShowcaseCarousel />

			<section className="story-trail seller-story-trail">
				<header className="trail-head">
					<div className="eyebrow eyebrow--hashtags">
						<span>#自主營運路徑</span>
						<span>#從建店到接單</span>
					</div>
					<h2>
						從<em>建立賣場</em>到<em>自行接單</em>
					</h2>
				</header>

				<div className="story-trail-inner">
					<svg
						className="story-trail-path"
						viewBox="0 0 1180 1100"
						preserveAspectRatio="none"
						aria-hidden="true"
					>
						<path d="M 230 200 C 360 320, 900 320, 950 540 C 1000 760, 380 780, 280 920" />
					</svg>

					<article className="trail-stop">
						<div className="trail-illus-wrap">
							<div
								className="trail-illus trail-illus--field"
								style={{ backgroundPosition: "center center", opacity: 1 }}
							/>
							<span
								className="trail-mascot trail-mascot--field"
								aria-hidden="true"
							>
								<BrandLogo variant="mark" size="fill" />
							</span>
						</div>
						<div className="trail-copy">
							<span className="trail-day">START</span>
							<h3>
								先把<em>賣場建立好</em>
							</h3>
							<p>
								依畫面填寫商家名稱、地址與介紹，不必先送出人工聯絡表；需要協助時，再找團隊討論。
							</p>
						</div>
					</article>

					<article className="trail-stop">
						<div className="trail-illus-wrap">
							<div
								className="trail-illus trail-illus--orchard"
								style={{ backgroundPosition: "center bottom", opacity: 0.8 }}
							/>
							<span
								className="trail-mascot trail-mascot--orchard"
								aria-hidden="true"
							>
								<BrandLogo variant="mark" size="fill" />
							</span>
						</div>
						<div className="trail-copy">
							<span className="trail-day">READY</span>
							<h3>
								完成<em>上架前準備</em>
							</h3>
							<p>
								補齊商品資料與適用的認證、收款、物流設定；各項服務依審核與啟用狀態使用。
							</p>
						</div>
					</article>

					<article className="trail-stop">
						<div className="trail-illus-wrap">
							<div
								className="trail-illus trail-illus--wheat"
								style={{ backgroundPosition: "center bottom", opacity: 0.8 }}
							/>
							<span
								className="trail-mascot trail-mascot--wheat"
								aria-hidden="true"
							>
								<BrandLogo variant="mark" size="fill" />
							</span>
						</div>
						<div className="trail-copy">
							<span className="trail-day">OPERATE</span>
							<h3>
								用賣家中心<em>持續經營</em>
							</h3>
							<p>
								自行管理商品、庫存、訂單與出貨。實際曝光、成交與回購仍取決於商品、營運及市場條件。
							</p>
						</div>
					</article>
				</div>
			</section>

			<section
				className="seller-faq seller-snap-section"
				aria-labelledby="seller-faq-title"
				data-seller-snap
			>
				<header className="sect-head sect-head--center">
					<div>
						<div className="eyebrow eyebrow--hashtags">
							<span>#開店FAQ</span>
							<span>#開店前必看</span>
						</div>
						<h2 id="seller-faq-title">
							開始前,你可能想<em>先問清楚</em>
						</h2>
					</div>
				</header>

				<div className="faq-list">
					{FAQS.map((item) => (
						<details className="faq-item" key={item.question} open={item.open}>
							<summary>
								<span className="faq-q">{item.question}</span>
								<span className="faq-toggle" aria-hidden="true" />
							</summary>
							<div className="faq-a">
								<p>{item.answer}</p>
							</div>
						</details>
					))}
				</div>
			</section>

			<section
				className="seller-cta seller-snap-section"
				id="seller"
				data-seller-snap
			>
				<div className="seller-inner">
					<div className="eyebrow eyebrow--hashtags eyebrow--on-dark">
						<span>#加入 Hawkr</span>
						<span>#一起開店</span>
					</div>
					<h2 className="seller-h" id="apply">
						<CharSplit text="準備好了?" />
						<br />
						<CharSplit text="一起" />
						<em>
							<CharSplit text="Hawkr" />
						</em>
					</h2>
					{/* 「攤商」與「我們」之間原本是 `——`，490f0a80 那次文案改寫把本頁的
					    破折號一律換成全形逗號（同檔 lede 即是），這一句漏了替換，只剩
					    JSX 換行折出來的半形空白在頂替標點。補回逗號。 */}
					<p className="seller-p">
						{
							"不論你是田裡的小農、職人加工坊，還是想轉型的傳統市場攤商，我們都想認識你。"
						}
					</p>
					<div className="seller-row">
						<Link className="btn btn-primary btn-lg" href="/seller/onboarding">
							開始建立賣場 →
						</Link>
						<Link
							className="btn btn-ghost btn-ghost--on-dark btn-lg"
							href="/seller/pricing"
						>
							查看合作方案
						</Link>
						<Link
							className="btn btn-ghost btn-ghost--on-dark btn-lg"
							href="/contact"
						>
							聯繫合作顧問
						</Link>
					</div>
				</div>
			</section>

			<SellerAnimations />
		</main>,
	];
}
