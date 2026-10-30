"use client";

import * as Sentry from "@sentry/nextjs";
import * as React from "react";
import ErrorState from "@/components/shared/ErrorState";
import RouteLoadingSkeleton from "@/components/shared/RouteLoadingSkeleton";
import { useAuth } from "@/lib/auth/context";
import {
	ACCESS_CLASSES,
	defaultPathForRole,
	loginPath,
	type RequiredAccess,
} from "@/lib/auth/redirect";
import { getActiveRole, type RoleSlot } from "@/lib/auth/storage";
import {
	hydrationClientSnapshot,
	hydrationServerSnapshot,
	useHydrationSafeClientRender,
} from "@/lib/hooks/useHydrationSafeClientRender";
import { splitLocale } from "@/lib/intl/paths";
import { INITIAL_LOADER_REACT_FALLBACK_MS } from "@/lib/loading/initial-loader-watchdog";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";

/**
 * Wraps an access-scoped zone (`/account/*` for buyers, `/seller-center/*` for
 * sellers, `/seller/measures/*` for any signed-in account). For a workspace
 * zone the client active role (from the signed-in account's roles) is the
 * boundary; if it doesn't match `required`, the user is sent back to the
 * default page for their active workspace. An `account` zone only needs a
 * signed-in account: it never compares, switches or reloads the workspace.
 * Visitors with no session cookie are redirected by middleware (which only
 * checks that the cookie is present). A visitor whose cookie the monolith
 * refuses (GetSession: unauthenticated) gets past it, so this guard sends them
 * to the login page, the way middleware does — unless the page had a signed-in
 * account earlier (the session ended mid-visit; AuthProvider takes them home).
 *
 * A hard entry remains behind InitialPageLoader until this guard publishes
 * its ready marker. Soft navigation normally arrives with AuthProvider
 * already hydrated and admits synchronously, so the skeleton below is usually
 * never painted at all. When admission is NOT synchronous the zone shows the
 * workspace skeleton (announced by `RouteLoadingSkeleton`'s own live region)
 * rather than an unexplained blank screen. If it is still stuck open past
 * INITIAL_LOADER_REACT_FALLBACK_MS (a slow/cold sessionSettled probe on a soft
 * nav, which InitialPageLoader's own watchdog never sees — it only guards the
 * first hard load), a terminal fallback replaces the skeleton and reports to
 * Sentry, mirroring InitialPageLoader's own terminal-timeout handling
 * (Sentry.captureMessage then a terminal render) for the same failure class.
 */
export default function RoleZoneGuard({
	required,
	children,
}: {
	required: RequiredAccess;
	children: React.ReactNode;
}) {
	return (
		// Both this boundary and the admission window below show the workspace
		// skeleton rather than nothing: `null` left the viewport blank and silent
		// for however long admission took, and the watchdog further down exists
		// precisely because that window is not always instant.
		<React.Suspense fallback={<RoleZoneGuardPending />}>
			<RoleZoneGuardInner required={required}>{children}</RoleZoneGuardInner>
		</React.Suspense>
	);
}

function RoleZoneGuardPending() {
	return <RouteLoadingSkeleton page variant="workspace" />;
}

// Per-tab latch: at most one auto workspace switch per admission attempt
// (sessionStorage survives the reload below but is not shared across tabs).
const AUTO_SWITCHED_KEY = "hkui:role-zone:auto-switched";

export const roleZoneClientSnapshot = hydrationClientSnapshot;
export const roleZoneServerSnapshot = hydrationServerSnapshot;

interface RoleZoneAdmission {
	hydrationSafeClientRender: boolean;
	hydrated: boolean;
	sessionSettled: boolean;
	isLoggedIn: boolean;
	ownsRequiredRole: boolean;
	ready: boolean;
	roleMatches: boolean;
}

export function roleZoneFallbackPath(
	activeRole: RoleSlot | null,
	storedRole: RoleSlot | null,
): string {
	return defaultPathForRole(storedRole ?? activeRole);
}

export function shouldAdmitRoleZone({
	hydrationSafeClientRender,
	hydrated,
	sessionSettled,
	isLoggedIn,
	ownsRequiredRole,
	ready,
	roleMatches,
}: RoleZoneAdmission): boolean {
	return (
		hydrationSafeClientRender &&
		hydrated &&
		sessionSettled &&
		isLoggedIn &&
		ownsRequiredRole &&
		(ready || roleMatches)
	);
}

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

function RoleZoneGuardInner({
	required,
	children,
}: {
	required: RequiredAccess;
	children: React.ReactNode;
}) {
	const {
		activeRole,
		hasSeller,
		hydrated,
		isLoggedIn,
		sessionSettled,
		switchRole,
	} = useAuth();
	const [ready, setReady] = React.useState(false);
	const everLoggedIn = React.useRef(false);
	if (isLoggedIn) everLoggedIn.current = true;
	const [terminalTimeout, setTerminalTimeout] = React.useState(false);
	const hydrationSafeClientRender = useHydrationSafeClientRender();
	const { workspace, owns } = ACCESS_CLASSES[required];
	const ownsRequiredRole = owns({ isLoggedIn, hasSeller });
	const roleMatches =
		hydrated &&
		sessionSettled &&
		isLoggedIn &&
		ownsRequiredRole &&
		(workspace === null ||
			activeRole === workspace ||
			(typeof window !== "undefined" && getActiveRole() === workspace));
	const admitted = shouldAdmitRoleZone({
		hydrationSafeClientRender,
		hydrated,
		sessionSettled,
		isLoggedIn,
		ownsRequiredRole,
		ready,
		roleMatches,
	});

	React.useEffect(() => {
		// AuthProvider hydrates identity (GetSession) on
		// mount. Until that resolves, activeRole is null — bailing out before
		// `hydrated` would redirect the user home on every page refresh even when
		// they hold a valid session.
		if (!hydrated) return;

		if (
			sessionSettled &&
			ownsRequiredRole &&
			(workspace === null || activeRole === workspace)
		) {
			setReady(true);
			// A completed stay in this zone re-arms the auto-switch for future
			// cross-workspace deep links in this tab.
			if (workspace !== null) unlatchAutoSwitch();
			return;
		}

		// Race-condition safety: switchRole() writes localStorage synchronously
		// (setStoredActiveRole) and then schedules a React state update via
		// setActive(). If router.replace() fires before that state update is
		// committed, the context `activeRole` here can still be stale even though
		// the new role is already persisted. Read the stored role directly as a
		// fallback before deciding to redirect away.
		if (
			sessionSettled &&
			ownsRequiredRole &&
			workspace !== null &&
			getActiveRole() === workspace
		) {
			setReady(true);
			return;
		}

		// `hydrated` alone can come from the render gate or a transient
		// unavailable, i.e. BEFORE the probe has answered — deciding then would
		// bounce a valid deep link (switchRole below needs the resolved
		// identity). Hold the skeleton until a real answer lands; the probe's
		// bounded retries guarantee this settles.
		if (!sessionSettled) return;

		// Deep link into the other workspace (e.g. a dual-role account with the
		// buyer workspace active opening a /seller-center/* share link, first
		// shipped by the storefront-preview URLs): if the account holds the
		// required role, switch the active workspace instead of silently
		// discarding the navigation intent. Mirrors the unauthenticated path,
		// which preserves intent via /login?role=…&redirect=….
		//
		// Then REBOOT the page rather than admit in place: flipping the role
		// mid-flight races the rest of the session pipeline (loginRole seeding,
		// the /buyers/me backfill, identity re-emits), which was observed
		// intermittently reverting the switch and bouncing an already-rendered
		// page. A full load starts every consumer on the new role — and pulls
		// the current build, same rationale as the hard nav below.
		//
		// The per-tab sessionStorage latch caps this at one auto-switch per
		// admission attempt, so two tabs deep-linked into opposite zones can't
		// reload-loop each other; it re-arms once a zone admits normally.
		if (
			workspace !== null &&
			!ready &&
			!autoSwitchLatched() &&
			switchRole(workspace)
		) {
			latchAutoSwitch();
			window.location.reload();
			return;
		}

		// A cookie the monolith refuses (and no account earlier on this page):
		// the login page of this locale, coming back to the URL they asked for.
		if (!isLoggedIn && !everLoggedIn.current) {
			const { prefix } = splitLocale(window.location.pathname);
			commitDocumentNavigation(
				`${prefix}${loginPath({
					...(workspace ? { role: workspace } : {}),
					redirect: `${window.location.pathname}${window.location.search}`,
				})}`,
				true,
			);
			return;
		}

		// No matching role on the account (or the session ended mid-visit). Don't push to /login —
		// that would dump the user onto a page with no navigation. Keep a resolved
		// seller inside the seller workspace; buyers and guests return home.
		// Hard nav (not router.replace): a stale-chunk skeleton hang here is the
		// exact bug being fixed — a full load pulls the current build.
		commitDocumentNavigation(
			roleZoneFallbackPath(activeRole, getActiveRole()),
			true,
		);
	}, [
		hydrated,
		sessionSettled,
		activeRole,
		ownsRequiredRole,
		workspace,
		ready,
		switchRole,
		isLoggedIn,
	]);

	// Latest-value box for the watchdog below: the timeout fires on a fixed
	// delay independent of the admission effect's own dependency churn, so a
	// closure captured when that effect last ran could be stale by the time
	// this reads it.
	const admissionSnapshotRef = React.useRef({
		hydrated,
		sessionSettled,
		isLoggedIn,
		ownsRequiredRole,
		roleMatches,
		ready,
	});
	React.useEffect(() => {
		admissionSnapshotRef.current = {
			hydrated,
			sessionSettled,
			isLoggedIn,
			ownsRequiredRole,
			roleMatches,
			ready,
		};
	});

	React.useEffect(() => {
		if (admitted) return;
		// Only InitialPageLoader's own watchdog would otherwise catch a stuck
		// admission, and it only runs once on the very first hard document
		// load — a soft nav into this zone (navbar link, router.push from
		// elsewhere) leaves nothing watching. Without this, a slow/cold
		// sessionSettled probe renders `null` for the full backend timeout
		// with zero visible feedback and zero Sentry signal.
		const timer = window.setTimeout(() => {
			Sentry.captureMessage("role zone guard terminal timeout", {
				level: "warning",
				fingerprint: ["role-zone-guard-timeout", required],
				tags: { required_role: required },
				extra: {
					pathname: window.location.pathname,
					...admissionSnapshotRef.current,
				},
			});
			setTerminalTimeout(true);
		}, INITIAL_LOADER_REACT_FALLBACK_MS);
		return () => window.clearTimeout(timer);
	}, [admitted, required]);

	if (terminalTimeout && !admitted) {
		return (
			<ErrorState
				error={new Error(`role zone guard timeout: ${required}`)}
				reset={() => window.location.reload()}
			/>
		);
	}
	if (!admitted) return <RoleZoneGuardPending />;
	return (
		<>
			<span data-hk-role-zone-ready={required} hidden />
			{children}
		</>
	);
}
