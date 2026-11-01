import ZoneGate from "@/components/shared/ZoneGate";
import AccountShell from "@/features/account/components/AccountShell";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function AccountLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<ZoneGate required="buyer">
			<AccountShell>{children}</AccountShell>
		</ZoneGate>
	);
}
