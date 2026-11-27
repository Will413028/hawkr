import SellerInquiryDetailPage from "@/features/seller/inquiries/SellerInquiryDetailPage";

export const metadata = { title: "平台詢問" };

interface PageProps {
	params: Promise<{ inquiryId: string }>;
}

export function generateStaticParams() {
	return [{ inquiryId: "0" }];
}

export default async function Page({ params }: PageProps) {
	const { inquiryId } = await params;
	return <SellerInquiryDetailPage inquiryId={decodeURIComponent(inquiryId)} />;
}
