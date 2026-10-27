"use client";

import dynamic from "next/dynamic";

const RoleSwitcher = dynamic(() => import("@/components/shared/RoleSwitcher"), {
	loading: () => null,
});

/** Loads development-only role controls without attaching them to prod roots. */
export default function DevRoleSwitcherSlot() {
	return <RoleSwitcher />;
}
