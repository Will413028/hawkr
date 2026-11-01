import "@/styles/features/seller-center.css";
import ZoneGate from "@/components/shared/ZoneGate";
import { SellerCenterShell } from "@/features/seller-center";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function SellerCenterLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<ZoneGate required="seller">
			<SellerCenterShell>{children}</SellerCenterShell>
		</ZoneGate>
	);
}
