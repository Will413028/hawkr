import "@/styles/features/seller-center.css";
import RoleZoneGuard from "@/components/shared/RoleZoneGuard";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function SellerMeasuresLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<RoleZoneGuard required="account">
			<div className="mx-auto w-full max-w-[880px] px-4 pb-12 pt-6 sm:px-5 sm:pt-10">
				{children}
			</div>
		</RoleZoneGuard>
	);
}
