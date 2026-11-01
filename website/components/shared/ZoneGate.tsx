import * as React from "react";
import RouteLoadingSkeleton from "@/components/shared/RouteLoadingSkeleton";
import WorkspaceAlign from "@/components/shared/WorkspaceAlign";
import ZoneUnavailable from "@/components/shared/ZoneUnavailable";
import { ACCESS_CLASSES, type RequiredAccess } from "@/lib/auth/redirect";
import { requireZone } from "@/lib/auth/require-zone";

/**
 * Wraps an access-scoped zone (`/account/*` and `/checkout/*` for buyers, `/seller-center/*` for sellers,
 * `/seller/measures/*` for any signed-in account). The server decides who may open it from the DAL's
 * `verifySession()` (`lib/auth/zone.ts`): a visitor goes to the login page, a signed-in account without
 * the zone's role goes home. middleware only does the optimistic "is the session cookie present" redirect.
 * The workspace (buyer or seller view) is client state and is aligned by `WorkspaceAlign`.
 *
 * The decision reads cookies and headers, so it sits behind a `<Suspense>` that shows the workspace
 * skeleton while the session answer streams in.
 */
export default function ZoneGate({
	required,
	children,
}: {
	required: RequiredAccess;
	children: React.ReactNode;
}) {
	return (
		<React.Suspense fallback={<ZonePending />}>
			<ZoneGateInner required={required}>{children}</ZoneGateInner>
		</React.Suspense>
	);
}

function ZonePending() {
	return <RouteLoadingSkeleton page variant="workspace" />;
}

async function ZoneGateInner({
	required,
	children,
}: {
	required: RequiredAccess;
	children: React.ReactNode;
}) {
	if ((await requireZone(required)) === "unavailable") {
		return <ZoneUnavailable required={required} />;
	}
	const { workspace } = ACCESS_CLASSES[required];
	if (workspace) {
		return (
			<WorkspaceAlign required={required} workspace={workspace}>
				{children}
			</WorkspaceAlign>
		);
	}
	return (
		<>
			<span data-hk-role-zone-ready={required} hidden />
			{children}
		</>
	);
}
