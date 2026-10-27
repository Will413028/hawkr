import { LegalPage } from "@/features/legal";
import DemoNotice from "@/features/legal/DemoNotice";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata(
	{
		title: "使用者政策",
		description:
			"Hawkr 示範專案的使用者政策頁：說明本站是作品集的示範專案，不收集真實的個人資料，請勿輸入真實資料。",
		path: "/user-policy",
	},
	{ robots: { index: true, follow: true } },
);

export default function UserPolicyPage() {
	return (
		<LegalPage
			solidCards
			title="使用者政策"
			enTitle="User Policy / Acceptable Use Policy"
			lastUpdated="2026-10-25"
			description="本站是作品集的示範專案，本頁只用來展示使用者政策的版面。"
			sections={[
				{
					id: "demo-notice",
					title: "一、示範專案說明",
					content: (
						<>
							<DemoNotice topic="使用者政策" />
						</>
					),
				},
			]}
		/>
	);
}
