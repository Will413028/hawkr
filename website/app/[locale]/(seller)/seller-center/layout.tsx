import "@/styles/features/seller-center.css";
import RoleZoneGuard from "@/components/shared/RoleZoneGuard";
import { SellerCenterShell } from "@/features/seller-center";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function SellerCenterLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<RoleZoneGuard required="seller">
			<SellerCenterShell>{children}</SellerCenterShell>
		</RoleZoneGuard>
	);
}
