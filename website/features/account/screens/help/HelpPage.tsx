import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";

export const metadata = { title: "客服中心" };

const FAQ = [
	{
		q: "訂單何時出貨？",
		a: "賣家通常在收到訂單後 1-3 個工作天備貨並交付物流。可在訂單詳情看到即時狀態。",
	},
	{
		q: "如何申請退貨？",
		a: "進入訂單詳情頁，點選「申請退換貨」即可選擇商品、原因與退貨方式。請於收到貨後 7 天內提出。",
	},
	{
		q: "會員卡是什麼？",
		a: "在任一商家完成首筆訂單後，系統會自動為你建立該商家的會員卡，記錄你在這家的累計消費；可在「會員卡」頁查看。",
	},
	{
		q: "怎麼追蹤喜歡的小農？",
		a: "進入小農頁，點「追蹤」即可。追蹤後他發新動態或上架新商品時你會收到通知。",
	},
	{
		q: "我可以同時是買家又是賣家嗎？",
		a: "可以。在 navbar 個人選單可隨時切換買家/賣家身份，兩個帳號互不影響。",
	},
];

export default function Page() {
	return (
		<main className="help-page">
			<SellerPageShell
				title="客服中心"
				subtitle="我們希望你逛得安心、買得開心、吃得健康。"
			>
				<section className="seller-card">
					<header className="seller-card__head">
						<h2>常見問題</h2>
					</header>
					<ul className="list-none m-0 p-0 flex flex-col gap-2">
						{FAQ.map((item) => (
							<li key={item.q}>
								<details className="group bg-content2 rounded-hkui-md p-[12px_16px]">
									<summary className="list-none [&::-webkit-details-marker]:hidden font-semibold cursor-pointer py-1 select-none flex items-center before:content-['›'] before:inline-block before:mr-2 before:transition-transform before:duration-200 group-open:before:rotate-90">
										{item.q}
									</summary>
									<p className="mt-2 text-sm text-foreground-muted leading-relaxed">
										{item.a}
									</p>
								</details>
							</li>
						))}
					</ul>
				</section>

				<section className="seller-card">
					<header className="seller-card__head">
						<h2>聯絡我們</h2>
					</header>
					<ul className="list-none m-0 p-0 flex flex-col gap-2">
						<li>
							<Link
								href="/account/messages"
								className="flex flex-col gap-0.5 p-3 px-4 bg-hk-surface-muted rounded-hk-md text-inherit no-underline transition-colors duration-200 ease-in-out hover:bg-hk-surface-panel"
							>
								<strong className="text-sm font-semibold text-hk-heading">
									聯絡客服
								</strong>
								<small className="text-xs text-foreground-soft">
									透過站內訊息發送（24 小時內回覆）
								</small>
							</Link>
						</li>
						<li>
							<Link
								href="/contact"
								className="flex flex-col gap-0.5 p-3 px-4 bg-hk-surface-muted rounded-hk-md text-inherit no-underline transition-colors duration-200 ease-in-out hover:bg-hk-surface-panel"
							>
								<strong className="text-sm font-semibold text-hk-heading">
									商業合作 / 媒體聯絡
								</strong>
								<small className="text-xs text-foreground-soft">
									supports@example.com
								</small>
							</Link>
						</li>
					</ul>
				</section>

				<section className="seller-card">
					<header className="seller-card__head">
						<h2>政策與條款</h2>
					</header>
					<ul className="list-none m-0 p-0 flex flex-col gap-2">
						<li>
							<Link
								href="/return-policy"
								className="flex flex-col gap-0.5 p-3 px-4 bg-hk-surface-muted rounded-hk-md text-inherit no-underline transition-colors duration-200 ease-in-out hover:bg-hk-surface-panel"
							>
								<strong className="text-sm font-semibold text-hk-heading">
									退換貨政策
								</strong>
								<small className="text-xs text-foreground-soft">
									適用範圍 / 流程 / 時效
								</small>
							</Link>
						</li>
						<li>
							<Link
								href="/terms"
								className="flex flex-col gap-0.5 p-3 px-4 bg-hk-surface-muted rounded-hk-md text-inherit no-underline transition-colors duration-200 ease-in-out hover:bg-hk-surface-panel"
							>
								<strong className="text-sm font-semibold text-hk-heading">
									服務條款
								</strong>
								<small className="text-xs text-foreground-soft" />
							</Link>
						</li>
						<li>
							<Link
								href="/privacy"
								className="flex flex-col gap-0.5 p-3 px-4 bg-hk-surface-muted rounded-hk-md text-inherit no-underline transition-colors duration-200 ease-in-out hover:bg-hk-surface-panel"
							>
								<strong className="text-sm font-semibold text-hk-heading">
									隱私政策
								</strong>
								<small className="text-xs text-foreground-soft" />
							</Link>
						</li>
					</ul>
				</section>
			</SellerPageShell>
		</main>
	);
}
