import { LegalPage } from "@/features/legal";
import DemoNotice from "@/features/legal/DemoNotice";
import { LegalBlocks } from "@/features/legal/documents/LegalBlocks";
import { FOOD_REGISTRATION_NOTICE } from "@/features/legal/documents/sellerFoodRegistrationNotice.gen";
import { KYC_PRIVACY_NOTICE } from "@/features/legal/documents/sellerKycNotice.gen";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata(
	{
		title: "隱私政策",
		description:
			"Hawkr 示範專案的隱私政策頁：說明本站是作品集的示範專案，不收集、不處理真實的個人資料，請勿輸入真實資料。",
		path: "/privacy",
	},
	{ robots: { index: true, follow: true } },
);

export default function PrivacyPage() {
	return (
		<LegalPage
			solidCards
			title="隱私政策"
			enTitle="Privacy Policy"
			lastUpdated="2026-10-25"
			description="本站是作品集的示範專案，不收集、不處理真實的個人資料，本頁只用來展示隱私政策的版面。"
			sections={[
				{
					id: "demo-notice",
					title: "一、示範專案說明",
					content: <DemoNotice topic="隱私政策" />,
				},
				{
					id: "seller-kyc-notice",
					title: "二、賣家身分認證告知事項",
					content: (
						<>
							<p>{`本節版本：${KYC_PRIVACY_NOTICE.version}`}</p>
							<LegalBlocks blocks={KYC_PRIVACY_NOTICE.blocks} />
						</>
					),
				},
				{
					id: "seller-food-registration-notice",
					title: "三、食品業者登錄告知事項",
					content: (
						<>
							<p>{`本節版本：${FOOD_REGISTRATION_NOTICE.version}`}</p>
							<LegalBlocks blocks={FOOD_REGISTRATION_NOTICE.blocks} />
						</>
					),
				},
			]}
		/>
	);
}
