import SellerInquiryDetailPage from "@/features/seller/inquiries/SellerInquiryDetailPage";

export const metadata = { title: "案件詢問" };

interface PageProps {
	params: Promise<{ inquiryId: string }>;
}

export function generateStaticParams() {
	// A sentinel PPR shell keeps the dynamic param from turning the whole route
	// into a blocking render. No real inquiry id is ever prerendered.
	return [{ inquiryId: "0" }];
}

export default async function Page({ params }: PageProps) {
	const { inquiryId } = await params;
	return <SellerInquiryDetailPage inquiryId={decodeURIComponent(inquiryId)} />;
}
