import { LegalPage } from "@/features/legal";
import DemoNotice from "@/features/legal/DemoNotice";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata(
	{
		title: "服務條款",
		description:
			"Hawkr 示範專案的服務條款頁：說明本站是作品集的示範專案，不提供真實交易、付款或配送，不構成任何契約或法律義務。",
		path: "/terms",
	},
	{ robots: { index: true, follow: true } },
);

export default function TermsPage() {
	return (
		<LegalPage
			solidCards
			title="服務條款"
			enTitle="Terms of Service"
			lastUpdated="2026-10-25"
			description="本站是作品集的示範專案，不提供真實交易，本頁只用來展示服務條款的版面。"
			sections={[
				{
					id: "demo-notice",
					title: "一、示範專案說明",
					content: (
						<>
							<DemoNotice topic="服務條款" />
						</>
					),
				},
				{
					id: "fees",
					title: "二、費用",
					content: (
						<>
							<p>
								本站不提供真實交易，不收取任何費用；頁面上出現的金額與費率都是虛構的示範資料。
							</p>
						</>
					),
				},
			]}
		/>
	);
}
