import type { ReactNode } from "react";
import RoleZoneGuard from "@/components/shared/RoleZoneGuard";

export default function CheckoutLayout({ children }: { children: ReactNode }) {
	return <RoleZoneGuard required="buyer">{children}</RoleZoneGuard>;
}
