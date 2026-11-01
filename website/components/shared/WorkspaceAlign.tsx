"use client";

import * as React from "react";
import RouteLoadingSkeleton from "@/components/shared/RouteLoadingSkeleton";
import { useAuth } from "@/lib/auth/context";
import { defaultPathForRole, type RequiredAccess } from "@/lib/auth/redirect";
import { getActiveRole, type RoleSlot } from "@/lib/auth/storage";
import { useHydrationSafeClientRender } from "@/lib/hooks/useHydrationSafeClientRender";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";

// Per-tab latch: at most one auto workspace switch per admission attempt
// (sessionStorage survives the reload below but is not shared across tabs).
const AUTO_SWITCHED_KEY = "hkui:role-zone:auto-switched";

function autoSwitchLatched(): boolean {
	try {
		return sessionStorage.getItem(AUTO_SWITCHED_KEY) === "1";
	} catch {
		return false;
	}
}

function latchAutoSwitch() {
	try {
		sessionStorage.setItem(AUTO_SWITCHED_KEY, "1");
	} catch {}
}

function unlatchAutoSwitch() {
	try {
		sessionStorage.removeItem(AUTO_SWITCHED_KEY);
	} catch {}
}

/**
 * Aligns the client workspace (buyer or seller view, stored in localStorage) with the zone the server
 * already admitted the account to (`ZoneGate`). Access is not decided here.
 *
 * Deep link into the other workspace (e.g. a dual-role account with the buyer workspace active opening a
 * /seller-center/* share link): switch the active workspace instead of silently discarding the navigation
 * intent, then reboot the page rather than admit in place — flipping the role mid-flight races the rest
 * of the session pipeline (the /buyers/me backfill, identity re-emits), and a full load starts every
 * consumer on the new role and pulls the current build. The per-tab latch caps this at one switch per
 * admission attempt, so two tabs deep-linked into opposite zones cannot reload-loop each other; it
 * re-arms once a zone admits normally. If the switch was already tried and the workspace still does not
 * match, the account goes to the default page of its current workspace.
 *
 * Nothing here acts until the session is `signedIn`: while it is `unknown` or `unavailable` there is no
 * identity to switch with, and navigating then would send a valid account to a default page.
 *
 * The stored workspace is browser-only state, so nothing here reads it before hydration: the skeleton is
 * what the server renders and what the first client pass hydrates.
 */
export default function WorkspaceAlign({
	required,
	workspace,
	children,
}: {
	required: RequiredAccess;
	workspace: RoleSlot;
	children: React.ReactNode;
}) {
	const { activeRole, status, switchRole } = useAuth();
	const hydrationSafeClientRender = useHydrationSafeClientRender();
	const [ready, setReady] = React.useState(false);
	const roleMatches =
		hydrationSafeClientRender &&
		(activeRole === workspace || getActiveRole() === workspace);

	React.useEffect(() => {
		if (!hydrationSafeClientRender) return;
		if (status !== "signedIn") return;
		if (roleMatches) {
			setReady(true);
			unlatchAutoSwitch();
			return;
		}
		if (!ready && !autoSwitchLatched() && switchRole(workspace)) {
			latchAutoSwitch();
			window.location.reload();
			return;
		}
		commitDocumentNavigation(
			defaultPathForRole(getActiveRole() ?? activeRole),
			true,
		);
	}, [
		hydrationSafeClientRender,
		roleMatches,
		ready,
		activeRole,
		status,
		switchRole,
		workspace,
	]);

	if (!ready && !roleMatches) {
		return <RouteLoadingSkeleton page variant="workspace" />;
	}
	return (
		<>
			<span data-hk-role-zone-ready={required} hidden />
			{children}
		</>
	);
}
