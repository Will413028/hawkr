import { LegalPage } from "@/features/legal";
import DemoNotice from "@/features/legal/DemoNotice";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata(
	{
		title: "退換貨政策",
		description:
			"Hawkr 示範專案的退換貨政策頁：說明本站是作品集的示範專案，不提供真實交易、付款或配送，沒有實際的退換貨與退款。",
		path: "/return-policy",
	},
	{ robots: { index: true, follow: true } },
);

export default function ReturnPolicyPage() {
	return (
		<LegalPage
			solidCards
			title="退換貨政策"
			enTitle="Return and Refund Policy"
			lastUpdated="2026-10-25"
			description="本站是作品集的示範專案，沒有實際的退換貨與退款，本頁只用來展示退換貨政策的版面。"
			sections={[
				{
					id: "demo-notice",
					title: "一、示範專案說明",
					content: (
						<>
							<DemoNotice topic="退換貨與退款" />
						</>
					),
				},
			]}
		/>
	);
}
