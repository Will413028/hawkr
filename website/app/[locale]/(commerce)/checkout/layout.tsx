import type { ReactNode } from "react";
import ZoneGate from "@/components/shared/ZoneGate";

export default function CheckoutLayout({ children }: { children: ReactNode }) {
	return <ZoneGate required="buyer">{children}</ZoneGate>;
}
