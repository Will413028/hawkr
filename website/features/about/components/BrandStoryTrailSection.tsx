import BrandLogo from "@/components/shared/BrandLogo";

/**
 * 品牌使命 3 卡 — copied from HomePage's `.story-trail` section so the
 * /about page can render the trust narrative independently. PR3 will
 * thin HomePage and let it import this same component instead of the
 * duplicate inline JSX. Classnames intentionally preserved to inherit
 * the existing CSS in `styles/features/home.css`.
 */
export default function BrandStoryTrailSection() {
	return (
		<section className="story-trail">
			<header className="trail-head">
				<div className="eyebrow">
					<span className="eb-dot" />
					安心購買 · WHY HAWKR
				</div>
				<h2>
					為什麼在<em>Hawkr</em>買？
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
						<div className="trail-illus trail-illus--field" />
						<span
							className="trail-mascot trail-mascot--field trail-mascot--portrait"
							aria-hidden="true"
						>
							<BrandLogo variant="mark" size="fill" />
						</span>
					</div>
					<div className="trail-copy">
						<h3>
							商家<em>認證</em>
						</h3>
						<p>
							合作商家需完成基本資料與身份確認，讓你知道正在向誰購買 ——
							每一筆訂單都能追溯到田裡的那個人。
						</p>
					</div>
				</article>

				<article className="trail-stop">
					<div className="trail-illus-wrap">
						<div className="trail-illus trail-illus--orchard" />
						<span
							className="trail-mascot trail-mascot--orchard trail-mascot--portrait"
							aria-hidden="true"
						>
							<BrandLogo variant="mark" size="fill" />
						</span>
					</div>
					<div className="trail-copy">
						<h3>
							交易有<em>保障</em>
						</h3>
						<p>
							結帳、付款、訂單通知與售後申請都在平台內完成，不用再翻私訊確認金額，售後爭議「Hawkr」介入調節。
						</p>
					</div>
				</article>

				<article className="trail-stop">
					<div className="trail-illus-wrap">
						<div className="trail-illus trail-illus--wheat" />
						<span
							className="trail-mascot trail-mascot--wheat trail-mascot--portrait"
							aria-hidden="true"
						>
							<BrandLogo variant="mark" size="fill" />
						</span>
					</div>
					<div className="trail-copy">
						<h3>
							找到<em>在地特色</em>
						</h3>
						<p>
							從產地、節氣到職人 ——
							商品頁清楚標示產地、商家、配送方式與保存資訊，讓你買的不只是食材，還是一段故事。
						</p>
					</div>
				</article>
			</div>
		</section>
	);
}
