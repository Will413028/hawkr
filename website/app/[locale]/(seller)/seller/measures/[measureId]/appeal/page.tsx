import SellerMeasureRequestPage from "@/features/seller/measures/SellerMeasureRequestPage";

export const metadata = { title: "提出申覆" };

interface PageProps {
	params: Promise<{ measureId: string }>;
}

export function generateStaticParams() {
	return [{ measureId: "0" }];
}

export default async function Page({ params }: PageProps) {
	const { measureId } = await params;
	return (
		<SellerMeasureRequestPage
			kind="appeal"
			measureId={decodeURIComponent(measureId)}
		/>
	);
}
