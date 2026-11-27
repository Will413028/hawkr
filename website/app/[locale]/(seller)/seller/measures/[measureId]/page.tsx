import SellerMeasureDetailPage from "@/features/seller/measures/SellerMeasureDetailPage";

export const metadata = { title: "措施與申覆" };

interface PageProps {
	params: Promise<{ measureId: string }>;
}

export function generateStaticParams() {
	return [{ measureId: "0" }];
}

export default async function Page({ params }: PageProps) {
	const { measureId } = await params;
	return <SellerMeasureDetailPage measureId={decodeURIComponent(measureId)} />;
}
