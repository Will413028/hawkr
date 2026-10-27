import RoleZoneGuard from "@/components/shared/RoleZoneGuard";
import AccountShell from "@/features/account/components/AccountShell";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function AccountLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<RoleZoneGuard required="buyer">
			<AccountShell>{children}</AccountShell>
		</RoleZoneGuard>
	);
}
