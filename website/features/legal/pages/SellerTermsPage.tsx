import { LegalPage } from "@/features/legal";
import { LegalBlocks } from "@/features/legal/documents/LegalBlocks";
import type { LegalBlock } from "@/features/legal/documents/legalDocument";
import { SELLER_TERMS } from "@/features/legal/documents/sellerTerms.gen";
import { pageMetadata } from "@/lib/seo/metadata";

export const generateMetadata = pageMetadata(
	{
		title: "賣家契約",
		description:
			"Hawkr 示範專案的賣家合作暨平台服務契約說明：說明本站是作品集的示範專案，不提供真實交易，不構成任何契約或法律義務。",
		path: "/seller-terms",
	},
	{ robots: { index: true, follow: true } },
);

interface TermsSection {
	id: string;
	title: string;
	blocks: LegalBlock[];
}

function sectionsOf(blocks: readonly LegalBlock[]): TermsSection[] {
	const sections: TermsSection[] = [
		{ id: "seller-terms-preamble", title: "前言", blocks: [] },
	];
	for (const block of blocks) {
		if (block.type === "h2") {
			sections.push({
				id: `seller-terms-${sections.length}`,
				title: block.text,
				blocks: [],
			});
		} else {
			sections[sections.length - 1].blocks.push(block);
		}
	}
	return sections;
}

export default function SellerTermsPage() {
	return (
		<LegalPage
			solidCards
			title={SELLER_TERMS.title}
			enTitle="Seller Agreement"
			lastUpdated={SELLER_TERMS.version}
			description={SELLER_TERMS.versionLine}
			sections={sectionsOf(SELLER_TERMS.blocks).map((section) => ({
				id: section.id,
				title: section.title,
				content: <LegalBlocks blocks={section.blocks} />,
			}))}
		/>
	);
}
