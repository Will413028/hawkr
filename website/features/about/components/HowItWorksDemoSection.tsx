import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";

/**
 * 3-step phone-mockup demo — copied from HomePage's `.demo` section.
 * Same PR3 thinning story as BrandStoryTrailSection.
 */
export default function HowItWorksDemoSection() {
	return (
		<section className="demo" id="how-it-works-demo">
			<header className="sect-head sect-head--center">
				<div>
					<div className="eyebrow">
						<span className="eb-dot" />
						使用流程 · HOW IT WORKS
					</div>
					<h2>
						從想吃什麼，
						<br />
						到送到家，<em>只要幾步</em>
					</h2>
					<p>
						不用在不同社群、私訊和轉帳紀錄裡來回確認
						——「Hawkr」把找商品、看商家、下單付款與售後流程整理在一起。
					</p>
				</div>
			</header>
			<div className="demo-grid">
				<article className="demo-step">
					<div className="phone phone--bare phone--tilt-l">
						<AppImage
							className="phone-screen"
							src="/placeholders/phone-screen.svg"
							alt="搜尋首頁畫面"
							width={520}
							height={1080}
							loading="eager"
							fetchPriority="low"
							sizes="(max-width: 760px) 280px, 320px"
							style={{ width: "100%", height: "auto" }}
						/>
						<span className="phone-tag pt-1">
							<AppIcon
								name="sparkle"
								size={14}
								className="inline-block mr-1 align-[-0.15em]"
							/>
							情境推薦
						</span>
						<span className="phone-tag pt-2">
							<AppIcon
								name="bolt"
								size={14}
								className="inline-block mr-1 align-[-0.15em]"
							/>
							快速找到
						</span>
					</div>
					<div className="step-meta">
						<div className="step-num">01</div>
						<h3>搜尋想吃的</h3>
						<p>
							輸入番茄、雞蛋、虱目魚，或從「今晚三菜一湯」、「週末火鍋」、「送長輩禮盒」等情境開始找。
						</p>
					</div>
				</article>

				<article className="demo-step">
					<div className="phone phone--bare phone--tilt-r">
						<AppImage
							className="phone-screen"
							src="/placeholders/phone-screen.svg"
							alt="商品頁畫面"
							width={520}
							height={1080}
							loading="eager"
							fetchPriority="low"
							sizes="(max-width: 760px) 280px, 320px"
							style={{ width: "100%", height: "auto" }}
						/>
						<span className="phone-tag pt-1">
							<AppIcon
								name="location"
								size={14}
								className="inline-block mr-1 align-[-0.15em]"
							/>
							看得到產地
						</span>
						<span className="phone-tag pt-2">
							<AppIcon
								name="user"
								size={14}
								className="inline-block mr-1 align-[-0.15em]"
							/>
							看得到商家
						</span>
					</div>
					<div className="step-meta">
						<div className="step-num">02</div>
						<h3>看清楚來源</h3>
						<p>
							商品頁清楚顯示產地、商家、保存方式與配送溫層。喜歡這間店，也可以直接追蹤，下次上架不漏接。
						</p>
					</div>
				</article>

				<article className="demo-step">
					<div className="phone phone--bare phone--tilt-l">
						<AppImage
							className="phone-screen"
							src="/placeholders/phone-screen.svg"
							alt="訂單列表畫面"
							width={520}
							height={1080}
							loading="eager"
							fetchPriority="low"
							sizes="(max-width: 760px) 280px, 320px"
							style={{ width: "100%", height: "auto" }}
						/>
						<span className="phone-tag pt-1">
							<AppIcon
								name="clipboard"
								size={14}
								className="inline-block mr-1 align-[-0.15em]"
							/>
							訂單清楚
						</span>
						<span className="phone-tag pt-2">
							<AppIcon
								name="shield"
								size={14}
								className="inline-block mr-1 align-[-0.15em]"
							/>
							售後保障
						</span>
					</div>
					<div className="step-meta">
						<div className="step-num">03</div>
						<h3>安心下單</h3>
						<p>
							結帳、付款、訂單通知與售後申請都在平台內完成。不用再翻私訊確認金額，也不用擔心找不到訂單紀錄。
						</p>
					</div>
				</article>
			</div>
		</section>
	);
}
